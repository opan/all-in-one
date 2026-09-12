package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/all-in-one/internal/config"
	httpHelper "github.com/all-in-one/internal/http"
	"github.com/all-in-one/internal/ratelimit"
	"github.com/all-in-one/internal/ratelimit/handler"
	"github.com/all-in-one/internal/ratelimit/middleware"
	"github.com/all-in-one/internal/ratelimit/model"
	"github.com/all-in-one/internal/ratelimit/repository"
	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"
)

// Service composes the ratelimit repository layer, keeps the in-memory rule
// cache warm, runs the background refresh/cleanup tickers, serves the
// admin-only management API via Handler, and exposes the enforcement
// middleware via LimiterMiddleware.
type Service struct {
	Store   repository.Storage
	Handler *handler.Handler

	cache      *ruleCache
	tokenCache *tokenCache
	limiter    *middleware.Limiter
	config     config.Config
	log        zerolog.Logger

	stopOnce sync.Once
	stop     chan struct{}
	wg       sync.WaitGroup
}

func NewService(ctx context.Context, db *sqlx.DB, config config.Config, log zerolog.Logger) (*Service, error) {
	store, err := repository.NewRepo(db, config)
	if err != nil {
		log.Error().Err(err).Msg("Failed to initiate ratelimit repository")
		return nil, err
	}

	cache := newRuleCache(store)
	s := &Service{
		Store:      store,
		cache:      cache,
		tokenCache: newTokenCache(config.RateLimit.External.TokenCacheTTL),
		limiter:    middleware.NewLimiter(cache, store.CounterRepo(), config),
		config:     config,
		log:        log,
		stop:       make(chan struct{}),
	}
	s.Handler = handler.NewHandler(s, config)

	if err := s.seed(ctx); err != nil {
		return nil, fmt.Errorf("seed rate limit rules: %w", err)
	}
	if err := s.cache.Reload(ctx); err != nil {
		return nil, fmt.Errorf("warm rate limit rule cache: %w", err)
	}

	s.startTickers()

	return s, nil
}

// RegisterAdminRoutes registers the Rate Limiting management API. Callers
// must apply admin-only gating (RequireAdmin) to router beforehand.
func (s *Service) RegisterAdminRoutes(router *mux.Router) {
	s.Handler.RegisterAdminRoutes(router)
}

// LimiterMiddleware returns the mux.MiddlewareFunc that enforces rate
// limits using this service's rule cache and counter repository. Attach it
// to any subrouter carrying rate-limited routes (docs/adr/RATE_LIMITING_ADR.md
// ADR-004).
func (s *Service) LimiterMiddleware() mux.MiddlewareFunc {
	return s.limiter.Middleware()
}

// seed inserts one rate_limit_rules row per Registry target (insert-if-
// absent, never clobbering a prior admin edit — docs/adr/RATE_LIMITING_ADR.md
// ADR-003). Safe — and intended — to call on every server start.
func (s *Service) seed(ctx context.Context) error {
	for _, t := range ratelimit.Registered() {
		rule := model.Rule{
			TargetKey:   t.Key,
			Enabled:     true,
			LimitCount:  t.DefaultLimit,
			WindowValue: t.DefaultWindowValue,
			WindowUnit:  t.DefaultWindowUnit,
		}
		if err := s.Store.RuleRepo().Seed(ctx, rule); err != nil {
			return fmt.Errorf("seed target %q: %w", t.Key, err)
		}
	}
	return nil
}

func (s *Service) startTickers() {
	s.wg.Add(2)
	go s.runRefreshTicker()
	go s.runCleanupTicker()
}

// runRefreshTicker is the periodic backstop for the rule cache (admin writes
// already trigger an immediate Reload — this just guards against a missed
// or out-of-process write, e.g. a direct DB edit).
func (s *Service) runRefreshTicker() {
	defer s.wg.Done()
	ticker := time.NewTicker(s.config.RateLimit.CacheRefreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := s.cache.Reload(context.Background()); err != nil {
				s.log.Error().Err(err).Msg("ratelimit: failed to refresh rule cache")
			}
		case <-s.stop:
			return
		}
	}
}

// runCleanupTicker prunes daily-quota counter rows older than
// CounterRetentionDays so rate_limit_counters doesn't grow unbounded.
func (s *Service) runCleanupTicker() {
	defer s.wg.Done()
	ticker := time.NewTicker(s.config.RateLimit.CleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			cutoff := time.Now().In(s.location()).
				AddDate(0, 0, -s.config.RateLimit.CounterRetentionDays).
				Format("2006-01-02")
			if _, err := s.Store.CounterRepo().DeleteOlderThan(context.Background(), cutoff); err != nil {
				s.log.Error().Err(err).Msg("ratelimit: failed to prune expired counters")
			}
		case <-s.stop:
			return
		}
	}
}

// Close stops both background tickers and the limiter's in-memory throttle
// store, waiting for the tickers to exit. It does not close the underlying
// storage (owned by the top-level internal/storage.Storage, shared across
// all app modules). Safe to call more than once.
func (s *Service) Close() error {
	s.stopOnce.Do(func() { close(s.stop) })
	s.wg.Wait()
	s.limiter.Stop()
	return nil
}

// location resolves the configured ratelimit timezone, falling back to UTC
// on an empty or invalid value (docs/adr/RATE_LIMITING_ADR.md ADR-006).
func (s *Service) location() *time.Location {
	if s.config.RateLimit.Timezone == "" {
		return time.UTC
	}
	loc, err := time.LoadLocation(s.config.RateLimit.Timezone)
	if err != nil {
		s.log.Warn().Err(err).Str("timezone", s.config.RateLimit.Timezone).
			Msg("ratelimit: invalid timezone, falling back to UTC")
		return time.UTC
	}
	return loc
}

// today returns the current calendar day (in the configured timezone) as a
// 'YYYY-MM-DD' string — the daily-quota bucket identity (ADR-006).
func (s *Service) today() string {
	return time.Now().In(s.location()).Format("2006-01-02")
}

// --- Admin operations (consumed by the handler package, P10/P11) ---

// ListTargets merges every Registry target's code metadata with its current
// effective rule. A target whose rule row hasn't been seeded yet (shouldn't
// happen after boot) falls back to its Registry defaults.
func (s *Service) ListTargets(ctx context.Context) ([]model.Target, error) {
	rules, err := s.Store.RuleRepo().List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list rules: %w", err)
	}
	byKey := make(map[string]model.Rule, len(rules))
	for _, r := range rules {
		byKey[r.TargetKey] = r
	}

	targets := ratelimit.Registered()
	out := make([]model.Target, 0, len(targets))
	for _, t := range targets {
		if rule, ok := byKey[t.Key]; ok {
			out = append(out, mergeTarget(t, rule))
		} else {
			out = append(out, defaultTarget(t))
		}
	}
	// Append external targets — DB rows with no Registry entry. Internal rows
	// are already covered by the Registry loop above.
	for _, rule := range rules {
		if rule.IsExternal {
			out = append(out, externalTarget(rule))
		}
	}
	return out, nil
}

// UpdateTarget applies a partial edit to a target's rule (nil patch fields
// left unchanged) and reloads the rule cache so the change takes effect on
// the next request, without a restart (ADR-008). Returns
// ratelimit.ErrUnknownTarget if key isn't in the Registry, or
// ratelimit.ErrInvalidWindowUnit if patch.WindowUnit is set to an
// unrecognized unit.
func (s *Service) UpdateTarget(ctx context.Context, key string, patch model.TargetPatch, updatedBy string) (model.Target, error) {
	def, isInternal := ratelimit.ByKey(key)

	current, err := s.Store.RuleRepo().Get(ctx, key)
	if err != nil {
		if errors.Is(err, httpHelper.ErrNotFound) {
			return model.Target{}, ratelimit.ErrUnknownTarget
		}
		return model.Target{}, fmt.Errorf("get rule %q: %w", key, err)
	}
	// An unknown key that isn't in the Registry and has no external row either
	// (defence in depth — Get above would already 404 it).
	if !isInternal && !current.IsExternal {
		return model.Target{}, ratelimit.ErrUnknownTarget
	}

	if patch.Enabled != nil {
		current.Enabled = *patch.Enabled
	}
	if patch.LimitCount != nil {
		current.LimitCount = *patch.LimitCount
	}
	if patch.WindowValue != nil {
		current.WindowValue = *patch.WindowValue
	}
	if patch.WindowUnit != nil {
		if _, err := windowDuration(1, *patch.WindowUnit); err != nil {
			return model.Target{}, err
		}
		current.WindowUnit = *patch.WindowUnit
	}
	current.UpdatedBy = &updatedBy

	if err := s.Store.RuleRepo().Update(ctx, current); err != nil {
		return model.Target{}, fmt.Errorf("update rule %q: %w", key, err)
	}
	if err := s.cache.Reload(ctx); err != nil {
		return model.Target{}, fmt.Errorf("reload rule cache: %w", err)
	}

	updated, err := s.Store.RuleRepo().Get(ctx, key)
	if err != nil {
		return model.Target{}, fmt.Errorf("get rule %q: %w", key, err)
	}
	if isInternal {
		return mergeTarget(def, updated), nil
	}
	return externalTarget(updated), nil
}

// ResetCounters clears today's daily-quota counters for a target (admin
// "reset"). It does not touch an in-memory throttle bucket for a throttle
// target — that best-effort clear is added once the middleware's memStore
// exists (P9). Returns ratelimit.ErrUnknownTarget if key isn't in the
// Registry.
func (s *Service) ResetCounters(ctx context.Context, key string) error {
	if _, ok := ratelimit.ByKey(key); !ok {
		// Not an internal target — allow only if a matching external row exists.
		rule, err := s.Store.RuleRepo().Get(ctx, key)
		if err != nil {
			if errors.Is(err, httpHelper.ErrNotFound) {
				return ratelimit.ErrUnknownTarget
			}
			return fmt.Errorf("get rule %q: %w", key, err)
		}
		if !rule.IsExternal {
			return ratelimit.ErrUnknownTarget
		}
	}
	return s.Store.CounterRepo().DeleteForTargetDay(ctx, key, s.today())
}

// ResetDefaults overwrites a target's rule back to its Registry defaults
// (clearing any admin edit, including UpdatedBy) and reloads the rule
// cache. Returns ratelimit.ErrUnknownTarget if key isn't in the Registry.
func (s *Service) ResetDefaults(ctx context.Context, key string) (model.Target, error) {
	def, ok := ratelimit.ByKey(key)
	if !ok {
		// An external target has no code-defined default to reset to. Only
		// distinguish "unknown" from "external" so the caller gets a precise
		// error (404 vs 400).
		rule, err := s.Store.RuleRepo().Get(ctx, key)
		if err != nil {
			if errors.Is(err, httpHelper.ErrNotFound) {
				return model.Target{}, ratelimit.ErrUnknownTarget
			}
			return model.Target{}, fmt.Errorf("get rule %q: %w", key, err)
		}
		if rule.IsExternal {
			return model.Target{}, ratelimit.ErrNotSupportedForExternal
		}
		return model.Target{}, ratelimit.ErrUnknownTarget
	}

	rule := model.Rule{
		TargetKey: def.Key, Enabled: true,
		LimitCount: def.DefaultLimit, WindowValue: def.DefaultWindowValue, WindowUnit: def.DefaultWindowUnit,
	}
	if err := s.Store.RuleRepo().ResetToDefault(ctx, rule); err != nil {
		return model.Target{}, fmt.Errorf("reset rule %q: %w", key, err)
	}
	if err := s.cache.Reload(ctx); err != nil {
		return model.Target{}, fmt.Errorf("reload rule cache: %w", err)
	}

	updated, err := s.Store.RuleRepo().Get(ctx, key)
	if err != nil {
		return model.Target{}, fmt.Errorf("get rule %q: %w", key, err)
	}
	return mergeTarget(def, updated), nil
}

// CreateExternalTarget creates a self-contained external target (a DB rule
// with no Registry entry) and reloads the cache so it enforces immediately.
// The key must not collide with a Registry key. Returns
// ratelimit.ErrExternalTargetExists on a duplicate key.
func (s *Service) CreateExternalTarget(ctx context.Context, t model.Target, createdBy string) (model.Target, error) {
	key := strings.TrimSpace(t.Key)
	if key == "" {
		return model.Target{}, fmt.Errorf("target key is required")
	}
	if _, ok := ratelimit.ByKey(key); ok {
		return model.Target{}, ratelimit.ErrExternalTargetExists
	}
	if !validScope(t.Scope) {
		return model.Target{}, fmt.Errorf("invalid scope %q", t.Scope)
	}
	if !validKind(t.Kind) {
		return model.Target{}, fmt.Errorf("invalid kind %q", t.Kind)
	}
	if t.LimitCount <= 0 {
		return model.Target{}, fmt.Errorf("limit_count must be positive")
	}
	if _, err := windowDuration(t.WindowValue, t.WindowUnit); err != nil {
		return model.Target{}, err
	}

	app := strings.TrimSpace(t.App)
	if app == "" {
		app = keyApp(key)
	}
	rule := model.Rule{
		TargetKey: key, Enabled: t.Enabled,
		LimitCount: t.LimitCount, WindowValue: t.WindowValue, WindowUnit: t.WindowUnit,
		App: app, IsExternal: true,
	}
	if name := strings.TrimSpace(t.Name); name != "" {
		rule.Name = &name
	}
	if desc := strings.TrimSpace(t.Description); desc != "" {
		rule.Description = &desc
	}
	scope := t.Scope
	rule.Scope = &scope
	kind := t.Kind
	rule.Kind = &kind
	if createdBy != "" {
		rule.UpdatedBy = &createdBy
	}

	if err := s.Store.RuleRepo().CreateExternal(ctx, rule); err != nil {
		return model.Target{}, err
	}
	if err := s.cache.Reload(ctx); err != nil {
		return model.Target{}, fmt.Errorf("reload rule cache: %w", err)
	}

	created, err := s.Store.RuleRepo().Get(ctx, key)
	if err != nil {
		return model.Target{}, fmt.Errorf("get rule %q: %w", key, err)
	}
	return externalTarget(created), nil
}

// DeleteExternalTarget deletes an external target and reloads the cache.
// Returns ratelimit.ErrNotSupportedForExternal for an internal (Registry)
// key, or ratelimit.ErrUnknownTarget if no such row exists.
func (s *Service) DeleteExternalTarget(ctx context.Context, key string) error {
	if _, ok := ratelimit.ByKey(key); ok {
		return ratelimit.ErrNotSupportedForExternal
	}
	rule, err := s.Store.RuleRepo().Get(ctx, key)
	if err != nil {
		if errors.Is(err, httpHelper.ErrNotFound) {
			return ratelimit.ErrUnknownTarget
		}
		return fmt.Errorf("get rule %q: %w", key, err)
	}
	if !rule.IsExternal {
		return ratelimit.ErrNotSupportedForExternal
	}
	if err := s.Store.RuleRepo().Delete(ctx, key); err != nil {
		return fmt.Errorf("delete rule %q: %w", key, err)
	}
	if err := s.cache.Reload(ctx); err != nil {
		return fmt.Errorf("reload rule cache: %w", err)
	}
	return nil
}

// EffectiveRule exposes the cached effective rule for a key (used by the check
// handler to validate a target before counting). ok is false on a cache miss.
func (s *Service) EffectiveRule(key string) (model.EffectiveRule, bool) {
	return s.cache.Effective(key)
}

// CheckExternal evaluates one external target against a caller-supplied bucket
// key and returns the decision. A cache miss (target unknown or not loaded)
// fails open with a warning (ADR-007); a counter-store error also fails open
// but is surfaced (non-nil error) so the caller can meter it.
func (s *Service) CheckExternal(ctx context.Context, targetKey, bucketKey string) (model.CheckResponse, error) {
	rule, ok := s.cache.Effective(targetKey)
	if !ok {
		s.log.Warn().Str("target", targetKey).
			Msg("ratelimit: external check for unknown/unloaded target, failing open")
		return model.CheckResponse{Allowed: true}, nil
	}
	allowed, retryAfter, remaining, err := s.limiter.Check(ctx, rule, bucketKey)
	if err != nil {
		return model.CheckResponse{Allowed: true, Limit: rule.LimitCount}, err
	}
	return model.CheckResponse{
		Allowed:           allowed,
		Limit:             rule.LimitCount,
		Remaining:         remaining,
		RetryAfterSeconds: int(retryAfter.Seconds()),
	}, nil
}

func validScope(s model.Scope) bool {
	switch s {
	case model.ScopeIP, model.ScopeUser, model.ScopeGlobal:
		return true
	}
	return false
}

func validKind(k model.Kind) bool {
	switch k {
	case model.KindThrottle, model.KindDailyQuota:
		return true
	}
	return false
}

// keyApp derives a default app label from a target key: the segment before the
// first dot (e.g. "cashflow" from "cashflow.entry.create").
func keyApp(key string) string {
	if i := strings.IndexByte(key, '.'); i > 0 {
		return key[:i]
	}
	return key
}

func mergeTarget(def ratelimit.TargetDef, rule model.Rule) model.Target {
	updatedAt := rule.UpdatedAt
	return model.Target{
		Key: def.Key, Name: def.Name, Description: def.Description,
		Scope: def.Scope, Kind: def.Kind, Method: def.Method, Path: def.Path,
		Enabled: rule.Enabled, LimitCount: rule.LimitCount,
		WindowValue: rule.WindowValue, WindowUnit: rule.WindowUnit,
		UpdatedAt: &updatedAt, UpdatedBy: rule.UpdatedBy,
		App: "all-in-one", IsExternal: false,
	}
}

func defaultTarget(def ratelimit.TargetDef) model.Target {
	return model.Target{
		Key: def.Key, Name: def.Name, Description: def.Description,
		Scope: def.Scope, Kind: def.Kind, Method: def.Method, Path: def.Path,
		Enabled: true, LimitCount: def.DefaultLimit,
		WindowValue: def.DefaultWindowValue, WindowUnit: def.DefaultWindowUnit,
		App: "all-in-one", IsExternal: false,
	}
}

// externalTarget builds the admin read view from an external DB row. Unlike an
// internal target, its identity (name, description, scope, kind) lives in the
// row itself, not the Registry, and it has no bound aio route.
func externalTarget(rule model.Rule) model.Target {
	updatedAt := rule.UpdatedAt
	t := model.Target{
		Key: rule.TargetKey, App: rule.App, IsExternal: true,
		Enabled: rule.Enabled, LimitCount: rule.LimitCount,
		WindowValue: rule.WindowValue, WindowUnit: rule.WindowUnit,
		UpdatedAt: &updatedAt, UpdatedBy: rule.UpdatedBy,
	}
	if rule.Name != nil {
		t.Name = *rule.Name
	}
	if rule.Description != nil {
		t.Description = *rule.Description
	}
	if rule.Scope != nil {
		t.Scope = *rule.Scope
	}
	if rule.Kind != nil {
		t.Kind = *rule.Kind
	}
	return t
}

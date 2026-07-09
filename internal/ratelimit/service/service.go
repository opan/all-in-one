package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/all-in-one/internal/config"
	"github.com/all-in-one/internal/ratelimit"
	"github.com/all-in-one/internal/ratelimit/handler"
	"github.com/all-in-one/internal/ratelimit/model"
	"github.com/all-in-one/internal/ratelimit/repository"
	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"
)

// Service composes the ratelimit repository layer, keeps the in-memory rule
// cache warm, runs the background refresh/cleanup tickers, and serves the
// admin-only management API via Handler.
type Service struct {
	Store   repository.Storage
	Handler *handler.Handler

	cache  *ruleCache
	config config.Config
	log    zerolog.Logger

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

	s := &Service{
		Store:  store,
		cache:  newRuleCache(store),
		config: config,
		log:    log,
		stop:   make(chan struct{}),
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

// Close stops both background tickers and waits for them to exit. It does
// not close the underlying storage (owned by the top-level
// internal/storage.Storage, shared across all app modules). Safe to call
// more than once.
func (s *Service) Close() error {
	s.stopOnce.Do(func() { close(s.stop) })
	s.wg.Wait()
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
	return out, nil
}

// UpdateTarget applies a partial edit to a target's rule (nil patch fields
// left unchanged) and reloads the rule cache so the change takes effect on
// the next request, without a restart (ADR-008). Returns
// ratelimit.ErrUnknownTarget if key isn't in the Registry, or
// ratelimit.ErrInvalidWindowUnit if patch.WindowUnit is set to an
// unrecognized unit.
func (s *Service) UpdateTarget(ctx context.Context, key string, patch model.TargetPatch, updatedBy string) (model.Target, error) {
	def, ok := ratelimit.ByKey(key)
	if !ok {
		return model.Target{}, ratelimit.ErrUnknownTarget
	}

	current, err := s.Store.RuleRepo().Get(ctx, key)
	if err != nil {
		return model.Target{}, fmt.Errorf("get rule %q: %w", key, err)
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
	return mergeTarget(def, updated), nil
}

// ResetCounters clears today's daily-quota counters for a target (admin
// "reset"). It does not touch an in-memory throttle bucket for a throttle
// target — that best-effort clear is added once the middleware's memStore
// exists (P9). Returns ratelimit.ErrUnknownTarget if key isn't in the
// Registry.
func (s *Service) ResetCounters(ctx context.Context, key string) error {
	if _, ok := ratelimit.ByKey(key); !ok {
		return ratelimit.ErrUnknownTarget
	}
	return s.Store.CounterRepo().DeleteForTargetDay(ctx, key, s.today())
}

// ResetDefaults overwrites a target's rule back to its Registry defaults
// (clearing any admin edit, including UpdatedBy) and reloads the rule
// cache. Returns ratelimit.ErrUnknownTarget if key isn't in the Registry.
func (s *Service) ResetDefaults(ctx context.Context, key string) (model.Target, error) {
	def, ok := ratelimit.ByKey(key)
	if !ok {
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

func mergeTarget(def ratelimit.TargetDef, rule model.Rule) model.Target {
	updatedAt := rule.UpdatedAt
	return model.Target{
		Key: def.Key, Name: def.Name, Description: def.Description,
		Scope: def.Scope, Kind: def.Kind, Method: def.Method, Path: def.Path,
		Enabled: rule.Enabled, LimitCount: rule.LimitCount,
		WindowValue: rule.WindowValue, WindowUnit: rule.WindowUnit,
		UpdatedAt: &updatedAt, UpdatedBy: rule.UpdatedBy,
	}
}

func defaultTarget(def ratelimit.TargetDef) model.Target {
	return model.Target{
		Key: def.Key, Name: def.Name, Description: def.Description,
		Scope: def.Scope, Kind: def.Kind, Method: def.Method, Path: def.Path,
		Enabled: true, LimitCount: def.DefaultLimit,
		WindowValue: def.DefaultWindowValue, WindowUnit: def.DefaultWindowUnit,
	}
}

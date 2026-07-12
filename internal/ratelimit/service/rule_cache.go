package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/all-in-one/internal/ratelimit"
	"github.com/all-in-one/internal/ratelimit/model"
	"github.com/all-in-one/internal/ratelimit/repository"
)

// ruleCache holds the effective (Registry + DB-overlaid) rule for every
// registered target in memory, so the limiter middleware never touches the
// DB to read config on the request hot path (docs/adr/RATE_LIMITING_ADR.md
// ADR-008). It is reloaded synchronously at boot, on every admin write, and
// by a periodic ticker (Service.runRefreshTicker).
//
// Its Effective/Reload methods structurally satisfy middleware.RuleProvider
// (declared in the middleware package once P9 lands) without an import
// cycle — the middleware package will depend on this one, not vice versa.
type ruleCache struct {
	store repository.Storage

	mu    sync.RWMutex
	rules map[string]model.EffectiveRule
}

func newRuleCache(store repository.Storage) *ruleCache {
	return &ruleCache{store: store, rules: make(map[string]model.EffectiveRule)}
}

// Effective returns the current effective rule for a target key. ok is
// false if the key is unknown to the Registry or hasn't been loaded yet —
// callers must treat a cache miss as "fail open" (ADR-007).
func (c *ruleCache) Effective(key string) (model.EffectiveRule, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	r, ok := c.rules[key]
	return r, ok
}

// Reload re-reads every rate_limit_rules row and merges each with its
// Registry entry's Scope/Kind, replacing the cache contents atomically.
func (c *ruleCache) Reload(ctx context.Context) error {
	dbRules, err := c.store.RuleRepo().List(ctx)
	if err != nil {
		return fmt.Errorf("list rules: %w", err)
	}
	byKey := make(map[string]model.Rule, len(dbRules))
	for _, r := range dbRules {
		byKey[r.TargetKey] = r
	}

	targets := ratelimit.Registered()
	next := make(map[string]model.EffectiveRule, len(targets))
	for _, t := range targets {
		dbRule, ok := byKey[t.Key]
		if !ok {
			// Not yet seeded — shouldn't happen after boot seed, but skip
			// rather than fail the whole reload; the target is simply
			// absent from the cache, which callers treat as fail-open.
			continue
		}
		window, err := windowDuration(dbRule.WindowValue, dbRule.WindowUnit)
		if err != nil {
			return fmt.Errorf("target %q: %w", t.Key, err)
		}
		next[t.Key] = model.EffectiveRule{
			Key:        t.Key,
			Scope:      t.Scope,
			Kind:       t.Kind,
			Enabled:    dbRule.Enabled,
			LimitCount: dbRule.LimitCount,
			Window:     window,
		}
	}

	c.mu.Lock()
	c.rules = next
	c.mu.Unlock()
	return nil
}

func windowDuration(value int, unit model.WindowUnit) (time.Duration, error) {
	switch unit {
	case model.WindowSecond:
		return time.Duration(value) * time.Second, nil
	case model.WindowMinute:
		return time.Duration(value) * time.Minute, nil
	case model.WindowHour:
		return time.Duration(value) * time.Hour, nil
	case model.WindowDay:
		return time.Duration(value) * 24 * time.Hour, nil
	default:
		return 0, ratelimit.ErrInvalidWindowUnit
	}
}

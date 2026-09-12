package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/all-in-one/internal/logging"
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

// Reload re-reads every rate_limit_rules row and rebuilds the cache,
// replacing its contents atomically. It iterates the DB rows (not the
// Registry) so external targets — DB rows with no Registry entry — are
// enforced too; iterating the Registry would silently drop them (the bug
// this inversion fixes, EXTERNAL_RATE_LIMIT plan ATTENTION #2).
//
// Per row:
//   - a Registry entry exists → merge: code wins for Scope/Kind, the DB
//     supplies the tunables (today's behavior for internal targets);
//   - is_external → build from the row's own scope/kind columns, since an
//     external target has no Registry entry to merge from;
//   - neither (an orphan, e.g. a Registry entry removed in code) → skip and
//     warn. This was silent before; now that "absent from the Registry" is a
//     valid state (external targets), a truly orphaned row is worth surfacing.
func (c *ruleCache) Reload(ctx context.Context) error {
	log := logging.GetLoggerFromContext(ctx)

	dbRules, err := c.store.RuleRepo().List(ctx)
	if err != nil {
		return fmt.Errorf("list rules: %w", err)
	}

	next := make(map[string]model.EffectiveRule, len(dbRules))
	for _, dbRule := range dbRules {
		if def, ok := ratelimit.ByKey(dbRule.TargetKey); ok {
			// Internal target: an invalid window unit here is a code/data bug
			// on a known target — fail the whole reload rather than quietly
			// dropping a rule that is supposed to be enforced.
			window, err := windowDuration(dbRule.WindowValue, dbRule.WindowUnit)
			if err != nil {
				return fmt.Errorf("target %q: %w", dbRule.TargetKey, err)
			}
			next[dbRule.TargetKey] = model.EffectiveRule{
				Key:        dbRule.TargetKey,
				Scope:      def.Scope,
				Kind:       def.Kind,
				Enabled:    dbRule.Enabled,
				LimitCount: dbRule.LimitCount,
				Window:     window,
			}
			continue
		}

		if dbRule.IsExternal {
			if dbRule.Scope == nil || dbRule.Kind == nil {
				log.Warn().Str("target", dbRule.TargetKey).
					Msg("ratelimit: external rule missing scope/kind, skipping")
				continue
			}
			// A bad window on one external row must not nuke the whole cache
			// (and with it every other target's enforcement) — skip just this
			// one and warn.
			window, err := windowDuration(dbRule.WindowValue, dbRule.WindowUnit)
			if err != nil {
				log.Warn().Err(err).Str("target", dbRule.TargetKey).
					Msg("ratelimit: external rule has invalid window unit, skipping")
				continue
			}
			next[dbRule.TargetKey] = model.EffectiveRule{
				Key:        dbRule.TargetKey,
				Scope:      *dbRule.Scope,
				Kind:       *dbRule.Kind,
				Enabled:    dbRule.Enabled,
				LimitCount: dbRule.LimitCount,
				Window:     window,
			}
			continue
		}

		log.Warn().Str("target", dbRule.TargetKey).
			Msg("ratelimit: DB rule has no Registry entry and is not external, skipping")
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

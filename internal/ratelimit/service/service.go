package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/all-in-one/internal/config"
	"github.com/all-in-one/internal/ratelimit"
	"github.com/all-in-one/internal/ratelimit/model"
	"github.com/all-in-one/internal/ratelimit/repository"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"
)

// Service composes the ratelimit repository layer, keeps the in-memory rule
// cache warm, and runs the background refresh/cleanup tickers. Its admin API
// Handler is constructed once the handler package exists (P10).
type Service struct {
	Store repository.Storage

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

	if err := s.seed(ctx); err != nil {
		return nil, fmt.Errorf("seed rate limit rules: %w", err)
	}
	if err := s.cache.Reload(ctx); err != nil {
		return nil, fmt.Errorf("warm rate limit rule cache: %w", err)
	}

	s.startTickers()

	return s, nil
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
			cutoff := time.Now().UTC().
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

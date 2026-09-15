package service

import (
	"context"
	"testing"
	"time"

	"github.com/all-in-one/internal/config"
	"github.com/all-in-one/internal/ratelimit"
	"github.com/all-in-one/internal/ratelimit/middleware"
	"github.com/all-in-one/internal/ratelimit/model"
	"github.com/all-in-one/internal/ratelimit/service/mocks"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newTestService(t *testing.T, store *mocks.MockStorage) *Service {
	t.Helper()
	cfg := config.Config{
		RateLimit: config.RateLimitConfig{
			CacheRefreshInterval: time.Hour,
			CleanupInterval:      time.Hour,
			CounterRetentionDays: 3,
		},
	}
	cache := newRuleCache(store)
	s := &Service{
		Store:   store,
		cache:   cache,
		limiter: middleware.NewLimiter(cache, nil, cfg),
		config:  cfg,
		log:     zerolog.Nop(),
		stop:    make(chan struct{}),
	}
	t.Cleanup(func() { s.limiter.Stop() })
	return s
}

func TestService_Seed_SeedsEveryRegistryTarget(t *testing.T) {
	ruleRepo := mocks.NewMockRuleRepository(t)
	seeded := make(map[string]bool)
	ruleRepo.EXPECT().Seed(mock.Anything, mock.MatchedBy(func(r model.Rule) bool {
		seeded[r.TargetKey] = true
		return true
	})).Return(nil).Times(len(ratelimit.Registered()))

	store := mocks.NewMockStorage(t)
	store.EXPECT().RuleRepo().Return(ruleRepo)

	svc := newTestService(t, store)
	require.NoError(t, svc.seed(context.Background()))

	for _, target := range ratelimit.Registered() {
		assert.True(t, seeded[target.Key], "target %q was not seeded", target.Key)
	}
}

func TestRuleCache_Reload_MergesRegistryAndDBRule(t *testing.T) {
	dbRules := []model.Rule{
		{TargetKey: ratelimit.TargetAuthLogin, Enabled: false, LimitCount: 5, WindowValue: 1, WindowUnit: model.WindowMinute},
	}
	for _, target := range ratelimit.Registered() {
		if target.Key == ratelimit.TargetAuthLogin {
			continue
		}
		dbRules = append(dbRules, model.Rule{
			TargetKey: target.Key, Enabled: true,
			LimitCount: target.DefaultLimit, WindowValue: target.DefaultWindowValue, WindowUnit: target.DefaultWindowUnit,
		})
	}

	ruleRepo := mocks.NewMockRuleRepository(t)
	ruleRepo.EXPECT().List(mock.Anything).Return(dbRules, nil)

	store := mocks.NewMockStorage(t)
	store.EXPECT().RuleRepo().Return(ruleRepo)

	cache := newRuleCache(store)
	require.NoError(t, cache.Reload(context.Background()))

	got, ok := cache.Effective(ratelimit.TargetAuthLogin)
	require.True(t, ok)
	// Enabled/LimitCount come from the DB row (admin-edited: disabled, limit 5)...
	assert.False(t, got.Enabled)
	assert.Equal(t, 5, got.LimitCount)
	assert.Equal(t, time.Minute, got.Window)
	// ...while Scope/Kind come from the code-defined Registry entry.
	def, _ := ratelimit.ByKey(ratelimit.TargetAuthLogin)
	assert.Equal(t, def.Scope, got.Scope)
	assert.Equal(t, def.Kind, got.Kind)
}

func rcScopePtr(s model.Scope) *model.Scope { return &s }
func rcKindPtr(k model.Kind) *model.Kind    { return &k }

func TestRuleCache_Reload_LoadsExternalTarget(t *testing.T) {
	dbRules := []model.Rule{{
		TargetKey: "cashflow.entry.create", Enabled: true,
		LimitCount: 1000, WindowValue: 1, WindowUnit: model.WindowDay,
		App: "cashflow", IsExternal: true,
		Scope: rcScopePtr(model.ScopeUser), Kind: rcKindPtr(model.KindDailyQuota),
	}}
	ruleRepo := mocks.NewMockRuleRepository(t)
	ruleRepo.EXPECT().List(mock.Anything).Return(dbRules, nil)

	store := mocks.NewMockStorage(t)
	store.EXPECT().RuleRepo().Return(ruleRepo)

	cache := newRuleCache(store)
	require.NoError(t, cache.Reload(context.Background()))

	got, ok := cache.Effective("cashflow.entry.create")
	require.True(t, ok, "external DB row must be enforced even without a Registry entry")
	// scope/kind come from the row's own columns, not the Registry
	assert.Equal(t, model.ScopeUser, got.Scope)
	assert.Equal(t, model.KindDailyQuota, got.Kind)
	assert.Equal(t, 1000, got.LimitCount)
	assert.Equal(t, 24*time.Hour, got.Window)
	assert.True(t, got.Enabled)
}

func TestRuleCache_Reload_SkipsOrphanRow(t *testing.T) {
	dbRules := []model.Rule{{
		// not a Registry key, and not marked external → orphan
		TargetKey: "removed.in.code", Enabled: true,
		LimitCount: 10, WindowValue: 1, WindowUnit: model.WindowMinute,
		App: "all-in-one", IsExternal: false,
	}}
	ruleRepo := mocks.NewMockRuleRepository(t)
	ruleRepo.EXPECT().List(mock.Anything).Return(dbRules, nil)

	store := mocks.NewMockStorage(t)
	store.EXPECT().RuleRepo().Return(ruleRepo)

	cache := newRuleCache(store)
	require.NoError(t, cache.Reload(context.Background()), "an orphan row is skipped, not fatal")

	_, ok := cache.Effective("removed.in.code")
	assert.False(t, ok, "an orphan row must not enter the cache")
}

func TestRuleCache_Reload_SkipsExternalRowMissingScopeKind(t *testing.T) {
	dbRules := []model.Rule{{
		TargetKey: "cashflow.broken", Enabled: true,
		LimitCount: 10, WindowValue: 1, WindowUnit: model.WindowMinute,
		App: "cashflow", IsExternal: true, // Scope/Kind nil
	}}
	ruleRepo := mocks.NewMockRuleRepository(t)
	ruleRepo.EXPECT().List(mock.Anything).Return(dbRules, nil)

	store := mocks.NewMockStorage(t)
	store.EXPECT().RuleRepo().Return(ruleRepo)

	cache := newRuleCache(store)
	require.NoError(t, cache.Reload(context.Background()))

	_, ok := cache.Effective("cashflow.broken")
	assert.False(t, ok, "a malformed external row must not enter the cache")
}

func TestRuleCache_Effective_UnknownKeyIsCacheMiss(t *testing.T) {
	ruleRepo := mocks.NewMockRuleRepository(t)
	ruleRepo.EXPECT().List(mock.Anything).Return(nil, nil)

	store := mocks.NewMockStorage(t)
	store.EXPECT().RuleRepo().Return(ruleRepo)

	cache := newRuleCache(store)
	require.NoError(t, cache.Reload(context.Background()))

	_, ok := cache.Effective("does.not.exist")
	assert.False(t, ok)
}

func TestWindowDuration(t *testing.T) {
	cases := []struct {
		unit  model.WindowUnit
		value int
		want  time.Duration
	}{
		{model.WindowSecond, 30, 30 * time.Second},
		{model.WindowMinute, 5, 5 * time.Minute},
		{model.WindowHour, 2, 2 * time.Hour},
		{model.WindowDay, 1, 24 * time.Hour},
	}
	for _, c := range cases {
		got, err := windowDuration(c.value, c.unit)
		require.NoError(t, err)
		assert.Equal(t, c.want, got)
	}
}

func TestWindowDuration_InvalidUnit(t *testing.T) {
	_, err := windowDuration(1, model.WindowUnit("fortnight"))
	assert.ErrorIs(t, err, ratelimit.ErrInvalidWindowUnit)
}

func TestRuleCache_Reload_InvalidWindowUnitFailsTheWholeReload(t *testing.T) {
	dbRules := []model.Rule{
		{TargetKey: ratelimit.TargetAuthLogin, Enabled: true, LimitCount: 1, WindowValue: 1, WindowUnit: model.WindowUnit("fortnight")},
	}
	ruleRepo := mocks.NewMockRuleRepository(t)
	ruleRepo.EXPECT().List(mock.Anything).Return(dbRules, nil)

	store := mocks.NewMockStorage(t)
	store.EXPECT().RuleRepo().Return(ruleRepo)

	cache := newRuleCache(store)
	err := cache.Reload(context.Background())
	assert.ErrorIs(t, err, ratelimit.ErrInvalidWindowUnit)
}

func TestService_Close_StopsTickers(t *testing.T) {
	ruleRepo := mocks.NewMockRuleRepository(t)
	ruleRepo.EXPECT().List(mock.Anything).Return(nil, nil).Maybe()

	counterRepo := mocks.NewMockCounterRepository(t)
	counterRepo.EXPECT().DeleteOlderThan(mock.Anything, mock.Anything).Return(int64(0), nil).Maybe()

	store := mocks.NewMockStorage(t)
	store.EXPECT().RuleRepo().Return(ruleRepo).Maybe()
	store.EXPECT().CounterRepo().Return(counterRepo).Maybe()

	svc := newTestService(t, store)
	svc.config.RateLimit.CacheRefreshInterval = time.Millisecond
	svc.config.RateLimit.CleanupInterval = time.Millisecond
	svc.startTickers()

	// let a few ticks fire so we know the goroutines are actually running
	time.Sleep(20 * time.Millisecond)

	done := make(chan struct{})
	go func() {
		svc.Close()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Close() did not return — ticker goroutines did not stop")
	}

	// Close must be idempotent
	require.NoError(t, svc.Close())
}

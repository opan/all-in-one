package service

import (
	"context"
	"testing"
	"time"

	"github.com/all-in-one/internal/ratelimit"
	"github.com/all-in-one/internal/ratelimit/model"
	"github.com/all-in-one/internal/ratelimit/service/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestService_ListTargets_MergesRuleAndFallsBackToDefaults(t *testing.T) {
	seeded := model.Rule{
		TargetKey: ratelimit.TargetAuthLogin, Enabled: false, LimitCount: 3,
		WindowValue: 1, WindowUnit: model.WindowMinute, UpdatedAt: time.Now(),
	}
	ruleRepo := mocks.NewMockRuleRepository(t)
	ruleRepo.EXPECT().List(mock.Anything).Return([]model.Rule{seeded}, nil)

	store := mocks.NewMockStorage(t)
	store.EXPECT().RuleRepo().Return(ruleRepo)

	svc := newTestService(t, store)
	targets, err := svc.ListTargets(context.Background())
	require.NoError(t, err)
	require.Len(t, targets, len(ratelimit.Registered()))

	byKey := make(map[string]model.Target, len(targets))
	for _, tg := range targets {
		byKey[tg.Key] = tg
	}

	// the seeded target reflects the DB row
	got := byKey[ratelimit.TargetAuthLogin]
	assert.False(t, got.Enabled)
	assert.Equal(t, 3, got.LimitCount)

	// a target with no DB row yet falls back to its Registry defaults
	def, ok := ratelimit.ByKey(ratelimit.TargetChatMessageSend)
	require.True(t, ok)
	fallback := byKey[ratelimit.TargetChatMessageSend]
	assert.True(t, fallback.Enabled)
	assert.Equal(t, def.DefaultLimit, fallback.LimitCount)
}

func TestService_UpdateTarget_UnknownKey(t *testing.T) {
	store := mocks.NewMockStorage(t)
	svc := newTestService(t, store)

	_, err := svc.UpdateTarget(context.Background(), "does.not.exist", model.TargetPatch{}, "admin")
	assert.ErrorIs(t, err, ratelimit.ErrUnknownTarget)
}

func TestService_UpdateTarget_AppliesPatchAndReloadsCache(t *testing.T) {
	key := ratelimit.TargetAuthLogin
	before := model.Rule{TargetKey: key, Enabled: true, LimitCount: 10, WindowValue: 1, WindowUnit: model.WindowMinute}
	after := model.Rule{TargetKey: key, Enabled: false, LimitCount: 20, WindowValue: 2, WindowUnit: model.WindowHour}

	ruleRepo := mocks.NewMockRuleRepository(t)
	ruleRepo.EXPECT().Get(mock.Anything, key).Return(before, nil).Once()
	ruleRepo.EXPECT().Update(mock.Anything, mock.MatchedBy(func(r model.Rule) bool {
		return r.TargetKey == key && !r.Enabled && r.LimitCount == 20 &&
			r.WindowValue == 2 && r.WindowUnit == model.WindowHour &&
			r.UpdatedBy != nil && *r.UpdatedBy == "admin"
	})).Return(nil)
	ruleRepo.EXPECT().List(mock.Anything).Return([]model.Rule{after}, nil)
	ruleRepo.EXPECT().Get(mock.Anything, key).Return(after, nil).Once()

	store := mocks.NewMockStorage(t)
	store.EXPECT().RuleRepo().Return(ruleRepo)

	svc := newTestService(t, store)
	enabled, limit, windowValue, windowUnit := false, 20, 2, model.WindowHour
	got, err := svc.UpdateTarget(context.Background(), key, model.TargetPatch{
		Enabled: &enabled, LimitCount: &limit, WindowValue: &windowValue, WindowUnit: &windowUnit,
	}, "admin")
	require.NoError(t, err)
	assert.False(t, got.Enabled)
	assert.Equal(t, 20, got.LimitCount)
	assert.Equal(t, 2, got.WindowValue)
	assert.Equal(t, model.WindowHour, got.WindowUnit)
}

func TestService_UpdateTarget_InvalidWindowUnit(t *testing.T) {
	key := ratelimit.TargetAuthLogin
	before := model.Rule{TargetKey: key, Enabled: true, LimitCount: 10, WindowValue: 1, WindowUnit: model.WindowMinute}

	ruleRepo := mocks.NewMockRuleRepository(t)
	ruleRepo.EXPECT().Get(mock.Anything, key).Return(before, nil)
	// Update must NOT be called — no expectation registered for it, so a
	// call would panic the mock and fail the test.

	store := mocks.NewMockStorage(t)
	store.EXPECT().RuleRepo().Return(ruleRepo)

	svc := newTestService(t, store)
	bad := model.WindowUnit("fortnight")
	_, err := svc.UpdateTarget(context.Background(), key, model.TargetPatch{WindowUnit: &bad}, "admin")
	assert.ErrorIs(t, err, ratelimit.ErrInvalidWindowUnit)
}

func TestService_ResetCounters_UnknownKey(t *testing.T) {
	store := mocks.NewMockStorage(t)
	svc := newTestService(t, store)

	err := svc.ResetCounters(context.Background(), "does.not.exist")
	assert.ErrorIs(t, err, ratelimit.ErrUnknownTarget)
}

func TestService_ResetCounters_DeletesTodayForTarget(t *testing.T) {
	key := ratelimit.TargetAuthLogin
	counterRepo := mocks.NewMockCounterRepository(t)
	counterRepo.EXPECT().DeleteForTargetDay(mock.Anything, key, mock.MatchedBy(func(day string) bool {
		_, err := time.Parse("2006-01-02", day)
		return err == nil
	})).Return(nil)

	store := mocks.NewMockStorage(t)
	store.EXPECT().CounterRepo().Return(counterRepo)

	svc := newTestService(t, store)
	require.NoError(t, svc.ResetCounters(context.Background(), key))
}

func TestService_ResetDefaults_UnknownKey(t *testing.T) {
	store := mocks.NewMockStorage(t)
	svc := newTestService(t, store)

	_, err := svc.ResetDefaults(context.Background(), "does.not.exist")
	assert.ErrorIs(t, err, ratelimit.ErrUnknownTarget)
}

func TestService_ResetDefaults_ResetsAndReloadsCache(t *testing.T) {
	key := ratelimit.TargetAuthLogin
	def, ok := ratelimit.ByKey(key)
	require.True(t, ok)
	reset := model.Rule{
		TargetKey: key, Enabled: true,
		LimitCount: def.DefaultLimit, WindowValue: def.DefaultWindowValue, WindowUnit: def.DefaultWindowUnit,
	}

	ruleRepo := mocks.NewMockRuleRepository(t)
	ruleRepo.EXPECT().ResetToDefault(mock.Anything, mock.MatchedBy(func(r model.Rule) bool {
		return r.TargetKey == key && r.Enabled && r.LimitCount == def.DefaultLimit
	})).Return(nil)
	ruleRepo.EXPECT().List(mock.Anything).Return([]model.Rule{reset}, nil)
	ruleRepo.EXPECT().Get(mock.Anything, key).Return(reset, nil)

	store := mocks.NewMockStorage(t)
	store.EXPECT().RuleRepo().Return(ruleRepo)

	svc := newTestService(t, store)
	got, err := svc.ResetDefaults(context.Background(), key)
	require.NoError(t, err)
	assert.True(t, got.Enabled)
	assert.Equal(t, def.DefaultLimit, got.LimitCount)
	assert.Nil(t, got.UpdatedBy)
}

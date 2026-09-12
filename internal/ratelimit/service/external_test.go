package service

import (
	"context"
	"testing"

	httpHelper "github.com/all-in-one/internal/http"
	"github.com/all-in-one/internal/ratelimit"
	"github.com/all-in-one/internal/ratelimit/model"
	"github.com/all-in-one/internal/ratelimit/service/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func extThrottleRule(key string) model.Rule {
	name := "Cashflow share view"
	return model.Rule{
		TargetKey: key, Enabled: true,
		LimitCount: 1, WindowValue: 1, WindowUnit: model.WindowMinute,
		App: "cashflow", IsExternal: true, Name: &name,
		Scope: rcScopePtr(model.ScopeIP), Kind: rcKindPtr(model.KindThrottle),
	}
}

func newExternalTarget() model.Target {
	return model.Target{
		Key: "cashflow.entry.create", App: "cashflow", Name: "Entry create",
		Scope: model.ScopeUser, Kind: model.KindDailyQuota,
		Enabled: true, LimitCount: 1000, WindowValue: 1, WindowUnit: model.WindowDay,
	}
}

func TestCreateExternalTarget_Success(t *testing.T) {
	tgt := newExternalTarget()
	ruleRepo := mocks.NewMockRuleRepository(t)
	ruleRepo.EXPECT().CreateExternal(mock.Anything, mock.MatchedBy(func(r model.Rule) bool {
		return r.TargetKey == tgt.Key && r.IsExternal && r.Scope != nil && *r.Scope == model.ScopeUser
	})).Return(nil)
	stored := extThrottleRule(tgt.Key) // shape doesn't matter for the read-back assertions below
	stored.TargetKey = tgt.Key
	stored.Scope = rcScopePtr(model.ScopeUser)
	stored.Kind = rcKindPtr(model.KindDailyQuota)
	stored.App = "cashflow"
	ruleRepo.EXPECT().List(mock.Anything).Return([]model.Rule{stored}, nil) // Reload
	ruleRepo.EXPECT().Get(mock.Anything, tgt.Key).Return(stored, nil)

	store := mocks.NewMockStorage(t)
	store.EXPECT().RuleRepo().Return(ruleRepo)
	svc := newTestService(t, store)

	got, err := svc.CreateExternalTarget(context.Background(), tgt, "admin")
	require.NoError(t, err)
	assert.True(t, got.IsExternal)
	assert.Equal(t, "cashflow", got.App)
	assert.Equal(t, model.ScopeUser, got.Scope)
}

func TestCreateExternalTarget_RejectsRegistryKeyCollision(t *testing.T) {
	store := mocks.NewMockStorage(t)
	svc := newTestService(t, store)

	tgt := newExternalTarget()
	tgt.Key = ratelimit.TargetAuthLogin // collides with a Registry key
	_, err := svc.CreateExternalTarget(context.Background(), tgt, "admin")
	assert.ErrorIs(t, err, ratelimit.ErrExternalTargetExists)
}

func TestCreateExternalTarget_RejectsInvalidScopeKind(t *testing.T) {
	store := mocks.NewMockStorage(t)
	svc := newTestService(t, store)

	bad := newExternalTarget()
	bad.Scope = model.Scope("planet")
	_, err := svc.CreateExternalTarget(context.Background(), bad, "admin")
	assert.Error(t, err)

	bad2 := newExternalTarget()
	bad2.Kind = model.Kind("vibes")
	_, err = svc.CreateExternalTarget(context.Background(), bad2, "admin")
	assert.Error(t, err)
}

func TestCreateExternalTarget_PropagatesDuplicateFromRepo(t *testing.T) {
	ruleRepo := mocks.NewMockRuleRepository(t)
	ruleRepo.EXPECT().CreateExternal(mock.Anything, mock.Anything).Return(ratelimit.ErrExternalTargetExists)
	store := mocks.NewMockStorage(t)
	store.EXPECT().RuleRepo().Return(ruleRepo)
	svc := newTestService(t, store)

	_, err := svc.CreateExternalTarget(context.Background(), newExternalTarget(), "admin")
	assert.ErrorIs(t, err, ratelimit.ErrExternalTargetExists)
}

func TestDeleteExternalTarget_Success(t *testing.T) {
	stored := extThrottleRule("cashflow.share.view.ip")
	ruleRepo := mocks.NewMockRuleRepository(t)
	ruleRepo.EXPECT().Get(mock.Anything, stored.TargetKey).Return(stored, nil)
	ruleRepo.EXPECT().Delete(mock.Anything, stored.TargetKey).Return(nil)
	ruleRepo.EXPECT().List(mock.Anything).Return(nil, nil) // Reload

	store := mocks.NewMockStorage(t)
	store.EXPECT().RuleRepo().Return(ruleRepo)
	svc := newTestService(t, store)

	require.NoError(t, svc.DeleteExternalTarget(context.Background(), stored.TargetKey))
}

func TestDeleteExternalTarget_RejectsInternalKey(t *testing.T) {
	store := mocks.NewMockStorage(t)
	svc := newTestService(t, store)

	err := svc.DeleteExternalTarget(context.Background(), ratelimit.TargetAuthLogin)
	assert.ErrorIs(t, err, ratelimit.ErrNotSupportedForExternal)
}

func TestDeleteExternalTarget_UnknownKey(t *testing.T) {
	ruleRepo := mocks.NewMockRuleRepository(t)
	ruleRepo.EXPECT().Get(mock.Anything, "cashflow.nope").Return(model.Rule{}, httpHelper.ErrNotFound)
	store := mocks.NewMockStorage(t)
	store.EXPECT().RuleRepo().Return(ruleRepo)
	svc := newTestService(t, store)

	err := svc.DeleteExternalTarget(context.Background(), "cashflow.nope")
	assert.ErrorIs(t, err, ratelimit.ErrUnknownTarget)
}

func TestResetDefaults_RejectsExternalTarget(t *testing.T) {
	stored := extThrottleRule("cashflow.share.view.ip")
	ruleRepo := mocks.NewMockRuleRepository(t)
	ruleRepo.EXPECT().Get(mock.Anything, stored.TargetKey).Return(stored, nil)
	store := mocks.NewMockStorage(t)
	store.EXPECT().RuleRepo().Return(ruleRepo)
	svc := newTestService(t, store)

	_, err := svc.ResetDefaults(context.Background(), stored.TargetKey)
	assert.ErrorIs(t, err, ratelimit.ErrNotSupportedForExternal)
}

func TestUpdateTarget_ExternalTarget(t *testing.T) {
	key := "cashflow.share.view.ip"
	before := extThrottleRule(key)
	after := extThrottleRule(key)
	after.LimitCount = 500

	ruleRepo := mocks.NewMockRuleRepository(t)
	ruleRepo.EXPECT().Get(mock.Anything, key).Return(before, nil).Once()
	ruleRepo.EXPECT().Update(mock.Anything, mock.MatchedBy(func(r model.Rule) bool {
		return r.TargetKey == key && r.LimitCount == 500
	})).Return(nil)
	ruleRepo.EXPECT().List(mock.Anything).Return([]model.Rule{after}, nil) // Reload
	ruleRepo.EXPECT().Get(mock.Anything, key).Return(after, nil).Once()

	store := mocks.NewMockStorage(t)
	store.EXPECT().RuleRepo().Return(ruleRepo)
	svc := newTestService(t, store)

	limit := 500
	got, err := svc.UpdateTarget(context.Background(), key, model.TargetPatch{LimitCount: &limit}, "admin")
	require.NoError(t, err)
	assert.True(t, got.IsExternal)
	assert.Equal(t, 500, got.LimitCount)
	assert.Equal(t, model.ScopeIP, got.Scope)
}

func TestCheckExternal_ThrottleAllowThenReject(t *testing.T) {
	key := "cashflow.share.view.ip"
	ruleRepo := mocks.NewMockRuleRepository(t)
	ruleRepo.EXPECT().List(mock.Anything).Return([]model.Rule{extThrottleRule(key)}, nil)
	store := mocks.NewMockStorage(t)
	store.EXPECT().RuleRepo().Return(ruleRepo)
	svc := newTestService(t, store)
	require.NoError(t, svc.cache.Reload(context.Background()))

	resp, err := svc.CheckExternal(context.Background(), key, "ip:1.1.1.1")
	require.NoError(t, err)
	assert.True(t, resp.Allowed)
	assert.Equal(t, 1, resp.Limit)
	assert.Equal(t, 0, resp.Remaining)

	resp, err = svc.CheckExternal(context.Background(), key, "ip:1.1.1.1")
	require.NoError(t, err)
	assert.False(t, resp.Allowed, "second call over a limit of 1 must be rejected")
	assert.Greater(t, resp.RetryAfterSeconds, 0)
}

func TestCheckExternal_CacheMissFailsOpen(t *testing.T) {
	store := mocks.NewMockStorage(t)
	svc := newTestService(t, store) // empty cache

	resp, err := svc.CheckExternal(context.Background(), "cashflow.unknown", "ip:1.1.1.1")
	require.NoError(t, err)
	assert.True(t, resp.Allowed, "an unknown/unloaded target must fail open")
}

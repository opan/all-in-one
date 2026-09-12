package middleware

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/all-in-one/internal/config"
	"github.com/all-in-one/internal/ratelimit/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Check is the decision core extracted from enforce (P6). These tests exercise
// it directly, without HTTP, the way the external check API (P11) calls it.

func newCheckLimiter(t *testing.T, counters *fakeCounterStore) *Limiter {
	t.Helper()
	l := newTestLimiter(&fakeRuleProvider{}, counters, config.RateLimitConfig{Enabled: true})
	t.Cleanup(l.Stop)
	return l
}

func TestCheck_Throttle_AllowsThenExhausts(t *testing.T) {
	l := newCheckLimiter(t, &fakeCounterStore{})
	rule := model.EffectiveRule{
		Key: "cashflow.share.view.ip", Scope: model.ScopeIP, Kind: model.KindThrottle,
		Enabled: true, LimitCount: 2, Window: time.Minute,
	}

	allowed, _, remaining, err := l.Check(context.Background(), rule, "ip:1.2.3.4")
	require.NoError(t, err)
	assert.True(t, allowed)
	assert.Equal(t, 1, remaining)

	allowed, _, remaining, err = l.Check(context.Background(), rule, "ip:1.2.3.4")
	require.NoError(t, err)
	assert.True(t, allowed)
	assert.Equal(t, 0, remaining)

	allowed, retryAfter, remaining, err := l.Check(context.Background(), rule, "ip:1.2.3.4")
	require.NoError(t, err)
	assert.False(t, allowed, "third call over a limit of 2 must be rejected")
	assert.Equal(t, 0, remaining)
	assert.Greater(t, retryAfter, time.Duration(0))
}

func TestCheck_DailyQuota_AllowsThenExhausts(t *testing.T) {
	counters := &fakeCounterStore{}
	l := newCheckLimiter(t, counters)
	rule := model.EffectiveRule{
		Key: "cashflow.entry.create", Scope: model.ScopeUser, Kind: model.KindDailyQuota,
		Enabled: true, LimitCount: 2, Window: 24 * time.Hour,
	}

	allowed, _, remaining, err := l.Check(context.Background(), rule, "user:42")
	require.NoError(t, err)
	assert.True(t, allowed)
	assert.Equal(t, 1, remaining)

	allowed, _, remaining, err = l.Check(context.Background(), rule, "user:42")
	require.NoError(t, err)
	assert.True(t, allowed)
	assert.Equal(t, 0, remaining)

	allowed, retryAfter, _, err := l.Check(context.Background(), rule, "user:42")
	require.NoError(t, err)
	assert.False(t, allowed, "third create over a quota of 2 must be rejected")
	assert.Greater(t, retryAfter, time.Duration(0), "retry-after should point at the next day boundary")
}

func TestCheck_DailyQuota_KeysAreIndependent(t *testing.T) {
	l := newCheckLimiter(t, &fakeCounterStore{})
	rule := model.EffectiveRule{
		Key: "cashflow.entry.create", Scope: model.ScopeUser, Kind: model.KindDailyQuota,
		Enabled: true, LimitCount: 1, Window: 24 * time.Hour,
	}

	allowed, _, _, err := l.Check(context.Background(), rule, "user:1")
	require.NoError(t, err)
	assert.True(t, allowed)

	// a different bucket key has its own budget
	allowed, _, _, err = l.Check(context.Background(), rule, "user:2")
	require.NoError(t, err)
	assert.True(t, allowed)
}

func TestCheck_DisabledRule_AllowsWithoutCounting(t *testing.T) {
	counters := &fakeCounterStore{}
	l := newCheckLimiter(t, counters)
	rule := model.EffectiveRule{
		Key: "cashflow.entry.create", Scope: model.ScopeUser, Kind: model.KindDailyQuota,
		Enabled: false, LimitCount: 5, Window: 24 * time.Hour,
	}

	allowed, retryAfter, remaining, err := l.Check(context.Background(), rule, "user:42")
	require.NoError(t, err)
	assert.True(t, allowed)
	assert.Equal(t, time.Duration(0), retryAfter)
	assert.Equal(t, 5, remaining)
	assert.Empty(t, counters.counts, "a disabled rule must not touch the counter store")
}

func TestCheck_CounterStoreError_FailsOpen(t *testing.T) {
	l := newCheckLimiter(t, &fakeCounterStore{err: errors.New("boom")})
	rule := model.EffectiveRule{
		Key: "cashflow.entry.create", Scope: model.ScopeUser, Kind: model.KindDailyQuota,
		Enabled: true, LimitCount: 1, Window: 24 * time.Hour,
	}

	allowed, _, _, err := l.Check(context.Background(), rule, "user:42")
	require.Error(t, err, "the store error is surfaced for the caller to log/meter")
	assert.True(t, allowed, "a counter-store error must fail open (ADR-007)")
}

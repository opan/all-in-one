package middleware

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemStore_AllowsUpToLimitThenRejects(t *testing.T) {
	s := newMemStore(time.Hour)
	defer s.Stop()

	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	limit := 3
	window := time.Minute

	for i := 0; i < limit; i++ {
		ok, retryAfter := s.allow("k", limit, window, now)
		require.True(t, ok, "request %d should be allowed", i+1)
		assert.Zero(t, retryAfter)
	}

	ok, retryAfter := s.allow("k", limit, window, now)
	assert.False(t, ok)
	assert.Equal(t, window, retryAfter) // rejected right after the window opened
}

func TestMemStore_RetryAfterCountsDownWithinWindow(t *testing.T) {
	s := newMemStore(time.Hour)
	defer s.Stop()

	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	limit := 1
	window := time.Minute

	ok, _ := s.allow("k", limit, window, now)
	require.True(t, ok)

	later := now.Add(20 * time.Second)
	ok, retryAfter := s.allow("k", limit, window, later)
	assert.False(t, ok)
	assert.Equal(t, 40*time.Second, retryAfter)
}

func TestMemStore_WindowResets(t *testing.T) {
	s := newMemStore(time.Hour)
	defer s.Stop()

	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	limit := 1
	window := time.Minute

	ok, _ := s.allow("k", limit, window, now)
	require.True(t, ok)

	ok, _ = s.allow("k", limit, window, now.Add(30*time.Second))
	require.False(t, ok, "still within the first window")

	// once the window has elapsed, a new window starts and the key is
	// allowed again
	ok, retryAfter := s.allow("k", limit, window, now.Add(window))
	assert.True(t, ok)
	assert.Zero(t, retryAfter)
}

func TestMemStore_KeysAreIndependent(t *testing.T) {
	s := newMemStore(time.Hour)
	defer s.Stop()

	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	limit := 1
	window := time.Minute

	ok, _ := s.allow("a", limit, window, now)
	require.True(t, ok)

	ok, _ = s.allow("b", limit, window, now)
	assert.True(t, ok, "a different key must have its own bucket")
}

func TestMemStore_CleanupEvictsExpiredBuckets(t *testing.T) {
	s := newMemStore(10 * time.Millisecond)
	defer s.Stop()

	now := time.Now()
	ok, _ := s.allow("k", 1, time.Millisecond, now)
	require.True(t, ok)

	require.Eventually(t, func() bool {
		s.mu.RLock()
		defer s.mu.RUnlock()
		_, exists := s.buckets["k"]
		return !exists
	}, time.Second, 5*time.Millisecond, "expired bucket was not evicted")
}

func TestMemStore_Stop_IsIdempotent(t *testing.T) {
	s := newMemStore(time.Hour)
	s.Stop()
	s.Stop() // must not panic
}

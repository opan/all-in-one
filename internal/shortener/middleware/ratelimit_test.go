package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRateLimiter_AllowsUnderLimit(t *testing.T) {
	rl := NewRateLimiter(5, time.Minute)
	defer rl.Stop()

	for i := 0; i < 5; i++ {
		ok, _ := rl.allow("user:abc")
		assert.True(t, ok, "request %d should be allowed", i+1)
	}
}

func TestRateLimiter_BlocksOverLimit(t *testing.T) {
	rl := NewRateLimiter(3, time.Minute)
	defer rl.Stop()

	for i := 0; i < 3; i++ {
		rl.allow("user:abc")
	}
	ok, retryAfter := rl.allow("user:abc")
	assert.False(t, ok)
	assert.Greater(t, retryAfter, time.Duration(0))
}

func TestRateLimiter_ResetsAfterWindow(t *testing.T) {
	rl := NewRateLimiter(2, 50*time.Millisecond)
	defer rl.Stop()

	rl.allow("user:abc")
	rl.allow("user:abc")
	ok, _ := rl.allow("user:abc")
	assert.False(t, ok, "should be blocked before window resets")

	time.Sleep(60 * time.Millisecond)

	ok, _ = rl.allow("user:abc")
	assert.True(t, ok, "should be allowed after window resets")
}

func TestRateLimiter_IsolatesKeys(t *testing.T) {
	rl := NewRateLimiter(1, time.Minute)
	defer rl.Stop()

	ok1, _ := rl.allow("user:alice")
	ok2, _ := rl.allow("user:bob")
	assert.True(t, ok1)
	assert.True(t, ok2)

	ok1Again, _ := rl.allow("user:alice")
	assert.False(t, ok1Again, "alice should be blocked; bob's bucket should not affect alice")
}

func TestRateLimiter_Wrap_Returns429(t *testing.T) {
	rl := NewRateLimiter(0, time.Minute) // limit=0 blocks everything
	defer rl.Stop()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.RemoteAddr = "1.2.3.4:5678"
	rr := httptest.NewRecorder()

	rl.Wrap(next)(rr, req)

	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
	assert.NotEmpty(t, rr.Header().Get("Retry-After"))
}

func TestRateLimiter_Wrap_PassesThrough(t *testing.T) {
	rl := NewRateLimiter(10, time.Minute)
	defer rl.Stop()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.RemoteAddr = "1.2.3.4:5678"
	rr := httptest.NewRecorder()

	rl.Wrap(next)(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
}

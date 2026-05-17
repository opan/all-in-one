package middleware

import (
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/all-in-one/internal/auth"
	httpHelper "github.com/all-in-one/internal/http"
)

type bucket struct {
	count   int
	resetAt time.Time
}

// RateLimiter is a fixed-window in-memory rate limiter keyed by user ID or IP.
type RateLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*bucket
	limit    int
	window   time.Duration
	stopOnce sync.Once
	stop     chan struct{}
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		buckets: make(map[string]*bucket),
		limit:   limit,
		window:  window,
		stop:    make(chan struct{}),
	}
	go rl.cleanup()
	return rl
}

// allow checks the rate limit for key and returns (allowed, retryAfter).
func (rl *RateLimiter) allow(key string) (bool, time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, ok := rl.buckets[key]
	if !ok || now.After(b.resetAt) {
		rl.buckets[key] = &bucket{count: 0, resetAt: now.Add(rl.window)}
		b = rl.buckets[key]
	}
	if b.count >= rl.limit {
		return false, b.resetAt.Sub(now)
	}
	b.count++
	return true, 0
}

// Wrap returns a HandlerFunc that enforces the rate limit, keyed by user ID or IP.
func (rl *RateLimiter) Wrap(next http.HandlerFunc) http.HandlerFunc {
	return rl.WrapWithKey(next, rateLimitKey)
}

// WrapWithKey is like Wrap but uses keyFn to derive the bucket key from the request.
func (rl *RateLimiter) WrapWithKey(next http.HandlerFunc, keyFn func(*http.Request) string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ok, retryAfter := rl.allow(keyFn(r))
		if !ok {
			w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())+1))
			httpHelper.SendError(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next(w, r)
	}
}

func (rl *RateLimiter) Stop() {
	rl.stopOnce.Do(func() { close(rl.stop) })
}

// cleanup removes expired buckets every window duration to prevent unbounded growth.
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.window)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			rl.mu.Lock()
			now := time.Now()
			for k, b := range rl.buckets {
				if now.After(b.resetAt) {
					delete(rl.buckets, k)
				}
			}
			rl.mu.Unlock()
		case <-rl.stop:
			return
		}
	}
}

func rateLimitKey(r *http.Request) string {
	if user, ok := auth.GetUserFromContext(r.Context()); ok && user.UserID != "" {
		return "user:" + user.UserID
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return "ip:" + r.RemoteAddr
	}
	return "ip:" + ip
}

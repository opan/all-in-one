package middleware

import (
	"sync"
	"time"
)

// bucket is one key's current fixed-window count.
type bucket struct {
	count   int
	resetAt time.Time
}

// memStore is the in-memory fixed-window throttle store used for `throttle`
// targets (docs/adr/RATE_LIMITING_ADR.md ADR-002) — the DB never sees these
// requests, so a flood is shed with a cheap map lookup instead of hitting
// the database the limiter exists to protect. It descends from the
// shortener's original fixed-window limiter (since folded into this
// app-feature — ADR-011), but limit and window are passed per call rather
// than fixed at construction, since an admin can retune a target's rule at
// runtime (ADR-008).
type memStore struct {
	mu      sync.RWMutex
	buckets map[string]*bucket

	stopOnce sync.Once
	stop     chan struct{}
}

func newMemStore(cleanupInterval time.Duration) *memStore {
	s := &memStore{
		buckets: make(map[string]*bucket),
		stop:    make(chan struct{}),
	}
	go s.cleanup(cleanupInterval)
	return s
}

// allow reports whether key may proceed under the given limit/window,
// evaluated at now (injectable so tests are deterministic). If rejected,
// the second return value is how long the caller should wait before
// retrying.
func (s *memStore) allow(key string, limit int, window time.Duration, now time.Time) (bool, time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, ok := s.buckets[key]
	if !ok || !now.Before(b.resetAt) {
		b = &bucket{count: 0, resetAt: now.Add(window)}
		s.buckets[key] = b
	}
	if b.count >= limit {
		return false, b.resetAt.Sub(now)
	}
	b.count++
	return true, 0
}

// Stop halts the cleanup goroutine. Safe to call more than once.
func (s *memStore) Stop() {
	s.stopOnce.Do(func() { close(s.stop) })
}

// cleanup evicts expired buckets on a ticker so the map doesn't grow
// unbounded from short-lived keys (e.g. once-seen IPs).
func (s *memStore) cleanup(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			now := time.Now()
			s.mu.Lock()
			for k, b := range s.buckets {
				if now.After(b.resetAt) {
					delete(s.buckets, k)
				}
			}
			s.mu.Unlock()
		case <-s.stop:
			return
		}
	}
}

package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/all-in-one/internal/config"
	"github.com/all-in-one/internal/ratelimit"
	"github.com/all-in-one/internal/ratelimit/model"
	"github.com/all-in-one/internal/tester"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- test doubles ---

type fakeRuleProvider struct {
	rules map[string]model.EffectiveRule
}

func (f *fakeRuleProvider) Effective(key string) (model.EffectiveRule, bool) {
	r, ok := f.rules[key]
	return r, ok
}

type fakeCounterStore struct {
	mu     sync.Mutex
	counts map[string]int
	err    error
}

func (f *fakeCounterStore) IncrAndGet(ctx context.Context, targetKey, bucketKey, day string) (int, error) {
	if f.err != nil {
		return 0, f.err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.counts == nil {
		f.counts = make(map[string]int)
	}
	k := targetKey + "|" + bucketKey + "|" + day
	f.counts[k]++
	return f.counts[k], nil
}

func okHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// newTestRouter mounts l.Middleware() on a /api/v1 subrouter carrying the
// same paths the real server registers, so mux.CurrentRoute(r).GetPathTemplate()
// resolves to the exact strings the Registry's TargetDef.Path values use
// (docs/adr/RATE_LIMITING_ADR.md ADR-004).
func newTestRouter(l *Limiter) *mux.Router {
	r := mux.NewRouter()
	sr := r.PathPrefix("/api/v1").Subrouter()
	sr.Use(l.Middleware())
	sr.HandleFunc("/sessions", okHandler).Methods(http.MethodPost)
	sr.HandleFunc("/users", okHandler).Methods(http.MethodPost)
	sr.HandleFunc("/chats/{id}/messages", okHandler).Methods(http.MethodPost)
	sr.HandleFunc("/unbound", okHandler).Methods(http.MethodGet)
	return r
}

func doRequest(router *mux.Router, method, path, remoteAddr string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	req.RemoteAddr = remoteAddr
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

func newTestLimiter(rules *fakeRuleProvider, counters *fakeCounterStore, cfg config.RateLimitConfig) *Limiter {
	if cfg.CleanupInterval == 0 {
		cfg.CleanupInterval = time.Hour
	}
	return NewLimiter(rules, counters, config.Config{RateLimit: cfg})
}

// --- Middleware() end-to-end tests ---

func TestLimiter_Throttle_AllowThenReject(t *testing.T) {
	rules := &fakeRuleProvider{rules: map[string]model.EffectiveRule{
		ratelimit.TargetAuthLogin: {Key: ratelimit.TargetAuthLogin, Scope: model.ScopeIP, Kind: model.KindThrottle, Enabled: true, LimitCount: 2, Window: time.Minute},
	}}
	l := newTestLimiter(rules, &fakeCounterStore{}, config.RateLimitConfig{Enabled: true})
	defer l.Stop()
	router := newTestRouter(l)

	for i := 0; i < 2; i++ {
		rr := doRequest(router, http.MethodPost, "/api/v1/sessions", "203.0.113.5:1234")
		assert.Equal(t, http.StatusOK, rr.Code, "request %d should be allowed", i+1)
	}

	rr := doRequest(router, http.MethodPost, "/api/v1/sessions", "203.0.113.5:1234")
	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
	assert.NotEmpty(t, rr.Header().Get("Retry-After"))
}

func TestLimiter_DailyQuota_AllowThenReject(t *testing.T) {
	rules := &fakeRuleProvider{rules: map[string]model.EffectiveRule{
		ratelimit.TargetAuthSignupIP: {Key: ratelimit.TargetAuthSignupIP, Scope: model.ScopeIP, Kind: model.KindDailyQuota, Enabled: true, LimitCount: 2, Window: 24 * time.Hour},
	}}
	l := newTestLimiter(rules, &fakeCounterStore{}, config.RateLimitConfig{Enabled: true})
	defer l.Stop()
	router := newTestRouter(l)

	for i := 0; i < 2; i++ {
		rr := doRequest(router, http.MethodPost, "/api/v1/users", "203.0.113.9:1234")
		assert.Equal(t, http.StatusOK, rr.Code, "request %d should be allowed", i+1)
	}

	rr := doRequest(router, http.MethodPost, "/api/v1/users", "203.0.113.9:1234")
	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
	assert.NotEmpty(t, rr.Header().Get("Retry-After"))
}

func TestLimiter_ThrottleBeforeDailyQuota_Signup(t *testing.T) {
	rules := &fakeRuleProvider{rules: map[string]model.EffectiveRule{
		ratelimit.TargetAuthSignupThrottleIP: {Key: ratelimit.TargetAuthSignupThrottleIP, Scope: model.ScopeIP, Kind: model.KindThrottle, Enabled: true, LimitCount: 1, Window: time.Minute},
		ratelimit.TargetAuthSignupIP:         {Key: ratelimit.TargetAuthSignupIP, Scope: model.ScopeIP, Kind: model.KindDailyQuota, Enabled: true, LimitCount: 20, Window: 24 * time.Hour},
	}}
	counters := &fakeCounterStore{}
	l := newTestLimiter(rules, counters, config.RateLimitConfig{Enabled: true})
	defer l.Stop()
	router := newTestRouter(l)

	rr := doRequest(router, http.MethodPost, "/api/v1/users", "203.0.113.20:1234")
	assert.Equal(t, http.StatusOK, rr.Code, "first request should be allowed")

	rr = doRequest(router, http.MethodPost, "/api/v1/users", "203.0.113.20:1234")
	assert.Equal(t, http.StatusTooManyRequests, rr.Code, "second request should be shed by the burst throttle, well under the daily quota's limit of 20")

	// The daily-quota counter is DB-backed (fakeCounterStore here); it must
	// only have been touched once, by the first allowed request — proving
	// orderThrottlesFirst shed the second request in-memory before it ever
	// reached the counter store (ADR-002).
	counters.mu.Lock()
	defer counters.mu.Unlock()
	assert.Len(t, counters.counts, 1, "the throttled request must not have reached the daily-quota counter store")
}

func TestLimiter_DisabledTarget_Passes(t *testing.T) {
	rules := &fakeRuleProvider{rules: map[string]model.EffectiveRule{
		ratelimit.TargetAuthLogin: {Key: ratelimit.TargetAuthLogin, Scope: model.ScopeIP, Kind: model.KindThrottle, Enabled: false, LimitCount: 1, Window: time.Minute},
	}}
	l := newTestLimiter(rules, &fakeCounterStore{}, config.RateLimitConfig{Enabled: true})
	defer l.Stop()
	router := newTestRouter(l)

	for i := 0; i < 5; i++ {
		rr := doRequest(router, http.MethodPost, "/api/v1/sessions", "203.0.113.5:1234")
		assert.Equal(t, http.StatusOK, rr.Code)
	}
}

func TestLimiter_UnknownRoute_Passes(t *testing.T) {
	l := newTestLimiter(&fakeRuleProvider{}, &fakeCounterStore{}, config.RateLimitConfig{Enabled: true})
	defer l.Stop()
	router := newTestRouter(l)

	rr := doRequest(router, http.MethodGet, "/api/v1/unbound", "203.0.113.5:1234")
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestLimiter_MasterSwitchOff_Passes(t *testing.T) {
	rules := &fakeRuleProvider{rules: map[string]model.EffectiveRule{
		// a limit of 0 would reject every request if the master switch were
		// respected — proves the switch, not a generous limit, is why it passes
		ratelimit.TargetAuthLogin: {Key: ratelimit.TargetAuthLogin, Scope: model.ScopeIP, Kind: model.KindThrottle, Enabled: true, LimitCount: 0, Window: time.Minute},
	}}
	l := newTestLimiter(rules, &fakeCounterStore{}, config.RateLimitConfig{Enabled: false})
	defer l.Stop()
	router := newTestRouter(l)

	rr := doRequest(router, http.MethodPost, "/api/v1/sessions", "203.0.113.5:1234")
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestLimiter_CounterStoreError_FailsOpen(t *testing.T) {
	rules := &fakeRuleProvider{rules: map[string]model.EffectiveRule{
		ratelimit.TargetAuthSignupIP: {Key: ratelimit.TargetAuthSignupIP, Scope: model.ScopeIP, Kind: model.KindDailyQuota, Enabled: true, LimitCount: 1, Window: 24 * time.Hour},
	}}
	l := newTestLimiter(rules, &fakeCounterStore{err: errors.New("boom")}, config.RateLimitConfig{Enabled: true})
	defer l.Stop()
	router := newTestRouter(l)

	rr := doRequest(router, http.MethodPost, "/api/v1/users", "203.0.113.5:1234")
	assert.Equal(t, http.StatusOK, rr.Code, "a counter store error must fail open, not reject")
}

func TestLimiter_UserScope_KeysPerUser(t *testing.T) {
	rules := &fakeRuleProvider{rules: map[string]model.EffectiveRule{
		ratelimit.TargetChatMessageSend: {Key: ratelimit.TargetChatMessageSend, Scope: model.ScopeUser, Kind: model.KindThrottle, Enabled: true, LimitCount: 1, Window: time.Minute},
	}}
	l := newTestLimiter(rules, &fakeCounterStore{}, config.RateLimitConfig{Enabled: true})
	defer l.Stop()
	router := newTestRouter(l)

	reqFor := func(userID string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/chats/abc/messages", nil)
		req.RemoteAddr = "203.0.113.5:1234"
		req = req.WithContext(tester.ContextWithUser(userID, "u", "u@example.com", uuid.New().String()))
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		return rr
	}

	assert.Equal(t, http.StatusOK, reqFor("user-a").Code)
	assert.Equal(t, http.StatusTooManyRequests, reqFor("user-a").Code, "second request from the same user should be throttled")
	assert.Equal(t, http.StatusOK, reqFor("user-b").Code, "a different user has its own bucket")
}

func TestLimiter_ClientIP_TrustProxyHeaders(t *testing.T) {
	rules := &fakeRuleProvider{rules: map[string]model.EffectiveRule{
		ratelimit.TargetAuthLogin: {Key: ratelimit.TargetAuthLogin, Scope: model.ScopeIP, Kind: model.KindThrottle, Enabled: true, LimitCount: 1, Window: time.Minute},
	}}

	reqWithXFF := func(xff string) *http.Request {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", nil)
		req.RemoteAddr = "10.0.0.1:1234" // shared proxy IP
		req.Header.Set("X-Forwarded-For", xff)
		return req
	}

	t.Run("trusted: XFF determines the bucket", func(t *testing.T) {
		l := newTestLimiter(rules, &fakeCounterStore{}, config.RateLimitConfig{Enabled: true, TrustProxyHeaders: true})
		defer l.Stop()
		router := newTestRouter(l)

		rr1 := httptest.NewRecorder()
		router.ServeHTTP(rr1, reqWithXFF("203.0.113.1"))
		assert.Equal(t, http.StatusOK, rr1.Code)

		rr2 := httptest.NewRecorder()
		router.ServeHTTP(rr2, reqWithXFF("203.0.113.2"))
		assert.Equal(t, http.StatusOK, rr2.Code, "a different XFF client must have its own bucket")
	})

	t.Run("untrusted: RemoteAddr determines the bucket regardless of XFF", func(t *testing.T) {
		l := newTestLimiter(rules, &fakeCounterStore{}, config.RateLimitConfig{Enabled: true, TrustProxyHeaders: false})
		defer l.Stop()
		router := newTestRouter(l)

		rr1 := httptest.NewRecorder()
		router.ServeHTTP(rr1, reqWithXFF("203.0.113.1"))
		assert.Equal(t, http.StatusOK, rr1.Code)

		rr2 := httptest.NewRecorder()
		router.ServeHTTP(rr2, reqWithXFF("203.0.113.2"))
		assert.Equal(t, http.StatusTooManyRequests, rr2.Code, "XFF must be ignored when untrusted")
	})

	t.Run("trusted: CF-Connecting-IP determines the bucket (Cloudflare Tunnel, ADR-012)", func(t *testing.T) {
		l := newTestLimiter(rules, &fakeCounterStore{}, config.RateLimitConfig{Enabled: true, TrustProxyHeaders: true})
		defer l.Stop()
		router := newTestRouter(l)

		reqWithCF := func(ip string) *http.Request {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", nil)
			req.RemoteAddr = "10.0.0.1:1234" // cloudflared's cluster-internal peer address
			req.Header.Set("CF-Connecting-IP", ip)
			return req
		}

		rr1 := httptest.NewRecorder()
		router.ServeHTTP(rr1, reqWithCF("198.51.100.1"))
		assert.Equal(t, http.StatusOK, rr1.Code)

		rr2 := httptest.NewRecorder()
		router.ServeHTTP(rr2, reqWithCF("198.51.100.2"))
		assert.Equal(t, http.StatusOK, rr2.Code, "a different CF-Connecting-IP client must have its own bucket")
	})

	t.Run("trusted: CF-Connecting-IP takes priority over XFF", func(t *testing.T) {
		l := newTestLimiter(rules, &fakeCounterStore{}, config.RateLimitConfig{Enabled: true, TrustProxyHeaders: true})
		defer l.Stop()
		router := newTestRouter(l)

		reqWithBoth := func(cfip, xff string) *http.Request {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", nil)
			req.RemoteAddr = "10.0.0.1:1234"
			req.Header.Set("CF-Connecting-IP", cfip)
			req.Header.Set("X-Forwarded-For", xff)
			return req
		}

		// Same CF-Connecting-IP, different (attacker-forgeable) XFF — must
		// share one bucket, proving CF-Connecting-IP wins.
		rr1 := httptest.NewRecorder()
		router.ServeHTTP(rr1, reqWithBoth("198.51.100.9", "203.0.113.1"))
		assert.Equal(t, http.StatusOK, rr1.Code)

		rr2 := httptest.NewRecorder()
		router.ServeHTTP(rr2, reqWithBoth("198.51.100.9", "203.0.113.2"))
		assert.Equal(t, http.StatusTooManyRequests, rr2.Code, "same CF-Connecting-IP must throttle even with a different XFF")
	})
}

// --- pure unit tests (no mux/HTTP round trip needed) ---

func TestOrderThrottlesFirst(t *testing.T) {
	throttle := ratelimit.TargetDef{Key: "t.throttle", Kind: model.KindThrottle}
	quota := ratelimit.TargetDef{Key: "t.quota", Kind: model.KindDailyQuota}

	got := orderThrottlesFirst([]ratelimit.TargetDef{quota, throttle})
	require.Len(t, got, 2)
	assert.Equal(t, "t.throttle", got[0].Key)
	assert.Equal(t, "t.quota", got[1].Key)
}

func TestLimiter_BucketKey(t *testing.T) {
	l := newTestLimiter(&fakeRuleProvider{}, &fakeCounterStore{}, config.RateLimitConfig{})
	defer l.Stop()

	t.Run("global scope ignores the request entirely", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		assert.Equal(t, "global", l.bucketKey(req, model.ScopeGlobal))
	})

	t.Run("ip scope keys by RemoteAddr", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "203.0.113.5:1234"
		assert.Equal(t, "ip:203.0.113.5", l.bucketKey(req, model.ScopeIP))
	})

	t.Run("user scope keys by authenticated user id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(tester.ContextWithUser("user-123", "u", "u@example.com", uuid.New().String()))
		assert.Equal(t, "user:user-123", l.bucketKey(req, model.ScopeUser))
	})

	t.Run("user scope without auth context falls back to ip", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "203.0.113.5:1234"
		assert.Equal(t, "ip:203.0.113.5", l.bucketKey(req, model.ScopeUser))
	})
}

package middleware

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/all-in-one/internal/auth"
	"github.com/all-in-one/internal/config"
	httpHelper "github.com/all-in-one/internal/http"
	"github.com/all-in-one/internal/logging"
	"github.com/all-in-one/internal/ratelimit"
	"github.com/all-in-one/internal/ratelimit/model"
	"github.com/gorilla/mux"
)

// RuleProvider is the subset of the ratelimit service's rule cache the
// middleware needs. Declared here (not imported from the service package)
// so the service package can satisfy it structurally — the service package
// imports middleware to obtain the mux.MiddlewareFunc it wires into the
// server, and this avoids the reverse import edge that would create a
// cycle (docs/adr/RATE_LIMITING_ADR.md Appendix A.4).
type RuleProvider interface {
	Effective(key string) (model.EffectiveRule, bool)
}

// CounterStore is the subset of the ratelimit counter repository the
// middleware needs for daily_quota targets. Declared here for the same
// import-cycle-avoidance reason as RuleProvider.
type CounterStore interface {
	IncrAndGet(ctx context.Context, targetKey, bucketKey, day string) (int, error)
}

// Limiter is the hybrid rate limit enforcement middleware (ADR-002):
// throttle targets are counted in an in-memory fixed-window store,
// daily_quota targets in the DB-backed CounterStore. It resolves which
// target(s) apply to a request from its matched mux route (ADR-004).
type Limiter struct {
	rules    RuleProvider
	counters CounterStore
	mem      *memStore
	config   config.Config
	metrics  *limiterMetrics
}

func NewLimiter(rules RuleProvider, counters CounterStore, config config.Config) *Limiter {
	return &Limiter{
		rules:    rules,
		counters: counters,
		mem:      newMemStore(config.RateLimit.CleanupInterval),
		config:   config,
		metrics:  newLimiterMetrics(),
	}
}

// Stop releases the in-memory throttle store's background cleanup
// goroutine. Call once the middleware is no longer in use (e.g. server
// shutdown).
func (l *Limiter) Stop() {
	l.mem.Stop()
}

// Middleware returns the mux.MiddlewareFunc to attach to any subrouter that
// carries rate-limited routes (ADR-004). It's a no-op unless the master
// switch is on and the matched route has at least one bound target.
func (l *Limiter) Middleware() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !l.config.RateLimit.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			route := mux.CurrentRoute(r)
			if route == nil {
				next.ServeHTTP(w, r)
				return
			}
			tmpl, err := route.GetPathTemplate()
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			targets := ratelimit.RouteBindings(r.Method, tmpl)
			if len(targets) == 0 {
				next.ServeHTTP(w, r)
				return
			}

			// Throttles are evaluated before quotas so an in-memory check
			// sheds a flood before any DB counter write happens (ADR-002).
			for _, def := range orderThrottlesFirst(targets) {
				if !l.enforce(w, r, def) {
					return // 429 already written
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// enforce evaluates a single target against the request. It returns false
// only when it has written a 429 rejection; every other outcome (allowed,
// disabled target, cache miss, or a counter-store error) returns true so
// the caller proceeds — a defect here fails open, never closed (ADR-007).
func (l *Limiter) enforce(w http.ResponseWriter, r *http.Request, def ratelimit.TargetDef) bool {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	rule, ok := l.rules.Effective(def.Key)
	if !ok || !rule.Enabled {
		return true
	}

	bucketKey := l.bucketKey(r, rule.Scope)

	switch rule.Kind {
	case model.KindThrottle:
		allowed, retryAfter := l.mem.allow(bucketKey, rule.LimitCount, rule.Window, time.Now())
		if !allowed {
			l.reject(ctx, w, def, rule, retryAfter)
			return false
		}
	case model.KindDailyQuota:
		count, err := l.counters.IncrAndGet(ctx, def.Key, bucketKey, l.today())
		if err != nil {
			log.Error().Err(err).Str("target", def.Key).Msg("ratelimit: counter store error, failing open")
			l.metrics.errors.Add(ctx, 1, otelAttr("target", def.Key))
			return true
		}
		if count > rule.LimitCount {
			l.reject(ctx, w, def, rule, l.retryAfterUntilMidnight())
			return false
		}
	}

	return true
}

func (l *Limiter) reject(ctx context.Context, w http.ResponseWriter, def ratelimit.TargetDef, rule model.EffectiveRule, retryAfter time.Duration) {
	l.metrics.rejected.Add(ctx, 1,
		otelAttr("target", def.Key),
		otelAttr("scope", string(rule.Scope)),
		otelAttr("kind", string(rule.Kind)),
	)
	w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())+1))
	httpHelper.SendError(w, "rate limit exceeded", http.StatusTooManyRequests)
}

// bucketKey derives the counting key for a rule's Scope (ADR-005). A
// user-scoped rule falls back to IP if no authenticated user is in the
// request context — this shouldn't happen given the middleware only runs
// after JWTAuth on gated subrouters (ADR-004), but a defect here must not
// silently collapse every unauthenticated request into one shared bucket.
func (l *Limiter) bucketKey(r *http.Request, scope model.Scope) string {
	switch scope {
	case model.ScopeUser:
		if claims, ok := auth.GetUserFromContext(r.Context()); ok && claims.UserID != "" {
			return "user:" + claims.UserID
		}
		return "ip:" + clientIP(r, l.config)
	case model.ScopeGlobal:
		return "global"
	default: // model.ScopeIP
		return "ip:" + clientIP(r, l.config)
	}
}

// location resolves the configured ratelimit timezone, falling back to UTC
// on an empty or invalid value (ADR-006).
func (l *Limiter) location() *time.Location {
	if l.config.RateLimit.Timezone == "" {
		return time.UTC
	}
	loc, err := time.LoadLocation(l.config.RateLimit.Timezone)
	if err != nil {
		return time.UTC
	}
	return loc
}

// today returns the current calendar day (in the configured timezone) as a
// 'YYYY-MM-DD' string — the daily-quota bucket identity (ADR-006).
func (l *Limiter) today() string {
	return time.Now().In(l.location()).Format("2006-01-02")
}

// retryAfterUntilMidnight is how long a caller should wait after exhausting
// a daily quota — the bucket resets at the next calendar-day boundary.
func (l *Limiter) retryAfterUntilMidnight() time.Duration {
	now := time.Now().In(l.location())
	midnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
	return midnight.Sub(now)
}

// orderThrottlesFirst returns defs with every throttle-kind target ahead of
// every daily_quota-kind target, preserving relative order within each
// group (ADR-002: shed a flood in-memory before any DB counter write).
func orderThrottlesFirst(defs []ratelimit.TargetDef) []ratelimit.TargetDef {
	ordered := make([]ratelimit.TargetDef, 0, len(defs))
	for _, d := range defs {
		if d.Kind == model.KindThrottle {
			ordered = append(ordered, d)
		}
	}
	for _, d := range defs {
		if d.Kind != model.KindThrottle {
			ordered = append(ordered, d)
		}
	}
	return ordered
}

// clientIP resolves the request's client IP. When cfg.RateLimit.TrustProxyHeaders
// is true, the left-most X-Forwarded-For entry (falling back to X-Real-IP)
// is trusted; otherwise r.RemoteAddr is used. Trusting these headers
// unconditionally would be spoofable, so it's an explicit opt-in
// (ADR-009).
func clientIP(r *http.Request, cfg config.Config) string {
	if cfg.RateLimit.TrustProxyHeaders {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			first := strings.SplitN(xff, ",", 2)[0]
			return strings.TrimSpace(first)
		}
		if xrip := r.Header.Get("X-Real-IP"); xrip != "" {
			return strings.TrimSpace(xrip)
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

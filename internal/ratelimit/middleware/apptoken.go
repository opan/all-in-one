package middleware

import (
	"context"
	"net/http"

	httpHelper "github.com/all-in-one/internal/http"
	"github.com/all-in-one/internal/logging"
	"github.com/all-in-one/internal/observability"
	"github.com/all-in-one/internal/ratelimit/model"
	"github.com/gorilla/mux"
	"go.opentelemetry.io/otel/metric"
)

// TokenVerifier resolves a plaintext app token to its row. The ratelimit
// service satisfies it structurally (VerifyToken); declared here so the
// middleware package doesn't import the service package (which imports this
// one — the same cycle-avoidance as RuleProvider).
type TokenVerifier interface {
	VerifyToken(ctx context.Context, plaintext string) (model.AppToken, error)
}

type contextKey string

const appTokenContextKey contextKey = "ratelimit.app_token"

// AppTokenFromContext returns the verified app token attached by
// AppTokenAuth.Middleware, if any.
func AppTokenFromContext(ctx context.Context) (model.AppToken, bool) {
	tok, ok := ctx.Value(appTokenContextKey).(model.AppToken)
	return tok, ok
}

// AppTokenAuth authenticates external callers of the rate-limit check API by
// their X-API-Key header, placing the verified token in the request context.
type AppTokenAuth struct {
	verifier TokenVerifier
	metrics  *appTokenMetrics
}

func NewAppTokenAuth(verifier TokenVerifier) *AppTokenAuth {
	return &AppTokenAuth{verifier: verifier, metrics: newAppTokenMetrics()}
}

// Middleware returns a mux.MiddlewareFunc that 401s any request without a
// valid, non-revoked app token. It never logs token material — not even
// truncated — on the failure path; token_prefix is logged only on success.
func (m *AppTokenAuth) Middleware() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			log := logging.GetLoggerFromContext(ctx)

			key := r.Header.Get("X-API-Key")
			if key == "" {
				log.Warn().Str("reason", "missing").Msg("ratelimit: app token auth failed")
				m.fail(ctx, w, "missing")
				return
			}

			tok, err := m.verifier.VerifyToken(ctx, key)
			if err != nil {
				// A revoked token is excluded at the SQL level, so it surfaces
				// here as "unknown" too — we deliberately do not distinguish
				// them to a caller. Never log the token, even truncated.
				log.Warn().Str("reason", "unknown").Msg("ratelimit: app token auth failed")
				m.fail(ctx, w, "unknown")
				return
			}

			log.Info().Str("token_prefix", tok.TokenPrefix).Str("app", tok.App).
				Msg("ratelimit: app token authenticated")
			ctx = context.WithValue(ctx, appTokenContextKey, tok)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func (m *AppTokenAuth) fail(ctx context.Context, w http.ResponseWriter, reason string) {
	m.metrics.authFailures.Add(ctx, 1, otelAttr("reason", reason))
	httpHelper.SendError(w, "unauthorized", http.StatusUnauthorized)
}

type appTokenMetrics struct {
	authFailures metric.Int64Counter
}

func newAppTokenMetrics() *appTokenMetrics {
	m := observability.Meter("ratelimit")
	f, _ := m.Int64Counter("aio.ratelimit.token_auth_failures",
		metric.WithDescription("App token auth failures on the external rate-limit API"),
	)
	return &appTokenMetrics{authFailures: f}
}

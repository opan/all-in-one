package middleware

import (
	"context"
	"net/http"

	"github.com/all-in-one/internal/auth"
	"github.com/all-in-one/internal/config"
	httpHelper "github.com/all-in-one/internal/http"
	"github.com/all-in-one/internal/logging"
	"github.com/all-in-one/internal/rbac/service"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Authz enforces RBAC feature/admin gating on top of an already-authenticated
// request (it must run after JWTAuth, which populates auth.UserClaims in the
// request context). See docs/adr/ACCESS_MANAGEMENT_ADR.md ADR-002/ADR-006.
type Authz struct {
	resolver *service.Resolver
	config   config.Config
	metrics  *authzMetrics
}

func NewAuthz(resolver *service.Resolver, config config.Config) *Authz {
	return &Authz{
		resolver: resolver,
		config:   config,
		metrics:  newAuthzMetrics(),
	}
}

// RequireFeature returns middleware that only allows the request through if
// the authenticated user can access featureKey (admin bypass > user override
// > group grant > default-deny).
func (a *Authz) RequireFeature(featureKey string) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			log := logging.GetLoggerFromContext(ctx)

			userID, isDirectAuth, ok := a.identify(w, r)
			if !ok {
				return
			}
			if isDirectAuth {
				if a.config.RBAC.DirectAuthIsAdmin {
					next.ServeHTTP(w, r)
					return
				}
				a.deny(w, ctx, featureKey, "direct_auth_not_admin")
				return
			}

			allowed, err := a.resolver.CanAccess(ctx, userID, featureKey)
			if err != nil {
				log.Error().Err(err).Str("feature", featureKey).Msg("rbac: failed to resolve feature access")
				httpHelper.SendError(w, "internal server error", http.StatusInternalServerError)
				return
			}
			if !allowed {
				a.deny(w, ctx, featureKey, "feature_denied")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireAdmin returns middleware that only allows the request through if
// the authenticated user belongs to the admin group.
func (a *Authz) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		log := logging.GetLoggerFromContext(ctx)

		userID, isDirectAuth, ok := a.identify(w, r)
		if !ok {
			return
		}
		if isDirectAuth {
			if a.config.RBAC.DirectAuthIsAdmin {
				next.ServeHTTP(w, r)
				return
			}
			a.deny(w, ctx, "access-management", "direct_auth_not_admin")
			return
		}

		isAdmin, err := a.resolver.IsAdmin(ctx, userID)
		if err != nil {
			log.Error().Err(err).Msg("rbac: failed to resolve admin status")
			httpHelper.SendError(w, "internal server error", http.StatusInternalServerError)
			return
		}
		if !isAdmin {
			a.deny(w, ctx, "access-management", "not_admin")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// identify extracts the authenticated user from the request context. It
// returns ok=false (having already written a response) if no user claims are
// present or the claims carry a malformed user ID. isDirectAuth reports the
// x-direct-auth-username dev bypass (SessionID=="direct-auth"), which
// fabricates a non-UUID UserID with no corresponding DB row — callers must
// branch on it before treating userID as meaningful.
func (a *Authz) identify(w http.ResponseWriter, r *http.Request) (userID uuid.UUID, isDirectAuth bool, ok bool) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	claims, found := auth.GetUserFromContext(ctx)
	if !found {
		log.Error().Msg("rbac: missing user claims in context")
		httpHelper.SendUnauthorized(w, "Unauthorized")
		return uuid.UUID{}, false, false
	}

	if claims.SessionID == "direct-auth" {
		return uuid.UUID{}, true, true
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		log.Error().Err(err).Str("user_id", claims.UserID).Msg("rbac: invalid user id in context")
		httpHelper.SendUnauthorized(w, "Unauthorized")
		return uuid.UUID{}, false, false
	}

	return userID, false, true
}

func (a *Authz) deny(w http.ResponseWriter, ctx context.Context, featureKey, reason string) {
	a.metrics.accessDenied.Add(ctx, 1, metric.WithAttributes(
		attribute.String("feature", featureKey),
		attribute.String("reason", reason),
	))
	httpHelper.SendError(w, "forbidden", http.StatusForbidden)
}

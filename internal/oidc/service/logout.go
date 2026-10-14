package service

import (
	"context"
	"net/http"

	"github.com/all-in-one/internal/observability"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// SessionStore is the subset of the authnz session repository logout needs.
type SessionStore interface {
	Delete(ctx context.Context, id uuid.UUID) error
}

const endSessionPath = EndpointPrefix + "end_session"

type pendingLogoutKey struct{}

// pendingLogout carries the HTTP request and response into the storage call
// that ends the session, since the library hands storage only a context.
type pendingLogout struct {
	w       http.ResponseWriter
	r       *http.Request
	cleared bool
}

// withAioLogout lets RP-initiated logout also end the user's aio session: the
// library validates the request and redirects back to the app, but it can't
// touch aio's own cookie and session row.
func (s *Service) withAioLogout(next http.Handler) http.Handler {
	logouts, _ := observability.Meter("oidc").Int64Counter("aio.oidc.logout",
		metric.WithDescription("RP-initiated logouts received from apps"))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != endSessionPath {
			next.ServeHTTP(w, r)
			return
		}
		pl := &pendingLogout{w: w, r: r}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), pendingLogoutKey{}, pl)))
		logouts.Add(r.Context(), 1, metric.WithAttributes(attribute.Bool("aio_session_cleared", pl.cleared)))
	})
}

// endAioSession runs once the library has accepted an end_session request.
// hintUserID is the subject of its id_token_hint ("" without one). aio's
// session is ended only when it belongs to that same user: a bare link, or a
// link carrying someone else's ID token, must not log users out (logout CSRF).
func (s *Service) endAioSession(ctx context.Context, hintUserID string) {
	pl, ok := ctx.Value(pendingLogoutKey{}).(*pendingLogout)
	if !ok || hintUserID == "" {
		return
	}
	sess, ok := s.aioSession(pl.r, false)
	if !ok {
		return
	}
	if sess.userID != hintUserID {
		s.log.Warn().Msg("oidc: logout hint is for a different user than the aio session, keeping aio session")
		return
	}
	if err := s.sessions.Delete(ctx, sess.id); err != nil {
		s.log.Error().Err(err).Str("session_id", sess.id.String()).Msg("oidc: failed to delete aio session on logout")
	}
	for _, name := range []string{"access_token", "refresh_token"} {
		http.SetCookie(pl.w, &http.Cookie{Name: name, Value: "", Path: "/", MaxAge: -1, HttpOnly: true,
			Secure: s.config.Auth.SecureCookie, SameSite: http.SameSiteLaxMode})
	}
	pl.cleared = true
}

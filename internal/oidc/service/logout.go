package service

import (
	"context"
	"errors"
	"net/http"

	"github.com/all-in-one/internal/observability"
	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	zoidc "github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// SessionStore is the subset of the authnz session repository logout needs.
type SessionStore interface {
	Delete(ctx context.Context, id uuid.UUID) error
}

const endSessionPath = EndpointPrefix + "end_session"

// withAioLogout wraps the provider so RP-initiated logout also ends the
// user's aio session: the library validates the request and redirects back to
// the app, but it can't touch aio's own cookie and session row.
func (s *Service) withAioLogout(next http.Handler) http.Handler {
	logouts, _ := observability.Meter("oidc").Int64Counter("aio.oidc.logout",
		metric.WithDescription("RP-initiated logouts received from apps"))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == endSessionPath {
			cleared := s.endAioSession(w, r)
			logouts.Add(r.Context(), 1, metric.WithAttributes(attribute.Bool("aio_session_cleared", cleared)))
		}
		next.ServeHTTP(w, r)
	})
}

// endAioSession clears aio's session only when the request carries an ID
// token aio issued (id_token_hint, expired is fine): without it, any page
// could log users out of aio just by linking here (logout CSRF).
func (s *Service) endAioSession(w http.ResponseWriter, r *http.Request) bool {
	ctx := r.Context()
	hint := r.FormValue("id_token_hint")
	if hint == "" {
		return false
	}
	// The provider sets the issuer on the context inside its own handler; this
	// runs before it, so set it here for the verifier.
	vctx := op.ContextWithIssuer(ctx, s.issuer)
	_, err := op.VerifyIDTokenHint[*zoidc.IDTokenClaims](vctx, hint, s.provider.IDTokenHintVerifier(vctx))
	var expired op.IDTokenHintExpiredError
	if err != nil && !errors.As(err, &expired) {
		s.log.Warn().Err(err).Msg("oidc: logout with an invalid id_token_hint, keeping aio session")
		return false
	}

	if sid, ok := s.aioSessionID(r); ok {
		if err := s.sessions.Delete(ctx, sid); err != nil {
			s.log.Error().Err(err).Str("session_id", sid.String()).Msg("oidc: failed to delete aio session on logout")
		}
	}
	for _, name := range []string{"access_token", "refresh_token"} {
		http.SetCookie(w, &http.Cookie{Name: name, Value: "", Path: "/", MaxAge: -1, HttpOnly: true,
			Secure: s.config.Auth.SecureCookie, SameSite: http.SameSiteLaxMode})
	}
	return true
}

// aioSessionID reads the session id from aio's access token cookie. Expiry is
// ignored (the session row may outlive the short-lived access token), but the
// signature is still checked.
func (s *Service) aioSessionID(r *http.Request) (uuid.UUID, bool) {
	c, err := r.Cookie("access_token")
	if err != nil {
		return uuid.Nil, false
	}
	tok, err := jwt.Parse(c.Value, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(s.config.Auth.JWTSecret), nil
	}, jwt.WithoutClaimsValidation())
	if err != nil {
		return uuid.Nil, false
	}
	sub, _ := tok.Claims.GetSubject()
	sid, err := uuid.Parse(sub)
	return sid, err == nil
}

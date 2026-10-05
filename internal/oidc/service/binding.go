package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/all-in-one/internal/oidc"
	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	zoidc "github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"
)

// Each auth request is tied to the browser that started it. Otherwise the id
// in an /oauth/login link is enough to steal a login: an attacker starts one,
// a victim's aio session completes it with no click, and the attacker follows
// the callback in their own browser to get an app session as the victim.
//   - completing a request requires the browser cookie set at authorize;
//   - issuing the code requires that browser to hold aio's session for the
//     user who completed it.
const (
	authorizePath         = EndpointPrefix + "authorize"
	authorizeCallbackPath = authorizePath + "/callback"
	// Covers the callback (/api/v1/oauth2/) and the complete API (/api/v1/oidc/).
	browserCookiePath = "/api/v1/"
)

type browserIDKey struct{}

func (s *Service) withBrowserBinding(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case authorizePath:
			id := s.browserID(w, r)
			r = r.WithContext(context.WithValue(r.Context(), browserIDKey{}, id))
		case authorizeCallbackPath:
			if !s.callbackAllowed(w, r) {
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// browserID keeps an existing id so logins started in several tabs at once
// all stay valid, and extends the cookie to cover the new request.
func (s *Service) browserID(w http.ResponseWriter, r *http.Request) string {
	id := oidc.BrowserIDFromRequest(r)
	if id == "" {
		b := make([]byte, oidc.BrowserIDBytes)
		_, _ = rand.Read(b) // never fails since Go 1.24
		id = base64.RawURLEncoding.EncodeToString(b)
	}
	http.SetCookie(w, &http.Cookie{
		Name: oidc.BrowserCookie, Value: id, Path: browserCookiePath,
		MaxAge:   int(s.config.Auth.OIDC.AuthRequestTTL / time.Second),
		HttpOnly: true, Secure: s.config.Auth.SecureCookie, SameSite: http.SameSiteLaxMode,
	})
	return id
}

func browserIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(browserIDKey{}).(string)
	return id
}

func hashBrowserID(id string) string {
	if id == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(id))
	return hex.EncodeToString(sum[:])
}

func (a *authRequest) startedBy(browserID string) bool {
	if a.BrowserHash == "" || browserID == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a.BrowserHash), []byte(hashBrowserID(browserID))) == 1
}

// callbackAllowed sends a refusal back to the app as access_denied. Unknown,
// expired or unfinished requests are left for the library to report.
func (s *Service) callbackAllowed(w http.ResponseWriter, r *http.Request) bool {
	req, ok := s.mem.request(r.FormValue("id"))
	if !ok || !req.done {
		return true
	}
	sameBrowser := req.startedBy(oidc.BrowserIDFromRequest(r))
	sess, hasSession := s.aioSession(r, true)
	if sameBrowser && hasSession && sess.userID == req.UserID {
		return true
	}
	s.log.Warn().Str("client_id", req.ClientID).Bool("same_browser", sameBrowser).Bool("has_session", hasSession).
		Msg("oidc: refused to issue a code to a browser that did not finish the login")
	ctx := op.ContextWithIssuer(r.Context(), s.issuer)
	op.AuthRequestError(w, r.WithContext(ctx), req,
		zoidc.ErrAccessDenied().WithDescription("the login was not finished in this browser"), s.provider)
	return false
}

type aioSession struct {
	id     uuid.UUID
	userID string
}

// aioSession reads aio's session from its access token cookie. Logout passes
// checkExpiry=false since the session row outlives the short-lived token.
func (s *Service) aioSession(r *http.Request, checkExpiry bool) (aioSession, bool) {
	c, err := r.Cookie("access_token")
	if err != nil {
		return aioSession{}, false
	}
	opts := []jwt.ParserOption{jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg(),
		jwt.SigningMethodHS384.Alg(), jwt.SigningMethodHS512.Alg()})}
	if !checkExpiry {
		opts = append(opts, jwt.WithoutClaimsValidation())
	}
	claims := jwt.MapClaims{}
	if _, err := jwt.ParseWithClaims(c.Value, claims, func(*jwt.Token) (any, error) {
		return []byte(s.config.Auth.JWTSecret), nil
	}, opts...); err != nil {
		return aioSession{}, false
	}
	if t, _ := claims["type"].(string); t == "2fa_challenge" {
		return aioSession{}, false
	}
	sub, _ := claims.GetSubject()
	sid, err := uuid.Parse(sub)
	if err != nil {
		return aioSession{}, false
	}
	userID, _ := claims["user_id"].(string)
	return aioSession{id: sid, userID: userID}, true
}

package service

import (
	"net/http"
	"net/url"
	"testing"

	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// aioCookie mints aio's own session cookie the way authnz does (HS256 over
// the JWT secret, sub = session id).
func aioCookie(t *testing.T, sid uuid.UUID) *http.Cookie {
	t.Helper()
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": sid.String()}).SignedString([]byte("test-secret"))
	require.NoError(t, err)
	return &http.Cookie{Name: "access_token", Value: tok}
}

func (e *flowEnv) idToken(t *testing.T) string {
	t.Helper()
	_, tokens := e.exchange(t, e.finishLogin(t, e.startLogin(t, nil)), e.secret)
	return tokens["id_token"].(string)
}

func (e *flowEnv) logout(t *testing.T, params url.Values, cookie *http.Cookie) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, e.srv.URL+EndpointPrefix+"end_session?"+params.Encode(), nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	res, err := noRedirect.Do(req)
	require.NoError(t, err)
	return res
}

func clearsCookie(res *http.Response, name string) bool {
	for _, c := range res.Cookies() {
		if c.Name == name && c.MaxAge < 0 {
			return true
		}
	}
	return false
}

func TestLogout_WithIDTokenHintEndsAioSessionAndReturnsToApp(t *testing.T) {
	e := newFlowEnv(t)
	sid := uuid.New()
	res := e.logout(t, url.Values{
		"id_token_hint":            {e.idToken(t)},
		"post_logout_redirect_uri": {testPostLogout},
		"state":                    {"bye"},
	}, aioCookie(t, sid))

	require.Equal(t, http.StatusFound, res.StatusCode)
	loc, _ := url.Parse(res.Header.Get("Location"))
	assert.Equal(t, testPostLogout, loc.Scheme+"://"+loc.Host+loc.Path, "back to the app's registered post-logout URL")
	assert.Equal(t, "bye", loc.Query().Get("state"))
	assert.Equal(t, []uuid.UUID{sid}, e.sessions.deleted, "aio's session row is deleted")
	assert.True(t, clearsCookie(res, "access_token"))
	assert.True(t, clearsCookie(res, "refresh_token"))
}

func TestLogout_WithoutValidHintKeepsAioSession(t *testing.T) {
	e := newFlowEnv(t)
	cases := []struct {
		name string
		hint string
	}{
		{"no hint", ""},
		{"forged hint", "eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJ4In0.c2lnbmF0dXJl"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			params := url.Values{"post_logout_redirect_uri": {testPostLogout}, "client_id": {"cashflow"}}
			if c.hint != "" {
				params.Set("id_token_hint", c.hint)
			}
			res := e.logout(t, params, aioCookie(t, uuid.New()))
			assert.Empty(t, e.sessions.deleted, "a bare link must not log users out of aio (logout CSRF)")
			assert.False(t, clearsCookie(res, "access_token"))
		})
	}
}

func TestLogout_UnregisteredPostLogoutURIIsNotFollowed(t *testing.T) {
	e := newFlowEnv(t)
	res := e.logout(t, url.Values{
		"id_token_hint":            {e.idToken(t)},
		"post_logout_redirect_uri": {"https://evil.example.com/"},
	}, nil)
	assert.NotContains(t, res.Header.Get("Location"), "evil.example.com")
}

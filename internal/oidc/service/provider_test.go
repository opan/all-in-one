package service

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	authnzModel "github.com/all-in-one/internal/authnz/model"
	"github.com/all-in-one/internal/config"
	"github.com/all-in-one/internal/oidc"
	"github.com/all-in-one/internal/oidc/model"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zitadel/oidc/v3/pkg/client/rp"
	zoidc "github.com/zitadel/oidc/v3/pkg/oidc"
)

const (
	testRedirect   = "http://localhost:9999/auth/callback"
	testPostLogout = "http://localhost:9999/"
)

type fakeUsers map[uuid.UUID]authnzModel.User

func (f fakeUsers) Find(_ context.Context, id uuid.UUID) (authnzModel.User, error) {
	u, ok := f[id]
	if !ok {
		return authnzModel.User{}, fmt.Errorf("user %s not found", id)
	}
	return u, nil
}

type flowEnv struct {
	svc      *Service
	srv      *httptest.Server
	secret   string
	user     authnzModel.User
	users    fakeUsers
	sessions *fakeSessions
}

type fakeSessions struct{ deleted []uuid.UUID }

func (f *fakeSessions) Delete(_ context.Context, id uuid.UUID) error {
	f.deleted = append(f.deleted, id)
	return nil
}

// newFlowEnv starts a real provider behind an httptest server, with a fresh
// SQLite DB, one registered client and one user.
func newFlowEnv(t *testing.T) *flowEnv {
	t.Helper()
	db, err := sqlx.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	schema, err := os.ReadFile("../../../db/migrations/sqlite3/11_add_oidc_clients_and_keys.up.sql")
	require.NoError(t, err)
	_, err = db.Exec(string(schema))
	require.NoError(t, err)

	var handler http.Handler
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { handler.ServeHTTP(w, r) }))
	t.Cleanup(srv.Close)

	cfg := config.Config{}
	cfg.Storage.Type = "sqlite"
	cfg.Auth.JWTSecret = "test-secret"
	cfg.Auth.TOTPEncryptionKey = testEncKey
	cfg.Auth.OIDC = config.OIDCConfig{Enabled: true, Issuer: srv.URL, AuthRequestTTL: time.Minute,
		AccessTokenLifetime: time.Minute, IDTokenLifetime: time.Minute}

	user := authnzModel.User{ID: uuid.New(), Username: "budi", Name: "Budi"}
	users := fakeUsers{user.ID: user}
	sessions := &fakeSessions{}
	svc, err := NewService(context.Background(), db, cfg, zerolog.Nop(), users, sessions)
	require.NoError(t, err)

	r := mux.NewRouter()
	r.Handle(DiscoveryPath, svc.ProviderHandler())
	r.PathPrefix(EndpointPrefix).Handler(svc.ProviderHandler())
	handler = r

	_, secret, err := svc.CreateClient(context.Background(), model.CreateClientInput{
		ID: "cashflow", Name: "Cashflow", RedirectURIs: []string{testRedirect},
		PostLogoutRedirectURIs: []string{testPostLogout},
	}, "admin")
	require.NoError(t, err)
	return &flowEnv{svc: svc, srv: srv, secret: secret, user: user, users: users, sessions: sessions}
}

var noRedirect = &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

func pkce() (verifier, challenge string) {
	verifier = "a-sufficiently-long-pkce-verifier-string-0123456789"
	sum := sha256.Sum256([]byte(verifier))
	return verifier, base64.RawURLEncoding.EncodeToString(sum[:])
}

// startLogin hits the authorize endpoint like an app would and returns the
// auth request id from aio's login page redirect.
func (e *flowEnv) startLogin(t *testing.T, extra url.Values) string {
	t.Helper()
	_, challenge := pkce()
	q := url.Values{
		"client_id": {"cashflow"}, "redirect_uri": {testRedirect}, "response_type": {"code"},
		"scope": {"openid profile email"}, "state": {"st4te"}, "nonce": {"n0nce"},
		"code_challenge": {challenge}, "code_challenge_method": {"S256"},
	}
	for k, v := range extra {
		q[k] = v
	}
	res, err := noRedirect.Get(e.srv.URL + EndpointPrefix + "authorize?" + q.Encode())
	require.NoError(t, err)
	require.Equal(t, http.StatusFound, res.StatusCode)
	loc, err := url.Parse(res.Header.Get("Location"))
	require.NoError(t, err)
	require.Equal(t, LoginPath, loc.Path, "unauthenticated users are sent to aio's login page")
	return loc.Query().Get("authRequestID")
}

// finishLogin completes the auth request and follows the callback, returning
// the code delivered to the app's redirect URI.
func (e *flowEnv) finishLogin(t *testing.T, id string) string {
	t.Helper()
	next, err := e.svc.CompleteAuthRequest(context.Background(), id, e.user.ID.String())
	require.NoError(t, err)
	res, err := noRedirect.Get(next)
	require.NoError(t, err)
	require.Equal(t, http.StatusFound, res.StatusCode)
	loc, err := url.Parse(res.Header.Get("Location"))
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(loc.String(), testRedirect), "redirected back to the app")
	assert.Equal(t, "st4te", loc.Query().Get("state"))
	return loc.Query().Get("code")
}

func (e *flowEnv) exchange(t *testing.T, code, secret string) (int, map[string]any) {
	t.Helper()
	verifier, _ := pkce()
	form := url.Values{"grant_type": {"authorization_code"}, "code": {code},
		"redirect_uri": {testRedirect}, "code_verifier": {verifier}}
	req, _ := http.NewRequest(http.MethodPost, e.srv.URL+EndpointPrefix+"token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("cashflow", secret)
	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer res.Body.Close()
	body := map[string]any{}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&body))
	return res.StatusCode, body
}

func TestProvider_DiscoveryAdvertisesAioEndpoints(t *testing.T) {
	e := newFlowEnv(t)
	res, err := http.Get(e.srv.URL + DiscoveryPath)
	require.NoError(t, err)
	var d map[string]any
	require.NoError(t, json.NewDecoder(res.Body).Decode(&d))
	assert.Equal(t, e.srv.URL, d["issuer"])
	assert.Equal(t, e.srv.URL+"/api/v1/oauth2/authorize", d["authorization_endpoint"])
	assert.Equal(t, e.srv.URL+"/api/v1/oauth2/token", d["token_endpoint"])
	assert.Equal(t, e.srv.URL+"/api/v1/oauth2/keys", d["jwks_uri"])
	assert.Equal(t, e.srv.URL+"/api/v1/oauth2/end_session", d["end_session_endpoint"])
	assert.NotContains(t, d["grant_types_supported"], "refresh_token")
}

func TestProvider_FullAuthorizationCodeFlow(t *testing.T) {
	e := newFlowEnv(t)

	id := e.startLogin(t, nil)
	info, err := e.svc.AuthRequestInfo(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, "Cashflow", info.ClientName, "the login page can say which app is asking")
	assert.False(t, info.Signup)

	code := e.finishLogin(t, id)
	status, tokens := e.exchange(t, code, e.secret)
	require.Equal(t, http.StatusOK, status, "token response: %v", tokens)

	// Verify the ID token exactly as an app would: signature against aio's
	// published keys, issuer, audience, expiry and nonce.
	ctx := context.Background()
	keys := rp.NewRemoteKeySet(http.DefaultClient, e.srv.URL+EndpointPrefix+"keys")
	verifier := rp.NewIDTokenVerifier(e.srv.URL, "cashflow", keys,
		rp.WithNonce(func(context.Context) string { return "n0nce" }))
	claims, err := rp.VerifyIDToken[*zoidc.IDTokenClaims](ctx, tokens["id_token"].(string), verifier)
	require.NoError(t, err)
	assert.Equal(t, e.user.ID.String(), claims.Subject)
	assert.Equal(t, "budi", claims.PreferredUsername, "profile claims are in the ID token itself")
	assert.Equal(t, "n0nce", claims.Nonce)
	assert.Empty(t, claims.Email, "users without an email get no email claim")

	// The access token works at the userinfo endpoint.
	req, _ := http.NewRequest(http.MethodGet, e.srv.URL+EndpointPrefix+"userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+tokens["access_token"].(string))
	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	var ui map[string]any
	require.NoError(t, json.NewDecoder(res.Body).Decode(&ui))
	assert.Equal(t, "budi", ui["preferred_username"])
}

func TestProvider_CodeIsSingleUse(t *testing.T) {
	e := newFlowEnv(t)
	code := e.finishLogin(t, e.startLogin(t, nil))

	status, _ := e.exchange(t, code, e.secret)
	require.Equal(t, http.StatusOK, status)
	status, body := e.exchange(t, code, e.secret)
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Equal(t, "invalid_grant", body["error"])
}

func TestProvider_WrongClientSecretRejected(t *testing.T) {
	e := newFlowEnv(t)
	code := e.finishLogin(t, e.startLogin(t, nil))
	status, body := e.exchange(t, code, "not-the-secret")
	assert.Equal(t, http.StatusUnauthorized, status)
	assert.Equal(t, "invalid_client", body["error"])
}

func TestProvider_UnregisteredRedirectURIRejected(t *testing.T) {
	e := newFlowEnv(t)
	_, challenge := pkce()
	q := url.Values{"client_id": {"cashflow"}, "redirect_uri": {"http://localhost:9999/evil"},
		"response_type": {"code"}, "scope": {"openid"}, "code_challenge": {challenge}, "code_challenge_method": {"S256"}}
	res, err := noRedirect.Get(e.srv.URL + EndpointPrefix + "authorize?" + q.Encode())
	require.NoError(t, err)
	if res.StatusCode == http.StatusFound {
		assert.NotContains(t, res.Header.Get("Location"), "localhost:9999/evil", "never redirect to an unregistered URI")
	} else {
		assert.GreaterOrEqual(t, res.StatusCode, 400)
	}
}

func TestProvider_PromptCreateMeansSignup(t *testing.T) {
	e := newFlowEnv(t)
	id := e.startLogin(t, url.Values{"prompt": {"create"}})
	info, err := e.svc.AuthRequestInfo(context.Background(), id)
	require.NoError(t, err)
	assert.True(t, info.Signup)
}

func TestCompleteAuthRequest_Errors(t *testing.T) {
	e := newFlowEnv(t)
	blocked := authnzModel.User{ID: uuid.New(), Username: "blocked", Blocked: true}
	e.users[blocked.ID] = blocked

	_, err := e.svc.CompleteAuthRequest(context.Background(), "no-such-request", e.user.ID.String())
	assert.ErrorIs(t, err, oidc.ErrAuthRequestNotFound)

	_, err = e.svc.CompleteAuthRequest(context.Background(), e.startLogin(t, nil), blocked.ID.String())
	assert.ErrorIs(t, err, oidc.ErrUserBlocked, "a blocked aio user can't log in to other apps")

	id := e.startLogin(t, nil)
	require.NoError(t, e.svc.RevokeClient(context.Background(), "cashflow"))
	_, err = e.svc.CompleteAuthRequest(context.Background(), id, e.user.ID.String())
	assert.ErrorIs(t, err, oidc.ErrClientRevoked, "revoking a client stops logins already in flight")
}

// Discovery lists implicit and jwt-bearer because the library hard-codes them;
// these tests pin down that neither is actually usable by aio's clients.
func TestProvider_ImplicitAndJWTBearerAreRefused(t *testing.T) {
	e := newFlowEnv(t)
	httpsRedirect := "https://app.example.com/auth/callback"
	_, _, err := e.svc.CreateClient(context.Background(), model.CreateClientInput{
		ID: "https-app", Name: "HTTPS app", RedirectURIs: []string{httpsRedirect},
	}, "admin")
	require.NoError(t, err)

	q := url.Values{"client_id": {"https-app"}, "redirect_uri": {httpsRedirect},
		"response_type": {"id_token token"}, "scope": {"openid"}, "nonce": {"n"}}
	res, err := noRedirect.Get(e.srv.URL + EndpointPrefix + "authorize?" + q.Encode())
	require.NoError(t, err)
	loc := res.Header.Get("Location")
	assert.NotContains(t, loc, LoginPath, "an implicit request never reaches the login page")
	assert.NotContains(t, loc, "access_token=", "no tokens are ever issued through the front channel")
	assert.True(t, res.StatusCode >= 400 || strings.Contains(loc, "error="),
		"refused: status %d, location %q", res.StatusCode, loc)

	form := url.Values{"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"}, "assertion": {"x.y.z"}}
	req, _ := http.NewRequest(http.MethodPost, e.srv.URL+EndpointPrefix+"token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	tres, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, tres.StatusCode, 400)
}

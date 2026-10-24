package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/all-in-one/internal/auth"
	"github.com/all-in-one/internal/authnz/middleware"
	"github.com/all-in-one/internal/authnz/model"
	"github.com/all-in-one/internal/authnz/repository"
	"github.com/all-in-one/internal/config"
	"github.com/all-in-one/internal/email"
	"github.com/all-in-one/internal/tester"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recoveryEnv runs the authnz routes against a real SQLite database with all
// migrations applied, with email captured by a Recorder (RFC-003 flows).
type recoveryEnv struct {
	t      *testing.T
	db     *sqlx.DB
	store  repository.Storage
	h      *Handler
	server *httptest.Server
	mail   *email.Recorder
}

func newRecoveryEnv(t *testing.T) *recoveryEnv {
	t.Helper()
	db, err := sqlx.Open("sqlite3", filepath.Join(t.TempDir(), "aio.db")+"?_foreign_keys=on")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	files, err := filepath.Glob("../../../db/migrations/sqlite3/*.up.sql")
	require.NoError(t, err)
	sort.Slice(files, func(i, j int) bool {
		return migrationNumber(files[i]) < migrationNumber(files[j])
	})
	for _, f := range files {
		sqlText, err := os.ReadFile(f)
		require.NoError(t, err)
		_, err = db.Exec(string(sqlText))
		require.NoError(t, err, f)
	}

	cfg := config.Config{
		Storage:  config.StorageConfig{Type: "sqlite"},
		Auth:     config.Auth{JWTSecret: "test-secret", SupportEmail: "admin@example.com"},
		Server:   config.ServerConfig{PublicURL: "https://aio.example.com"},
		Email:    config.EmailConfig{Enabled: true, From: "All-in-one <aio@example.com>"},
		DemoMode: config.DemoModeConfig{Username: "demo"},
	}
	store, err := repository.NewRepo(db, cfg)
	require.NoError(t, err)

	h := NewHandler(store, cfg)
	h.async = func(f func()) { f() }
	rec := &email.Recorder{}
	h.SetMailer(email.NewMailer(rec, "aio.example.com"))

	r := mux.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req.WithContext(tester.WithLogger(req.Context(), zerolog.Nop())))
		})
	})
	api := r.PathPrefix("/api/v1").Subrouter()
	h.RegisterPublicRoutes(api.NewRoute().Subrouter())
	self := api.NewRoute().Subrouter()
	self.Use(middleware.NewJWTMiddleware(cfg, store.SessionRepo()).JWTAuth)
	h.RegisterAuthenticatedRoutes(self)
	admin := api.NewRoute().Subrouter()
	admin.Use(middleware.NewJWTMiddleware(cfg, store.SessionRepo()).JWTAuth)
	h.RegisterAdminRoutes(admin)

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return &recoveryEnv{t: t, db: db, store: store, h: h, server: srv, mail: rec}
}

func migrationNumber(path string) int {
	var n int
	for _, c := range filepath.Base(path) {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// client is a browser with its own cookie jar.
func (e *recoveryEnv) client() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{Jar: jar}
}

type apiResult struct {
	Status  int
	Message string          `json:"message"`
	Error   string          `json:"error"`
	Data    json.RawMessage `json:"data"`
}

func (e *recoveryEnv) do(c *http.Client, method, path string, body any) apiResult {
	e.t.Helper()
	var buf bytes.Buffer
	if body != nil {
		require.NoError(e.t, json.NewEncoder(&buf).Encode(body))
	}
	req, err := http.NewRequest(method, e.server.URL+path, &buf)
	require.NoError(e.t, err)
	req.Header.Set("Content-Type", "application/json")
	res, err := c.Do(req)
	require.NoError(e.t, err)
	defer res.Body.Close()
	out := apiResult{Status: res.StatusCode}
	_ = json.NewDecoder(res.Body).Decode(&out)
	return out
}

func (e *recoveryEnv) addUser(username, password, verifiedEmail string) model.User {
	e.t.Helper()
	hash, err := auth.HashPassword(password)
	require.NoError(e.t, err)
	u := model.User{ID: uuid.New(), Username: username, PasswordHash: hash}
	require.NoError(e.t, e.store.UserRepo().Create(tester.ContextWithLogger(), u))
	if verifiedEmail != "" {
		require.NoError(e.t, e.store.UserRepo().SetVerifiedEmail(tester.ContextWithLogger(), u.ID, verifiedEmail, time.Now()))
	}
	got, err := e.store.UserRepo().Find(tester.ContextWithLogger(), u.ID)
	require.NoError(e.t, err)
	return got
}

func (e *recoveryEnv) login(c *http.Client, username, password string) (apiResult, LoginResponse) {
	e.t.Helper()
	res := e.do(c, http.MethodPost, "/api/v1/sessions", map[string]string{"username": username, "password": password})
	var lr LoginResponse
	if res.Status == http.StatusCreated {
		require.NoError(e.t, json.Unmarshal(res.Data, &lr))
	}
	return res, lr
}

var tempPasswordInEmail = regexp.MustCompile(`(?m)^    ([a-z0-9]{4}-[a-z0-9]{4}-[a-z0-9]{4})$`)

func lastTempPassword(t *testing.T, rec *email.Recorder) string {
	t.Helper()
	msgs := rec.Messages()
	require.NotEmpty(t, msgs)
	m := tempPasswordInEmail.FindStringSubmatch(msgs[len(msgs)-1].Text)
	require.Len(t, m, 2, "no temporary password in: %s", msgs[len(msgs)-1].Text)
	return m[1]
}

func TestRecovery_ForgotPasswordFlow(t *testing.T) {
	e := newRecoveryEnv(t)
	e.addUser("budi", "old-password", "budi@example.com")
	browser := e.client()

	res := e.do(browser, http.MethodPost, "/api/v1/auth/password/forgot", map[string]string{"login": "budi", "locale": "id"})
	require.Equal(t, http.StatusOK, res.Status)
	require.Len(t, e.mail.Messages(), 1)
	msg := e.mail.Messages()[0]
	assert.Equal(t, "budi@example.com", msg.To)
	assert.Equal(t, "Kata sandi sementara kamu", msg.Subject)
	assert.Contains(t, msg.Text, "aio.example.com")
	temp := lastTempPassword(t, e.mail)

	// The old password still works while the temporary one is pending.
	res, lr := e.login(e.client(), "budi", "old-password")
	require.Equal(t, http.StatusCreated, res.Status)
	assert.False(t, lr.PasswordChangeRequired)

	// ...and using it voided the temporary password, so ask again (after the cooldown).
	_, err := e.db.Exec(`UPDATE password_resets SET created_at = ?`, time.Now().Add(-time.Hour).UTC())
	require.NoError(t, err)
	e.do(browser, http.MethodPost, "/api/v1/auth/password/forgot", map[string]string{"login": "BUDI@example.com"})
	require.Len(t, e.mail.Messages(), 2, "a verified email works as the login too")
	newTemp := lastTempPassword(t, e.mail)
	res, _ = e.login(e.client(), "budi", temp)
	assert.Equal(t, http.StatusUnauthorized, res.Status, "the first temporary password was voided")

	// Typed in upper case without dashes still works, and restricts the session.
	res, lr = e.login(browser, "budi", strings.ToUpper(strings.ReplaceAll(newTemp, "-", "")))
	require.Equal(t, http.StatusCreated, res.Status)
	assert.True(t, lr.PasswordChangeRequired)
	assert.False(t, lr.EmailPrompt)

	me := e.do(browser, http.MethodGet, "/api/v1/users/me", nil)
	require.Equal(t, http.StatusOK, me.Status)
	assert.Contains(t, string(me.Data), `"password_change_required":true`)
	blocked := e.do(browser, http.MethodPost, "/api/v1/users/2fa/setup", nil)
	assert.Equal(t, http.StatusForbidden, blocked.Status)
	assert.Equal(t, "password_change_required", blocked.Error)

	res, _ = e.login(e.client(), "budi", newTemp)
	assert.Equal(t, http.StatusUnauthorized, res.Status, "a temporary password works once")

	set := e.do(browser, http.MethodPost, "/api/v1/users/password/set", map[string]string{"password": "brand-new"})
	require.Equal(t, http.StatusOK, set.Status, set.Error)
	assert.Equal(t, "Your password was changed", e.mail.Messages()[len(e.mail.Messages())-1].Subject)
	me = e.do(browser, http.MethodGet, "/api/v1/users/me", nil)
	assert.Contains(t, string(me.Data), `"password_change_required":false`)
	assert.Equal(t, http.StatusConflict, e.do(browser, http.MethodPost, "/api/v1/users/password/set",
		map[string]string{"password": "again!"}).Status, "only restricted sessions use this endpoint")

	res, _ = e.login(e.client(), "budi", "old-password")
	assert.Equal(t, http.StatusUnauthorized, res.Status)
	res, _ = e.login(e.client(), "budi", "brand-new")
	assert.Equal(t, http.StatusCreated, res.Status)
}

func TestRecovery_ForgotPasswordSendsNothingWithoutAVerifiedEmail(t *testing.T) {
	e := newRecoveryEnv(t)
	u := e.addUser("nomail", "pw-12345", "")
	require.NoError(t, e.store.UserRepo().UpdateEmail(tester.ContextWithLogger(), u.ID, "unverified@example.com"))
	blockedUser := e.addUser("blocked", "pw-12345", "blocked@example.com")
	require.NoError(t, e.store.UserRepo().SetBlocked(tester.ContextWithLogger(), blockedUser.ID, true))
	e.addUser("demo", "demo123", "demo@example.com")

	for _, login := range []string{"nomail", "unverified@example.com", "nobody", "nobody@example.com", "blocked", "demo"} {
		res := e.do(e.client(), http.MethodPost, "/api/v1/auth/password/forgot", map[string]string{"login": login})
		assert.Equal(t, http.StatusOK, res.Status, login)
		assert.Equal(t, forgotPasswordAnswer, res.Message, "same answer for %s", login)
	}
	assert.Empty(t, e.mail.Messages())
}

func TestRecovery_ForgotPasswordLimits(t *testing.T) {
	e := newRecoveryEnv(t)
	e.addUser("budi", "pw-12345", "budi@example.com")
	ask := func() {
		e.do(e.client(), http.MethodPost, "/api/v1/auth/password/forgot", map[string]string{"login": "budi"})
	}

	ask()
	ask()
	assert.Len(t, e.mail.Messages(), 1, "one email per 5 minutes")

	for i := 0; i < 6; i++ {
		_, err := e.db.Exec(`UPDATE password_resets SET created_at = ?`, time.Now().Add(-10*time.Minute).UTC())
		require.NoError(t, err)
		ask()
	}
	assert.Len(t, e.mail.Messages(), resetDailyLimit, "at most 5 a day")
}

func TestRecovery_WrongTemporaryPasswordsVoidIt(t *testing.T) {
	e := newRecoveryEnv(t)
	e.addUser("budi", "pw-12345", "budi@example.com")
	e.do(e.client(), http.MethodPost, "/api/v1/auth/password/forgot", map[string]string{"login": "budi"})
	temp := lastTempPassword(t, e.mail)

	for i := 0; i < tempPasswordMaxAttempts; i++ {
		res, _ := e.login(e.client(), "budi", "aaaa-bbbb-cccc")
		require.Equal(t, http.StatusUnauthorized, res.Status)
	}
	res, _ := e.login(e.client(), "budi", temp)
	assert.Equal(t, http.StatusUnauthorized, res.Status)
}

func TestRecovery_TemporaryPasswordWith2FAStaysRestricted(t *testing.T) {
	e := newRecoveryEnv(t)
	u := e.addUser("budi", "pw-12345", "budi@example.com")
	_, err := e.db.Exec(`UPDATE users SET totp_enabled = 1 WHERE id = ?`, u.ID.String())
	require.NoError(t, err)
	e.do(e.client(), http.MethodPost, "/api/v1/auth/password/forgot", map[string]string{"login": "budi"})

	res := e.do(e.client(), http.MethodPost, "/api/v1/sessions", map[string]string{"username": "budi", "password": lastTempPassword(t, e.mail)})
	require.Equal(t, http.StatusOK, res.Status)
	var challenge model.TOTPChallengeResponse
	require.NoError(t, json.Unmarshal(res.Data, &challenge))
	require.True(t, challenge.Requires2FA)

	_, _, pcr, err := e.h.validateChallengeToken(tester.ContextWithLogger(), challenge.ChallengeToken)
	require.NoError(t, err)
	assert.True(t, pcr, "the 2FA step must keep the session restricted")
}

func TestRecovery_EmailVerification(t *testing.T) {
	e := newRecoveryEnv(t)
	e.addUser("budi", "pw-12345", "")
	e.addUser("sari", "pw-12345", "sari@example.com")
	browser := e.client()
	_, lr := e.login(browser, "budi", "pw-12345")
	assert.True(t, lr.EmailPrompt, "users without a verified email are asked to add one")

	assert.Equal(t, http.StatusConflict, e.do(browser, http.MethodPut, "/api/v1/users/me/email",
		map[string]string{"email": "SARI@example.com"}).Status, "another account's verified email")

	res := e.do(browser, http.MethodPut, "/api/v1/users/me/email", map[string]string{"email": " Budi@Example.com ", "locale": "id"})
	require.Equal(t, http.StatusOK, res.Status, res.Error)
	msg := e.mail.Messages()[len(e.mail.Messages())-1]
	assert.Equal(t, "budi@example.com", msg.To)
	assert.Equal(t, "Konfirmasi email kamu", msg.Subject)
	token := regexp.MustCompile(`https://aio\.example\.com/verify-email\?token=([\w-]+)`).FindStringSubmatch(msg.Text)
	require.Len(t, token, 2)

	me := e.do(browser, http.MethodGet, "/api/v1/users/me", nil)
	assert.Contains(t, string(me.Data), `"pending_email":"budi@example.com"`)
	assert.Contains(t, string(me.Data), `"email_verified":false`)
	_, lr = e.login(e.client(), "budi", "pw-12345")
	assert.False(t, lr.EmailPrompt, "not asked again while a link is pending")

	// Verification works from any browser, without logging in.
	assert.Equal(t, http.StatusBadRequest, e.do(e.client(), http.MethodPost, "/api/v1/auth/email/verify", map[string]string{"token": "nope"}).Status)
	res = e.do(e.client(), http.MethodPost, "/api/v1/auth/email/verify", map[string]string{"token": token[1]})
	require.Equal(t, http.StatusOK, res.Status, res.Error)
	assert.Equal(t, http.StatusBadRequest, e.do(e.client(), http.MethodPost, "/api/v1/auth/email/verify",
		map[string]string{"token": token[1]}).Status, "links work once")

	me = e.do(browser, http.MethodGet, "/api/v1/users/me", nil)
	assert.Contains(t, string(me.Data), `"email":"budi@example.com"`)
	assert.Contains(t, string(me.Data), `"email_verified":true`)

	// Changing a verified email notifies the old address once the new one is confirmed.
	e.do(browser, http.MethodPut, "/api/v1/users/me/email", map[string]string{"email": "budi2@example.com"})
	token = regexp.MustCompile(`token=([\w-]+)`).FindStringSubmatch(e.mail.Messages()[len(e.mail.Messages())-1].Text)
	sent := len(e.mail.Messages())
	require.Equal(t, http.StatusOK, e.do(e.client(), http.MethodPost, "/api/v1/auth/email/verify", map[string]string{"token": token[1]}).Status)
	require.Len(t, e.mail.Messages(), sent+1)
	notice := e.mail.Messages()[sent]
	assert.Equal(t, "budi@example.com", notice.To)
	assert.Contains(t, notice.Text, "b•••@example.com")
}

func TestRecovery_SnoozeEmailPrompt(t *testing.T) {
	e := newRecoveryEnv(t)
	e.addUser("budi", "pw-12345", "")
	browser := e.client()
	_, lr := e.login(browser, "budi", "pw-12345")
	require.True(t, lr.EmailPrompt)
	require.Equal(t, http.StatusOK, e.do(browser, http.MethodPost, "/api/v1/users/me/email-prompt/snooze", nil).Status)
	_, lr = e.login(e.client(), "budi", "pw-12345")
	assert.False(t, lr.EmailPrompt)
}

func TestRecovery_SignupEmailWaitsForVerification(t *testing.T) {
	e := newRecoveryEnv(t)
	res := e.do(e.client(), http.MethodPost, "/api/v1/users", map[string]string{
		"username": "baru", "password": "pw-12345", "email": "Baru@example.com", "locale": "id"})
	require.Equal(t, http.StatusCreated, res.Status, res.Error)
	u, err := e.store.UserRepo().FindByUsername(tester.ContextWithLogger(), "baru")
	require.NoError(t, err)
	assert.Empty(t, u.Email, "stored only once verified")
	require.Len(t, e.mail.Messages(), 1)
	assert.Equal(t, "baru@example.com", e.mail.Messages()[0].To)
}

func TestRecovery_AdminSendsTemporaryPassword(t *testing.T) {
	e := newRecoveryEnv(t)
	adminUser := e.addUser("admin", "pw-12345", "")
	target := e.addUser("budi", "pw-12345", "")
	adminBrowser := e.client()
	e.login(adminBrowser, "admin", "pw-12345")
	_ = adminUser

	path := "/api/v1/admin/users/" + target.ID.String()
	res := e.do(adminBrowser, http.MethodPost, path+"/password-reset", nil)
	assert.Equal(t, http.StatusConflict, res.Status, "needs a verified email first")

	require.Equal(t, http.StatusOK, e.do(adminBrowser, http.MethodPatch, path,
		map[string]any{"email": "budi@example.com", "email_verified": true}).Status)
	res = e.do(adminBrowser, http.MethodPost, path+"/password-reset", map[string]string{"locale": "id"})
	require.Equal(t, http.StatusOK, res.Status, res.Error)
	assert.Equal(t, "temporary password sent to b•••@example.com", res.Message)
	assert.NotContains(t, string(res.Data), lastTempPassword(t, e.mail), "the admin never sees it")

	res2, lr := e.login(e.client(), "budi", lastTempPassword(t, e.mail))
	require.Equal(t, http.StatusCreated, res2.Status)
	assert.True(t, lr.PasswordChangeRequired)

	var issuedBy string
	require.NoError(t, e.db.Get(&issuedBy, `SELECT issued_by FROM password_resets`))
	assert.Equal(t, adminUser.ID.String(), issuedBy)
}

func TestRecovery_EmailDisabled(t *testing.T) {
	e := newRecoveryEnv(t)
	e.h.SetMailer(nil)
	e.addUser("budi", "pw-12345", "")
	assert.Equal(t, http.StatusServiceUnavailable, e.do(e.client(), http.MethodPost, "/api/v1/auth/password/forgot",
		map[string]string{"login": "budi"}).Status)
	browser := e.client()
	_, lr := e.login(browser, "budi", "pw-12345")
	assert.False(t, lr.EmailPrompt)
	assert.Equal(t, http.StatusServiceUnavailable, e.do(browser, http.MethodPut, "/api/v1/users/me/email",
		map[string]string{"email": "budi@example.com"}).Status)
}

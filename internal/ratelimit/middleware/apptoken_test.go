package middleware

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/all-in-one/internal/logging"
	"github.com/all-in-one/internal/ratelimit"
	"github.com/all-in-one/internal/ratelimit/model"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeVerifier struct {
	token model.AppToken
	err   error
	seen  string // the plaintext it was called with
}

func (f *fakeVerifier) VerifyToken(ctx context.Context, plaintext string) (model.AppToken, error) {
	f.seen = plaintext
	if f.err != nil {
		return model.AppToken{}, f.err
	}
	return f.token, nil
}

// serveWithLogger runs the middleware around a handler that records whether it
// ran and the token it saw, capturing everything logged to buf.
func serveWithLogger(t *testing.T, auth *AppTokenAuth, apiKey string, buf *bytes.Buffer) (*httptest.ResponseRecorder, bool, model.AppToken) {
	t.Helper()
	var ran bool
	var seenTok model.AppToken
	handler := auth.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ran = true
		seenTok, _ = AppTokenFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ratelimit/check", nil)
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}
	logger := zerolog.New(buf)
	req = req.WithContext(context.WithValue(req.Context(), logging.LoggerKey, &logger))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec, ran, seenTok
}

func TestAppTokenAuth_ValidToken(t *testing.T) {
	tok := model.AppToken{ID: "t1", App: "cashflow", TokenPrefix: "aio_cashflo", ScopePrefix: "cashflow."}
	auth := NewAppTokenAuth(&fakeVerifier{token: tok})

	var buf bytes.Buffer
	rec, ran, seen := serveWithLogger(t, auth, "aio_cashflow_secretvalue", &buf)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, ran, "handler should run for a valid token")
	assert.Equal(t, "t1", seen.ID, "verified token is placed in the context")
}

func TestAppTokenAuth_MissingHeader(t *testing.T) {
	auth := NewAppTokenAuth(&fakeVerifier{token: model.AppToken{}})

	var buf bytes.Buffer
	rec, ran, _ := serveWithLogger(t, auth, "", &buf)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.False(t, ran, "handler must not run without an API key")
}

func TestAppTokenAuth_UnknownToken(t *testing.T) {
	auth := NewAppTokenAuth(&fakeVerifier{err: ratelimit.ErrTokenNotFound})

	var buf bytes.Buffer
	rec, ran, _ := serveWithLogger(t, auth, "aio_cashflow_bogus", &buf)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.False(t, ran)
}

func TestAppTokenAuth_RevokedTokenIsUnauthorized(t *testing.T) {
	// A revoked token surfaces as ErrTokenNotFound (SQL-level exclusion).
	auth := NewAppTokenAuth(&fakeVerifier{err: ratelimit.ErrTokenNotFound})

	var buf bytes.Buffer
	rec, ran, _ := serveWithLogger(t, auth, "aio_cashflow_revoked", &buf)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.False(t, ran)
}

func TestAppTokenAuth_NeverLogsTokenMaterial(t *testing.T) {
	secret := "aio_cashflow_supersecretvalue123"

	// failure path
	authFail := NewAppTokenAuth(&fakeVerifier{err: ratelimit.ErrTokenNotFound})
	var failBuf bytes.Buffer
	serveWithLogger(t, authFail, secret, &failBuf)
	assert.NotContains(t, failBuf.String(), secret, "the token must never appear in failure logs")
	assert.NotContains(t, failBuf.String(), "supersecret")

	// success path — token_prefix is fine, full secret is not
	tok := model.AppToken{ID: "t1", App: "cashflow", TokenPrefix: "aio_cashflo"}
	authOK := NewAppTokenAuth(&fakeVerifier{token: tok})
	var okBuf bytes.Buffer
	serveWithLogger(t, authOK, secret, &okBuf)
	assert.NotContains(t, okBuf.String(), secret, "the full token must never appear in success logs")
	assert.True(t, strings.Contains(okBuf.String(), "aio_cashflo"), "the display prefix is logged on success")
}

func TestAppTokenAuth_ForwardsHeaderToVerifier(t *testing.T) {
	v := &fakeVerifier{token: model.AppToken{ID: "t1"}}
	auth := NewAppTokenAuth(v)

	var buf bytes.Buffer
	serveWithLogger(t, auth, "aio_cashflow_value", &buf)
	require.Equal(t, "aio_cashflow_value", v.seen, "the raw X-API-Key is passed to the verifier")
}

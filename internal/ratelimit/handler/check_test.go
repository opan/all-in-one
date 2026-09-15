package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/all-in-one/internal/config"
	"github.com/all-in-one/internal/ratelimit"
	"github.com/all-in-one/internal/ratelimit/handler/mocks"
	"github.com/all-in-one/internal/ratelimit/middleware"
	"github.com/all-in-one/internal/ratelimit/model"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type stubVerifier struct {
	token model.AppToken
	err   error
}

func (s stubVerifier) VerifyToken(ctx context.Context, plaintext string) (model.AppToken, error) {
	return s.token, s.err
}

func externalEnabledConfig() config.Config {
	cfg := config.Config{}
	cfg.RateLimit.External.Enabled = true
	return cfg
}

func newCheckRouter(h *Handler, verifier middleware.TokenVerifier) *mux.Router {
	r := mux.NewRouter()
	auth := middleware.NewAppTokenAuth(verifier)
	sub := r.PathPrefix("").Subrouter()
	sub.Use(auth.Middleware())
	h.RegisterCheckRoutes(sub)
	return r
}

func postCheck(t *testing.T, router *mux.Router, apiKey string, body model.CheckRequest) *httptest.ResponseRecorder {
	t.Helper()
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/ratelimit/check", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

var cashflowToken = model.AppToken{ID: "t1", App: "cashflow", TokenPrefix: "aio_cashflo", ScopePrefix: "cashflow."}

func TestCheck_Disabled_Returns503(t *testing.T) {
	svc := mocks.NewMockService(t)
	h := NewHandler(svc, config.Config{}) // External.Enabled defaults false
	router := newCheckRouter(h, stubVerifier{token: cashflowToken})

	rr := postCheck(t, router, "aio_cashflow_x", model.CheckRequest{TargetKey: "cashflow.entry.create", BucketKey: "user:42"})
	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
}

func TestCheck_MissingToken_Returns401(t *testing.T) {
	svc := mocks.NewMockService(t)
	h := NewHandler(svc, externalEnabledConfig())
	router := newCheckRouter(h, stubVerifier{err: ratelimit.ErrTokenNotFound})

	rr := postCheck(t, router, "", model.CheckRequest{TargetKey: "cashflow.entry.create", BucketKey: "user:42"})
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestCheck_TargetOutsideScope_Returns403(t *testing.T) {
	svc := mocks.NewMockService(t)
	h := NewHandler(svc, externalEnabledConfig())
	router := newCheckRouter(h, stubVerifier{token: cashflowToken})

	rr := postCheck(t, router, "aio_cashflow_x", model.CheckRequest{TargetKey: "otherapp.thing", BucketKey: "user:42"})
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestCheck_UnknownTarget_Returns404(t *testing.T) {
	svc := mocks.NewMockService(t)
	svc.EXPECT().EffectiveRule("cashflow.entry.create").Return(model.EffectiveRule{}, false)
	h := NewHandler(svc, externalEnabledConfig())
	router := newCheckRouter(h, stubVerifier{token: cashflowToken})

	rr := postCheck(t, router, "aio_cashflow_x", model.CheckRequest{TargetKey: "cashflow.entry.create", BucketKey: "user:42"})
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestCheck_InternalTargetNotCallable_Returns404(t *testing.T) {
	svc := mocks.NewMockService(t)
	// exists but is internal → must not be callable via /check
	svc.EXPECT().EffectiveRule("cashflow.entry.create").Return(model.EffectiveRule{Key: "cashflow.entry.create", Scope: model.ScopeUser, IsExternal: false}, true)
	h := NewHandler(svc, externalEnabledConfig())
	router := newCheckRouter(h, stubVerifier{token: cashflowToken})

	rr := postCheck(t, router, "aio_cashflow_x", model.CheckRequest{TargetKey: "cashflow.entry.create", BucketKey: "user:42"})
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestCheck_BucketKeyScopeMismatch_Returns400(t *testing.T) {
	svc := mocks.NewMockService(t)
	svc.EXPECT().EffectiveRule("cashflow.entry.create").Return(model.EffectiveRule{Key: "cashflow.entry.create", Scope: model.ScopeUser, IsExternal: true}, true)
	h := NewHandler(svc, externalEnabledConfig())
	router := newCheckRouter(h, stubVerifier{token: cashflowToken})

	// user-scoped target but an ip: bucket key
	rr := postCheck(t, router, "aio_cashflow_x", model.CheckRequest{TargetKey: "cashflow.entry.create", BucketKey: "ip:1.2.3.4"})
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestCheck_Allowed_Returns200(t *testing.T) {
	svc := mocks.NewMockService(t)
	svc.EXPECT().EffectiveRule("cashflow.entry.create").Return(model.EffectiveRule{Key: "cashflow.entry.create", Scope: model.ScopeUser, IsExternal: true}, true)
	svc.EXPECT().CheckExternal(mock.Anything, "cashflow.entry.create", "user:42").
		Return(model.CheckResponse{Allowed: true, Limit: 1000, Remaining: 941}, nil)
	h := NewHandler(svc, externalEnabledConfig())
	router := newCheckRouter(h, stubVerifier{token: cashflowToken})

	rr := postCheck(t, router, "aio_cashflow_x", model.CheckRequest{TargetKey: "cashflow.entry.create", BucketKey: "user:42"})
	require.Equal(t, http.StatusOK, rr.Code)

	var resp struct {
		Success bool                `json:"success"`
		Data    model.CheckResponse `json:"data"`
	}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.True(t, resp.Success)
	assert.True(t, resp.Data.Allowed)
	assert.Equal(t, 941, resp.Data.Remaining)
}

func TestCheck_Rejected_Is200Not429(t *testing.T) {
	svc := mocks.NewMockService(t)
	svc.EXPECT().EffectiveRule("cashflow.entry.create").Return(model.EffectiveRule{Key: "cashflow.entry.create", Scope: model.ScopeUser, IsExternal: true}, true)
	svc.EXPECT().CheckExternal(mock.Anything, "cashflow.entry.create", "user:42").
		Return(model.CheckResponse{Allowed: false, Limit: 1000, Remaining: 0, RetryAfterSeconds: 47320}, nil)
	h := NewHandler(svc, externalEnabledConfig())
	router := newCheckRouter(h, stubVerifier{token: cashflowToken})

	rr := postCheck(t, router, "aio_cashflow_x", model.CheckRequest{TargetKey: "cashflow.entry.create", BucketKey: "user:42"})
	require.Equal(t, http.StatusOK, rr.Code, "a rejection is 200 + allowed:false, never 429")

	var resp struct {
		Data model.CheckResponse `json:"data"`
	}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.False(t, resp.Data.Allowed)
	assert.Equal(t, 47320, resp.Data.RetryAfterSeconds)
}

func TestCheck_CounterStoreError_FailsOpen200(t *testing.T) {
	svc := mocks.NewMockService(t)
	svc.EXPECT().EffectiveRule("cashflow.entry.create").Return(model.EffectiveRule{Key: "cashflow.entry.create", Scope: model.ScopeUser, IsExternal: true}, true)
	svc.EXPECT().CheckExternal(mock.Anything, "cashflow.entry.create", "user:42").
		Return(model.CheckResponse{Allowed: true}, errors.New("counter store boom"))
	h := NewHandler(svc, externalEnabledConfig())
	router := newCheckRouter(h, stubVerifier{token: cashflowToken})

	rr := postCheck(t, router, "aio_cashflow_x", model.CheckRequest{TargetKey: "cashflow.entry.create", BucketKey: "user:42"})
	require.Equal(t, http.StatusOK, rr.Code)

	var resp struct {
		Data model.CheckResponse `json:"data"`
	}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.True(t, resp.Data.Allowed, "a counter-store error must fail open")
}

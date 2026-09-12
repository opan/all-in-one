package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/all-in-one/internal/config"
	"github.com/all-in-one/internal/ratelimit"
	"github.com/all-in-one/internal/ratelimit/handler/mocks"
	"github.com/all-in-one/internal/ratelimit/model"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func postJSON(t *testing.T, router *mux.Router, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(method, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

func TestCreateToken_ReturnsPlaintextOnceInBody(t *testing.T) {
	svc := mocks.NewMockService(t)
	stored := model.AppToken{ID: "t1", App: "cashflow", Name: "prod", TokenPrefix: "aio_cashflo", ScopePrefix: "cashflow."}
	svc.EXPECT().CreateToken(mock.Anything, "cashflow", "prod", "", mock.Anything).
		Return(stored, "aio_cashflow_THEPLAINTEXT", nil)

	h := NewHandler(svc, config.Config{})
	router := newTestRouter(h)

	rr := postJSON(t, router, http.MethodPost, "/ratelimit/tokens", createTokenRequest{App: "cashflow", Name: "prod"})
	require.Equal(t, http.StatusCreated, rr.Code)

	body := rr.Body.String()
	assert.Contains(t, body, "aio_cashflow_THEPLAINTEXT", "the plaintext token is in the create response")
	assert.Contains(t, body, "not be shown again")
}

func TestListTokens_DoesNotLeakPlaintext(t *testing.T) {
	svc := mocks.NewMockService(t)
	svc.EXPECT().ListTokens(mock.Anything).Return([]model.AppToken{
		{ID: "t1", App: "cashflow", TokenPrefix: "aio_cashflo", TokenHash: "sha256hash", ScopePrefix: "cashflow."},
	}, nil)

	h := NewHandler(svc, config.Config{})
	router := newTestRouter(h)

	rr := doRequest(t, router, http.MethodGet, "/ratelimit/tokens")
	require.Equal(t, http.StatusOK, rr.Code)
	body := rr.Body.String()
	assert.NotContains(t, body, "sha256hash", "the hash must never appear in a list response")
	assert.NotContains(t, body, "token_hash")
	assert.Contains(t, body, "aio_cashflo", "the display prefix is fine")
}

func TestCreateToken_RequiresAppAndName(t *testing.T) {
	svc := mocks.NewMockService(t)
	h := NewHandler(svc, config.Config{})
	router := newTestRouter(h)

	rr := postJSON(t, router, http.MethodPost, "/ratelimit/tokens", createTokenRequest{App: "cashflow"})
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestRevokeToken_HappyPath(t *testing.T) {
	svc := mocks.NewMockService(t)
	svc.EXPECT().RevokeToken(mock.Anything, "t1").Return(nil)

	h := NewHandler(svc, config.Config{})
	router := newTestRouter(h)

	rr := doRequest(t, router, http.MethodDelete, "/ratelimit/tokens/t1")
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestCreateExternalTarget_HappyPath(t *testing.T) {
	svc := mocks.NewMockService(t)
	created := model.Target{Key: "cashflow.entry.create", App: "cashflow", IsExternal: true, Scope: model.ScopeUser, Kind: model.KindDailyQuota, Enabled: true, LimitCount: 1000, WindowValue: 1, WindowUnit: model.WindowDay}
	svc.EXPECT().CreateExternalTarget(mock.Anything, mock.MatchedBy(func(tg model.Target) bool {
		return tg.Key == "cashflow.entry.create" && tg.Enabled // default enabled=true
	}), mock.Anything).Return(created, nil)

	h := NewHandler(svc, config.Config{})
	router := newTestRouter(h)

	rr := postJSON(t, router, http.MethodPost, "/ratelimit/targets/external", createExternalTargetRequest{
		Key: "cashflow.entry.create", App: "cashflow", Scope: model.ScopeUser, Kind: model.KindDailyQuota,
		LimitCount: 1000, WindowValue: 1, WindowUnit: model.WindowDay,
	})
	require.Equal(t, http.StatusCreated, rr.Code)
}

func TestCreateExternalTarget_DuplicateReturns409(t *testing.T) {
	svc := mocks.NewMockService(t)
	svc.EXPECT().CreateExternalTarget(mock.Anything, mock.Anything, mock.Anything).
		Return(model.Target{}, ratelimit.ErrExternalTargetExists)

	h := NewHandler(svc, config.Config{})
	router := newTestRouter(h)

	rr := postJSON(t, router, http.MethodPost, "/ratelimit/targets/external", createExternalTargetRequest{
		Key: "cashflow.entry.create", Scope: model.ScopeUser, Kind: model.KindDailyQuota, LimitCount: 1, WindowValue: 1, WindowUnit: model.WindowDay,
	})
	assert.Equal(t, http.StatusConflict, rr.Code)
}

func TestDeleteExternalTarget_HappyPath(t *testing.T) {
	svc := mocks.NewMockService(t)
	svc.EXPECT().DeleteExternalTarget(mock.Anything, "cashflow.entry.create").Return(nil)

	h := NewHandler(svc, config.Config{})
	router := newTestRouter(h)

	rr := doRequest(t, router, http.MethodDelete, "/ratelimit/targets/cashflow.entry.create")
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestDeleteExternalTarget_InternalTargetReturns400(t *testing.T) {
	svc := mocks.NewMockService(t)
	svc.EXPECT().DeleteExternalTarget(mock.Anything, "auth.login").Return(ratelimit.ErrNotSupportedForExternal)

	h := NewHandler(svc, config.Config{})
	router := newTestRouter(h)

	rr := doRequest(t, router, http.MethodDelete, "/ratelimit/targets/auth.login")
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

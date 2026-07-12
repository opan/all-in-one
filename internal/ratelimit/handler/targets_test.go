package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/all-in-one/internal/config"
	httpHelper "github.com/all-in-one/internal/http"
	"github.com/all-in-one/internal/ratelimit/handler/mocks"
	"github.com/all-in-one/internal/ratelimit/model"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newTestRouter(h *Handler) *mux.Router {
	r := mux.NewRouter()
	h.RegisterAdminRoutes(r)
	return r
}

func doRequest(t *testing.T, router *mux.Router, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader(nil))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

func decodeResponse(t *testing.T, rr *httptest.ResponseRecorder) httpHelper.Response {
	t.Helper()
	var resp httpHelper.Response
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	return resp
}

func TestListTargets_HappyPath(t *testing.T) {
	targets := []model.Target{
		{Key: "auth.login", Name: "Login attempts", Scope: model.ScopeIP, Kind: model.KindThrottle, Enabled: true, LimitCount: 10, WindowValue: 1, WindowUnit: model.WindowMinute},
	}

	svc := mocks.NewMockService(t)
	svc.EXPECT().ListTargets(mock.Anything).Return(targets, nil)

	h := NewHandler(svc, config.Config{})
	router := newTestRouter(h)

	rr := doRequest(t, router, http.MethodGet, "/ratelimit/targets")
	assert.Equal(t, http.StatusOK, rr.Code)

	resp := decodeResponse(t, rr)
	assert.True(t, resp.Success)

	var got []model.Target
	b, err := json.Marshal(resp.Data)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(b, &got))
	require.Len(t, got, 1)
	assert.Equal(t, "auth.login", got[0].Key)
}

func TestListTargets_ServiceError(t *testing.T) {
	svc := mocks.NewMockService(t)
	svc.EXPECT().ListTargets(mock.Anything).Return(nil, errors.New("db error"))

	h := NewHandler(svc, config.Config{})
	router := newTestRouter(h)

	rr := doRequest(t, router, http.MethodGet, "/ratelimit/targets")
	assert.Equal(t, http.StatusInternalServerError, rr.Code)

	resp := decodeResponse(t, rr)
	assert.False(t, resp.Success)
}

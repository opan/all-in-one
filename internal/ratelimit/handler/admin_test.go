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
	"github.com/all-in-one/internal/tester"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func doAdminRequest(t *testing.T, router http.Handler, method, path, username string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var bodyReader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		bodyReader = bytes.NewReader(b)
	} else {
		bodyReader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, bodyReader)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(tester.ContextWithUser("u-1", username, username+"@example.com", "sess-1"))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

func TestUpdateTarget_PartialPatch(t *testing.T) {
	limit := 20
	patch := model.TargetPatch{LimitCount: &limit}

	svc := mocks.NewMockService(t)
	svc.EXPECT().UpdateTarget(mock.Anything, "auth.login", mock.MatchedBy(func(p model.TargetPatch) bool {
		return p.Enabled == nil && p.LimitCount != nil && *p.LimitCount == 20 &&
			p.WindowValue == nil && p.WindowUnit == nil
	}), "admin").Return(model.Target{Key: "auth.login", LimitCount: 20}, nil)

	h := NewHandler(svc, config.Config{})
	router := newTestRouter(h)

	rr := doAdminRequest(t, router, http.MethodPatch, "/ratelimit/targets/auth.login", "admin", patch)
	assert.Equal(t, http.StatusOK, rr.Code)

	resp := decodeResponse(t, rr)
	assert.True(t, resp.Success)
}

func TestUpdateTarget_UsesAuthenticatedUsernameAsUpdatedBy(t *testing.T) {
	svc := mocks.NewMockService(t)
	svc.EXPECT().UpdateTarget(mock.Anything, "auth.login", mock.Anything, "alice").
		Return(model.Target{Key: "auth.login"}, nil)

	h := NewHandler(svc, config.Config{})
	router := newTestRouter(h)

	rr := doAdminRequest(t, router, http.MethodPatch, "/ratelimit/targets/auth.login", "alice", model.TargetPatch{})
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestUpdateTarget_RejectsLimitLessThanOne(t *testing.T) {
	zero := 0
	svc := mocks.NewMockService(t)
	// UpdateTarget must NOT be called — no expectation registered for it.

	h := NewHandler(svc, config.Config{})
	router := newTestRouter(h)

	rr := doAdminRequest(t, router, http.MethodPatch, "/ratelimit/targets/auth.login", "admin", model.TargetPatch{LimitCount: &zero})
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	resp := decodeResponse(t, rr)
	assert.False(t, resp.Success)
}

func TestUpdateTarget_InvalidBody(t *testing.T) {
	svc := mocks.NewMockService(t)
	h := NewHandler(svc, config.Config{})
	router := newTestRouter(h)

	req := httptest.NewRequest(http.MethodPatch, "/ratelimit/targets/auth.login", bytes.NewReader([]byte("{not json")))
	req = req.WithContext(tester.ContextWithUser("u-1", "admin", "admin@example.com", "sess-1"))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestUpdateTarget_UnknownTarget(t *testing.T) {
	svc := mocks.NewMockService(t)
	svc.EXPECT().UpdateTarget(mock.Anything, "does.not.exist", mock.Anything, "admin").
		Return(model.Target{}, ratelimit.ErrUnknownTarget)

	h := NewHandler(svc, config.Config{})
	router := newTestRouter(h)

	rr := doAdminRequest(t, router, http.MethodPatch, "/ratelimit/targets/does.not.exist", "admin", model.TargetPatch{})
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestUpdateTarget_InvalidWindowUnit(t *testing.T) {
	bad := model.WindowUnit("fortnight")
	svc := mocks.NewMockService(t)
	svc.EXPECT().UpdateTarget(mock.Anything, "auth.login", mock.Anything, "admin").
		Return(model.Target{}, ratelimit.ErrInvalidWindowUnit)

	h := NewHandler(svc, config.Config{})
	router := newTestRouter(h)

	rr := doAdminRequest(t, router, http.MethodPatch, "/ratelimit/targets/auth.login", "admin", model.TargetPatch{WindowUnit: &bad})
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestResetCounters_HappyPath(t *testing.T) {
	svc := mocks.NewMockService(t)
	svc.EXPECT().ResetCounters(mock.Anything, "auth.login").Return(nil)

	h := NewHandler(svc, config.Config{})
	router := newTestRouter(h)

	rr := doAdminRequest(t, router, http.MethodPost, "/ratelimit/targets/auth.login/reset", "admin", nil)
	assert.Equal(t, http.StatusOK, rr.Code)

	resp := decodeResponse(t, rr)
	assert.True(t, resp.Success)
}

func TestResetCounters_UnknownTarget(t *testing.T) {
	svc := mocks.NewMockService(t)
	svc.EXPECT().ResetCounters(mock.Anything, "does.not.exist").Return(ratelimit.ErrUnknownTarget)

	h := NewHandler(svc, config.Config{})
	router := newTestRouter(h)

	rr := doAdminRequest(t, router, http.MethodPost, "/ratelimit/targets/does.not.exist/reset", "admin", nil)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestResetDefaults_HappyPath(t *testing.T) {
	svc := mocks.NewMockService(t)
	svc.EXPECT().ResetDefaults(mock.Anything, "auth.login").
		Return(model.Target{Key: "auth.login", Enabled: true, LimitCount: 10}, nil)

	h := NewHandler(svc, config.Config{})
	router := newTestRouter(h)

	rr := doAdminRequest(t, router, http.MethodPost, "/ratelimit/targets/auth.login/reset-defaults", "admin", nil)
	assert.Equal(t, http.StatusOK, rr.Code)

	resp := decodeResponse(t, rr)
	assert.True(t, resp.Success)
}

func TestResetDefaults_UnknownTarget(t *testing.T) {
	svc := mocks.NewMockService(t)
	svc.EXPECT().ResetDefaults(mock.Anything, "does.not.exist").Return(model.Target{}, ratelimit.ErrUnknownTarget)

	h := NewHandler(svc, config.Config{})
	router := newTestRouter(h)

	rr := doAdminRequest(t, router, http.MethodPost, "/ratelimit/targets/does.not.exist/reset-defaults", "admin", nil)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

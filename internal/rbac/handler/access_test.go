package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/all-in-one/internal/config"
	httpHelper "github.com/all-in-one/internal/http"
	"github.com/all-in-one/internal/rbac"
	"github.com/all-in-one/internal/rbac/handler/mocks"
	"github.com/all-in-one/internal/rbac/model"
	"github.com/google/uuid"
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

func doRequest(t *testing.T, router *mux.Router, method, path string, body any) *httptest.ResponseRecorder {
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

func TestListFeatures_Success(t *testing.T) {
	svc := mocks.NewMockService(t)
	svc.On("ListFeatures", mock.Anything).Return([]model.Feature{
		{Key: rbac.FeatureListing, Name: "Listings"},
	}, nil)

	h := NewHandler(svc, config.Config{})
	rr := doRequest(t, newTestRouter(h), http.MethodGet, "/access/features", nil)

	assert.Equal(t, http.StatusOK, rr.Code)
	resp := decodeResponse(t, rr)
	assert.True(t, resp.Success)
}

func TestListGroups_Success(t *testing.T) {
	svc := mocks.NewMockService(t)
	svc.On("ListGroups", mock.Anything).Return([]model.Group{
		{Name: rbac.GroupRegularUser, FeatureKeys: []string{rbac.FeatureListing}},
	}, nil)

	h := NewHandler(svc, config.Config{})
	rr := doRequest(t, newTestRouter(h), http.MethodGet, "/access/groups", nil)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestCreateGroup_Success(t *testing.T) {
	svc := mocks.NewMockService(t)
	svc.On("CreateGroup", mock.Anything, "listing-only", "desc", []string{rbac.FeatureListing}).
		Return(model.Group{ID: uuid.New(), Name: "listing-only"}, nil)

	h := NewHandler(svc, config.Config{})
	rr := doRequest(t, newTestRouter(h), http.MethodPost, "/access/groups", CreateGroupRequest{
		Name: "listing-only", Description: "desc", FeatureKeys: []string{rbac.FeatureListing},
	})

	assert.Equal(t, http.StatusCreated, rr.Code)
}

func TestCreateGroup_MissingName(t *testing.T) {
	svc := mocks.NewMockService(t)
	// CreateGroup must NOT be called — validation fails before the service call.

	h := NewHandler(svc, config.Config{})
	rr := doRequest(t, newTestRouter(h), http.MethodPost, "/access/groups", CreateGroupRequest{
		Description: "desc",
	})

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestGetGroup_NotFound(t *testing.T) {
	id := uuid.New()
	svc := mocks.NewMockService(t)
	svc.On("GetGroup", mock.Anything, id).Return(model.Group{}, httpHelper.ErrNotFound)

	h := NewHandler(svc, config.Config{})
	rr := doRequest(t, newTestRouter(h), http.MethodGet, "/access/groups/"+id.String(), nil)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestGetGroup_InvalidID(t *testing.T) {
	svc := mocks.NewMockService(t)
	h := NewHandler(svc, config.Config{})
	rr := doRequest(t, newTestRouter(h), http.MethodGet, "/access/groups/not-a-uuid", nil)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestUpdateGroup_BuiltinConflict(t *testing.T) {
	id := uuid.New()
	svc := mocks.NewMockService(t)
	svc.On("UpdateGroup", mock.Anything, id, "renamed-admin", "desc").
		Return(model.Group{}, rbac.ErrBuiltinGroup)

	h := NewHandler(svc, config.Config{})
	rr := doRequest(t, newTestRouter(h), http.MethodPut, "/access/groups/"+id.String(), UpdateGroupRequest{
		Name: "renamed-admin", Description: "desc",
	})

	assert.Equal(t, http.StatusConflict, rr.Code)
}

func TestDeleteGroup_BuiltinConflict(t *testing.T) {
	id := uuid.New()
	svc := mocks.NewMockService(t)
	svc.On("DeleteGroup", mock.Anything, id).Return(rbac.ErrBuiltinGroup)

	h := NewHandler(svc, config.Config{})
	rr := doRequest(t, newTestRouter(h), http.MethodDelete, "/access/groups/"+id.String(), nil)

	assert.Equal(t, http.StatusConflict, rr.Code)
}

func TestDeleteGroup_Success(t *testing.T) {
	id := uuid.New()
	svc := mocks.NewMockService(t)
	svc.On("DeleteGroup", mock.Anything, id).Return(nil)

	h := NewHandler(svc, config.Config{})
	rr := doRequest(t, newTestRouter(h), http.MethodDelete, "/access/groups/"+id.String(), nil)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestSetGroupFeatures_AdminGroupConflict(t *testing.T) {
	id := uuid.New()
	svc := mocks.NewMockService(t)
	svc.On("SetGroupFeatures", mock.Anything, id, []string{rbac.FeatureListing}).
		Return(model.Group{}, rbac.ErrBuiltinGroup)

	h := NewHandler(svc, config.Config{})
	rr := doRequest(t, newTestRouter(h), http.MethodPut, "/access/groups/"+id.String()+"/features", SetGroupFeaturesRequest{
		FeatureKeys: []string{rbac.FeatureListing},
	})

	assert.Equal(t, http.StatusConflict, rr.Code)
}

func TestListUsers_Success(t *testing.T) {
	svc := mocks.NewMockService(t)
	svc.On("ListUsers", mock.Anything).Return([]model.UserAccessRow{
		{Username: "admin", IsAdmin: true},
	}, nil)

	h := NewHandler(svc, config.Config{})
	rr := doRequest(t, newTestRouter(h), http.MethodGet, "/access/users", nil)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestAssignUserGroup_LastAdminConflict(t *testing.T) {
	userID := uuid.New()
	groupID := uuid.New()
	groupIDStr := groupID.String()

	svc := mocks.NewMockService(t)
	svc.On("AssignUserGroup", mock.Anything, userID, &groupID).Return(rbac.ErrLastAdmin)

	h := NewHandler(svc, config.Config{})
	rr := doRequest(t, newTestRouter(h), http.MethodPut, "/access/users/"+userID.String()+"/group", AssignUserGroupRequest{
		GroupID: &groupIDStr,
	})

	assert.Equal(t, http.StatusConflict, rr.Code)
}

func TestAssignUserGroup_Unassign(t *testing.T) {
	userID := uuid.New()

	svc := mocks.NewMockService(t)
	svc.On("AssignUserGroup", mock.Anything, userID, (*uuid.UUID)(nil)).Return(nil)

	h := NewHandler(svc, config.Config{})
	rr := doRequest(t, newTestRouter(h), http.MethodPut, "/access/users/"+userID.String()+"/group", AssignUserGroupRequest{
		GroupID: nil,
	})

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestAssignUserGroup_InvalidGroupID(t *testing.T) {
	userID := uuid.New()
	bogus := "not-a-uuid"

	svc := mocks.NewMockService(t)
	// AssignUserGroup must NOT be called — the group id fails to parse first.

	h := NewHandler(svc, config.Config{})
	rr := doRequest(t, newTestRouter(h), http.MethodPut, "/access/users/"+userID.String()+"/group", AssignUserGroupRequest{
		GroupID: &bogus,
	})

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestListUserOverrides_Success(t *testing.T) {
	userID := uuid.New()
	svc := mocks.NewMockService(t)
	svc.On("ListUserOverrides", mock.Anything, userID).Return([]model.FeatureOverrideView{
		{FeatureKey: rbac.FeatureListing, Allow: true},
	}, nil)

	h := NewHandler(svc, config.Config{})
	rr := doRequest(t, newTestRouter(h), http.MethodGet, "/access/users/"+userID.String()+"/overrides", nil)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestSetUserOverrides_Success(t *testing.T) {
	userID := uuid.New()
	svc := mocks.NewMockService(t)
	svc.On("SetUserOverrides", mock.Anything, userID, []model.FeatureOverrideView{
		{FeatureKey: rbac.FeatureListing, Allow: true},
	}).Return(nil)

	h := NewHandler(svc, config.Config{})
	rr := doRequest(t, newTestRouter(h), http.MethodPut, "/access/users/"+userID.String()+"/overrides", SetUserOverridesRequest{
		Overrides: []model.FeatureOverrideView{{FeatureKey: rbac.FeatureListing, Allow: true}},
	})

	assert.Equal(t, http.StatusOK, rr.Code)
}

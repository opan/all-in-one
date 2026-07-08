package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	httpHelper "github.com/all-in-one/internal/http"
	"github.com/all-in-one/internal/shortener/handler/mocks"
	"github.com/all-in-one/internal/shortener/model"
	"github.com/all-in-one/internal/tester"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func makeLinkWithOwner(code, ownerID, ownerUsername string) model.ShortLinkWithOwner {
	return model.ShortLinkWithOwner{
		ShortLink:     makeLink(code),
		OwnerID:       ownerID,
		OwnerUsername: ownerUsername,
	}
}

// ---- ListAllShortLinks ----

func TestListAllShortLinks_Success(t *testing.T) {
	mockRepo := mocks.NewMockShortLinkRepository(t)
	h := newHandler(mockRepo)

	links := []model.ShortLinkWithOwner{
		makeLinkWithOwner("aaa1111", "user-1", "alice"),
		makeLinkWithOwner("bbb2222", "", ""),
	}
	mockRepo.On("ListAll", mock.Anything, uint32(1), uint32(20)).Return(links, uint32(2), nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/shortener/links", nil)
	req = req.WithContext(tester.ContextWithLogger())
	rr := httptest.NewRecorder()

	h.ListAllShortLinks(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp httpHelper.Response
	assert.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.True(t, resp.Success)
	mockRepo.AssertExpectations(t)
}

func TestListAllShortLinks_DBError(t *testing.T) {
	mockRepo := mocks.NewMockShortLinkRepository(t)
	h := newHandler(mockRepo)

	mockRepo.On("ListAll", mock.Anything, uint32(1), uint32(20)).
		Return(nil, uint32(0), errors.New("db error"))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/shortener/links", nil)
	req = req.WithContext(tester.ContextWithLogger())
	rr := httptest.NewRecorder()

	h.ListAllShortLinks(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	mockRepo.AssertExpectations(t)
}

// ---- AdminModerateShortLink ----

func TestAdminModerateShortLink_Deactivate(t *testing.T) {
	mockRepo := mocks.NewMockShortLinkRepository(t)
	h := newHandler(mockRepo)

	mockRepo.On("SetActiveByCode", mock.Anything, "abc1234", false).Return(nil)

	body, _ := json.Marshal(adminModerateShortLinkRequest{IsActive: false})
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/shortener/links/abc1234", bytes.NewReader(body))
	req = req.WithContext(tester.ContextWithLogger())
	req = mux.SetURLVars(req, map[string]string{"code": "abc1234"})
	rr := httptest.NewRecorder()

	h.AdminModerateShortLink(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	mockRepo.AssertExpectations(t)
}

func TestAdminModerateShortLink_InvalidBody(t *testing.T) {
	h := newHandler(mocks.NewMockShortLinkRepository(t))

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/shortener/links/abc1234", bytes.NewReader([]byte("not json")))
	req = req.WithContext(tester.ContextWithLogger())
	req = mux.SetURLVars(req, map[string]string{"code": "abc1234"})
	rr := httptest.NewRecorder()

	h.AdminModerateShortLink(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestAdminModerateShortLink_NotFound(t *testing.T) {
	mockRepo := mocks.NewMockShortLinkRepository(t)
	h := newHandler(mockRepo)

	mockRepo.On("SetActiveByCode", mock.Anything, "gone", true).Return(httpHelper.ErrNotFound)

	body, _ := json.Marshal(adminModerateShortLinkRequest{IsActive: true})
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/shortener/links/gone", bytes.NewReader(body))
	req = req.WithContext(tester.ContextWithLogger())
	req = mux.SetURLVars(req, map[string]string{"code": "gone"})
	rr := httptest.NewRecorder()

	h.AdminModerateShortLink(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
	mockRepo.AssertExpectations(t)
}

// ---- AdminDeleteShortLink ----

func TestAdminDeleteShortLink_Success(t *testing.T) {
	mockRepo := mocks.NewMockShortLinkRepository(t)
	h := newHandler(mockRepo)

	mockRepo.On("DeleteByCode", mock.Anything, "abc1234").Return(nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/shortener/links/abc1234", nil)
	req = req.WithContext(tester.ContextWithLogger())
	req = mux.SetURLVars(req, map[string]string{"code": "abc1234"})
	rr := httptest.NewRecorder()

	h.AdminDeleteShortLink(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	mockRepo.AssertExpectations(t)
}

func TestAdminDeleteShortLink_NotFound(t *testing.T) {
	mockRepo := mocks.NewMockShortLinkRepository(t)
	h := newHandler(mockRepo)

	mockRepo.On("DeleteByCode", mock.Anything, "gone").Return(httpHelper.ErrNotFound)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/shortener/links/gone", nil)
	req = req.WithContext(tester.ContextWithLogger())
	req = mux.SetURLVars(req, map[string]string{"code": "gone"})
	rr := httptest.NewRecorder()

	h.AdminDeleteShortLink(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
	mockRepo.AssertExpectations(t)
}

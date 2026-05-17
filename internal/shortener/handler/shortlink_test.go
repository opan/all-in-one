package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/all-in-one/internal/config"
	httpHelper "github.com/all-in-one/internal/http"
	"github.com/all-in-one/internal/shortener/handler/mocks"
	"github.com/all-in-one/internal/shortener/model"
	"github.com/all-in-one/internal/shortener/repository"
	"github.com/all-in-one/internal/tester"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const testUserID = "01JTEST000000000000000001"

// mockStorage wraps the generated mock for use as repository.Storage.
type mockStorage struct {
	repo repository.ShortLinkRepository
}

func (m *mockStorage) ShortLinkRepo() repository.ShortLinkRepository { return m.repo }
func (m *mockStorage) Close() error                                   { return nil }

func newHandler(repo repository.ShortLinkRepository) *Handler {
	cfg := config.Config{
		Shortener: config.ShortenerConfig{
			CodeLength:       7,
			MaxCreateRetries: 5,
			URL: config.ShortenerURLConfig{
				MaxLength:      2048,
				AllowedSchemes: []string{"http", "https"},
				BlockedHosts:   []string{},
			},
		},
	}
	return NewHandler(&mockStorage{repo: repo}, cfg, zerolog.Nop())
}

func makeLink(code string) model.ShortLink {
	return model.ShortLink{
		ID:        newULID(),
		Code:      code,
		TargetURL: "https://example.com",
		CreatedAt: time.Now().UTC(),
		IsActive:  true,
	}
}

// ---- CreateShortLink ----

func TestCreateShortLink_Success(t *testing.T) {
	mockRepo := mocks.NewMockShortLinkRepository(t)
	h := newHandler(mockRepo)

	expected := makeLink("abc1234")
	mockRepo.On("Create", mock.Anything, mock.MatchedBy(func(l model.ShortLink) bool {
		return l.TargetURL == "https://example.com" && l.IsActive
	}), testUserID).Return(expected, nil)

	body, _ := json.Marshal(createShortLinkRequest{TargetURL: "https://example.com"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/shortener/links", bytes.NewReader(body))
	req = req.WithContext(tester.ContextWithUser(testUserID, "user", "u@e.com", "sess"))
	rr := httptest.NewRecorder()

	h.CreateShortLink(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	var resp httpHelper.Response
	assert.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.True(t, resp.Success)
	mockRepo.AssertExpectations(t)
}

func TestCreateShortLink_InvalidURL(t *testing.T) {
	h := newHandler(mocks.NewMockShortLinkRepository(t))

	body, _ := json.Marshal(createShortLinkRequest{TargetURL: "javascript:alert(1)"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/shortener/links", bytes.NewReader(body))
	req = req.WithContext(tester.ContextWithUser(testUserID, "user", "u@e.com", "sess"))
	rr := httptest.NewRecorder()

	h.CreateShortLink(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestCreateShortLink_CustomCodeRejected(t *testing.T) {
	h := newHandler(mocks.NewMockShortLinkRepository(t))

	custom := "myslug"
	body, _ := json.Marshal(createShortLinkRequest{TargetURL: "https://example.com", CustomCode: &custom})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/shortener/links", bytes.NewReader(body))
	req = req.WithContext(tester.ContextWithUser(testUserID, "user", "u@e.com", "sess"))
	rr := httptest.NewRecorder()

	h.CreateShortLink(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestCreateShortLink_DBError(t *testing.T) {
	mockRepo := mocks.NewMockShortLinkRepository(t)
	h := newHandler(mockRepo)

	mockRepo.On("Create", mock.Anything, mock.Anything, mock.Anything).
		Return(model.ShortLink{}, errors.New("db error"))

	body, _ := json.Marshal(createShortLinkRequest{TargetURL: "https://example.com"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/shortener/links", bytes.NewReader(body))
	req = req.WithContext(tester.ContextWithUser(testUserID, "user", "u@e.com", "sess"))
	rr := httptest.NewRecorder()

	h.CreateShortLink(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	mockRepo.AssertExpectations(t)
}

// ---- ListShortLinks ----

func TestListShortLinks_Success(t *testing.T) {
	mockRepo := mocks.NewMockShortLinkRepository(t)
	h := newHandler(mockRepo)

	links := []model.ShortLink{makeLink("aaa1111"), makeLink("bbb2222")}
	mockRepo.On("ListByOwner", mock.Anything, testUserID, uint32(1), uint32(20)).
		Return(links, uint32(2), nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/shortener/links", nil)
	req = req.WithContext(tester.ContextWithUser(testUserID, "user", "u@e.com", "sess"))
	rr := httptest.NewRecorder()

	h.ListShortLinks(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp httpHelper.Response
	assert.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.True(t, resp.Success)
	mockRepo.AssertExpectations(t)
}

// ---- GetShortLink ----

func TestGetShortLink_Success(t *testing.T) {
	mockRepo := mocks.NewMockShortLinkRepository(t)
	h := newHandler(mockRepo)

	link := makeLink("abc1234")
	mockRepo.On("GetByCodeOwned", mock.Anything, "abc1234", testUserID).Return(link, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/shortener/links/abc1234", nil)
	req = req.WithContext(tester.ContextWithUser(testUserID, "user", "u@e.com", "sess"))
	req = mux.SetURLVars(req, map[string]string{"code": "abc1234"})
	rr := httptest.NewRecorder()

	h.GetShortLink(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	mockRepo.AssertExpectations(t)
}

func TestGetShortLink_OwnershipMismatch(t *testing.T) {
	mockRepo := mocks.NewMockShortLinkRepository(t)
	h := newHandler(mockRepo)

	mockRepo.On("GetByCodeOwned", mock.Anything, "abc1234", testUserID).Return(model.ShortLink{}, httpHelper.ErrNotFound)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/shortener/links/abc1234", nil)
	req = req.WithContext(tester.ContextWithUser(testUserID, "user", "u@e.com", "sess"))
	req = mux.SetURLVars(req, map[string]string{"code": "abc1234"})
	rr := httptest.NewRecorder()

	h.GetShortLink(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
	mockRepo.AssertExpectations(t)
}

func TestGetShortLink_NotFound(t *testing.T) {
	mockRepo := mocks.NewMockShortLinkRepository(t)
	h := newHandler(mockRepo)

	mockRepo.On("GetByCodeOwned", mock.Anything, "notexist", testUserID).Return(model.ShortLink{}, httpHelper.ErrNotFound)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/shortener/links/notexist", nil)
	req = req.WithContext(tester.ContextWithUser(testUserID, "user", "u@e.com", "sess"))
	req = mux.SetURLVars(req, map[string]string{"code": "notexist"})
	rr := httptest.NewRecorder()

	h.GetShortLink(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// ---- DeleteShortLink ----

func TestDeleteShortLink_Success(t *testing.T) {
	mockRepo := mocks.NewMockShortLinkRepository(t)
	h := newHandler(mockRepo)

	mockRepo.On("Delete", mock.Anything, "abc1234", testUserID).Return(nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/shortener/links/abc1234", nil)
	req = req.WithContext(tester.ContextWithUser(testUserID, "user", "u@e.com", "sess"))
	req = mux.SetURLVars(req, map[string]string{"code": "abc1234"})
	rr := httptest.NewRecorder()

	h.DeleteShortLink(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	mockRepo.AssertExpectations(t)
}

func TestDeleteShortLink_NotFound(t *testing.T) {
	mockRepo := mocks.NewMockShortLinkRepository(t)
	h := newHandler(mockRepo)

	mockRepo.On("Delete", mock.Anything, "gone", testUserID).Return(httpHelper.ErrNotFound)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/shortener/links/gone", nil)
	req = req.WithContext(tester.ContextWithUser(testUserID, "user", "u@e.com", "sess"))
	req = mux.SetURLVars(req, map[string]string{"code": "gone"})
	rr := httptest.NewRecorder()

	h.DeleteShortLink(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// ---- ResolveShortLink ----

func TestResolveShortLink_Success(t *testing.T) {
	mockRepo := mocks.NewMockShortLinkRepository(t)
	h := newHandler(mockRepo)

	link := makeLink("abc1234")
	mockRepo.On("GetByCode", mock.Anything, "abc1234").Return(link, nil)
	mockRepo.On("IncrementClick", mock.Anything, "abc1234", mock.Anything).Return(nil)

	req := httptest.NewRequest(http.MethodGet, "/r/abc1234", nil)
	req = req.WithContext(tester.ContextWithLogger())
	req = mux.SetURLVars(req, map[string]string{"code": "abc1234"})
	rr := httptest.NewRecorder()

	h.ResolveShortLink(rr, req)

	assert.Equal(t, http.StatusFound, rr.Code)
	assert.Equal(t, "https://example.com", rr.Header().Get("Location"))
	assert.Equal(t, "private, no-store", rr.Header().Get("Cache-Control"))
	mockRepo.AssertExpectations(t)
}

func TestResolveShortLink_Disabled(t *testing.T) {
	mockRepo := mocks.NewMockShortLinkRepository(t)
	h := newHandler(mockRepo)

	link := makeLink("abc1234")
	link.IsActive = false
	mockRepo.On("GetByCode", mock.Anything, "abc1234").Return(link, nil)

	req := httptest.NewRequest(http.MethodGet, "/r/abc1234", nil)
	req = req.WithContext(tester.ContextWithLogger())
	req = mux.SetURLVars(req, map[string]string{"code": "abc1234"})
	rr := httptest.NewRecorder()

	h.ResolveShortLink(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestResolveShortLink_Expired(t *testing.T) {
	mockRepo := mocks.NewMockShortLinkRepository(t)
	h := newHandler(mockRepo)

	past := time.Now().Add(-1 * time.Hour)
	link := makeLink("abc1234")
	link.ExpiresAt = &past
	mockRepo.On("GetByCode", mock.Anything, "abc1234").Return(link, nil)

	req := httptest.NewRequest(http.MethodGet, "/r/abc1234", nil)
	req = req.WithContext(tester.ContextWithLogger())
	req = mux.SetURLVars(req, map[string]string{"code": "abc1234"})
	rr := httptest.NewRecorder()

	h.ResolveShortLink(rr, req)

	assert.Equal(t, http.StatusGone, rr.Code)
}

func TestResolveShortLink_NotFound(t *testing.T) {
	mockRepo := mocks.NewMockShortLinkRepository(t)
	h := newHandler(mockRepo)

	mockRepo.On("GetByCode", mock.Anything, "nope").Return(model.ShortLink{}, httpHelper.ErrNotFound)

	req := httptest.NewRequest(http.MethodGet, "/r/nope", nil)
	req = req.WithContext(tester.ContextWithLogger())
	req = mux.SetURLVars(req, map[string]string{"code": "nope"})
	rr := httptest.NewRecorder()

	h.ResolveShortLink(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

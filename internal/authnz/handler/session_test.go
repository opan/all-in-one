package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/all-in-one/internal/auth"
	"github.com/all-in-one/internal/authnz/handler/mocks"
	"github.com/all-in-one/internal/authnz/model"
	"github.com/all-in-one/internal/config"
	"github.com/all-in-one/internal/tester"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockTransaction implements query.QueryOptions for testing
type MockTransaction struct{}

func (m *MockTransaction) Commit() error {
	return nil
}

func (m *MockTransaction) Rollback() error {
	return nil
}

func TestCreateSession_Success(t *testing.T) {
	username := "testuser"
	now := time.Now().UTC()
	pwd, _ := auth.HashPassword("random")

	expectedUser := model.User{
		ID:           uuid.New(),
		Username:     username,
		Email:        "test@example.com",
		LastLogin:    &now,
		PasswordHash: pwd,
	}

	mockUserRepo := mocks.NewMockUserRepository(t)
	mockUserRepo.On("FindByUsername", mock.Anything, username).Return(expectedUser, nil)

	mockTrx := &MockTransaction{}
	mockSessionRepo := mocks.NewMockSessionRepository(t)
	mockSessionRepo.On("CreateTrx", mock.Anything).Return(mockTrx, nil)
	mockSessionRepo.On("Create", mock.Anything, mock.AnythingOfType("model.Session"), mock.Anything).Return(nil)

	storage := &MockStorage{
		userRepo:    mockUserRepo,
		sessionRepo: mockSessionRepo,
	}

	Handler := NewHandler(storage, config.Config{})

	reqBody := map[string]string{
		"username": username,
		"password": "random",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/sessions", bytes.NewReader(body))
	req = req.WithContext(tester.ContextWithLogger())

	rr := httptest.NewRecorder()
	Handler.CreateSession(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
}

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

func TestCreateSession_InvalidJSON(t *testing.T) {
	mockUserRepo := mocks.NewMockUserRepository(t)
	mockSessionRepo := mocks.NewMockSessionRepository(t)

	storage := &MockStorage{
		userRepo:    mockUserRepo,
		sessionRepo: mockSessionRepo,
	}

	handler := NewHandler(storage, config.Config{})

	req := httptest.NewRequest(http.MethodPost, "/sessions", bytes.NewReader([]byte("invalid json")))
	req = req.WithContext(tester.ContextWithLogger())

	rr := httptest.NewRecorder()
	handler.CreateSession(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestCreateSession_UserNotFound(t *testing.T) {
	username := "nonexistent"

	mockUserRepo := mocks.NewMockUserRepository(t)
	mockUserRepo.On("FindByUsername", mock.Anything, username).Return(model.User{}, assert.AnError)

	mockSessionRepo := mocks.NewMockSessionRepository(t)

	storage := &MockStorage{
		userRepo:    mockUserRepo,
		sessionRepo: mockSessionRepo,
	}

	handler := NewHandler(storage, config.Config{})

	reqBody := map[string]string{
		"username": username,
		"password": "password",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/sessions", bytes.NewReader(body))
	req = req.WithContext(tester.ContextWithLogger())

	rr := httptest.NewRecorder()
	handler.CreateSession(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestCreateSession_InvalidPassword(t *testing.T) {
	username := "testuser"
	now := time.Now().UTC()
	pwd, _ := auth.HashPassword("correctpassword")

	expectedUser := model.User{
		ID:           uuid.New(),
		Username:     username,
		Email:        "test@example.com",
		LastLogin:    &now,
		PasswordHash: pwd,
	}

	mockUserRepo := mocks.NewMockUserRepository(t)
	mockUserRepo.On("FindByUsername", mock.Anything, username).Return(expectedUser, nil)

	mockSessionRepo := mocks.NewMockSessionRepository(t)

	storage := &MockStorage{
		userRepo:    mockUserRepo,
		sessionRepo: mockSessionRepo,
	}

	handler := NewHandler(storage, config.Config{})

	reqBody := map[string]string{
		"username": username,
		"password": "wrongpassword",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/sessions", bytes.NewReader(body))
	req = req.WithContext(tester.ContextWithLogger())

	rr := httptest.NewRecorder()
	handler.CreateSession(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestDeleteSession_Success(t *testing.T) {
	sessionID := uuid.New()
	userID := uuid.New()

	mockUserRepo := mocks.NewMockUserRepository(t)
	mockSessionRepo := mocks.NewMockSessionRepository(t)
	mockSessionRepo.On("Delete", mock.Anything, sessionID).Return(nil)

	storage := &MockStorage{
		userRepo:    mockUserRepo,
		sessionRepo: mockSessionRepo,
	}

	cfg := config.Config{
		Auth: config.Auth{
			JWTSecret: "test-secret-key",
		},
	}
	handler := NewHandler(storage, cfg)

	// Create a valid access token
	accessToken, _ := handler.createAccessToken(sessionID, model.User{
		ID:       userID,
		Username: "testuser",
		Email:    "test@example.com",
	})

	req := httptest.NewRequest(http.MethodDelete, "/sessions", nil)
	req.AddCookie(&http.Cookie{
		Name:  "access_token",
		Value: accessToken,
	})
	req = req.WithContext(tester.ContextWithLogger())

	rr := httptest.NewRecorder()
	handler.DeleteSession(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	mockSessionRepo.AssertExpectations(t)
}

func TestDeleteSession_MissingToken(t *testing.T) {
	mockUserRepo := mocks.NewMockUserRepository(t)
	mockSessionRepo := mocks.NewMockSessionRepository(t)

	storage := &MockStorage{
		userRepo:    mockUserRepo,
		sessionRepo: mockSessionRepo,
	}

	handler := NewHandler(storage, config.Config{})

	req := httptest.NewRequest(http.MethodDelete, "/sessions", nil)
	req = req.WithContext(tester.ContextWithLogger())

	rr := httptest.NewRecorder()
	handler.DeleteSession(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestDeleteSession_InvalidToken(t *testing.T) {
	mockUserRepo := mocks.NewMockUserRepository(t)
	mockSessionRepo := mocks.NewMockSessionRepository(t)

	storage := &MockStorage{
		userRepo:    mockUserRepo,
		sessionRepo: mockSessionRepo,
	}

	handler := NewHandler(storage, config.Config{
		Auth: config.Auth{
			JWTSecret: "test-secret-key",
		},
	})

	req := httptest.NewRequest(http.MethodDelete, "/sessions", nil)
	req.AddCookie(&http.Cookie{
		Name:  "access_token",
		Value: "invalid-token",
	})
	req = req.WithContext(tester.ContextWithLogger())

	rr := httptest.NewRecorder()
	handler.DeleteSession(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestDeleteSession_DeleteError(t *testing.T) {
	sessionID := uuid.New()
	userID := uuid.New()

	mockUserRepo := mocks.NewMockUserRepository(t)
	mockSessionRepo := mocks.NewMockSessionRepository(t)
	mockSessionRepo.On("Delete", mock.Anything, sessionID).Return(assert.AnError)

	storage := &MockStorage{
		userRepo:    mockUserRepo,
		sessionRepo: mockSessionRepo,
	}

	cfg := config.Config{
		Auth: config.Auth{
			JWTSecret: "test-secret-key",
		},
	}
	handler := NewHandler(storage, cfg)

	accessToken, _ := handler.createAccessToken(sessionID, model.User{
		ID:       userID,
		Username: "testuser",
		Email:    "test@example.com",
	})

	req := httptest.NewRequest(http.MethodDelete, "/sessions", nil)
	req.AddCookie(&http.Cookie{
		Name:  "access_token",
		Value: accessToken,
	})
	req = req.WithContext(tester.ContextWithLogger())

	rr := httptest.NewRecorder()
	handler.DeleteSession(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestRefreshToken_Success(t *testing.T) {
	sessionID := uuid.New()
	userID := uuid.New()
	now := time.Now().UTC()

	user := model.User{
		ID:       userID,
		Username: "testuser",
		Email:    "test@example.com",
	}

	session := model.Session{
		ID:                 sessionID,
		UserID:             userID,
		CreatedAt:          now,
		AccessTokenExpiry:  int(now.Add(-5 * time.Minute).Unix()),   // Expired
		RefreshTokenExpiry: int(now.Add(7 * 24 * time.Hour).Unix()), // Still valid
	}

	mockUserRepo := mocks.NewMockUserRepository(t)
	mockUserRepo.On("Find", mock.Anything, userID).Return(user, nil)

	mockSessionRepo := mocks.NewMockSessionRepository(t)
	mockSessionRepo.On("Get", mock.Anything, sessionID).Return(&session, nil)

	storage := &MockStorage{
		userRepo:    mockUserRepo,
		sessionRepo: mockSessionRepo,
	}

	cfg := config.Config{
		Auth: config.Auth{
			JWTSecret: "test-secret-key",
		},
	}
	handler := NewHandler(storage, cfg)

	refreshToken, _ := handler.createRefreshToken(sessionID)

	req := httptest.NewRequest(http.MethodPost, "/sessions/refresh", nil)
	req.AddCookie(&http.Cookie{
		Name:  "refresh_token",
		Value: refreshToken,
	})
	req = req.WithContext(tester.ContextWithLogger())

	rr := httptest.NewRecorder()
	handler.RefreshToken(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestRefreshToken_MissingToken(t *testing.T) {
	mockUserRepo := mocks.NewMockUserRepository(t)
	mockSessionRepo := mocks.NewMockSessionRepository(t)

	storage := &MockStorage{
		userRepo:    mockUserRepo,
		sessionRepo: mockSessionRepo,
	}

	handler := NewHandler(storage, config.Config{})

	req := httptest.NewRequest(http.MethodPost, "/sessions/refresh", nil)
	req = req.WithContext(tester.ContextWithLogger())

	rr := httptest.NewRecorder()
	handler.RefreshToken(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestRefreshToken_InvalidToken(t *testing.T) {
	mockUserRepo := mocks.NewMockUserRepository(t)
	mockSessionRepo := mocks.NewMockSessionRepository(t)

	storage := &MockStorage{
		userRepo:    mockUserRepo,
		sessionRepo: mockSessionRepo,
	}

	handler := NewHandler(storage, config.Config{
		Auth: config.Auth{
			JWTSecret: "test-secret-key",
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/sessions/refresh", nil)
	req.AddCookie(&http.Cookie{
		Name:  "refresh_token",
		Value: "invalid-token",
	})
	req = req.WithContext(tester.ContextWithLogger())

	rr := httptest.NewRecorder()
	handler.RefreshToken(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestRefreshToken_SessionNotFound(t *testing.T) {
	sessionID := uuid.New()

	mockUserRepo := mocks.NewMockUserRepository(t)
	mockSessionRepo := mocks.NewMockSessionRepository(t)
	mockSessionRepo.On("Get", mock.Anything, sessionID).Return((*model.Session)(nil), assert.AnError)

	storage := &MockStorage{
		userRepo:    mockUserRepo,
		sessionRepo: mockSessionRepo,
	}

	cfg := config.Config{
		Auth: config.Auth{
			JWTSecret: "test-secret-key",
		},
	}
	handler := NewHandler(storage, cfg)

	refreshToken, _ := handler.createRefreshToken(sessionID)

	req := httptest.NewRequest(http.MethodPost, "/sessions/refresh", nil)
	req.AddCookie(&http.Cookie{
		Name:  "refresh_token",
		Value: refreshToken,
	})
	req = req.WithContext(tester.ContextWithLogger())

	rr := httptest.NewRecorder()
	handler.RefreshToken(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestRefreshToken_AccessTokenStillValid(t *testing.T) {
	sessionID := uuid.New()
	userID := uuid.New()
	now := time.Now().UTC()

	session := model.Session{
		ID:                 sessionID,
		UserID:             userID,
		CreatedAt:          now,
		AccessTokenExpiry:  int(now.Add(10 * time.Minute).Unix()), // Still valid
		RefreshTokenExpiry: int(now.Add(7 * 24 * time.Hour).Unix()),
	}

	mockUserRepo := mocks.NewMockUserRepository(t)
	mockSessionRepo := mocks.NewMockSessionRepository(t)
	mockSessionRepo.On("Get", mock.Anything, sessionID).Return(&session, nil)

	storage := &MockStorage{
		userRepo:    mockUserRepo,
		sessionRepo: mockSessionRepo,
	}

	cfg := config.Config{
		Auth: config.Auth{
			JWTSecret: "test-secret-key",
		},
	}
	handler := NewHandler(storage, cfg)

	refreshToken, _ := handler.createRefreshToken(sessionID)

	req := httptest.NewRequest(http.MethodPost, "/sessions/refresh", nil)
	req.AddCookie(&http.Cookie{
		Name:  "refresh_token",
		Value: refreshToken,
	})
	req = req.WithContext(tester.ContextWithLogger())

	rr := httptest.NewRecorder()
	handler.RefreshToken(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestRefreshToken_RefreshTokenExpired(t *testing.T) {
	sessionID := uuid.New()
	userID := uuid.New()
	now := time.Now().UTC()

	session := model.Session{
		ID:                 sessionID,
		UserID:             userID,
		CreatedAt:          now,
		AccessTokenExpiry:  int(now.Add(-10 * time.Minute).Unix()), // Expired
		RefreshTokenExpiry: int(now.Add(-1 * time.Hour).Unix()),    // Also expired
	}

	mockUserRepo := mocks.NewMockUserRepository(t)
	mockSessionRepo := mocks.NewMockSessionRepository(t)
	mockSessionRepo.On("Get", mock.Anything, sessionID).Return(&session, nil)

	storage := &MockStorage{
		userRepo:    mockUserRepo,
		sessionRepo: mockSessionRepo,
	}

	cfg := config.Config{
		Auth: config.Auth{
			JWTSecret: "test-secret-key",
		},
	}
	handler := NewHandler(storage, cfg)

	refreshToken, _ := handler.createRefreshToken(sessionID)

	req := httptest.NewRequest(http.MethodPost, "/sessions/refresh", nil)
	req.AddCookie(&http.Cookie{
		Name:  "refresh_token",
		Value: refreshToken,
	})
	req = req.WithContext(tester.ContextWithLogger())

	rr := httptest.NewRecorder()
	handler.RefreshToken(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestRefreshToken_UserNotFound(t *testing.T) {
	sessionID := uuid.New()
	userID := uuid.New()
	now := time.Now().UTC()

	session := model.Session{
		ID:                 sessionID,
		UserID:             userID,
		CreatedAt:          now,
		AccessTokenExpiry:  int(now.Add(-5 * time.Minute).Unix()),
		RefreshTokenExpiry: int(now.Add(7 * 24 * time.Hour).Unix()),
	}

	mockUserRepo := mocks.NewMockUserRepository(t)
	mockUserRepo.On("Find", mock.Anything, userID).Return(model.User{}, assert.AnError)

	mockSessionRepo := mocks.NewMockSessionRepository(t)
	mockSessionRepo.On("Get", mock.Anything, sessionID).Return(&session, nil)

	storage := &MockStorage{
		userRepo:    mockUserRepo,
		sessionRepo: mockSessionRepo,
	}

	cfg := config.Config{
		Auth: config.Auth{
			JWTSecret: "test-secret-key",
		},
	}
	handler := NewHandler(storage, cfg)

	refreshToken, _ := handler.createRefreshToken(sessionID)

	req := httptest.NewRequest(http.MethodPost, "/sessions/refresh", nil)
	req.AddCookie(&http.Cookie{
		Name:  "refresh_token",
		Value: refreshToken,
	})
	req = req.WithContext(tester.ContextWithLogger())

	rr := httptest.NewRecorder()
	handler.RefreshToken(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestVerifySession_Success(t *testing.T) {
	sessionID := uuid.New()
	userID := uuid.New()

	mockUserRepo := mocks.NewMockUserRepository(t)
	mockSessionRepo := mocks.NewMockSessionRepository(t)

	storage := &MockStorage{
		userRepo:    mockUserRepo,
		sessionRepo: mockSessionRepo,
	}

	cfg := config.Config{
		Auth: config.Auth{
			JWTSecret: "test-secret-key",
		},
	}
	handler := NewHandler(storage, cfg)

	accessToken, _ := handler.createAccessToken(sessionID, model.User{
		ID:       userID,
		Username: "testuser",
		Email:    "test@example.com",
	})

	req := httptest.NewRequest(http.MethodGet, "/sessions/verify", nil)
	req.AddCookie(&http.Cookie{
		Name:  "access_token",
		Value: accessToken,
	})
	req = req.WithContext(tester.ContextWithLogger())

	rr := httptest.NewRecorder()
	handler.VerifySession(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestVerifySession_MissingToken(t *testing.T) {
	mockUserRepo := mocks.NewMockUserRepository(t)
	mockSessionRepo := mocks.NewMockSessionRepository(t)

	storage := &MockStorage{
		userRepo:    mockUserRepo,
		sessionRepo: mockSessionRepo,
	}

	handler := NewHandler(storage, config.Config{})

	req := httptest.NewRequest(http.MethodGet, "/sessions/verify", nil)
	req = req.WithContext(tester.ContextWithLogger())

	rr := httptest.NewRecorder()
	handler.VerifySession(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestVerifySession_InvalidToken(t *testing.T) {
	mockUserRepo := mocks.NewMockUserRepository(t)
	mockSessionRepo := mocks.NewMockSessionRepository(t)

	storage := &MockStorage{
		userRepo:    mockUserRepo,
		sessionRepo: mockSessionRepo,
	}

	handler := NewHandler(storage, config.Config{
		Auth: config.Auth{
			JWTSecret: "test-secret-key",
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/sessions/verify", nil)
	req.AddCookie(&http.Cookie{
		Name:  "access_token",
		Value: "invalid-token",
	})
	req = req.WithContext(tester.ContextWithLogger())

	rr := httptest.NewRecorder()
	handler.VerifySession(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

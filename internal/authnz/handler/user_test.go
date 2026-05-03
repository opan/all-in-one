package handler

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/all-in-one/internal/auth"
	"github.com/all-in-one/internal/authnz/handler/mocks"
	"github.com/all-in-one/internal/authnz/model"
	"github.com/all-in-one/internal/authnz/repository"
	"github.com/all-in-one/internal/config"
	httpHelper "github.com/all-in-one/internal/http"
	"github.com/all-in-one/internal/tester"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockStorage is a wrapper for generated mocks
type MockStorage struct {
	userRepo    repository.UserRepository
	sessionRepo repository.SessionRepository
	totpRepo    repository.TOTPRepository
}

func (m *MockStorage) UserRepo() repository.UserRepository {
	return m.userRepo
}

func (m *MockStorage) SessionRepo() repository.SessionRepository {
	return m.sessionRepo
}

func (m *MockStorage) TOTPRepo() repository.TOTPRepository {
	return m.totpRepo
}

func (m *MockStorage) Close() error {
	return nil
}

func TestGetCurrentUser_Success(t *testing.T) {
	userID := uuid.New()
	now := time.Now().UTC()
	expectedUser := model.User{
		ID:        userID,
		Username:  "testuser",
		Email:     "test@example.com",
		Name:      "Test User",
		LastLogin: &now,
	}

	mockUserRepo := mocks.NewMockUserRepository(t)
	mockUserRepo.On("Find", mock.Anything, userID).Return(expectedUser, nil)

	storage := &MockStorage{
		userRepo:    mockUserRepo,
		sessionRepo: mocks.NewMockSessionRepository(t),
	}

	handler := NewHandler(storage, config.Config{})

	req := httptest.NewRequest(http.MethodGet, "/users/me", nil)
	ctx := tester.ContextWithUser(userID.String(), "testuser", "test@example.com", uuid.New().String())
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()

	handler.GetCurrentUser(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response httpHelper.Response
	err := json.NewDecoder(rr.Body).Decode(&response)
	assert.NoError(t, err)
	assert.True(t, response.Success)

	// Verify user data
	userData, err := json.Marshal(response.Data)
	assert.NoError(t, err)

	var user model.User
	err = json.Unmarshal(userData, &user)
	assert.NoError(t, err)
	assert.Equal(t, expectedUser.ID, user.ID)
	assert.Equal(t, expectedUser.Username, user.Username)

	mockUserRepo.AssertExpectations(t)
}

func TestGetCurrentUser_NoUserInContext(t *testing.T) {
	storage := &MockStorage{
		userRepo:    mocks.NewMockUserRepository(t),
		sessionRepo: mocks.NewMockSessionRepository(t),
	}

	handler := NewHandler(storage, config.Config{})

	req := httptest.NewRequest(http.MethodGet, "/users/me", nil)
	req = req.WithContext(tester.ContextWithLogger())

	rr := httptest.NewRecorder()

	handler.GetCurrentUser(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)

	var response httpHelper.Response
	err := json.NewDecoder(rr.Body).Decode(&response)
	assert.NoError(t, err)
	assert.False(t, response.Success)
}

func TestGetCurrentUser_InvalidUUID(t *testing.T) {
	storage := &MockStorage{
		userRepo:    mocks.NewMockUserRepository(t),
		sessionRepo: mocks.NewMockSessionRepository(t),
	}

	handler := NewHandler(storage, config.Config{})

	req := httptest.NewRequest(http.MethodGet, "/users/me", nil)
	ctx := tester.ContextWithUser("invalid-uuid", "testuser", "test@example.com", uuid.New().String())
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()

	handler.GetCurrentUser(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestGetCurrentUser_UserNotFoundInDB(t *testing.T) {
	userID := uuid.New()

	mockUserRepo := mocks.NewMockUserRepository(t)
	mockUserRepo.On("Find", mock.Anything, userID).Return(model.User{}, sql.ErrNoRows)

	storage := &MockStorage{
		userRepo:    mockUserRepo,
		sessionRepo: mocks.NewMockSessionRepository(t),
	}

	handler := NewHandler(storage, config.Config{})

	req := httptest.NewRequest(http.MethodGet, "/users/me", nil)
	ctx := tester.ContextWithUser(userID.String(), "testuser", "test@example.com", uuid.New().String())
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()

	handler.GetCurrentUser(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	mockUserRepo.AssertExpectations(t)
}

func TestRegisterUser_Success(t *testing.T) {
	mockUserRepo := mocks.NewMockUserRepository(t)
	mockUserRepo.On("FindByUsername", mock.Anything, "newuser").Return(model.User{}, sql.ErrNoRows)
	mockUserRepo.On("Create", mock.Anything, mock.AnythingOfType("model.User"), mock.Anything).Return(nil)

	storage := &MockStorage{
		userRepo:    mockUserRepo,
		sessionRepo: mocks.NewMockSessionRepository(t),
	}

	handler := NewHandler(storage, config.Config{})

	reqBody := RegisterUserRequest{
		Username: "newuser",
		Password: "password123",
		Email:    "newuser@example.com",
		Name:     "New User",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/users/register", bytes.NewReader(body))
	req = req.WithContext(tester.ContextWithLogger())

	rr := httptest.NewRecorder()

	handler.RegisterUser(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)

	var response httpHelper.Response
	err := json.NewDecoder(rr.Body).Decode(&response)
	assert.NoError(t, err)
	assert.True(t, response.Success)
	assert.Equal(t, "user created successfully", response.Message)

	mockUserRepo.AssertExpectations(t)
}

func TestRegisterUser_InvalidJSON(t *testing.T) {
	storage := &MockStorage{
		userRepo:    mocks.NewMockUserRepository(t),
		sessionRepo: mocks.NewMockSessionRepository(t),
	}

	handler := NewHandler(storage, config.Config{})

	req := httptest.NewRequest(http.MethodPost, "/users/register", bytes.NewReader([]byte("invalid json")))
	req = req.WithContext(tester.ContextWithLogger())

	rr := httptest.NewRecorder()

	handler.RegisterUser(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestRegisterUser_MissingUsername(t *testing.T) {
	storage := &MockStorage{
		userRepo:    mocks.NewMockUserRepository(t),
		sessionRepo: mocks.NewMockSessionRepository(t),
	}

	handler := NewHandler(storage, config.Config{})

	reqBody := RegisterUserRequest{
		Username: "",
		Password: "password123",
		Email:    "test@example.com",
		Name:     "Test User",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/users/register", bytes.NewReader(body))
	req = req.WithContext(tester.ContextWithLogger())

	rr := httptest.NewRecorder()

	handler.RegisterUser(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var response httpHelper.Response
	err := json.NewDecoder(rr.Body).Decode(&response)
	assert.NoError(t, err)
	assert.Equal(t, "username and password are required", response.Error)
}

func TestRegisterUser_MissingPassword(t *testing.T) {
	storage := &MockStorage{
		userRepo:    mocks.NewMockUserRepository(t),
		sessionRepo: mocks.NewMockSessionRepository(t),
	}

	handler := NewHandler(storage, config.Config{})

	reqBody := RegisterUserRequest{
		Username: "testuser",
		Password: "",
		Email:    "test@example.com",
		Name:     "Test User",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/users/register", bytes.NewReader(body))
	req = req.WithContext(tester.ContextWithLogger())

	rr := httptest.NewRecorder()

	handler.RegisterUser(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestRegisterUser_UserAlreadyExists(t *testing.T) {
	existingUserID := uuid.New()
	mockUserRepo := mocks.NewMockUserRepository(t)
	mockUserRepo.On("FindByUsername", mock.Anything, "existinguser").Return(model.User{
		ID:       existingUserID,
		Username: "existinguser",
	}, nil)

	storage := &MockStorage{
		userRepo:    mockUserRepo,
		sessionRepo: mocks.NewMockSessionRepository(t),
	}

	handler := NewHandler(storage, config.Config{})

	reqBody := RegisterUserRequest{
		Username: "existinguser",
		Password: "password123",
		Email:    "test@example.com",
		Name:     "Test User",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/users/register", bytes.NewReader(body))
	req = req.WithContext(tester.ContextWithLogger())

	rr := httptest.NewRecorder()

	handler.RegisterUser(rr, req)

	assert.Equal(t, http.StatusConflict, rr.Code)

	var response httpHelper.Response
	err := json.NewDecoder(rr.Body).Decode(&response)
	assert.NoError(t, err)
	assert.Equal(t, "user already exists", response.Error)

	mockUserRepo.AssertExpectations(t)
}

func TestRegisterUser_DatabaseError(t *testing.T) {
	mockUserRepo := mocks.NewMockUserRepository(t)
	mockUserRepo.On("FindByUsername", mock.Anything, "newuser").Return(model.User{}, errors.New("database connection error"))

	storage := &MockStorage{
		userRepo:    mockUserRepo,
		sessionRepo: mocks.NewMockSessionRepository(t),
	}

	handler := NewHandler(storage, config.Config{})

	reqBody := RegisterUserRequest{
		Username: "newuser",
		Password: "password123",
		Email:    "test@example.com",
		Name:     "Test User",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/users/register", bytes.NewReader(body))
	req = req.WithContext(tester.ContextWithLogger())

	rr := httptest.NewRecorder()

	handler.RegisterUser(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	mockUserRepo.AssertExpectations(t)
}

func TestRegisterUser_CreateError(t *testing.T) {
	mockUserRepo := mocks.NewMockUserRepository(t)
	mockUserRepo.On("FindByUsername", mock.Anything, "newuser").Return(model.User{}, sql.ErrNoRows)
	mockUserRepo.On("Create", mock.Anything, mock.AnythingOfType("model.User"), mock.Anything).Return(errors.New("failed to insert user"))

	storage := &MockStorage{
		userRepo:    mockUserRepo,
		sessionRepo: mocks.NewMockSessionRepository(t),
	}

	handler := NewHandler(storage, config.Config{})

	reqBody := RegisterUserRequest{
		Username: "newuser",
		Password: "password123",
		Email:    "test@example.com",
		Name:     "Test User",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/users/register", bytes.NewReader(body))
	req = req.WithContext(tester.ContextWithLogger())

	rr := httptest.NewRecorder()

	handler.RegisterUser(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	mockUserRepo.AssertExpectations(t)
}

func TestResetPasswordUser_Success(t *testing.T) {
	userID := uuid.New()
	hashedPassword, _ := auth.HashPassword("oldpassword")

	mockUserRepo := mocks.NewMockUserRepository(t)
	mockUserRepo.On("Find", mock.Anything, userID).Return(model.User{
		ID:           userID,
		Username:     "testuser",
		PasswordHash: hashedPassword,
	}, nil)
	mockUserRepo.On("Update", mock.Anything, userID, mock.AnythingOfType("model.User"), mock.Anything).Return(nil)

	storage := &MockStorage{
		userRepo:    mockUserRepo,
		sessionRepo: mocks.NewMockSessionRepository(t),
	}

	handler := NewHandler(storage, config.Config{})

	reqBody := model.UserPasswordReset{
		CurrentPassword: "oldpassword",
		Password:        "newpassword123",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPut, "/users/me/password", bytes.NewReader(body))
	ctx := tester.ContextWithUser(userID.String(), "testuser", "test@example.com", uuid.New().String())
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()

	handler.ResetPasswordUser(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response httpHelper.Response
	err := json.NewDecoder(rr.Body).Decode(&response)
	assert.NoError(t, err)
	assert.True(t, response.Success)

	mockUserRepo.AssertExpectations(t)
}

func TestResetPasswordUser_NoUserInContext(t *testing.T) {
	storage := &MockStorage{
		userRepo:    mocks.NewMockUserRepository(t),
		sessionRepo: mocks.NewMockSessionRepository(t),
	}

	handler := NewHandler(storage, config.Config{})

	reqBody := model.UserPasswordReset{
		CurrentPassword: "oldpassword",
		Password:        "newpassword123",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPut, "/users/me/password", bytes.NewReader(body))
	req = req.WithContext(tester.ContextWithLogger())

	rr := httptest.NewRecorder()

	handler.ResetPasswordUser(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestResetPasswordUser_InvalidJSON(t *testing.T) {
	userID := uuid.New()

	storage := &MockStorage{
		userRepo:    mocks.NewMockUserRepository(t),
		sessionRepo: mocks.NewMockSessionRepository(t),
	}

	handler := NewHandler(storage, config.Config{})

	req := httptest.NewRequest(http.MethodPut, "/users/me/password", bytes.NewReader([]byte("invalid json")))
	ctx := tester.ContextWithUser(userID.String(), "testuser", "test@example.com", uuid.New().String())
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()

	handler.ResetPasswordUser(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestResetPasswordUser_InvalidCurrentPassword(t *testing.T) {
	userID := uuid.New()
	hashedPassword, _ := auth.HashPassword("correctpassword")

	mockUserRepo := mocks.NewMockUserRepository(t)
	mockUserRepo.On("Find", mock.Anything, userID).Return(model.User{
		ID:           userID,
		Username:     "testuser",
		PasswordHash: hashedPassword,
	}, nil)

	storage := &MockStorage{
		userRepo:    mockUserRepo,
		sessionRepo: mocks.NewMockSessionRepository(t),
	}

	handler := NewHandler(storage, config.Config{})

	reqBody := model.UserPasswordReset{
		CurrentPassword: "wrongpassword",
		Password:        "newpassword123",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPut, "/users/me/password", bytes.NewReader(body))
	ctx := tester.ContextWithUser(userID.String(), "testuser", "test@example.com", uuid.New().String())
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()

	handler.ResetPasswordUser(rr, req)

	// Note: The current handler implementation treats bcrypt.ErrMismatchedHashAndPassword as an internal error
	// This returns 500 instead of 401. This test reflects the current behavior.
	assert.Equal(t, http.StatusInternalServerError, rr.Code)

	var response httpHelper.Response
	err := json.NewDecoder(rr.Body).Decode(&response)
	assert.NoError(t, err)
	assert.Equal(t, "failed to check current password", response.Error)

	mockUserRepo.AssertExpectations(t)
}

func TestResetPasswordUser_UserNotFoundInDB(t *testing.T) {
	userID := uuid.New()

	mockUserRepo := mocks.NewMockUserRepository(t)
	mockUserRepo.On("Find", mock.Anything, userID).Return(model.User{}, sql.ErrNoRows)

	storage := &MockStorage{
		userRepo:    mockUserRepo,
		sessionRepo: mocks.NewMockSessionRepository(t),
	}

	handler := NewHandler(storage, config.Config{})

	reqBody := model.UserPasswordReset{
		CurrentPassword: "oldpassword",
		Password:        "newpassword123",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPut, "/users/me/password", bytes.NewReader(body))
	ctx := tester.ContextWithUser(userID.String(), "testuser", "test@example.com", uuid.New().String())
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()

	handler.ResetPasswordUser(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	mockUserRepo.AssertExpectations(t)
}

func TestResetPasswordUser_UpdateError(t *testing.T) {
	userID := uuid.New()
	hashedPassword, _ := auth.HashPassword("oldpassword")

	mockUserRepo := mocks.NewMockUserRepository(t)
	mockUserRepo.On("Find", mock.Anything, userID).Return(model.User{
		ID:           userID,
		Username:     "testuser",
		PasswordHash: hashedPassword,
	}, nil)
	mockUserRepo.On("Update", mock.Anything, userID, mock.AnythingOfType("model.User"), mock.Anything).Return(errors.New("database update failed"))

	storage := &MockStorage{
		userRepo:    mockUserRepo,
		sessionRepo: mocks.NewMockSessionRepository(t),
	}

	handler := NewHandler(storage, config.Config{})

	reqBody := model.UserPasswordReset{
		CurrentPassword: "oldpassword",
		Password:        "newpassword123",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPut, "/users/me/password", bytes.NewReader(body))
	ctx := tester.ContextWithUser(userID.String(), "testuser", "test@example.com", uuid.New().String())
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()

	handler.ResetPasswordUser(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	mockUserRepo.AssertExpectations(t)
}

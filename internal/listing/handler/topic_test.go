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
	"github.com/all-in-one/internal/listing/handler/mocks"
	"github.com/all-in-one/internal/listing/model"
	"github.com/all-in-one/internal/tester"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestGetTopics_Success(t *testing.T) {
	userID := uuid.New()
	now := time.Now().UTC()

	expectedTopics := []model.Topic{
		{
			ID:        1,
			Name:      "Topic 1",
			UserID:    userID,
			CreatedAt: now,
		},
		{
			ID:        2,
			Name:      "Topic 2",
			UserID:    userID,
			CreatedAt: now,
		},
	}

	mockItemRepo := mocks.NewMockItemRepository(t)
	mockTopicRepo := mocks.NewMockTopicRepository(t)

	mockTopicRepo.On("GetAll", mock.Anything, userID).Return(expectedTopics, nil)

	storage := &MockStorage{
		itemRepo:  mockItemRepo,
		topicRepo: mockTopicRepo,
	}

	handler := NewHandler(storage, config.Config{})

	req := httptest.NewRequest(http.MethodGet, "/topics", nil)
	ctx := tester.ContextWithUser(userID.String(), "testuser", "test@example.com", uuid.New().String())
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()

	handler.GetTopics(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response httpHelper.Response
	err := json.NewDecoder(rr.Body).Decode(&response)
	assert.NoError(t, err)
	assert.True(t, response.Success)

	topicsData, err := json.Marshal(response.Data)
	assert.NoError(t, err)

	var topics []model.Topic
	err = json.Unmarshal(topicsData, &topics)
	assert.NoError(t, err)
	assert.Len(t, topics, 2)
	assert.Equal(t, expectedTopics[0].Name, topics[0].Name)
	assert.Equal(t, expectedTopics[1].Name, topics[1].Name)

	mockTopicRepo.AssertExpectations(t)
}

func TestGetTopics_NoUserInContext(t *testing.T) {
	storage := &MockStorage{
		itemRepo:  mocks.NewMockItemRepository(t),
		topicRepo: mocks.NewMockTopicRepository(t),
	}

	handler := NewHandler(storage, config.Config{})

	req := httptest.NewRequest(http.MethodGet, "/topics", nil)
	ctx := tester.ContextWithLogger()
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()

	handler.GetTopics(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)

	var response httpHelper.Response
	err := json.NewDecoder(rr.Body).Decode(&response)
	assert.NoError(t, err)
	assert.False(t, response.Success)
	assert.NotNil(t, response.Error)
}

func TestGetTopics_InvalidUserID(t *testing.T) {
	storage := &MockStorage{
		itemRepo:  mocks.NewMockItemRepository(t),
		topicRepo: mocks.NewMockTopicRepository(t),
	}

	handler := NewHandler(storage, config.Config{})

	req := httptest.NewRequest(http.MethodGet, "/topics", nil)
	ctx := tester.ContextWithUser("invalid-uuid", "testuser", "test@example.com", uuid.New().String())
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()

	handler.GetTopics(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)

	var response httpHelper.Response
	err := json.NewDecoder(rr.Body).Decode(&response)
	assert.NoError(t, err)
	assert.False(t, response.Success)
}

func TestGetTopics_DatabaseError(t *testing.T) {
	userID := uuid.New()

	mockItemRepo := mocks.NewMockItemRepository(t)
	mockTopicRepo := mocks.NewMockTopicRepository(t)

	mockTopicRepo.On("GetAll", mock.Anything, userID).Return([]model.Topic{}, errors.New("database error"))

	storage := &MockStorage{
		itemRepo:  mockItemRepo,
		topicRepo: mockTopicRepo,
	}

	handler := NewHandler(storage, config.Config{})

	req := httptest.NewRequest(http.MethodGet, "/topics", nil)
	ctx := tester.ContextWithUser(userID.String(), "testuser", "test@example.com", uuid.New().String())
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()

	handler.GetTopics(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)

	mockTopicRepo.AssertExpectations(t)
}

func TestGetTopic_Success(t *testing.T) {
	topicID := 1
	userID := uuid.New()
	now := time.Now().UTC()

	expectedTopic := model.Topic{
		ID:        topicID,
		Name:      "Test Topic",
		UserID:    userID,
		CreatedAt: now,
	}

	mockItemRepo := mocks.NewMockItemRepository(t)
	mockTopicRepo := mocks.NewMockTopicRepository(t)

	mockTopicRepo.On("Get", mock.Anything, topicID).Return(expectedTopic, nil)

	storage := &MockStorage{
		itemRepo:  mockItemRepo,
		topicRepo: mockTopicRepo,
	}

	handler := NewHandler(storage, config.Config{})

	req := httptest.NewRequest(http.MethodGet, "/topics/1", nil)
	ctx := tester.ContextWithLogger()
	req = req.WithContext(ctx)
	req = mux.SetURLVars(req, map[string]string{"id": "1"})

	rr := httptest.NewRecorder()

	handler.GetTopic(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response httpHelper.Response
	err := json.NewDecoder(rr.Body).Decode(&response)
	assert.NoError(t, err)
	assert.True(t, response.Success)

	topicData, err := json.Marshal(response.Data)
	assert.NoError(t, err)

	var topic model.Topic
	err = json.Unmarshal(topicData, &topic)
	assert.NoError(t, err)
	assert.Equal(t, expectedTopic.Name, topic.Name)
	assert.Equal(t, expectedTopic.ID, topic.ID)

	mockTopicRepo.AssertExpectations(t)
}

func TestGetTopic_InvalidID(t *testing.T) {
	storage := &MockStorage{
		itemRepo:  mocks.NewMockItemRepository(t),
		topicRepo: mocks.NewMockTopicRepository(t),
	}

	handler := NewHandler(storage, config.Config{})

	req := httptest.NewRequest(http.MethodGet, "/topics/invalid", nil)
	ctx := tester.ContextWithLogger()
	req = req.WithContext(ctx)
	req = mux.SetURLVars(req, map[string]string{"id": "invalid"})

	rr := httptest.NewRecorder()

	handler.GetTopic(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var response httpHelper.Response
	err := json.NewDecoder(rr.Body).Decode(&response)
	assert.NoError(t, err)
	assert.False(t, response.Success)
	assert.NotNil(t, response.Error)
}

func TestGetTopic_NotFound(t *testing.T) {
	topicID := 999

	mockItemRepo := mocks.NewMockItemRepository(t)
	mockTopicRepo := mocks.NewMockTopicRepository(t)

	mockTopicRepo.On("Get", mock.Anything, topicID).Return(model.Topic{}, httpHelper.ErrNotFound)

	storage := &MockStorage{
		itemRepo:  mockItemRepo,
		topicRepo: mockTopicRepo,
	}

	handler := NewHandler(storage, config.Config{})

	req := httptest.NewRequest(http.MethodGet, "/topics/999", nil)
	ctx := tester.ContextWithLogger()
	req = req.WithContext(ctx)
	req = mux.SetURLVars(req, map[string]string{"id": "999"})

	rr := httptest.NewRecorder()

	handler.GetTopic(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)

	var response httpHelper.Response
	err := json.NewDecoder(rr.Body).Decode(&response)
	assert.NoError(t, err)
	assert.False(t, response.Success)

	mockTopicRepo.AssertExpectations(t)
}

func TestGetTopic_DatabaseError(t *testing.T) {
	topicID := 1

	mockItemRepo := mocks.NewMockItemRepository(t)
	mockTopicRepo := mocks.NewMockTopicRepository(t)

	mockTopicRepo.On("Get", mock.Anything, topicID).Return(model.Topic{}, errors.New("database error"))

	storage := &MockStorage{
		itemRepo:  mockItemRepo,
		topicRepo: mockTopicRepo,
	}

	handler := NewHandler(storage, config.Config{})

	req := httptest.NewRequest(http.MethodGet, "/topics/1", nil)
	ctx := tester.ContextWithLogger()
	req = req.WithContext(ctx)
	req = mux.SetURLVars(req, map[string]string{"id": "1"})

	rr := httptest.NewRecorder()

	handler.GetTopic(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)

	mockTopicRepo.AssertExpectations(t)
}

func TestCreateTopic_Success(t *testing.T) {
	userID := uuid.New()
	now := time.Now().UTC()

	newTopicInput := model.Topic{
		Name: "New Topic",
	}

	newTopicWithUser := model.Topic{
		Name:   "New Topic",
		UserID: userID,
	}

	createdTopic := model.Topic{
		ID:        1,
		Name:      "New Topic",
		UserID:    userID,
		CreatedAt: now,
	}

	mockItemRepo := mocks.NewMockItemRepository(t)
	mockTopicRepo := mocks.NewMockTopicRepository(t)

	mockTopicRepo.On("Create", mock.Anything, newTopicWithUser).Return(createdTopic, nil)

	storage := &MockStorage{
		itemRepo:  mockItemRepo,
		topicRepo: mockTopicRepo,
	}

	handler := NewHandler(storage, config.Config{})

	body, _ := json.Marshal(newTopicInput)
	req := httptest.NewRequest(http.MethodPost, "/topics", bytes.NewBuffer(body))
	ctx := tester.ContextWithUser(userID.String(), "testuser", "test@example.com", uuid.New().String())
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()

	handler.CreateTopic(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)

	var response httpHelper.Response
	err := json.NewDecoder(rr.Body).Decode(&response)
	assert.NoError(t, err)
	assert.True(t, response.Success)
	assert.Equal(t, "Topic created successfully", response.Message)

	topicData, err := json.Marshal(response.Data)
	assert.NoError(t, err)

	var topic model.Topic
	err = json.Unmarshal(topicData, &topic)
	assert.NoError(t, err)
	assert.Equal(t, createdTopic.Name, topic.Name)
	assert.Equal(t, createdTopic.ID, topic.ID)

	mockTopicRepo.AssertExpectations(t)
}

func TestCreateTopic_NoUserInContext(t *testing.T) {
	storage := &MockStorage{
		itemRepo:  mocks.NewMockItemRepository(t),
		topicRepo: mocks.NewMockTopicRepository(t),
	}

	handler := NewHandler(storage, config.Config{})

	body, _ := json.Marshal(model.Topic{Name: "New Topic"})
	req := httptest.NewRequest(http.MethodPost, "/topics", bytes.NewBuffer(body))
	ctx := tester.ContextWithLogger()
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()

	handler.CreateTopic(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)

	var response httpHelper.Response
	err := json.NewDecoder(rr.Body).Decode(&response)
	assert.NoError(t, err)
	assert.False(t, response.Success)
}

func TestCreateTopic_InvalidUserID(t *testing.T) {
	storage := &MockStorage{
		itemRepo:  mocks.NewMockItemRepository(t),
		topicRepo: mocks.NewMockTopicRepository(t),
	}

	handler := NewHandler(storage, config.Config{})

	body, _ := json.Marshal(model.Topic{Name: "New Topic"})
	req := httptest.NewRequest(http.MethodPost, "/topics", bytes.NewBuffer(body))
	ctx := tester.ContextWithUser("invalid-uuid", "testuser", "test@example.com", uuid.New().String())
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()

	handler.CreateTopic(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)

	var response httpHelper.Response
	err := json.NewDecoder(rr.Body).Decode(&response)
	assert.NoError(t, err)
	assert.False(t, response.Success)
}

func TestCreateTopic_InvalidJSON(t *testing.T) {
	storage := &MockStorage{
		itemRepo:  mocks.NewMockItemRepository(t),
		topicRepo: mocks.NewMockTopicRepository(t),
	}

	handler := NewHandler(storage, config.Config{})

	req := httptest.NewRequest(http.MethodPost, "/topics", bytes.NewBufferString("invalid json"))
	ctx := tester.ContextWithUser(uuid.New().String(), "testuser", "test@example.com", uuid.New().String())
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()

	handler.CreateTopic(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var response httpHelper.Response
	err := json.NewDecoder(rr.Body).Decode(&response)
	assert.NoError(t, err)
	assert.False(t, response.Success)
}

func TestCreateTopic_MissingName(t *testing.T) {
	storage := &MockStorage{
		itemRepo:  mocks.NewMockItemRepository(t),
		topicRepo: mocks.NewMockTopicRepository(t),
	}

	handler := NewHandler(storage, config.Config{})

	newTopic := model.Topic{
		Name: "",
	}

	body, _ := json.Marshal(newTopic)
	req := httptest.NewRequest(http.MethodPost, "/topics", bytes.NewBuffer(body))
	ctx := tester.ContextWithUser(uuid.New().String(), "testuser", "test@example.com", uuid.New().String())
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()

	handler.CreateTopic(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var response httpHelper.Response
	err := json.NewDecoder(rr.Body).Decode(&response)
	assert.NoError(t, err)
	assert.False(t, response.Success)
}

func TestCreateTopic_DatabaseError(t *testing.T) {
	userID := uuid.New()

	newTopicInput := model.Topic{
		Name: "New Topic",
	}

	newTopicWithUser := model.Topic{
		Name:   "New Topic",
		UserID: userID,
	}

	mockItemRepo := mocks.NewMockItemRepository(t)
	mockTopicRepo := mocks.NewMockTopicRepository(t)

	mockTopicRepo.On("Create", mock.Anything, newTopicWithUser).Return(model.Topic{}, errors.New("database error"))

	storage := &MockStorage{
		itemRepo:  mockItemRepo,
		topicRepo: mockTopicRepo,
	}

	handler := NewHandler(storage, config.Config{})

	body, _ := json.Marshal(newTopicInput)
	req := httptest.NewRequest(http.MethodPost, "/topics", bytes.NewBuffer(body))
	ctx := tester.ContextWithUser(userID.String(), "testuser", "test@example.com", uuid.New().String())
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()

	handler.CreateTopic(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)

	mockTopicRepo.AssertExpectations(t)
}

func TestUpdateTopic_Success(t *testing.T) {
	topicID := 1
	userID := uuid.New()
	now := time.Now().UTC()

	updatedTopic := model.Topic{
		Name: "Updated Topic",
	}

	resultTopic := model.Topic{
		ID:        topicID,
		Name:      "Updated Topic",
		UserID:    userID,
		CreatedAt: now,
	}

	mockItemRepo := mocks.NewMockItemRepository(t)
	mockTopicRepo := mocks.NewMockTopicRepository(t)

	mockTopicRepo.On("Update", mock.Anything, topicID, updatedTopic).Return(resultTopic, nil)

	storage := &MockStorage{
		itemRepo:  mockItemRepo,
		topicRepo: mockTopicRepo,
	}

	handler := NewHandler(storage, config.Config{})

	body, _ := json.Marshal(updatedTopic)
	req := httptest.NewRequest(http.MethodPut, "/topics/1", bytes.NewBuffer(body))
	ctx := tester.ContextWithLogger()
	req = req.WithContext(ctx)
	req = mux.SetURLVars(req, map[string]string{"id": "1"})

	rr := httptest.NewRecorder()

	handler.UpdateTopic(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response httpHelper.Response
	err := json.NewDecoder(rr.Body).Decode(&response)
	assert.NoError(t, err)
	assert.True(t, response.Success)
	assert.Equal(t, "Topic updated successfully", response.Message)

	topicData, err := json.Marshal(response.Data)
	assert.NoError(t, err)

	var topic model.Topic
	err = json.Unmarshal(topicData, &topic)
	assert.NoError(t, err)
	assert.Equal(t, resultTopic.Name, topic.Name)

	mockTopicRepo.AssertExpectations(t)
}

func TestUpdateTopic_InvalidID(t *testing.T) {
	storage := &MockStorage{
		itemRepo:  mocks.NewMockItemRepository(t),
		topicRepo: mocks.NewMockTopicRepository(t),
	}

	handler := NewHandler(storage, config.Config{})

	updatedTopic := model.Topic{
		Name: "Updated Topic",
	}

	body, _ := json.Marshal(updatedTopic)
	req := httptest.NewRequest(http.MethodPut, "/topics/invalid", bytes.NewBuffer(body))
	ctx := tester.ContextWithLogger()
	req = req.WithContext(ctx)
	req = mux.SetURLVars(req, map[string]string{"id": "invalid"})

	rr := httptest.NewRecorder()

	handler.UpdateTopic(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var response httpHelper.Response
	err := json.NewDecoder(rr.Body).Decode(&response)
	assert.NoError(t, err)
	assert.False(t, response.Success)
}

func TestUpdateTopic_InvalidJSON(t *testing.T) {
	storage := &MockStorage{
		itemRepo:  mocks.NewMockItemRepository(t),
		topicRepo: mocks.NewMockTopicRepository(t),
	}

	handler := NewHandler(storage, config.Config{})

	req := httptest.NewRequest(http.MethodPut, "/topics/1", bytes.NewBufferString("invalid json"))
	ctx := tester.ContextWithLogger()
	req = req.WithContext(ctx)
	req = mux.SetURLVars(req, map[string]string{"id": "1"})

	rr := httptest.NewRecorder()

	handler.UpdateTopic(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var response httpHelper.Response
	err := json.NewDecoder(rr.Body).Decode(&response)
	assert.NoError(t, err)
	assert.False(t, response.Success)
}

func TestUpdateTopic_MissingName(t *testing.T) {
	storage := &MockStorage{
		itemRepo:  mocks.NewMockItemRepository(t),
		topicRepo: mocks.NewMockTopicRepository(t),
	}

	handler := NewHandler(storage, config.Config{})

	updatedTopic := model.Topic{
		Name: "",
	}

	body, _ := json.Marshal(updatedTopic)
	req := httptest.NewRequest(http.MethodPut, "/topics/1", bytes.NewBuffer(body))
	ctx := tester.ContextWithLogger()
	req = req.WithContext(ctx)
	req = mux.SetURLVars(req, map[string]string{"id": "1"})

	rr := httptest.NewRecorder()

	handler.UpdateTopic(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var response httpHelper.Response
	err := json.NewDecoder(rr.Body).Decode(&response)
	assert.NoError(t, err)
	assert.False(t, response.Success)
}

func TestUpdateTopic_NotFound(t *testing.T) {
	topicID := 999

	updatedTopic := model.Topic{
		Name: "Updated Topic",
	}

	mockItemRepo := mocks.NewMockItemRepository(t)
	mockTopicRepo := mocks.NewMockTopicRepository(t)

	mockTopicRepo.On("Update", mock.Anything, topicID, updatedTopic).Return(model.Topic{}, httpHelper.ErrNotFound)

	storage := &MockStorage{
		itemRepo:  mockItemRepo,
		topicRepo: mockTopicRepo,
	}

	handler := NewHandler(storage, config.Config{})

	body, _ := json.Marshal(updatedTopic)
	req := httptest.NewRequest(http.MethodPut, "/topics/999", bytes.NewBuffer(body))
	ctx := tester.ContextWithLogger()
	req = req.WithContext(ctx)
	req = mux.SetURLVars(req, map[string]string{"id": "999"})

	rr := httptest.NewRecorder()

	handler.UpdateTopic(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)

	var response httpHelper.Response
	err := json.NewDecoder(rr.Body).Decode(&response)
	assert.NoError(t, err)
	assert.False(t, response.Success)

	mockTopicRepo.AssertExpectations(t)
}

func TestUpdateTopic_DatabaseError(t *testing.T) {
	topicID := 1

	updatedTopic := model.Topic{
		Name: "Updated Topic",
	}

	mockItemRepo := mocks.NewMockItemRepository(t)
	mockTopicRepo := mocks.NewMockTopicRepository(t)

	mockTopicRepo.On("Update", mock.Anything, topicID, updatedTopic).Return(model.Topic{}, errors.New("database error"))

	storage := &MockStorage{
		itemRepo:  mockItemRepo,
		topicRepo: mockTopicRepo,
	}

	handler := NewHandler(storage, config.Config{})

	body, _ := json.Marshal(updatedTopic)
	req := httptest.NewRequest(http.MethodPut, "/topics/1", bytes.NewBuffer(body))
	ctx := tester.ContextWithLogger()
	req = req.WithContext(ctx)
	req = mux.SetURLVars(req, map[string]string{"id": "1"})

	rr := httptest.NewRecorder()

	handler.UpdateTopic(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)

	mockTopicRepo.AssertExpectations(t)
}

func TestDeleteTopic_Success(t *testing.T) {
	topicID := 1

	mockItemRepo := mocks.NewMockItemRepository(t)
	mockTopicRepo := mocks.NewMockTopicRepository(t)
	mockTrx := mocks.NewMockQueryOptions(t)

	mockTopicRepo.On("CreateTrx", mock.Anything).Return(mockTrx, nil)
	mockItemRepo.On("DeleteByTopicID", mock.Anything, topicID, mockTrx).Return(nil)
	mockTopicRepo.On("Delete", mock.Anything, topicID, mockTrx).Return(nil)
	mockTrx.On("Commit").Return(nil)
	mockTrx.On("Rollback").Return(nil)

	storage := &MockStorage{
		itemRepo:  mockItemRepo,
		topicRepo: mockTopicRepo,
	}

	handler := NewHandler(storage, config.Config{})

	req := httptest.NewRequest(http.MethodDelete, "/topics/1", nil)
	ctx := tester.ContextWithLogger()
	req = req.WithContext(ctx)
	req = mux.SetURLVars(req, map[string]string{"id": "1"})

	rr := httptest.NewRecorder()

	handler.DeleteTopic(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response httpHelper.Response
	err := json.NewDecoder(rr.Body).Decode(&response)
	assert.NoError(t, err)
	assert.True(t, response.Success)
	assert.Equal(t, "Topic deleted successfully", response.Message)

	mockTopicRepo.AssertExpectations(t)
	mockItemRepo.AssertExpectations(t)
	mockTrx.AssertExpectations(t)
}

func TestDeleteTopic_InvalidID(t *testing.T) {
	storage := &MockStorage{
		itemRepo:  mocks.NewMockItemRepository(t),
		topicRepo: mocks.NewMockTopicRepository(t),
	}

	handler := NewHandler(storage, config.Config{})

	req := httptest.NewRequest(http.MethodDelete, "/topics/invalid", nil)
	ctx := tester.ContextWithLogger()
	req = req.WithContext(ctx)
	req = mux.SetURLVars(req, map[string]string{"id": "invalid"})

	rr := httptest.NewRecorder()

	handler.DeleteTopic(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var response httpHelper.Response
	err := json.NewDecoder(rr.Body).Decode(&response)
	assert.NoError(t, err)
	assert.False(t, response.Success)
}

func TestDeleteTopic_CreateTrxError(t *testing.T) {
	mockItemRepo := mocks.NewMockItemRepository(t)
	mockTopicRepo := mocks.NewMockTopicRepository(t)

	mockTopicRepo.On("CreateTrx", mock.Anything).Return(nil, errors.New("transaction error"))

	storage := &MockStorage{
		itemRepo:  mockItemRepo,
		topicRepo: mockTopicRepo,
	}

	handler := NewHandler(storage, config.Config{})

	req := httptest.NewRequest(http.MethodDelete, "/topics/1", nil)
	ctx := tester.ContextWithLogger()
	req = req.WithContext(ctx)
	req = mux.SetURLVars(req, map[string]string{"id": "1"})

	rr := httptest.NewRecorder()

	handler.DeleteTopic(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)

	mockTopicRepo.AssertExpectations(t)
}

func TestDeleteTopic_DeleteItemsError(t *testing.T) {
	topicID := 1

	mockItemRepo := mocks.NewMockItemRepository(t)
	mockTopicRepo := mocks.NewMockTopicRepository(t)
	mockTrx := mocks.NewMockQueryOptions(t)

	mockTopicRepo.On("CreateTrx", mock.Anything).Return(mockTrx, nil)
	mockItemRepo.On("DeleteByTopicID", mock.Anything, topicID, mockTrx).Return(errors.New("delete items error"))
	mockTrx.On("Rollback").Return(nil)

	storage := &MockStorage{
		itemRepo:  mockItemRepo,
		topicRepo: mockTopicRepo,
	}

	handler := NewHandler(storage, config.Config{})

	req := httptest.NewRequest(http.MethodDelete, "/topics/1", nil)
	ctx := tester.ContextWithLogger()
	req = req.WithContext(ctx)
	req = mux.SetURLVars(req, map[string]string{"id": "1"})

	rr := httptest.NewRecorder()

	handler.DeleteTopic(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)

	mockTopicRepo.AssertExpectations(t)
	mockItemRepo.AssertExpectations(t)
}

func TestDeleteTopic_NotFound(t *testing.T) {
	topicID := 999

	mockItemRepo := mocks.NewMockItemRepository(t)
	mockTopicRepo := mocks.NewMockTopicRepository(t)
	mockTrx := mocks.NewMockQueryOptions(t)

	mockTopicRepo.On("CreateTrx", mock.Anything).Return(mockTrx, nil)
	mockItemRepo.On("DeleteByTopicID", mock.Anything, topicID, mockTrx).Return(nil)
	mockTopicRepo.On("Delete", mock.Anything, topicID, mockTrx).Return(httpHelper.ErrNotFound)
	mockTrx.On("Rollback").Return(nil)

	storage := &MockStorage{
		itemRepo:  mockItemRepo,
		topicRepo: mockTopicRepo,
	}

	handler := NewHandler(storage, config.Config{})

	req := httptest.NewRequest(http.MethodDelete, "/topics/999", nil)
	ctx := tester.ContextWithLogger()
	req = req.WithContext(ctx)
	req = mux.SetURLVars(req, map[string]string{"id": "999"})

	rr := httptest.NewRecorder()

	handler.DeleteTopic(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)

	var response httpHelper.Response
	err := json.NewDecoder(rr.Body).Decode(&response)
	assert.NoError(t, err)
	assert.False(t, response.Success)

	mockTopicRepo.AssertExpectations(t)
	mockItemRepo.AssertExpectations(t)
}

func TestDeleteTopic_DeleteTopicError(t *testing.T) {
	topicID := 1

	mockItemRepo := mocks.NewMockItemRepository(t)
	mockTopicRepo := mocks.NewMockTopicRepository(t)
	mockTrx := mocks.NewMockQueryOptions(t)

	mockTopicRepo.On("CreateTrx", mock.Anything).Return(mockTrx, nil)
	mockItemRepo.On("DeleteByTopicID", mock.Anything, topicID, mockTrx).Return(nil)
	mockTopicRepo.On("Delete", mock.Anything, topicID, mockTrx).Return(errors.New("delete error"))
	mockTrx.On("Rollback").Return(nil)

	storage := &MockStorage{
		itemRepo:  mockItemRepo,
		topicRepo: mockTopicRepo,
	}

	handler := NewHandler(storage, config.Config{})

	req := httptest.NewRequest(http.MethodDelete, "/topics/1", nil)
	ctx := tester.ContextWithLogger()
	req = req.WithContext(ctx)
	req = mux.SetURLVars(req, map[string]string{"id": "1"})

	rr := httptest.NewRecorder()

	handler.DeleteTopic(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)

	mockTopicRepo.AssertExpectations(t)
	mockItemRepo.AssertExpectations(t)
}

func TestDeleteTopic_CommitError(t *testing.T) {
	topicID := 1

	mockItemRepo := mocks.NewMockItemRepository(t)
	mockTopicRepo := mocks.NewMockTopicRepository(t)
	mockTrx := mocks.NewMockQueryOptions(t)

	mockTopicRepo.On("CreateTrx", mock.Anything).Return(mockTrx, nil)
	mockItemRepo.On("DeleteByTopicID", mock.Anything, topicID, mockTrx).Return(nil)
	mockTopicRepo.On("Delete", mock.Anything, topicID, mockTrx).Return(nil)
	mockTrx.On("Commit").Return(errors.New("commit error"))
	mockTrx.On("Rollback").Return(nil)

	storage := &MockStorage{
		itemRepo:  mockItemRepo,
		topicRepo: mockTopicRepo,
	}

	handler := NewHandler(storage, config.Config{})

	req := httptest.NewRequest(http.MethodDelete, "/topics/1", nil)
	ctx := tester.ContextWithLogger()
	req = req.WithContext(ctx)
	req = mux.SetURLVars(req, map[string]string{"id": "1"})

	rr := httptest.NewRecorder()

	handler.DeleteTopic(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)

	mockTopicRepo.AssertExpectations(t)
	mockItemRepo.AssertExpectations(t)
	mockTrx.AssertExpectations(t)
}

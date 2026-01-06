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
	"github.com/all-in-one/internal/listing/repository"
	"github.com/all-in-one/internal/tester"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockStorage is a wrapper for generated mocks
type MockStorage struct {
	itemRepo  repository.ItemRepository
	topicRepo repository.TopicRepository
}

func (m *MockStorage) ItemRepo() repository.ItemRepository {
	return m.itemRepo
}

func (m *MockStorage) TopicRepo() repository.TopicRepository {
	return m.topicRepo
}

func (m *MockStorage) Close() error {
	return nil
}

func TestGetItems_Success(t *testing.T) {
	topicID := 1
	userID := uuid.New()
	now := time.Now().UTC()

	expectedTopic := model.Topic{
		ID:        topicID,
		Name:      "Test Topic",
		UserID:    userID,
		CreatedAt: now,
	}

	expectedItems := []model.Item{
		{
			ID:        1,
			TopicID:   topicID,
			Title:     "Item 1",
			CreatedAt: now,
		},
		{
			ID:        2,
			TopicID:   topicID,
			Title:     "Item 2",
			CreatedAt: now,
		},
	}

	mockItemRepo := mocks.NewMockItemRepository(t)
	mockTopicRepo := mocks.NewMockTopicRepository(t)

	mockTopicRepo.On("Get", mock.Anything, topicID).Return(expectedTopic, nil)
	mockItemRepo.On("GetByTopicID", mock.Anything, topicID).Return(expectedItems, nil)

	storage := &MockStorage{
		itemRepo:  mockItemRepo,
		topicRepo: mockTopicRepo,
	}

	handler := NewHandler(storage, config.Config{})

	req := httptest.NewRequest(http.MethodGet, "/topics/1/items", nil)
	ctx := tester.ContextWithUser(userID.String(), "testuser", "test@example.com", uuid.New().String())
	req = req.WithContext(ctx)
	req = mux.SetURLVars(req, map[string]string{"topic_id": "1"})

	rr := httptest.NewRecorder()

	handler.GetItems(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response httpHelper.Response
	err := json.NewDecoder(rr.Body).Decode(&response)
	assert.NoError(t, err)
	assert.True(t, response.Success)

	itemsData, err := json.Marshal(response.Data)
	assert.NoError(t, err)

	var items []model.Item
	err = json.Unmarshal(itemsData, &items)
	assert.NoError(t, err)
	assert.Len(t, items, 2)
	assert.Equal(t, expectedItems[0].Title, items[0].Title)

	mockTopicRepo.AssertExpectations(t)
	mockItemRepo.AssertExpectations(t)
}

func TestGetItems_InvalidTopicID(t *testing.T) {
	storage := &MockStorage{
		itemRepo:  mocks.NewMockItemRepository(t),
		topicRepo: mocks.NewMockTopicRepository(t),
	}

	handler := NewHandler(storage, config.Config{})

	req := httptest.NewRequest(http.MethodGet, "/topics/invalid/items", nil)
	ctx := tester.ContextWithLogger()
	req = req.WithContext(ctx)
	req = mux.SetURLVars(req, map[string]string{"topic_id": "invalid"})

	rr := httptest.NewRecorder()

	handler.GetItems(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var response httpHelper.Response
	err := json.NewDecoder(rr.Body).Decode(&response)
	assert.NoError(t, err)
	assert.False(t, response.Success)
	assert.NotNil(t, response.Error)
}

func TestGetItems_TopicNotFound(t *testing.T) {
	topicID := 999
	userID := uuid.New()

	mockItemRepo := mocks.NewMockItemRepository(t)
	mockTopicRepo := mocks.NewMockTopicRepository(t)

	mockTopicRepo.On("Get", mock.Anything, topicID).Return(model.Topic{}, httpHelper.ErrNotFound)

	storage := &MockStorage{
		itemRepo:  mockItemRepo,
		topicRepo: mockTopicRepo,
	}

	handler := NewHandler(storage, config.Config{})

	req := httptest.NewRequest(http.MethodGet, "/topics/999/items", nil)
	ctx := tester.ContextWithUser(userID.String(), "testuser", "test@example.com", uuid.New().String())
	req = req.WithContext(ctx)
	req = mux.SetURLVars(req, map[string]string{"topic_id": "999"})

	rr := httptest.NewRecorder()

	handler.GetItems(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)

	var response httpHelper.Response
	err := json.NewDecoder(rr.Body).Decode(&response)
	assert.NoError(t, err)
	assert.False(t, response.Success)

	mockTopicRepo.AssertExpectations(t)
}

func TestGetItems_DatabaseError(t *testing.T) {
	topicID := 1
	userID := uuid.New()

	expectedTopic := model.Topic{
		ID:     topicID,
		Name:   "Test Topic",
		UserID: userID,
	}

	mockItemRepo := mocks.NewMockItemRepository(t)
	mockTopicRepo := mocks.NewMockTopicRepository(t)

	mockTopicRepo.On("Get", mock.Anything, topicID).Return(expectedTopic, nil)
	mockItemRepo.On("GetByTopicID", mock.Anything, topicID).Return([]model.Item{}, errors.New("database error"))

	storage := &MockStorage{
		itemRepo:  mockItemRepo,
		topicRepo: mockTopicRepo,
	}

	handler := NewHandler(storage, config.Config{})

	req := httptest.NewRequest(http.MethodGet, "/topics/1/items", nil)
	ctx := tester.ContextWithUser(userID.String(), "testuser", "test@example.com", uuid.New().String())
	req = req.WithContext(ctx)
	req = mux.SetURLVars(req, map[string]string{"topic_id": "1"})

	rr := httptest.NewRecorder()

	handler.GetItems(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)

	mockTopicRepo.AssertExpectations(t)
	mockItemRepo.AssertExpectations(t)
}

func TestGetItem_Success(t *testing.T) {
	topicID := 1
	itemID := 1
	userID := uuid.New()
	now := time.Now().UTC()

	expectedTopic := model.Topic{
		ID:     topicID,
		Name:   "Test Topic",
		UserID: userID,
	}

	expectedItem := model.Item{
		ID:        itemID,
		TopicID:   topicID,
		Title:     "Test Item",
		CreatedAt: now,
	}

	mockItemRepo := mocks.NewMockItemRepository(t)
	mockTopicRepo := mocks.NewMockTopicRepository(t)

	mockTopicRepo.On("Get", mock.Anything, topicID).Return(expectedTopic, nil)
	mockItemRepo.On("Get", mock.Anything, itemID).Return(expectedItem, nil)

	storage := &MockStorage{
		itemRepo:  mockItemRepo,
		topicRepo: mockTopicRepo,
	}

	handler := NewHandler(storage, config.Config{})

	req := httptest.NewRequest(http.MethodGet, "/topics/1/items/1", nil)
	ctx := tester.ContextWithUser(userID.String(), "testuser", "test@example.com", uuid.New().String())
	req = req.WithContext(ctx)
	req = mux.SetURLVars(req, map[string]string{"topic_id": "1", "id": "1"})

	rr := httptest.NewRecorder()

	handler.GetItem(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response httpHelper.Response
	err := json.NewDecoder(rr.Body).Decode(&response)
	assert.NoError(t, err)
	assert.True(t, response.Success)

	itemData, err := json.Marshal(response.Data)
	assert.NoError(t, err)

	var item model.Item
	err = json.Unmarshal(itemData, &item)
	assert.NoError(t, err)
	assert.Equal(t, expectedItem.ID, item.ID)
	assert.Equal(t, expectedItem.Title, item.Title)

	mockTopicRepo.AssertExpectations(t)
	mockItemRepo.AssertExpectations(t)
}

func TestGetItem_ItemNotInTopic(t *testing.T) {
	topicID := 1
	itemID := 1
	userID := uuid.New()

	expectedTopic := model.Topic{
		ID:     topicID,
		Name:   "Test Topic",
		UserID: userID,
	}

	expectedItem := model.Item{
		ID:      itemID,
		TopicID: 2, // Different topic ID
		Title:   "Test Item",
	}

	mockItemRepo := mocks.NewMockItemRepository(t)
	mockTopicRepo := mocks.NewMockTopicRepository(t)

	mockTopicRepo.On("Get", mock.Anything, topicID).Return(expectedTopic, nil)
	mockItemRepo.On("Get", mock.Anything, itemID).Return(expectedItem, nil)

	storage := &MockStorage{
		itemRepo:  mockItemRepo,
		topicRepo: mockTopicRepo,
	}

	handler := NewHandler(storage, config.Config{})

	req := httptest.NewRequest(http.MethodGet, "/topics/1/items/1", nil)
	ctx := tester.ContextWithUser(userID.String(), "testuser", "test@example.com", uuid.New().String())
	req = req.WithContext(ctx)
	req = mux.SetURLVars(req, map[string]string{"topic_id": "1", "id": "1"})

	rr := httptest.NewRecorder()

	handler.GetItem(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)

	mockTopicRepo.AssertExpectations(t)
	mockItemRepo.AssertExpectations(t)
}

func TestCreateItem_Success(t *testing.T) {
	topicID := 1
	userID := uuid.New()
	now := time.Now().UTC()

	expectedTopic := model.Topic{
		ID:     topicID,
		Name:   "Test Topic",
		UserID: userID,
	}

	newItem := model.Item{
		Title: "New Item",
	}

	createdItem := model.Item{
		ID:        1,
		TopicID:   topicID,
		Title:     "New Item",
		CreatedAt: now,
	}

	mockItemRepo := mocks.NewMockItemRepository(t)
	mockTopicRepo := mocks.NewMockTopicRepository(t)

	mockTopicRepo.On("Get", mock.Anything, topicID).Return(expectedTopic, nil)
	mockItemRepo.On("Create", mock.Anything, mock.MatchedBy(func(item model.Item) bool {
		return item.Title == "New Item" && item.TopicID == topicID
	})).Return(createdItem, nil)

	storage := &MockStorage{
		itemRepo:  mockItemRepo,
		topicRepo: mockTopicRepo,
	}

	handler := NewHandler(storage, config.Config{})

	body, _ := json.Marshal(newItem)
	req := httptest.NewRequest(http.MethodPost, "/topics/1/items", bytes.NewReader(body))
	ctx := tester.ContextWithUser(userID.String(), "testuser", "test@example.com", uuid.New().String())
	req = req.WithContext(ctx)
	req = mux.SetURLVars(req, map[string]string{"topic_id": "1"})

	rr := httptest.NewRecorder()

	handler.CreateItem(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)

	var response httpHelper.Response
	err := json.NewDecoder(rr.Body).Decode(&response)
	assert.NoError(t, err)
	assert.True(t, response.Success)
	assert.Equal(t, "Item created successfully", response.Message)

	mockTopicRepo.AssertExpectations(t)
	mockItemRepo.AssertExpectations(t)
}

func TestCreateItem_MissingTitle(t *testing.T) {
	topicID := 1
	userID := uuid.New()

	expectedTopic := model.Topic{
		ID:     topicID,
		Name:   "Test Topic",
		UserID: userID,
	}

	mockItemRepo := mocks.NewMockItemRepository(t)
	mockTopicRepo := mocks.NewMockTopicRepository(t)

	mockTopicRepo.On("Get", mock.Anything, topicID).Return(expectedTopic, nil)

	storage := &MockStorage{
		itemRepo:  mockItemRepo,
		topicRepo: mockTopicRepo,
	}

	handler := NewHandler(storage, config.Config{})

	newItem := model.Item{
		Title: "", // Empty title
	}

	body, _ := json.Marshal(newItem)
	req := httptest.NewRequest(http.MethodPost, "/topics/1/items", bytes.NewReader(body))
	ctx := tester.ContextWithUser(userID.String(), "testuser", "test@example.com", uuid.New().String())
	req = req.WithContext(ctx)
	req = mux.SetURLVars(req, map[string]string{"topic_id": "1"})

	rr := httptest.NewRecorder()

	handler.CreateItem(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var response httpHelper.Response
	err := json.NewDecoder(rr.Body).Decode(&response)
	assert.NoError(t, err)
	assert.False(t, response.Success)

	mockTopicRepo.AssertExpectations(t)
}

func TestCreateItem_InvalidJSON(t *testing.T) {
	topicID := 1
	userID := uuid.New()

	expectedTopic := model.Topic{
		ID:     topicID,
		Name:   "Test Topic",
		UserID: userID,
	}

	mockItemRepo := mocks.NewMockItemRepository(t)
	mockTopicRepo := mocks.NewMockTopicRepository(t)

	mockTopicRepo.On("Get", mock.Anything, topicID).Return(expectedTopic, nil)

	storage := &MockStorage{
		itemRepo:  mockItemRepo,
		topicRepo: mockTopicRepo,
	}

	handler := NewHandler(storage, config.Config{})

	req := httptest.NewRequest(http.MethodPost, "/topics/1/items", bytes.NewReader([]byte("invalid json")))
	ctx := tester.ContextWithUser(userID.String(), "testuser", "test@example.com", uuid.New().String())
	req = req.WithContext(ctx)
	req = mux.SetURLVars(req, map[string]string{"topic_id": "1"})

	rr := httptest.NewRecorder()

	handler.CreateItem(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)

	mockTopicRepo.AssertExpectations(t)
}

func TestUpdateItem_Success(t *testing.T) {
	topicID := 1
	itemID := 1
	userID := uuid.New()
	now := time.Now().UTC()

	expectedTopic := model.Topic{
		ID:     topicID,
		Name:   "Test Topic",
		UserID: userID,
	}

	existingItem := model.Item{
		ID:        itemID,
		TopicID:   topicID,
		Title:     "Old Title",
		CreatedAt: now,
	}

	updatedItem := model.Item{
		Title: "Updated Title",
	}

	expectedUpdatedItem := model.Item{
		ID:        itemID,
		TopicID:   topicID,
		Title:     "Updated Title",
		CreatedAt: now,
		UpdatedAt: now,
	}

	mockItemRepo := mocks.NewMockItemRepository(t)
	mockTopicRepo := mocks.NewMockTopicRepository(t)

	mockTopicRepo.On("Get", mock.Anything, topicID).Return(expectedTopic, nil)
	mockItemRepo.On("Get", mock.Anything, itemID).Return(existingItem, nil)
	mockItemRepo.On("Update", mock.Anything, itemID, mock.MatchedBy(func(item model.Item) bool {
		return item.Title == "Updated Title" && item.TopicID == topicID
	})).Return(expectedUpdatedItem, nil)

	storage := &MockStorage{
		itemRepo:  mockItemRepo,
		topicRepo: mockTopicRepo,
	}

	handler := NewHandler(storage, config.Config{})

	body, _ := json.Marshal(updatedItem)
	req := httptest.NewRequest(http.MethodPut, "/topics/1/items/1", bytes.NewReader(body))
	ctx := tester.ContextWithUser(userID.String(), "testuser", "test@example.com", uuid.New().String())
	req = req.WithContext(ctx)
	req = mux.SetURLVars(req, map[string]string{"topic_id": "1", "id": "1"})

	rr := httptest.NewRecorder()

	handler.UpdateItem(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response httpHelper.Response
	err := json.NewDecoder(rr.Body).Decode(&response)
	assert.NoError(t, err)
	assert.True(t, response.Success)

	mockTopicRepo.AssertExpectations(t)
	mockItemRepo.AssertExpectations(t)
}

func TestUpdateItem_ItemNotInTopic(t *testing.T) {
	topicID := 1
	itemID := 1
	userID := uuid.New()

	expectedTopic := model.Topic{
		ID:     topicID,
		Name:   "Test Topic",
		UserID: userID,
	}

	existingItem := model.Item{
		ID:      itemID,
		TopicID: 2, // Different topic
		Title:   "Test Item",
	}

	mockItemRepo := mocks.NewMockItemRepository(t)
	mockTopicRepo := mocks.NewMockTopicRepository(t)

	mockTopicRepo.On("Get", mock.Anything, topicID).Return(expectedTopic, nil)
	mockItemRepo.On("Get", mock.Anything, itemID).Return(existingItem, nil)

	storage := &MockStorage{
		itemRepo:  mockItemRepo,
		topicRepo: mockTopicRepo,
	}

	handler := NewHandler(storage, config.Config{})

	updatedItem := model.Item{
		Title: "Updated Title",
	}

	body, _ := json.Marshal(updatedItem)
	req := httptest.NewRequest(http.MethodPut, "/topics/1/items/1", bytes.NewReader(body))
	ctx := tester.ContextWithUser(userID.String(), "testuser", "test@example.com", uuid.New().String())
	req = req.WithContext(ctx)
	req = mux.SetURLVars(req, map[string]string{"topic_id": "1", "id": "1"})

	rr := httptest.NewRecorder()

	handler.UpdateItem(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)

	mockTopicRepo.AssertExpectations(t)
	mockItemRepo.AssertExpectations(t)
}

func TestDeleteItem_Success(t *testing.T) {
	topicID := 1
	itemID := 1
	userID := uuid.New()

	expectedTopic := model.Topic{
		ID:     topicID,
		Name:   "Test Topic",
		UserID: userID,
	}

	existingItem := model.Item{
		ID:      itemID,
		TopicID: topicID,
		Title:   "Test Item",
	}

	mockItemRepo := mocks.NewMockItemRepository(t)
	mockTopicRepo := mocks.NewMockTopicRepository(t)

	mockTopicRepo.On("Get", mock.Anything, topicID).Return(expectedTopic, nil)
	mockItemRepo.On("Get", mock.Anything, itemID).Return(existingItem, nil)
	mockItemRepo.On("Delete", mock.Anything, itemID).Return(nil)

	storage := &MockStorage{
		itemRepo:  mockItemRepo,
		topicRepo: mockTopicRepo,
	}

	handler := NewHandler(storage, config.Config{})

	req := httptest.NewRequest(http.MethodDelete, "/topics/1/items/1", nil)
	ctx := tester.ContextWithUser(userID.String(), "testuser", "test@example.com", uuid.New().String())
	req = req.WithContext(ctx)
	req = mux.SetURLVars(req, map[string]string{"topic_id": "1", "id": "1"})

	rr := httptest.NewRecorder()

	handler.DeleteItem(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response httpHelper.Response
	err := json.NewDecoder(rr.Body).Decode(&response)
	assert.NoError(t, err)
	assert.True(t, response.Success)
	assert.Equal(t, "Item deleted successfully", response.Message)

	mockTopicRepo.AssertExpectations(t)
	mockItemRepo.AssertExpectations(t)
}

func TestDeleteItem_ItemNotFound(t *testing.T) {
	topicID := 1
	itemID := 999
	userID := uuid.New()

	expectedTopic := model.Topic{
		ID:     topicID,
		Name:   "Test Topic",
		UserID: userID,
	}

	mockItemRepo := mocks.NewMockItemRepository(t)
	mockTopicRepo := mocks.NewMockTopicRepository(t)

	mockTopicRepo.On("Get", mock.Anything, topicID).Return(expectedTopic, nil)
	mockItemRepo.On("Get", mock.Anything, itemID).Return(model.Item{}, httpHelper.ErrNotFound)

	storage := &MockStorage{
		itemRepo:  mockItemRepo,
		topicRepo: mockTopicRepo,
	}

	handler := NewHandler(storage, config.Config{})

	req := httptest.NewRequest(http.MethodDelete, "/topics/1/items/999", nil)
	ctx := tester.ContextWithUser(userID.String(), "testuser", "test@example.com", uuid.New().String())
	req = req.WithContext(ctx)
	req = mux.SetURLVars(req, map[string]string{"topic_id": "1", "id": "999"})

	rr := httptest.NewRecorder()

	handler.DeleteItem(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)

	mockTopicRepo.AssertExpectations(t)
	mockItemRepo.AssertExpectations(t)
}

func TestDeleteItem_ItemNotInTopic(t *testing.T) {
	topicID := 1
	itemID := 1
	userID := uuid.New()

	expectedTopic := model.Topic{
		ID:     topicID,
		Name:   "Test Topic",
		UserID: userID,
	}

	existingItem := model.Item{
		ID:      itemID,
		TopicID: 2, // Different topic
		Title:   "Test Item",
	}

	mockItemRepo := mocks.NewMockItemRepository(t)
	mockTopicRepo := mocks.NewMockTopicRepository(t)

	mockTopicRepo.On("Get", mock.Anything, topicID).Return(expectedTopic, nil)
	mockItemRepo.On("Get", mock.Anything, itemID).Return(existingItem, nil)

	storage := &MockStorage{
		itemRepo:  mockItemRepo,
		topicRepo: mockTopicRepo,
	}

	handler := NewHandler(storage, config.Config{})

	req := httptest.NewRequest(http.MethodDelete, "/topics/1/items/1", nil)
	ctx := tester.ContextWithUser(userID.String(), "testuser", "test@example.com", uuid.New().String())
	req = req.WithContext(ctx)
	req = mux.SetURLVars(req, map[string]string{"topic_id": "1", "id": "1"})

	rr := httptest.NewRecorder()

	handler.DeleteItem(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)

	mockTopicRepo.AssertExpectations(t)
	mockItemRepo.AssertExpectations(t)
}

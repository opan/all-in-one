package storage

import (
	"errors"
	"sync"
	"time"

	"github.com/all-in-one/pkg/models"
)

// Errors
var (
	ErrItemNotFound = errors.New("item not found")
)

// MemoryStorage implements ItemStorage with an in-memory data store
type MemoryStorage struct {
	items  map[int]models.Item
	lastID int
	mutex  sync.RWMutex
}

// NewMemoryStorage creates a new memory-based storage
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		items:  make(map[int]models.Item),
		lastID: 0,
	}
}

// GetAll returns all items
func (s *MemoryStorage) GetAll() ([]models.Item, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	items := make([]models.Item, 0, len(s.items))
	for _, item := range s.items {
		items = append(items, item)
	}

	return items, nil
}

// Get returns an item by ID
func (s *MemoryStorage) Get(id int) (models.Item, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	item, exists := s.items[id]
	if !exists {
		return models.Item{}, ErrItemNotFound
	}

	return item, nil
}

// Create adds a new item
func (s *MemoryStorage) Create(item models.Item) (models.Item, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Assign ID and timestamps
	s.lastID++
	item.ID = s.lastID
	item.CreatedAt = time.Now()
	item.UpdatedAt = time.Now()

	// Store the item
	s.items[item.ID] = item

	return item, nil
}

// Update modifies an existing item
func (s *MemoryStorage) Update(id int, item models.Item) (models.Item, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	existingItem, exists := s.items[id]
	if !exists {
		return models.Item{}, ErrItemNotFound
	}

	// Update item while preserving ID and CreatedAt
	item.ID = id
	item.CreatedAt = existingItem.CreatedAt
	item.UpdatedAt = time.Now()

	s.items[id] = item

	return item, nil
}

// Delete removes an item
func (s *MemoryStorage) Delete(id int) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	_, exists := s.items[id]
	if !exists {
		return ErrItemNotFound
	}

	delete(s.items, id)
	return nil
}

// InitializeSampleData adds sample data to the storage
func (s *MemoryStorage) InitializeSampleData() int {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	sampleItems := []models.Item{
		{
			Name:        "Sample Task 1",
			Description: "This is a sample task for testing",
		},
		{
			Name:        "Sample Task 2",
			Description: "Another sample task with different content",
		},
		{
			Name:        "Sample Task 3",
			Description: "Third sample task for demonstration",
		},
	}

	for _, item := range sampleItems {
		s.lastID++
		item.ID = s.lastID
		item.CreatedAt = time.Now()
		item.UpdatedAt = time.Now()
		s.items[item.ID] = item
	}

	return len(sampleItems)
}

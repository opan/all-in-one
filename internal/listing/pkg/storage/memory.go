package storage

import (
	"sync"
	"time"

	"github.com/all-in-one/internal/common"
	"github.com/all-in-one/internal/listing/pkg/model"
)

// MemoryStorage implements Storage with an in-memory data store
type MemoryStorage struct {
	items  map[int]model.Item
	lastID int
	mutex  sync.RWMutex
}

// NewMemoryStorage creates a new memory-based storage for listings
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		items:  make(map[int]model.Item),
		lastID: 0,
	}
}

// GetAll returns all items
func (s *MemoryStorage) GetAll() ([]model.Item, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	items := make([]model.Item, 0, len(s.items))
	for _, item := range s.items {
		items = append(items, item)
	}

	return items, nil
}

// Get returns an item by ID
func (s *MemoryStorage) Get(id int) (model.Item, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	item, exists := s.items[id]
	if !exists {
		return model.Item{}, common.ErrNotFound
	}

	return item, nil
}

// Create adds a new item
func (s *MemoryStorage) Create(item model.Item) (model.Item, error) {
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
func (s *MemoryStorage) Update(id int, item model.Item) (model.Item, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	existingItem, exists := s.items[id]
	if !exists {
		return model.Item{}, common.ErrNotFound
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
		return common.ErrNotFound
	}

	delete(s.items, id)
	return nil
}

// InitializeSampleData adds sample data to the storage
func (s *MemoryStorage) InitializeSampleData() int {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	sampleItems := []model.Item{
		{
			Title:       "Sample Task 1",
			Description: "This is a sample task for testing",
		},
		{
			Title:       "Sample Task 2",
			Description: "Another sample task with different content",
		},
		{
			Title:       "Sample Task 3",
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

package memory

import (
	"sync"
	"time"

	"github.com/all-in-one/internal/common"
	"github.com/all-in-one/internal/listing/pkg/model"
)

// itemRepository implements the item repository with in-memory storage
type itemRepository struct {
	items  map[int]model.Item
	lastID int
	mutex  sync.RWMutex
}

// newItemRepository creates a new memory-based item repository
func newItemRepository() *itemRepository {
	return &itemRepository{
		items:  make(map[int]model.Item),
		lastID: 0,
	}
}

// GetAll returns all items
func (r *itemRepository) GetAll() ([]model.Item, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	items := make([]model.Item, 0, len(r.items))
	for _, item := range r.items {
		items = append(items, item)
	}

	return items, nil
}

// Get returns an item by ID
func (r *itemRepository) Get(id int) (model.Item, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	item, exists := r.items[id]
	if !exists {
		return model.Item{}, common.ErrNotFound
	}

	return item, nil
}

// Create adds a new item
func (r *itemRepository) Create(item model.Item) (model.Item, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Assign ID and timestamps
	r.lastID++
	item.ID = r.lastID
	item.CreatedAt = time.Now()
	item.UpdatedAt = time.Now()

	// Store the item
	r.items[item.ID] = item

	return item, nil
}

// Update modifies an existing item
func (r *itemRepository) Update(id int, item model.Item) (model.Item, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	existingItem, exists := r.items[id]
	if !exists {
		return model.Item{}, common.ErrNotFound
	}

	// Update item while preserving ID and CreatedAt
	item.ID = id
	item.CreatedAt = existingItem.CreatedAt
	item.UpdatedAt = time.Now()

	r.items[id] = item

	return item, nil
}

// Delete removes an item
func (r *itemRepository) Delete(id int) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	_, exists := r.items[id]
	if !exists {
		return common.ErrNotFound
	}

	delete(r.items, id)
	return nil
}

// InitializeSampleData adds sample data to the storage
func (r *itemRepository) InitializeSampleData() int {
	r.mutex.Lock()
	defer r.mutex.Unlock()

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
		r.lastID++
		item.ID = r.lastID
		item.CreatedAt = time.Now()
		item.UpdatedAt = time.Now()
		r.items[item.ID] = item
	}

	return len(sampleItems)
}

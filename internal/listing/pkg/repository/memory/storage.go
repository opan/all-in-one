package memory

import (
	"github.com/all-in-one/internal/listing/pkg/model"
)

// ItemRepository defines the interface for item storage operations (local copy to avoid import cycle)
type ItemRepository interface {
	GetAll() ([]model.Item, error)
	Get(id int) (model.Item, error)
	Create(item model.Item) (model.Item, error)
	Update(id int, item model.Item) (model.Item, error)
	Delete(id int) error
}

// Storage defines the main storage interface (local copy to avoid import cycle)
type Storage interface {
	ItemRepo() ItemRepository
	Close() error
	InitializeSampleData() int
}

// storage implements Storage with in-memory storage
type storage struct {
	itemRepo *itemRepository
}

// NewStorage creates a new memory-based storage
func NewStorage() Storage {
	return &storage{
		itemRepo: newItemRepository(),
	}
}

// ItemRepo returns the item repository
func (s *storage) ItemRepo() ItemRepository {
	return s.itemRepo
}

// Close closes the storage connection (no-op for memory storage)
func (s *storage) Close() error {
	return nil
}

// InitializeSampleData adds sample data to the storage
func (s *storage) InitializeSampleData() int {

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
		_, _ = s.itemRepo.Create(item)
	}

	return len(sampleItems)
}

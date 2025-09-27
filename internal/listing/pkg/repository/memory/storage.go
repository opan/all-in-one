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
	InitializeSampleData() int
}

// Storage defines the main storage interface (local copy to avoid import cycle)
type Storage interface {
	Items() ItemRepository
	Close() error
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

// Items returns the item repository
func (s *storage) Items() ItemRepository {
	return s.itemRepo
}

// Close closes the storage connection (no-op for memory storage)
func (s *storage) Close() error {
	return nil
}

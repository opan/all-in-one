package repository

import "github.com/all-in-one/internal/listing/pkg/model"

// ItemRepository defines the interface for item storage operations
type ItemRepository interface {
	// GetAll returns all listing items
	GetAll() ([]model.Item, error)

	// Get returns a listing item by ID
	Get(id int) (model.Item, error)

	// Create adds a new listing item
	Create(item model.Item) (model.Item, error)

	// Update modifies an existing listing item
	Update(id int, item model.Item) (model.Item, error)

	// Delete removes a listing item
	Delete(id int) error

	// InitializeSampleData adds sample data to the storage
	InitializeSampleData() int
}

// Storage defines the main storage interface that aggregates all repositories
type Storage interface {
	// Items returns the item repository
	Items() ItemRepository

	// Close closes the storage connection
	Close() error
}

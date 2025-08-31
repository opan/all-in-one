package storage

import (
	"github.com/all-in-one/pkg/models"
)

// ItemStorage defines the interface for item storage operations
type ItemStorage interface {
	// GetAll returns all items
	GetAll() ([]models.Item, error)

	// Get returns an item by ID
	Get(id int) (models.Item, error)

	// Create adds a new item
	Create(item models.Item) (models.Item, error)

	// Update modifies an existing item
	Update(id int, item models.Item) (models.Item, error)

	// Delete removes an item
	Delete(id int) error
}

package model

import "time"

// Item represents a listing item
type Item struct {
	ID          int       `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Storage defines the interface for listing storage operations
type Storage interface {
	// GetAll returns all listing items
	GetAll() ([]Item, error)

	// Get returns a listing item by ID
	Get(id int) (Item, error)

	// Create adds a new listing item
	Create(item Item) (Item, error)

	// Update modifies an existing listing item
	Update(id int, item Item) (Item, error)

	// Delete removes a listing item
	Delete(id int) error

	// InitializeSampleData adds sample data to the storage
	InitializeSampleData() int
}

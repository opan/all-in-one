package repository

import (
	"github.com/all-in-one/internal/listing/pkg/model"
)

// ItemRepository defines the interface for item storage operations
type ItemRepository interface {
	// GetAll returns all listing items
	GetAll() ([]model.Item, error)

	// GetByTopicID returns all items for a specific topic
	GetByTopicID(topicID int) ([]model.Item, error)

	// Get returns a listing item by ID
	Get(id int) (model.Item, error)

	// Create adds a new listing item
	Create(item model.Item) (model.Item, error)

	// Update modifies an existing listing item
	Update(id int, item model.Item) (model.Item, error)

	// Delete removes a listing item
	Delete(id int) error
	DeleteByTopicID(topicID int) error
}

type TopicRepository interface {
	GetAll() ([]model.Topic, error)
	Get(id int) (model.Topic, error)
	Create(item model.Topic) (model.Topic, error)
	Update(id int, item model.Topic) (model.Topic, error)
	Delete(id int) error
}

// Storage defines the main storage interface that aggregates all repositories
type Storage interface {

	// ItemRepo returns the item repository
	ItemRepo() ItemRepository
	TopicRepo() TopicRepository

	// Close closes the storage connection
	Close() error

	InitializeSampleData() int
}

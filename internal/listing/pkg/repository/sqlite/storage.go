package sqlite

import (
	"context"

	"github.com/all-in-one/internal/config"
	"github.com/all-in-one/internal/listing/pkg/model"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

// ItemRepository defines the interface for item storage operations (local copy to avoid import cycle)
type ItemRepository interface {
	GetAll() ([]model.Item, error)
	Get(id int) (model.Item, error)
	Create(item model.Item) (model.Item, error)
	Update(id int, item model.Item) (model.Item, error)
	Delete(id int) error
}

type TopicRepository interface {
	GetAll() ([]model.Topic, error)
	Get(id int) (model.Topic, error)
	Create(topic model.Topic) (model.Topic, error)
	Update(id int, topic model.Topic) (model.Topic, error)
	Delete(id int) error
}

// Storage defines the main storage interface (local copy to avoid import cycle)
type Storage interface {
	ItemRepo() ItemRepository
	TopicRepo() TopicRepository
	Close() error
	InitializeSampleData() int
}

// storage implements Storage with SQLite storage
type storage struct {
	db        *sqlx.DB
	itemRepo  *itemRepository
	topicRepo *topicRepository
}

// NewStorage creates a new SQLite-based storage
func NewStorage(ctx context.Context, config config.Config) (Storage, error) {
	db, err := sqlx.Open("sqlite3", config.Storage.SQLite.DBPath)
	if err != nil {
		return nil, err
	}

	// Create table if it doesn't exist
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS listing_items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			description TEXT,
			created_at TIMESTAMP,
			updated_at TIMESTAMP
		)
	`)
	if err != nil {
		db.Close()
		return nil, err
	}

	return &storage{
		db:        db,
		itemRepo:  newItemRepository(db),
		topicRepo: newTopicRepository(db),
	}, nil
}

// ItemRepo returns the item repository
func (s *storage) ItemRepo() ItemRepository {
	return s.itemRepo
}

func (s *storage) TopicRepo() TopicRepository {
	return s.topicRepo
}

// Close closes the database connection
func (s *storage) Close() error {
	return s.db.Close()
}

func (s *storage) InitializeSampleData() int {
	// Check if there's already data
	items, err := s.itemRepo.GetAll()
	if err != nil || len(items) > 0 {
		return 0 // Don't add sample data if there's an error or if data exists
	}

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
		_, err := s.itemRepo.Create(item)
		if err != nil {
			return 0
		}
	}

	return len(sampleItems)
}

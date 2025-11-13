package sqlite

import (
	"context"
	"fmt"

	"github.com/all-in-one/internal/config"
	"github.com/all-in-one/internal/listing/pkg/model"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

// storage implements the Storage interface from repository package
type storage struct {
	db        *sqlx.DB
	itemRepo  *itemRepository
	topicRepo *topicRepository
}

// NewStorage creates a new SQLite-based storage
func NewStorage(ctx context.Context, config config.Config) (*storage, error) {
	db, err := sqlx.Open("sqlite3", config.Storage.SQLite.DBPath)
	if err != nil {
		return nil, err
	}

	err = dbMigrate(db)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to execute db migration: %w", err)
	}

	return &storage{
		db:        db,
		itemRepo:  newItemRepository(db),
		topicRepo: newTopicRepository(db),
	}, nil
}

func dbMigrate(db *sqlx.DB) error {
	migration := `CREATE TABLE IF NOT EXISTS topics (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			description TEXT,
			created_at TIMESTAMP,
			updated_at TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			topic_id INTEGER,
			title TEXT NOT NULL,
			description TEXT,
			created_at TIMESTAMP,
			updated_at TIMESTAMP,
			FOREIGN KEY (topic_id) REFERENCES topics(id)
		);`
	_, err := db.Exec(migration)

	return err
}

// ItemRepo returns the item repository
func (s *storage) ItemRepo() *itemRepository {
	return s.itemRepo
}

func (s *storage) TopicRepo() *topicRepository {
	return s.topicRepo
}

// Close closes the database connection
func (s *storage) Close() error {
	return s.db.Close()
}

func (s *storage) InitializeSampleData(ctx context.Context) int {
	topics, err := s.topicRepo.GetAll(ctx)
	if err != nil || len(topics) > 0 {
		return 0
	}

	// Check if there's already data
	items, err := s.itemRepo.GetAll(ctx)
	if err != nil || len(items) > 0 {
		return 0 // Don't add sample data if there's an error or if data exists
	}

	sampleTopics := []model.Topic{
		{
			Name:        "Books to Read",
			Description: "A list of must-read books",
		},
		{
			Name:        "Movies to Watch",
			Description: "A collection of classic and modern movies",
		},
	}

	var sampleItems []model.Item

	for _, topic := range sampleTopics {
		createdTopic, err := s.topicRepo.Create(ctx, topic)
		if err != nil {
			return 0
		}

		sampleItems = []model.Item{
			{
				TopicID:     createdTopic.ID,
				Title:       "Sample Item 1",
				Description: "This is a sample item for testing",
			},
			{
				TopicID:     createdTopic.ID,
				Title:       "Sample Item 2",
				Description: "Another sample item with different content",
			},
			{
				TopicID:     createdTopic.ID,
				Title:       "Sample Item 3",
				Description: "Third sample item for demonstration",
			},
		}

		for _, item := range sampleItems {
			_, err := s.itemRepo.Create(ctx, item)
			if err != nil {
				return 0
			}
		}

	}

	return len(sampleTopics)
}

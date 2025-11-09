package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/all-in-one/internal/config"
	"github.com/all-in-one/internal/listing/pkg/model"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

// Storage implements the storage interface with SQLite backend
// Exported so that the repository adapter can reference it
type Storage = storage

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

// type QueryOption func(*QueryOptions)

// type QueryOptions struct {
// 	tx *sqlx.Tx
// }

// func (s *storage) WithTx(tx *sqlx.Tx) QueryOption {
// 	return func(opts *QueryOptions) {
// 		opts.tx = tx
// 	}
// }

// func (s *storage) Tx(ctx context.Context) (*sqlx.Tx, error) {
// 	tx, err := s.db.BeginTxx(ctx, &sql.TxOptions{})
// 	if err != nil {
// 		return nil, fmt.Errorf("unable to begin transaction: %w", err)
// 	}

// 	return tx, nil
// }

type queryOptions struct {
	tx *sqlx.Tx
}

func (q *queryOptions) TxCommit() error {
	return q.tx.Commit()
}

func (q *queryOptions) TxRollback() error {
	return q.tx.Rollback()
}

func (s *storage) CreateTx(ctx context.Context) (*queryOptions, error) {
	tx, err := s.db.BeginTxx(ctx, &sql.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("unable to begin transaction: %w", err)
	}

	return &queryOptions{
		tx: tx,
	}, nil
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

func (s *storage) InitializeSampleData() int {
	topics, err := s.topicRepo.GetAll()
	if err != nil || len(topics) > 0 {
		return 0 // Don't add sample data if there's an error or if data exists
	}

	// Check if there's already data
	items, err := s.itemRepo.GetAll()
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
		createdTopic, err := s.topicRepo.Create(topic)
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
			_, err := s.itemRepo.Create(item)
			if err != nil {
				return 0
			}
		}

	}

	return len(sampleTopics)
}

package sqlite

import (
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog"
)

// storage implements the Storage interface from repository package
type storage struct {
	db          *sqlx.DB
	log         zerolog.Logger
	itemRepo    *itemRepository
	topicRepo   *topicRepository
	shouldClose bool
}

// NewFromDB creates a storage from an existing shared DB connection.
func NewFromDB(db *sqlx.DB, log zerolog.Logger) *storage {
	return &storage{
		db:          db,
		log:         log,
		itemRepo:    newItemRepository(db),
		topicRepo:   newTopicRepository(db),
		shouldClose: false,
	}
}

// ItemRepo returns the item repository
func (s *storage) ItemRepo() *itemRepository {
	return s.itemRepo
}

func (s *storage) TopicRepo() *topicRepository {
	return s.topicRepo
}

// Close closes the database connection if this storage owns it.
func (s *storage) Close() error {
	if s.shouldClose && s.db != nil {
		return s.db.Close()
	}
	return nil
}

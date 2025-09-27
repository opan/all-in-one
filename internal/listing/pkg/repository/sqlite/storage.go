package sqlite

import (
	"database/sql"

	"github.com/all-in-one/internal/listing/pkg/model"
	_ "github.com/mattn/go-sqlite3"
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

// storage implements Storage with SQLite storage
type storage struct {
	db       *sql.DB
	itemRepo *itemRepository
}

// NewStorage creates a new SQLite-based storage
func NewStorage(dbPath string) (Storage, error) {
	db, err := sql.Open("sqlite3", dbPath)
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
		db:       db,
		itemRepo: newItemRepository(db),
	}, nil
}

// Items returns the item repository
func (s *storage) Items() ItemRepository {
	return s.itemRepo
}

// Close closes the database connection
func (s *storage) Close() error {
	return s.db.Close()
}

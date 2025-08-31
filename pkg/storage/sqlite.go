package storage

import (
	"database/sql"
	"errors"
	"time"

	"github.com/all-in-one/pkg/models"
	_ "github.com/mattn/go-sqlite3"
)

// SQLiteStorage implements ItemStorage with an SQLite database
type SQLiteStorage struct {
	db *sql.DB
}

// NewSQLiteStorage creates a new SQLite-based storage
func NewSQLiteStorage(dbPath string) (*SQLiteStorage, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	// Create table if it doesn't exist
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			description TEXT,
			created_at TIMESTAMP,
			updated_at TIMESTAMP
		)
	`)
	if err != nil {
		return nil, err
	}

	return &SQLiteStorage{db: db}, nil
}

// GetAll returns all items
func (s *SQLiteStorage) GetAll() ([]models.Item, error) {
	rows, err := s.db.Query(`
		SELECT id, title, description, created_at, updated_at 
		FROM items
		ORDER BY id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.Item
	for rows.Next() {
		var item models.Item
		var createdAt, updatedAt string

		err := rows.Scan(&item.ID, &item.Name, &item.Description, &createdAt, &updatedAt)
		if err != nil {
			return nil, err
		}

		// Parse timestamps
		item.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		item.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

		items = append(items, item)
	}

	return items, nil
}

// Get returns an item by ID
func (s *SQLiteStorage) Get(id int) (models.Item, error) {
	var item models.Item
	var createdAt, updatedAt string

	err := s.db.QueryRow(`
		SELECT id, Name, description, created_at, updated_at 
		FROM items 
		WHERE id = ?
	`, id).Scan(&item.ID, &item.Name, &item.Description, &createdAt, &updatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.Item{}, ErrItemNotFound
		}
		return models.Item{}, err
	}

	// Parse timestamps
	item.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	item.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	return item, nil
}

// Create adds a new item
func (s *SQLiteStorage) Create(item models.Item) (models.Item, error) {
	now := time.Now().Format(time.RFC3339)

	result, err := s.db.Exec(`
		INSERT INTO items (Name, description, created_at, updated_at) 
		VALUES (?, ?, ?, ?)
	`, item.Name, item.Description, now, now)

	if err != nil {
		return models.Item{}, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return models.Item{}, err
	}

	// Set the returned item with current values
	item.ID = int(id)
	item.CreatedAt, _ = time.Parse(time.RFC3339, now)
	item.UpdatedAt = item.CreatedAt

	return item, nil
}

// Update modifies an existing item
func (s *SQLiteStorage) Update(id int, item models.Item) (models.Item, error) {
	// First check if the item exists
	existingItem, err := s.Get(id)
	if err != nil {
		return models.Item{}, err
	}

	now := time.Now().Format(time.RFC3339)

	_, err = s.db.Exec(`
		UPDATE items 
		SET Name = ?, description = ?, updated_at = ? 
		WHERE id = ?
	`, item.Name, item.Description, now, id)

	if err != nil {
		return models.Item{}, err
	}

	// Set the returned item with updated values
	item.ID = id
	item.CreatedAt = existingItem.CreatedAt
	item.UpdatedAt, _ = time.Parse(time.RFC3339, now)

	return item, nil
}

// Delete removes an item
func (s *SQLiteStorage) Delete(id int) error {
	// First check if the item exists
	_, err := s.Get(id)
	if err != nil {
		return err
	}

	_, err = s.db.Exec("DELETE FROM items WHERE id = ?", id)
	return err
}

// InitializeSampleData adds sample data to the storage
func (s *SQLiteStorage) InitializeSampleData() int {
	// Check if there's already data
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM items").Scan(&count)
	if err != nil || count > 0 {
		return 0 // Don't add sample data if there's an error or if data exists
	}

	sampleItems := []models.Item{
		{
			Name:        "Sample Task 1",
			Description: "This is a sample task for testing",
		},
		{
			Name:        "Sample Task 2",
			Description: "Another sample task with different content",
		},
		{
			Name:        "Sample Task 3",
			Description: "Third sample task for demonstration",
		},
	}

	for _, item := range sampleItems {
		_, err := s.Create(item)
		if err != nil {
			return 0
		}
	}

	return len(sampleItems)
}

// Close closes the database connection
func (s *SQLiteStorage) Close() error {
	return s.db.Close()
}

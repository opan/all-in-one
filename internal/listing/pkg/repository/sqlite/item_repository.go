package sqlite

import (
	"database/sql"
	"time"

	"github.com/all-in-one/internal/common"
	"github.com/all-in-one/internal/listing/pkg/model"
	_ "github.com/mattn/go-sqlite3"
)

// itemRepository implements the item repository with SQLite storage
type itemRepository struct {
	db *sql.DB
}

// newItemRepository creates a new SQLite-based item repository
func newItemRepository(db *sql.DB) *itemRepository {
	return &itemRepository{db: db}
}

// GetAll returns all items
func (r *itemRepository) GetAll() ([]model.Item, error) {
	rows, err := r.db.Query(`
		SELECT id, title, description, created_at, updated_at 
		FROM listing_items
		ORDER BY id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.Item
	for rows.Next() {
		var item model.Item
		var createdAt, updatedAt string

		err := rows.Scan(&item.ID, &item.Title, &item.Description, &createdAt, &updatedAt)
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
func (r *itemRepository) Get(id int) (model.Item, error) {
	var item model.Item
	var createdAt, updatedAt string

	err := r.db.QueryRow(`
		SELECT id, title, description, created_at, updated_at 
		FROM listing_items 
		WHERE id = ?
	`, id).Scan(&item.ID, &item.Title, &item.Description, &createdAt, &updatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return model.Item{}, common.ErrNotFound
		}
		return model.Item{}, err
	}

	// Parse timestamps
	item.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	item.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	return item, nil
}

// Create adds a new item
func (r *itemRepository) Create(item model.Item) (model.Item, error) {
	now := time.Now().Format(time.RFC3339)

	result, err := r.db.Exec(`
		INSERT INTO listing_items (title, description, created_at, updated_at) 
		VALUES (?, ?, ?, ?)
	`, item.Title, item.Description, now, now)

	if err != nil {
		return model.Item{}, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return model.Item{}, err
	}

	// Set the returned item with current values
	item.ID = int(id)
	item.CreatedAt, _ = time.Parse(time.RFC3339, now)
	item.UpdatedAt = item.CreatedAt

	return item, nil
}

// Update modifies an existing item
func (r *itemRepository) Update(id int, item model.Item) (model.Item, error) {
	// First check if the item exists
	existingItem, err := r.Get(id)
	if err != nil {
		return model.Item{}, err
	}

	now := time.Now().Format(time.RFC3339)

	_, err = r.db.Exec(`
		UPDATE listing_items 
		SET title = ?, description = ?, updated_at = ? 
		WHERE id = ?
	`, item.Title, item.Description, now, id)

	if err != nil {
		return model.Item{}, err
	}

	// Set the returned item with updated values
	item.ID = id
	item.CreatedAt = existingItem.CreatedAt
	item.UpdatedAt, _ = time.Parse(time.RFC3339, now)

	return item, nil
}

// Delete removes an item
func (r *itemRepository) Delete(id int) error {
	// First check if the item exists
	_, err := r.Get(id)
	if err != nil {
		return err
	}

	_, err = r.db.Exec("DELETE FROM listing_items WHERE id = ?", id)
	return err
}

// InitializeSampleData adds sample data to the storage
func (r *itemRepository) InitializeSampleData() int {
	// Check if there's already data
	var count int
	err := r.db.QueryRow("SELECT COUNT(*) FROM listing_items").Scan(&count)
	if err != nil || count > 0 {
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
		_, err := r.Create(item)
		if err != nil {
			return 0
		}
	}

	return len(sampleItems)
}

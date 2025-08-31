package book

import "time"

// Book represents a book entity
type Book struct {
	ID        int       `json:"id"`
	Title     string    `json:"title"`
	Author    string    `json:"author"`
	ISBN      string    `json:"isbn"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Storage defines the interface for book storage operations
type Storage interface {
	// GetAll returns all books
	GetAll() ([]Book, error)

	// Get returns a book by ID
	Get(id int) (Book, error)

	// Create adds a new book
	Create(book Book) (Book, error)

	// Update modifies an existing book
	Update(id int, book Book) (Book, error)

	// Delete removes a book
	Delete(id int) error
}

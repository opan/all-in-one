package book

import (
	"database/sql"
	"time"

	"github.com/all-in-one/pkg/common"
	_ "github.com/mattn/go-sqlite3"
)

// SQLiteStorage implements Storage with an SQLite database
type SQLiteStorage struct {
	db *sql.DB
}

// NewSQLiteStorage creates a new SQLite-based storage for books
func NewSQLiteStorage(dbPath string) (*SQLiteStorage, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	// Create table if it doesn't exist
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS books (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			author TEXT NOT NULL,
			isbn TEXT,
			created_at TIMESTAMP,
			updated_at TIMESTAMP
		)
	`)
	if err != nil {
		return nil, err
	}

	return &SQLiteStorage{db: db}, nil
}

// GetAll returns all books
func (s *SQLiteStorage) GetAll() ([]Book, error) {
	rows, err := s.db.Query(`
		SELECT id, title, author, isbn, created_at, updated_at 
		FROM books
		ORDER BY id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var books []Book
	for rows.Next() {
		var book Book
		var createdAt, updatedAt string

		err := rows.Scan(&book.ID, &book.Title, &book.Author, &book.ISBN, &createdAt, &updatedAt)
		if err != nil {
			return nil, err
		}

		// Parse timestamps
		book.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		book.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

		books = append(books, book)
	}

	return books, nil
}

// Get returns a book by ID
func (s *SQLiteStorage) Get(id int) (Book, error) {
	var book Book
	var createdAt, updatedAt string

	err := s.db.QueryRow(`
		SELECT id, title, author, isbn, created_at, updated_at 
		FROM books 
		WHERE id = ?
	`, id).Scan(&book.ID, &book.Title, &book.Author, &book.ISBN, &createdAt, &updatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return Book{}, common.ErrNotFound
		}
		return Book{}, err
	}

	// Parse timestamps
	book.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	book.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	return book, nil
}

// Create adds a new book
func (s *SQLiteStorage) Create(book Book) (Book, error) {
	now := time.Now().Format(time.RFC3339)

	result, err := s.db.Exec(`
		INSERT INTO books (title, author, isbn, created_at, updated_at) 
		VALUES (?, ?, ?, ?, ?)
	`, book.Title, book.Author, book.ISBN, now, now)

	if err != nil {
		return Book{}, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return Book{}, err
	}

	// Set the returned book with current values
	book.ID = int(id)
	book.CreatedAt, _ = time.Parse(time.RFC3339, now)
	book.UpdatedAt = book.CreatedAt

	return book, nil
}

// Update modifies an existing book
func (s *SQLiteStorage) Update(id int, book Book) (Book, error) {
	// First check if the book exists
	existingBook, err := s.Get(id)
	if err != nil {
		return Book{}, err
	}

	now := time.Now().Format(time.RFC3339)

	_, err = s.db.Exec(`
		UPDATE books 
		SET title = ?, author = ?, isbn = ?, updated_at = ? 
		WHERE id = ?
	`, book.Title, book.Author, book.ISBN, now, id)

	if err != nil {
		return Book{}, err
	}

	// Set the returned book with updated values
	book.ID = id
	book.CreatedAt = existingBook.CreatedAt
	book.UpdatedAt, _ = time.Parse(time.RFC3339, now)

	return book, nil
}

// Delete removes a book
func (s *SQLiteStorage) Delete(id int) error {
	// First check if the book exists
	_, err := s.Get(id)
	if err != nil {
		return err
	}

	_, err = s.db.Exec("DELETE FROM books WHERE id = ?", id)
	return err
}

// InitializeSampleData adds sample data to the storage
func (s *SQLiteStorage) InitializeSampleData() int {
	// Check if there's already data
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM books").Scan(&count)
	if err != nil || count > 0 {
		return 0 // Don't add sample data if there's an error or if data exists
	}

	sampleBooks := []Book{
		{
			Title:  "The Go Programming Language",
			Author: "Alan A. A. Donovan and Brian W. Kernighan",
			ISBN:   "978-0134190440",
		},
		{
			Title:  "Clean Code",
			Author: "Robert C. Martin",
			ISBN:   "978-0132350884",
		},
		{
			Title:  "Design Patterns",
			Author: "Erich Gamma, Richard Helm, Ralph Johnson, John Vlissides",
			ISBN:   "978-0201633610",
		},
	}

	for _, book := range sampleBooks {
		_, err := s.Create(book)
		if err != nil {
			return 0
		}
	}

	return len(sampleBooks)
}

// Close closes the database connection
func (s *SQLiteStorage) Close() error {
	return s.db.Close()
}

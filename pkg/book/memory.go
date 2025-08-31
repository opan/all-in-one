package book

import (
	"sync"
	"time"

	"github.com/all-in-one/pkg/common"
)

// MemoryStorage implements Storage with an in-memory data store
type MemoryStorage struct {
	books  map[int]Book
	lastID int
	mutex  sync.RWMutex
}

// NewMemoryStorage creates a new memory-based storage for books
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		books:  make(map[int]Book),
		lastID: 0,
	}
}

// GetAll returns all books
func (s *MemoryStorage) GetAll() ([]Book, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	books := make([]Book, 0, len(s.books))
	for _, book := range s.books {
		books = append(books, book)
	}

	return books, nil
}

// Get returns a book by ID
func (s *MemoryStorage) Get(id int) (Book, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	book, exists := s.books[id]
	if !exists {
		return Book{}, common.ErrNotFound
	}

	return book, nil
}

// Create adds a new book
func (s *MemoryStorage) Create(book Book) (Book, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Assign ID and timestamps
	s.lastID++
	book.ID = s.lastID
	book.CreatedAt = time.Now()
	book.UpdatedAt = time.Now()

	// Store the book
	s.books[book.ID] = book

	return book, nil
}

// Update modifies an existing book
func (s *MemoryStorage) Update(id int, book Book) (Book, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	existingBook, exists := s.books[id]
	if !exists {
		return Book{}, common.ErrNotFound
	}

	// Update book while preserving ID and CreatedAt
	book.ID = id
	book.CreatedAt = existingBook.CreatedAt
	book.UpdatedAt = time.Now()

	s.books[id] = book

	return book, nil
}

// Delete removes a book
func (s *MemoryStorage) Delete(id int) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	_, exists := s.books[id]
	if !exists {
		return common.ErrNotFound
	}

	delete(s.books, id)
	return nil
}

// InitializeSampleData adds sample data to the storage
func (s *MemoryStorage) InitializeSampleData() int {
	s.mutex.Lock()
	defer s.mutex.Unlock()

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
		s.lastID++
		book.ID = s.lastID
		book.CreatedAt = time.Now()
		book.UpdatedAt = time.Now()
		s.books[book.ID] = book
	}

	return len(sampleBooks)
}

package book

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/all-in-one/pkg/common"
	"github.com/gorilla/mux"
)

// Handler manages HTTP requests for the book service
type Handler struct {
	store Storage
}

// NewHandler creates a new book handler
func NewHandler(store Storage) *Handler {
	return &Handler{
		store: store,
	}
}

// RegisterRoutes registers the book routes to the given router
func (h *Handler) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/books", h.GetBooks).Methods("GET")
	router.HandleFunc("/books", h.CreateBook).Methods("POST")
	router.HandleFunc("/books/{id}", h.GetBook).Methods("GET")
	router.HandleFunc("/books/{id}", h.UpdateBook).Methods("PUT")
	router.HandleFunc("/books/{id}", h.DeleteBook).Methods("DELETE")
}

// GET /books - Get all books
func (h *Handler) GetBooks(w http.ResponseWriter, r *http.Request) {
	books, err := h.store.GetAll()
	if err != nil {
		sendError(w, "Failed to retrieve books", http.StatusInternalServerError)
		return
	}

	response := common.Response{
		Success: true,
		Data:    books,
	}

	sendJSON(w, response, http.StatusOK)
}

// GET /books/{id} - Get book by ID
func (h *Handler) GetBook(w http.ResponseWriter, r *http.Request) {
	id, err := getIDFromRequest(r)
	if err != nil {
		sendError(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	book, err := h.store.Get(id)
	if err != nil {
		if err == common.ErrNotFound {
			sendError(w, "Book not found", http.StatusNotFound)
			return
		}
		sendError(w, "Failed to retrieve book", http.StatusInternalServerError)
		return
	}

	response := common.Response{
		Success: true,
		Data:    book,
	}

	sendJSON(w, response, http.StatusOK)
}

// POST /books - Create a new book
func (h *Handler) CreateBook(w http.ResponseWriter, r *http.Request) {
	var newBook Book
	if err := json.NewDecoder(r.Body).Decode(&newBook); err != nil {
		sendError(w, "Invalid JSON data", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if newBook.Title == "" {
		sendError(w, "Title is required", http.StatusBadRequest)
		return
	}
	if newBook.Author == "" {
		sendError(w, "Author is required", http.StatusBadRequest)
		return
	}

	createdBook, err := h.store.Create(newBook)
	if err != nil {
		sendError(w, "Failed to create book", http.StatusInternalServerError)
		return
	}

	response := common.Response{
		Success: true,
		Message: "Book created successfully",
		Data:    createdBook,
	}

	sendJSON(w, response, http.StatusCreated)
}

// PUT /books/{id} - Update an existing book
func (h *Handler) UpdateBook(w http.ResponseWriter, r *http.Request) {
	id, err := getIDFromRequest(r)
	if err != nil {
		sendError(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	var updatedBook Book
	if err := json.NewDecoder(r.Body).Decode(&updatedBook); err != nil {
		sendError(w, "Invalid JSON data", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if updatedBook.Title == "" {
		sendError(w, "Title is required", http.StatusBadRequest)
		return
	}
	if updatedBook.Author == "" {
		sendError(w, "Author is required", http.StatusBadRequest)
		return
	}

	result, err := h.store.Update(id, updatedBook)
	if err != nil {
		if err == common.ErrNotFound {
			sendError(w, "Book not found", http.StatusNotFound)
			return
		}
		sendError(w, "Failed to update book", http.StatusInternalServerError)
		return
	}

	response := common.Response{
		Success: true,
		Message: "Book updated successfully",
		Data:    result,
	}

	sendJSON(w, response, http.StatusOK)
}

// DELETE /books/{id} - Delete a book
func (h *Handler) DeleteBook(w http.ResponseWriter, r *http.Request) {
	id, err := getIDFromRequest(r)
	if err != nil {
		sendError(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	err = h.store.Delete(id)
	if err != nil {
		if err == common.ErrNotFound {
			sendError(w, "Book not found", http.StatusNotFound)
			return
		}
		sendError(w, "Failed to delete book", http.StatusInternalServerError)
		return
	}

	response := common.Response{
		Success: true,
		Message: "Book deleted successfully",
	}

	sendJSON(w, response, http.StatusOK)
}

// Helper Functions

// getIDFromRequest extracts the ID from the request URL
func getIDFromRequest(r *http.Request) (int, error) {
	vars := mux.Vars(r)
	return strconv.Atoi(vars["id"])
}

// sendJSON sends a JSON response
func sendJSON(w http.ResponseWriter, data interface{}, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// sendError sends an error response
func sendError(w http.ResponseWriter, message string, statusCode int) {
	response := common.Response{
		Success: false,
		Error:   message,
	}
	sendJSON(w, response, statusCode)
}

package listing

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/all-in-one/pkg/models"
	"github.com/all-in-one/pkg/storage"
	"github.com/gorilla/mux"
)

// Handler manages HTTP requests for the listing service
type Handler struct {
	store storage.ItemStorage
}

// NewHandler creates a new listing handler
func NewHandler(store storage.ItemStorage) *Handler {
	return &Handler{
		store: store,
	}
}

// RegisterRoutes registers the listing routes to the given router
func (h *Handler) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/items", h.GetItems).Methods("GET")
	router.HandleFunc("/items", h.CreateItem).Methods("POST")
	router.HandleFunc("/items/{id}", h.GetItem).Methods("GET")
	router.HandleFunc("/items/{id}", h.UpdateItem).Methods("PUT")
	router.HandleFunc("/items/{id}", h.DeleteItem).Methods("DELETE")
}

// GET /items - Get all items
func (h *Handler) GetItems(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.GetAll()
	if err != nil {
		sendError(w, "Failed to retrieve items", http.StatusInternalServerError)
		return
	}

	response := models.Response{
		Success: true,
		Data:    items,
	}

	sendJSON(w, response, http.StatusOK)
}

// GET /items/{id} - Get item by ID
func (h *Handler) GetItem(w http.ResponseWriter, r *http.Request) {
	id, err := getIDFromRequest(r)
	if err != nil {
		sendError(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	item, err := h.store.Get(id)
	if err != nil {
		if err == storage.ErrItemNotFound {
			sendError(w, "Item not found", http.StatusNotFound)
			return
		}
		sendError(w, "Failed to retrieve item", http.StatusInternalServerError)
		return
	}

	response := models.Response{
		Success: true,
		Data:    item,
	}

	sendJSON(w, response, http.StatusOK)
}

// POST /items - Create a new item
func (h *Handler) CreateItem(w http.ResponseWriter, r *http.Request) {
	var newItem models.Item
	if err := json.NewDecoder(r.Body).Decode(&newItem); err != nil {
		sendError(w, "Invalid JSON data", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if newItem.Title == "" {
		sendError(w, "Title is required", http.StatusBadRequest)
		return
	}

	createdItem, err := h.store.Create(newItem)
	if err != nil {
		sendError(w, "Failed to create item", http.StatusInternalServerError)
		return
	}

	response := models.Response{
		Success: true,
		Message: "Item created successfully",
		Data:    createdItem,
	}

	sendJSON(w, response, http.StatusCreated)
}

// PUT /items/{id} - Update an existing item
func (h *Handler) UpdateItem(w http.ResponseWriter, r *http.Request) {
	id, err := getIDFromRequest(r)
	if err != nil {
		sendError(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	var updatedItem models.Item
	if err := json.NewDecoder(r.Body).Decode(&updatedItem); err != nil {
		sendError(w, "Invalid JSON data", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if updatedItem.Title == "" {
		sendError(w, "Title is required", http.StatusBadRequest)
		return
	}

	result, err := h.store.Update(id, updatedItem)
	if err != nil {
		if err == storage.ErrItemNotFound {
			sendError(w, "Item not found", http.StatusNotFound)
			return
		}
		sendError(w, "Failed to update item", http.StatusInternalServerError)
		return
	}

	response := models.Response{
		Success: true,
		Message: "Item updated successfully",
		Data:    result,
	}

	sendJSON(w, response, http.StatusOK)
}

// DELETE /items/{id} - Delete an item
func (h *Handler) DeleteItem(w http.ResponseWriter, r *http.Request) {
	id, err := getIDFromRequest(r)
	if err != nil {
		sendError(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	err = h.store.Delete(id)
	if err != nil {
		if err == storage.ErrItemNotFound {
			sendError(w, "Item not found", http.StatusNotFound)
			return
		}
		sendError(w, "Failed to delete item", http.StatusInternalServerError)
		return
	}

	response := models.Response{
		Success: true,
		Message: "Item deleted successfully",
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
	response := models.Response{
		Success: false,
		Error:   message,
	}
	sendJSON(w, response, statusCode)
}

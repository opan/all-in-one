package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	httpHelper "github.com/all-in-one/internal/http"
	"github.com/all-in-one/internal/listing/pkg/model"
	"github.com/all-in-one/internal/listing/pkg/repository"
	"github.com/gorilla/mux"
)

// Handler manages HTTP requests for the listing service
type Handler struct {
	storage repository.Storage
}

// NewHandler creates a new listing handler
func NewHandler(storage repository.Storage) *Handler {
	return &Handler{
		storage: storage,
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

// GetItems godoc
// @Summary Get all items
// @Description Retrieve a list of all listing items
// @Tags items
// @Produce json
// @Success 200 {object} httpHelper.Response{data=[]model.Item} "List of items"
// @Failure 500 {object} httpHelper.Response "Internal server error"
// @Router /items [get]
func (h *Handler) GetItems(w http.ResponseWriter, r *http.Request) {
	items, err := h.storage.Items().GetAll()
	if err != nil {
		sendError(w, "Failed to retrieve items", http.StatusInternalServerError)
		return
	}

	response := httpHelper.Response{
		Success: true,
		Data:    items,
	}

	sendJSON(w, response, http.StatusOK)
}

// GetItem godoc
// @Summary Get item by ID
// @Description Retrieve a single item by its ID
// @Tags items
// @Produce json
// @Param id path int true "Item ID"
// @Success 200 {object} httpHelper.Response{data=model.Item} "Item details"
// @Failure 400 {object} httpHelper.Response "Invalid ID"
// @Failure 404 {object} httpHelper.Response "Item not found"
// @Failure 500 {object} httpHelper.Response "Internal server error"
// @Router /items/{id} [get]
func (h *Handler) GetItem(w http.ResponseWriter, r *http.Request) {
	id, err := getIDFromRequest(r)
	if err != nil {
		sendError(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	item, err := h.storage.Items().Get(id)
	if err != nil {
		if err == httpHelper.ErrNotFound {
			sendError(w, "Item not found", http.StatusNotFound)
			return
		}
		sendError(w, "Failed to retrieve item", http.StatusInternalServerError)
		return
	}

	response := httpHelper.Response{
		Success: true,
		Data:    item,
	}

	sendJSON(w, response, http.StatusOK)
}

// CreateItem godoc
// @Summary Create a new item
// @Description Create a new listing item
// @Tags items
// @Accept json
// @Produce json
// @Param item body model.Item true "Item to create"
// @Success 201 {object} httpHelper.Response{data=model.Item} "Item created successfully"
// @Failure 400 {object} httpHelper.Response "Invalid JSON data or missing required fields"
// @Failure 500 {object} httpHelper.Response "Internal server error"
// @Router /items [post]
func (h *Handler) CreateItem(w http.ResponseWriter, r *http.Request) {
	var newItem model.Item
	if err := json.NewDecoder(r.Body).Decode(&newItem); err != nil {
		sendError(w, "Invalid JSON data", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if newItem.Title == "" {
		sendError(w, "Title is required", http.StatusBadRequest)
		return
	}

	createdItem, err := h.storage.Items().Create(newItem)
	if err != nil {
		sendError(w, "Failed to create item", http.StatusInternalServerError)
		return
	}

	response := httpHelper.Response{
		Success: true,
		Message: "Item created successfully",
		Data:    createdItem,
	}

	sendJSON(w, response, http.StatusCreated)
}

// UpdateItem godoc
// @Summary Update an existing item
// @Description Update an existing listing item by ID
// @Tags items
// @Accept json
// @Produce json
// @Param id path int true "Item ID"
// @Param item body model.Item true "Updated item data"
// @Success 200 {object} httpHelper.Response{data=model.Item} "Item updated successfully"
// @Failure 400 {object} httpHelper.Response "Invalid ID or JSON data"
// @Failure 404 {object} httpHelper.Response "Item not found"
// @Failure 500 {object} httpHelper.Response "Internal server error"
// @Router /items/{id} [put]
func (h *Handler) UpdateItem(w http.ResponseWriter, r *http.Request) {
	id, err := getIDFromRequest(r)
	if err != nil {
		sendError(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	var updatedItem model.Item
	if err := json.NewDecoder(r.Body).Decode(&updatedItem); err != nil {
		sendError(w, "Invalid JSON data", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if updatedItem.Title == "" {
		sendError(w, "Title is required", http.StatusBadRequest)
		return
	}

	result, err := h.storage.Items().Update(id, updatedItem)
	if err != nil {
		if err == httpHelper.ErrNotFound {
			sendError(w, "Item not found", http.StatusNotFound)
			return
		}
		sendError(w, "Failed to update item", http.StatusInternalServerError)
		return
	}

	response := httpHelper.Response{
		Success: true,
		Message: "Item updated successfully",
		Data:    result,
	}

	sendJSON(w, response, http.StatusOK)
}

// DeleteItem godoc
// @Summary Delete an item
// @Description Delete a listing item by ID
// @Tags items
// @Produce json
// @Param id path int true "Item ID"
// @Success 200 {object} httpHelper.Response "Item deleted successfully"
// @Failure 400 {object} httpHelper.Response "Invalid ID"
// @Failure 404 {object} httpHelper.Response "Item not found"
// @Failure 500 {object} httpHelper.Response "Internal server error"
// @Router /items/{id} [delete]
func (h *Handler) DeleteItem(w http.ResponseWriter, r *http.Request) {
	id, err := getIDFromRequest(r)
	if err != nil {
		sendError(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	err = h.storage.Items().Delete(id)
	if err != nil {
		if err == httpHelper.ErrNotFound {
			sendError(w, "Item not found", http.StatusNotFound)
			return
		}
		sendError(w, "Failed to delete item", http.StatusInternalServerError)
		return
	}

	response := httpHelper.Response{
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
	response := httpHelper.Response{
		Success: false,
		Error:   message,
	}
	sendJSON(w, response, statusCode)
}

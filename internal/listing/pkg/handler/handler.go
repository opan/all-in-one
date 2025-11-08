package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	httpHelper "github.com/all-in-one/internal/http"
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
	// Topic routes
	router.HandleFunc("/topics", h.GetTopics).Methods("GET")
	router.HandleFunc("/topics", h.CreateTopic).Methods("POST")
	router.HandleFunc("/topics/{id}", h.GetTopic).Methods("GET")
	router.HandleFunc("/topics/{id}", h.UpdateTopic).Methods("PUT")
	router.HandleFunc("/topics/{id}", h.DeleteTopic).Methods("DELETE")

	// Item routes (nested under topics)
	router.HandleFunc("/topics/{topic_id}/items", h.GetItems).Methods("GET")
	router.HandleFunc("/topics/{topic_id}/items", h.CreateItem).Methods("POST")
	router.HandleFunc("/topics/{topic_id}/items/{id}", h.GetItem).Methods("GET")
	router.HandleFunc("/topics/{topic_id}/items/{id}", h.UpdateItem).Methods("PUT")
	router.HandleFunc("/topics/{topic_id}/items/{id}", h.DeleteItem).Methods("DELETE")
}

// Helper Functions

// getIDFromRequest extracts the ID from the request URL
func getIDFromRequest(r *http.Request) (int, error) {
	vars := mux.Vars(r)
	return strconv.Atoi(vars["id"])
}

// getTopicIDFromRequest extracts the topic ID from the request URL
func getTopicIDFromRequest(r *http.Request) (int, error) {
	vars := mux.Vars(r)
	return strconv.Atoi(vars["topic_id"])
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

package handler

import (
	"net/http"
	"strconv"

	"github.com/all-in-one/internal/config"
	"github.com/all-in-one/internal/listing/repository"
	"github.com/gorilla/mux"
)

// Handler manages HTTP requests for the listing service
type Handler struct {
	storage repository.Storage
	config  config.Config
	metrics *handlerMetrics
}

// NewHandler creates a new listing handler
func NewHandler(storage repository.Storage, config config.Config) *Handler {
	return &Handler{
		storage: storage,
		config:  config,
		metrics: newHandlerMetrics(),
	}
}

// RegisterRoutes registers the listing routes to the given router
func (h *Handler) RegisterRoutes(router *mux.Router) {
	// implement later
}

// RegisterAuthenticatedRoutes registers routes that require JWT authentication
func (h *Handler) RegisterAuthenticatedRoutes(router *mux.Router) {
	router.HandleFunc("/topics", h.GetTopics).Methods("GET")
	router.HandleFunc("/topics", h.CreateTopic).Methods("POST")
	router.HandleFunc("/topics/{id}", h.GetTopic).Methods("GET")
	router.HandleFunc("/topics/{id}", h.UpdateTopic).Methods("PUT")
	router.HandleFunc("/topics/{id}", h.DeleteTopic).Methods("DELETE")

	router.HandleFunc("/topics/{topic_id}/items", h.GetItems).Methods("GET")
	router.HandleFunc("/topics/{topic_id}/items", h.CreateItem).Methods("POST")
	router.HandleFunc("/topics/{topic_id}/items/{id}", h.GetItem).Methods("GET")
	router.HandleFunc("/topics/{topic_id}/items/{id}", h.UpdateItem).Methods("PUT")
	router.HandleFunc("/topics/{topic_id}/items/{id}", h.DeleteItem).Methods("DELETE")

}

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

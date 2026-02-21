package handler

import (
	"github.com/all-in-one/internal/chat/repository"
	"github.com/all-in-one/internal/chat/websocket"
	"github.com/all-in-one/internal/config"
	"github.com/gorilla/mux"
)

// Handler manages HTTP requests for the chat service
type Handler struct {
	storage repository.Storage
	config  config.Config
	hub     *websocket.Hub
}

// NewHandler creates a new chat handler
func NewHandler(storage repository.Storage, config config.Config, hub *websocket.Hub) *Handler {
	return &Handler{
		storage: storage,
		config:  config,
		hub:     hub,
	}
}

// RegisterAuthenticatedRoutes registers routes that require JWT authentication
func (h *Handler) RegisterAuthenticatedRoutes(router *mux.Router) {
	// Session management
	router.HandleFunc("/chats", h.GetSessions).Methods("GET")
	router.HandleFunc("/chats", h.CreateSession).Methods("POST")
	router.HandleFunc("/chats/{id}", h.GetSession).Methods("GET")
	router.HandleFunc("/chats/{id}", h.UpdateSession).Methods("PUT")
	router.HandleFunc("/chats/{id}", h.DeleteSession).Methods("DELETE")
	router.HandleFunc("/chats/{id}/leave", h.LeaveSession).Methods("POST")

	// Message endpoints
	router.HandleFunc("/chats/{id}/messages", h.GetMessages).Methods("GET")
	router.HandleFunc("/chats/{id}/messages", h.SendMessage).Methods("POST")

	// WebSocket endpoint
	router.HandleFunc("/chats/{id}/ws", h.HandleWebSocket)

	// User search for invitations (reuse authnz user search if available)
	// For now, we'll implement a simple search in the handler
	router.HandleFunc("/users/search", h.SearchUsers).Methods("GET")
}

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
	metrics *handlerMetrics
}

// NewHandler creates a new chat handler
func NewHandler(storage repository.Storage, config config.Config, hub *websocket.Hub) *Handler {
	return &Handler{
		storage: storage,
		config:  config,
		hub:     hub,
		metrics: newHandlerMetrics(),
	}
}

// RegisterAuthenticatedRoutes registers routes that require JWT authentication
func (h *Handler) RegisterAuthenticatedRoutes(router *mux.Router) {
	// Invite routes – MUST be registered before /chats/{id} to prevent the
	// gorilla/mux wildcard from swallowing "invites" as an {id} segment.
	router.HandleFunc("/chats/invites", h.CreateInvite).Methods("POST")
	router.HandleFunc("/chats/invites/received", h.GetReceivedInvites).Methods("GET")
	router.HandleFunc("/chats/invites/sent", h.GetSentInvites).Methods("GET")
	router.HandleFunc("/chats/invites/{inviteID}/respond", h.RespondToInvite).Methods("POST")
	router.HandleFunc("/chats/invites/{inviteID}", h.CancelInvite).Methods("DELETE")

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

	// WebSocket endpoint (user-level, not session-specific)
	router.HandleFunc("/ws", h.HandleWebSocket)

	// User search for invitations (reuse authnz user search if available)
	// For now, we'll implement a simple search in the handler
	router.HandleFunc("/users/search", h.SearchUsers).Methods("GET")
}

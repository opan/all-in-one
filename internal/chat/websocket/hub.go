package websocket

import (
	"sync"

	"github.com/all-in-one/internal/chat/model"
	"github.com/rs/zerolog"
)

// Hub maintains the set of active clients and broadcasts messages to the clients
type Hub struct {
	// Registered clients per session (sessionID -> clients)
	sessions map[string]map[*Client]bool

	// Inbound messages from clients
	broadcast chan *BroadcastMessage

	// Register requests from clients
	register chan *Client

	// Unregister requests from clients
	unregister chan *Client

	// Mutex for thread-safe access to sessions
	mu sync.RWMutex

	// Logger
	log zerolog.Logger
}

// BroadcastMessage represents a message to be broadcast to a session
type BroadcastMessage struct {
	SessionID string
	Message   model.WebSocketMessage
}

// NewHub creates a new Hub
func NewHub(log zerolog.Logger) *Hub {
	return &Hub{
		sessions:   make(map[string]map[*Client]bool),
		broadcast:  make(chan *BroadcastMessage, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		log:        log,
	}
}

// Run starts the hub's main event loop
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.registerClient(client)

		case client := <-h.unregister:
			h.unregisterClient(client)

		case message := <-h.broadcast:
			h.broadcastMessage(message)
		}
	}
}

// registerClient adds a client to the hub
func (h *Hub) registerClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.sessions[client.sessionID] == nil {
		h.sessions[client.sessionID] = make(map[*Client]bool)
	}
	h.sessions[client.sessionID][client] = true

	h.log.Info().
		Str("session_id", client.sessionID).
		Str("user_id", client.userID.String()).
		Int("total_clients", len(h.sessions[client.sessionID])).
		Msg("Client registered to session")
}

// unregisterClient removes a client from the hub
func (h *Hub) unregisterClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if clients, ok := h.sessions[client.sessionID]; ok {
		if _, ok := clients[client]; ok {
			delete(clients, client)
			close(client.send)

			// Remove session if no clients left
			if len(clients) == 0 {
				delete(h.sessions, client.sessionID)
			}

			h.log.Info().
				Str("session_id", client.sessionID).
				Str("user_id", client.userID.String()).
				Int("remaining_clients", len(clients)).
				Msg("Client unregistered from session")
		}
	}
}

// broadcastMessage sends a message to all clients in a session
func (h *Hub) broadcastMessage(bm *BroadcastMessage) {
	h.mu.RLock()
	clients := h.sessions[bm.SessionID]
	h.mu.RUnlock()

	if clients == nil {
		h.log.Warn().
			Str("session_id", bm.SessionID).
			Msg("No clients found for session")
		return
	}

	for client := range clients {
		select {
		case client.send <- bm.Message:
			// Message sent successfully
		default:
			// Client's send channel is full, unregister the client
			h.log.Warn().
				Str("session_id", client.sessionID).
				Str("user_id", client.userID.String()).
				Msg("Client send buffer full, unregistering")
			go func(c *Client) {
				h.unregister <- c
			}(client)
		}
	}

	h.log.Debug().
		Str("session_id", bm.SessionID).
		Int("client_count", len(clients)).
		Msg("Message broadcast to session")
}

// Broadcast sends a message to all clients in a session
func (h *Hub) Broadcast(sessionID string, message model.WebSocketMessage) {
	h.broadcast <- &BroadcastMessage{
		SessionID: sessionID,
		Message:   message,
	}
}

// Register registers a client with the hub
func (h *Hub) Register(client *Client) {
	h.register <- client
}

// Unregister unregisters a client from the hub
func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

// GetSessionClientCount returns the number of connected clients for a session
func (h *Hub) GetSessionClientCount(sessionID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if clients, ok := h.sessions[sessionID]; ok {
		return len(clients)
	}
	return 0
}

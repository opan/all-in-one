package websocket

import (
	"context"
	"sync"

	"github.com/all-in-one/internal/chat/model"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Hub maintains the set of active clients and broadcasts messages to the clients
type Hub struct {
	// Registered clients per user (userID -> client)
	users map[string]*Client

	// Cache of session participants (sessionID -> []userID)
	sessionParticipants map[string][]string

	// Inbound messages from clients
	broadcast chan *BroadcastMessage

	// Register requests from clients
	register chan *Client

	// Unregister requests from clients
	unregister chan *Client

	// Mutex for thread-safe access to users and cache
	mu sync.RWMutex

	// Logger
	log zerolog.Logger

	msgReceived metric.Int64Counter
	msgSent     metric.Int64Counter
	meterReg    metric.Registration
}

// BroadcastMessage represents a message to be broadcast to session participants
type BroadcastMessage struct {
	SessionID    string
	Message      model.WebSocketMessage
	Participants []string // Optional: userIDs to send to (if known)
	Ctx          context.Context
}

// NewHub creates a new Hub
func NewHub(log zerolog.Logger) *Hub {
	h := &Hub{
		users:               make(map[string]*Client),
		sessionParticipants: make(map[string][]string),
		broadcast:           make(chan *BroadcastMessage, 256),
		register:            make(chan *Client),
		unregister:          make(chan *Client),
		log:                 log,
	}

	meter := otel.GetMeterProvider().Meter("chat")

	connActive, _ := meter.Int64ObservableGauge(
		"chat.websocket.connections.active",
		metric.WithDescription("Number of active WebSocket connections"),
	)
	h.msgReceived, _ = meter.Int64Counter(
		"chat.websocket.messages.received",
		metric.WithDescription("Number of WebSocket messages received from clients"),
	)
	h.msgSent, _ = meter.Int64Counter(
		"chat.websocket.messages.sent",
		metric.WithDescription("Number of WebSocket messages sent to clients"),
	)

	h.meterReg, _ = meter.RegisterCallback(func(_ context.Context, o metric.Observer) error {
		h.mu.RLock()
		defer h.mu.RUnlock()
		o.ObserveInt64(connActive, int64(len(h.users)))
		return nil
	}, connActive)

	return h
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

	userID := client.userID.String()

	if existingClient, exists := h.users[userID]; exists {
		h.log.Info().
			Str("user_id", userID).
			Msg("Replacing existing WebSocket connection for user")

		go func(oldClient *Client) {
			if oldClient.cancel != nil {
				oldClient.cancel()
			}
			close(oldClient.send)
		}(existingClient)
	}

	h.users[userID] = client

	h.log.Info().
		Str("user_id", userID).
		Int("total_connected_users", len(h.users)).
		Msg("Client registered")
}

// unregisterClient removes a client from the hub
func (h *Hub) unregisterClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	userID := client.userID.String()

	if currentClient, ok := h.users[userID]; ok && currentClient == client {
		delete(h.users, userID)
		close(client.send)

		h.log.Info().
			Str("user_id", userID).
			Int("remaining_connected_users", len(h.users)).
			Msg("Client unregistered")
	}
}

// broadcastMessage sends a message to all participants of a session
func (h *Hub) broadcastMessage(bm *BroadcastMessage) {
	h.mu.RLock()
	participants := bm.Participants
	if len(participants) == 0 {
		participants = h.sessionParticipants[bm.SessionID]
	}
	h.mu.RUnlock()

	if len(participants) == 0 {
		h.log.Warn().
			Str("session_id", bm.SessionID).
			Msg("No participants provided or cached for session")
		return
	}

	sentCount := 0
	h.mu.RLock()
	for _, userID := range participants {
		if client, ok := h.users[userID]; ok {
			select {
			case client.send <- outboundMessage{msg: bm.Message, ctx: bm.Ctx}:
				sentCount++
			default:
				h.log.Warn().
					Str("user_id", userID).
					Msg("Client send buffer full, unregistering")
				go func(c *Client) {
					h.unregister <- c
				}(client)
			}
		}
	}
	h.mu.RUnlock()

	h.log.Debug().
		Str("session_id", bm.SessionID).
		Int("participant_count", len(participants)).
		Int("sent_count", sentCount).
		Msg("Message broadcast to session participants")
}

// Broadcast sends a message to all participants in a session
func (h *Hub) Broadcast(ctx context.Context, sessionID string, message model.WebSocketMessage) {
	h.broadcast <- &BroadcastMessage{
		SessionID: sessionID,
		Message:   message,
		Ctx:       ctx,
	}
}

// BroadcastToUsers sends a message to specific users
func (h *Hub) BroadcastToUsers(ctx context.Context, sessionID string, message model.WebSocketMessage, participants []string) {
	h.broadcast <- &BroadcastMessage{
		SessionID:    sessionID,
		Message:      message,
		Participants: participants,
		Ctx:          ctx,
	}
}

// CacheSessionParticipants stores participant list for a session
func (h *Hub) CacheSessionParticipants(sessionID string, participants []string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.sessionParticipants[sessionID] = participants

	h.log.Debug().
		Str("session_id", sessionID).
		Int("participant_count", len(participants)).
		Msg("Cached session participants")
}

// InvalidateSessionCache clears cached participants for a session
func (h *Hub) InvalidateSessionCache(sessionID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.sessionParticipants, sessionID)

	h.log.Debug().
		Str("session_id", sessionID).
		Msg("Invalidated session participant cache")
}

// Register registers a client with the hub
func (h *Hub) Register(client *Client) {
	h.register <- client
}

// Unregister unregisters a client from the hub
func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

// GetConnectedUserCount returns the total number of connected users
func (h *Hub) GetConnectedUserCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.users)
}

// IsUserConnected checks if a user has an active WebSocket connection
func (h *Hub) IsUserConnected(userID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.users[userID]
	return ok
}

// RecordMessageReceived increments the received messages counter.
// msgType is the WebSocket message type (e.g. "message", "typing").
func (h *Hub) RecordMessageReceived(ctx context.Context, msgType string) {
	h.msgReceived.Add(ctx, 1, metric.WithAttributes(attribute.String("chat.message.type", msgType)))
}

// RecordMessageSent increments the sent messages counter.
func (h *Hub) RecordMessageSent(ctx context.Context) {
	h.msgSent.Add(ctx, 1)
}

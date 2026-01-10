package websocket

import (
	"encoding/json"
	"time"

	"github.com/all-in-one/internal/chat/model"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period (must be less than pongWait)
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer
	maxMessageSize = 8192 // 8KB
)

// Client represents a single WebSocket connection
type Client struct {
	// The hub that manages this client
	hub *Hub

	// The websocket connection
	conn *websocket.Conn

	// Buffered channel of outbound messages
	send chan model.WebSocketMessage

	// The session ID this client is connected to
	sessionID string

	// The user ID of the connected user
	userID uuid.UUID

	// Username of the connected user
	username string

	// Logger
	log zerolog.Logger
}

// NewClient creates a new WebSocket client
func NewClient(hub *Hub, conn *websocket.Conn, sessionID string, userID uuid.UUID, username string, log zerolog.Logger) *Client {
	return &Client{
		hub:       hub,
		conn:      conn,
		send:      make(chan model.WebSocketMessage, 256),
		sessionID: sessionID,
		userID:    userID,
		username:  username,
		log:       log,
	}
}

// readPump pumps messages from the websocket connection to the hub
func (c *Client) ReadPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.log.Error().Err(err).
					Str("session_id", c.sessionID).
					Str("user_id", c.userID.String()).
					Msg("WebSocket unexpected close")
			}
			break
		}

		// Parse the incoming message
		var wsMsg model.WebSocketMessage
		if err := json.Unmarshal(message, &wsMsg); err != nil {
			c.log.Error().Err(err).Str("raw_message", string(message)).Msg("Failed to parse WebSocket message")
			c.sendError("Invalid message format")
			continue
		}

		// Set timestamp if not provided
		if wsMsg.Timestamp.IsZero() {
			wsMsg.Timestamp = time.Now()
		}

		c.log.Debug().
			Str("type", wsMsg.Type).
			Str("session_id", c.sessionID).
			Str("user_id", c.userID.String()).
			Msg("Received WebSocket message")

		// Handle different message types
		// For now, we just broadcast all messages back to the session
		// The handler will process and store actual chat messages
		c.hub.Broadcast(c.sessionID, wsMsg)
	}
}

// writePump pumps messages from the hub to the websocket connection
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// Write the message as JSON
			if err := c.conn.WriteJSON(message); err != nil {
				c.log.Error().Err(err).Msg("Failed to write WebSocket message")
				return
			}

			c.log.Debug().
				Str("type", message.Type).
				Str("session_id", c.sessionID).
				Msg("Sent WebSocket message to client")

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// sendError sends an error message to the client
func (c *Client) sendError(errorMsg string) {
	errorPayload := model.ErrorPayload{
		Error: errorMsg,
	}

	wsMsg := model.WebSocketMessage{
		Type:      "error",
		Payload:   errorPayload,
		Timestamp: time.Now(),
	}

	select {
	case c.send <- wsMsg:
	default:
		c.log.Warn().Str("error", errorMsg).Msg("Failed to send error message to client")
	}
}

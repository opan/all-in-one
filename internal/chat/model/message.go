package model

import "time"

// Chat represents a chat message
type Chat struct {
	ID            string    `json:"id" db:"id"`
	ChatSessionID string    `json:"chat_session_id" db:"chat_session_id"`
	UserID        string    `json:"user_id" db:"user_id"`
	Message       string    `json:"message" db:"message"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`

	// Optional: Include username for frontend convenience
	Username string `json:"username,omitempty" db:"username"`
}

// CreateMessageRequest is the request payload for creating a message
type CreateMessageRequest struct {
	Message string `json:"message"`
}

// WebSocketMessage represents a WebSocket message envelope
type WebSocketMessage struct {
	Type      string      `json:"type"`      // message, join, leave, typing, error
	Payload   interface{} `json:"payload"`   // Message data
	Timestamp time.Time   `json:"timestamp"` // Message timestamp
}

// MessagePayload represents the payload of a message WebSocket event
type MessagePayload struct {
	ID            string    `json:"id"`
	ChatSessionID string    `json:"chat_session_id"`
	UserID        string    `json:"user_id"`
	Username      string    `json:"username"`
	Message       string    `json:"message"`
	CreatedAt     time.Time `json:"created_at"`
}

// TypingPayload represents the payload of a typing WebSocket event
type TypingPayload struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	IsTyping bool   `json:"is_typing"`
}

// ErrorPayload represents the payload of an error WebSocket event
type ErrorPayload struct {
	Error string `json:"error"`
}

package repository

import (
	"context"

	"github.com/all-in-one/internal/chat/model"
	"github.com/google/uuid"
)

// SessionRepository defines the interface for chat session storage operations
type SessionRepository interface {
	// GetAllByUserID returns all sessions where the user is a party
	GetAllByUserID(ctx context.Context, userID uuid.UUID) ([]model.ChatSession, error)

	// Get returns a session by ID
	Get(ctx context.Context, id string) (model.ChatSession, error)

	// Create creates a new chat session
	Create(ctx context.Context, session model.ChatSession) (model.ChatSession, error)

	// Update updates an existing chat session
	Update(ctx context.Context, id string, session model.ChatSession) (model.ChatSession, error)

	// Delete soft deletes a session by setting status to 'deleted'
	Delete(ctx context.Context, id string) error

	// AddParty adds a user to the session's parties list
	AddParty(ctx context.Context, sessionID string, userID uuid.UUID) error
}

// MessageRepository defines the interface for chat message storage operations
type MessageRepository interface {
	// GetBySessionID returns all messages for a session with optional limit
	GetBySessionID(ctx context.Context, sessionID string, limit int) ([]model.Chat, error)

	// Create creates a new message
	Create(ctx context.Context, chat model.Chat) (model.Chat, error)

	// Get returns a message by ID
	Get(ctx context.Context, id string) (model.Chat, error)
}

// Storage defines the main storage interface that aggregates all repositories
type Storage interface {
	// SessionRepo returns the session repository
	SessionRepo() SessionRepository

	// MessageRepo returns the message repository
	MessageRepo() MessageRepository

	// Close closes the storage connection
	Close() error
}

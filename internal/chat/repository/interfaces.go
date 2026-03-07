package repository

import (
	"context"

	"github.com/all-in-one/internal/chat/model"
	"github.com/google/uuid"
)

// SessionRepository defines the interface for chat session storage operations
type SessionRepository interface {
	// GetAllByUserID returns all sessions where the user is an active participant
	GetAllByUserID(ctx context.Context, userID uuid.UUID) ([]model.ChatSession, error)

	// GetAllByUserIDWithPagination returns sessions where the user is an active participant with pagination
	GetAllByUserIDWithPagination(ctx context.Context, userID uuid.UUID, limit int, offset int) ([]model.ChatSession, error)

	// Get returns a session by ID with participants
	Get(ctx context.Context, id string) (model.ChatSession, error)

	// GetByParticipantHash finds a session by participant hash
	GetByParticipantHash(ctx context.Context, hash string) (model.ChatSession, error)

	// GetParticipants returns all participants for a session
	GetParticipants(ctx context.Context, sessionID string) ([]model.SessionParticipant, error)

	// Create creates a new chat session with participants
	Create(ctx context.Context, session model.ChatSession) (model.ChatSession, error)

	// Update updates an existing chat session
	Update(ctx context.Context, id string, session model.ChatSession) (model.ChatSession, error)

	// Delete soft deletes a session by setting status to 'deleted'
	Delete(ctx context.Context, id string) error

	// AddParticipant adds or reactivates a participant in the session
	AddParticipant(ctx context.Context, sessionID string, userID uuid.UUID) error

	// RemoveParticipant marks a participant as having left the session
	// Auto-deletes session if less than 2 active participants remain
	RemoveParticipant(ctx context.Context, sessionID string, userID uuid.UUID) error

	// GetActiveParticipantCount returns the number of active participants
	GetActiveParticipantCount(ctx context.Context, sessionID string) (int, error)

	// AddParty is deprecated - use AddParticipant instead
	AddParty(ctx context.Context, sessionID string, userID uuid.UUID) error
}

// MessageRepository defines the interface for chat message storage operations
type MessageRepository interface {
	// GetBySessionID returns all messages for a session with optional limit
	GetBySessionID(ctx context.Context, sessionID string, limit int) ([]model.ChatMessage, error)

	// Create creates a new message
	Create(ctx context.Context, chat model.ChatMessage) (model.ChatMessage, error)

	// Get returns a message by ID
	Get(ctx context.Context, id string) (model.ChatMessage, error)
}

// UserSearchResult represents a user in search results
type UserSearchResult struct {
	ID       string `json:"id" db:"id"`
	Username string `json:"username" db:"username"`
	Email    string `json:"email" db:"email"`
	Name     string `json:"name" db:"name"`
}

// InviteRepository defines the interface for chat invite storage operations
type InviteRepository interface {
	// Create inserts a new invite record
	Create(ctx context.Context, invite model.ChatInvite) (model.ChatInvite, error)

	// GetByID returns an invite by its ID
	GetByID(ctx context.Context, id string) (model.ChatInvite, error)

	// GetPendingByInviteeID returns all pending invites received by a user
	GetPendingByInviteeID(ctx context.Context, inviteeID uuid.UUID) ([]model.ChatInvite, error)

	// GetSentByInviterID returns all invites sent by a user
	GetSentByInviterID(ctx context.Context, inviterID uuid.UUID) ([]model.ChatInvite, error)

	// GetByBatchID returns all invites sharing the same batch ID
	GetByBatchID(ctx context.Context, batchID string) ([]model.ChatInvite, error)

	// UpdateStatus updates the status of a single invite
	UpdateStatus(ctx context.Context, id string, status string) error

	// UpdateBatchSessionID sets the session_id on all invites in a batch.
	// Called when the first invitee accepts a new-session invite and a session is created.
	UpdateBatchSessionID(ctx context.Context, batchID string, sessionID string) error

	// HasPendingInvite checks whether inviter already has a pending new-session invite to invitee.
	// Scoped to session_id IS NULL to avoid conflating new-session invites with session-specific ones.
	HasPendingInvite(ctx context.Context, inviterID, inviteeID string) (bool, error)

	// HasPendingInviteForSession checks whether there is already a pending invite
	// for a specific existing session targeting the given invitee.
	HasPendingInviteForSession(ctx context.Context, sessionID, inviteeID string) (bool, error)
}

// Storage defines the main storage interface that aggregates all repositories
type Storage interface {
	// SessionRepo returns the session repository
	SessionRepo() SessionRepository

	// MessageRepo returns the message repository
	MessageRepo() MessageRepository

	// SearchUsers searches for users by username or name
	SearchUsers(ctx context.Context, query string, excludeUserID uuid.UUID, limit int) ([]UserSearchResult, error)

	// InviteRepo returns the invite repository
	InviteRepo() InviteRepository

	// Close closes the storage connection
	Close() error
}

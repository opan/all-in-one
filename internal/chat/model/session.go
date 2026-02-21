package model

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"time"
)

// ChatSession represents a chat session between multiple parties
type ChatSession struct {
	ID              string    `json:"id" db:"id"`
	ParticipantHash string    `json:"participant_hash" db:"participant_hash"`
	Status          string    `json:"status" db:"status"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time `json:"updated_at" db:"updated_at"`
	CreatedBy       string    `json:"created_by" db:"created_by"`

	// Not stored in DB, populated by queries
	Participants []SessionParticipant `json:"participants,omitempty" db:"-"`
	LastMessage  *ChatMessage         `json:"last_message,omitempty" db:"-"`
}

// SessionParticipant represents a participant in a chat session
type SessionParticipant struct {
	SessionID string     `json:"session_id" db:"session_id"`
	UserID    string     `json:"user_id" db:"user_id"`
	JoinedAt  time.Time  `json:"joined_at" db:"joined_at"`
	LeftAt    *time.Time `json:"left_at,omitempty" db:"left_at"`

	// Not stored in DB, populated by joins
	Username string `json:"username,omitempty" db:"username"`
}

// SessionStatus constants
const (
	SessionStatusActive   = "active"
	SessionStatusArchived = "archived"
	SessionStatusDeleted  = "deleted"
)

// IsActive returns true if the participant is currently active in the session
func (sp *SessionParticipant) IsActive() bool {
	return sp.LeftAt == nil
}

// GenerateParticipantHash creates a hash from sorted participant IDs
// This ensures uniqueness of sessions regardless of participant order
func GenerateParticipantHash(participantIDs []string) string {
	if len(participantIDs) == 0 {
		return ""
	}

	// Sort participant IDs to ensure consistent hash
	sorted := make([]string, len(participantIDs))
	copy(sorted, participantIDs)
	sort.Strings(sorted)

	// Create hash of sorted IDs
	h := sha256.New()
	h.Write([]byte(strings.Join(sorted, ",")))
	return hex.EncodeToString(h.Sum(nil))
}

// GetActiveParticipants returns only active participants (left_at is NULL)
func (cs *ChatSession) GetActiveParticipants() []SessionParticipant {
	active := make([]SessionParticipant, 0)
	for _, p := range cs.Participants {
		if p.IsActive() {
			active = append(active, p)
		}
	}
	return active
}

// GetActiveParticipantIDs returns IDs of active participants
func (cs *ChatSession) GetActiveParticipantIDs() []string {
	active := cs.GetActiveParticipants()
	ids := make([]string, len(active))
	for i, p := range active {
		ids[i] = p.UserID
	}
	return ids
}

// CreateSessionRequest is the request payload for creating a chat session
type CreateSessionRequest struct {
	Participants []string `json:"participants"` // Array of user IDs
}

// UpdateSessionRequest is the request payload for updating a chat session
type UpdateSessionRequest struct {
	AddParticipants []string `json:"add_participants,omitempty"` // Array of user IDs to add
	Status          string   `json:"status,omitempty"`
}

// AddParticipantRequest is the request payload for adding a participant
type AddParticipantRequest struct {
	UserID string `json:"user_id"`
}

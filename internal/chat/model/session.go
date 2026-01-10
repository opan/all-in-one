package model

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// ChatSession represents a chat session between multiple parties
type ChatSession struct {
	ID        string    `json:"id" db:"id"`
	Parties   string    `json:"parties" db:"parties"` // comma-separated user IDs
	Status    string    `json:"status" db:"status"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
	CreatedBy string    `json:"created_by" db:"created_by"`
}

// SessionStatus constants
const (
	SessionStatusActive   = "active"
	SessionStatusArchived = "archived"
	SessionStatusDeleted  = "deleted"
)

// GetPartyIDs returns slice of user IDs from the comma-separated parties string
func (cs *ChatSession) GetPartyIDs() []uuid.UUID {
	if cs.Parties == "" {
		return []uuid.UUID{}
	}

	parts := strings.Split(cs.Parties, ",")
	ids := make([]uuid.UUID, 0, len(parts))
	for _, p := range parts {
		if id, err := uuid.Parse(strings.TrimSpace(p)); err == nil {
			ids = append(ids, id)
		}
	}
	return ids
}

// SetPartyIDs sets parties from UUID slice
func (cs *ChatSession) SetPartyIDs(ids []uuid.UUID) {
	if len(ids) == 0 {
		cs.Parties = ""
		return
	}

	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = id.String()
	}
	cs.Parties = strings.Join(parts, ",")
}

// HasParty checks if a user ID is in the parties list
func (cs *ChatSession) HasParty(userID uuid.UUID) bool {
	partyIDs := cs.GetPartyIDs()
	for _, id := range partyIDs {
		if id == userID {
			return true
		}
	}
	return false
}

// AddParty adds a user ID to the parties list if not already present
func (cs *ChatSession) AddParty(userID uuid.UUID) {
	if cs.HasParty(userID) {
		return
	}

	partyIDs := cs.GetPartyIDs()
	partyIDs = append(partyIDs, userID)
	cs.SetPartyIDs(partyIDs)
}

// CreateSessionRequest is the request payload for creating a chat session
type CreateSessionRequest struct {
	Parties []string `json:"parties"` // Array of user IDs
}

// UpdateSessionRequest is the request payload for updating a chat session
type UpdateSessionRequest struct {
	Parties []string `json:"parties,omitempty"` // Array of user IDs to add
	Status  string   `json:"status,omitempty"`
}

package model

import "time"

// ChatInvite represents an invitation sent from one user to another to join a chat session.
// Invites are the only public-facing mechanism for creating or joining sessions.
type ChatInvite struct {
	ID        string    `json:"id" db:"id"`
	BatchID   string    `json:"batch_id" db:"batch_id"`
	InviterID string    `json:"inviter_id" db:"inviter_id"`
	InviteeID string    `json:"invitee_id" db:"invitee_id"`
	SessionID *string   `json:"session_id,omitempty" db:"session_id"`
	Status    string    `json:"status" db:"status"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`

	// Populated by joins, not stored in DB
	InviterUsername string `json:"inviter_username,omitempty" db:"inviter_username"`
	InviteeUsername string `json:"invitee_username,omitempty" db:"invitee_username"`
}

// Invite status constants
const (
	InviteStatusPending   = "pending"
	InviteStatusAccepted  = "accepted"
	InviteStatusDeclined  = "declined"
	InviteStatusCancelled = "cancelled"
)

// CreateInviteRequest is the payload for sending invite(s).
// If SessionID is set, invitees are invited into an existing session (must be an active participant to do this).
// If SessionID is nil, a new session will be created upon the first acceptance.
type CreateInviteRequest struct {
	Participants []string `json:"participants"`         // user IDs to invite
	SessionID    *string  `json:"session_id,omitempty"` // non-nil = invite into existing session
}

// RespondInviteRequest is the payload for accepting or declining an invite.
type RespondInviteRequest struct {
	Action string `json:"action"` // "accept" or "decline"
}

// RespondInviteResponse is returned from the respond endpoint.
// On accept, Session is populated so the frontend can navigate directly to the chat.
// On decline, Session is nil.
type RespondInviteResponse struct {
	Invite  ChatInvite   `json:"invite"`
	Session *ChatSession `json:"session,omitempty"`
}

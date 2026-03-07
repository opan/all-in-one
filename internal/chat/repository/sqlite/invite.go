package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/all-in-one/internal/chat/model"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"
)

type inviteRepository struct {
	db  *sqlx.DB
	log zerolog.Logger
}

// NewInviteRepository creates a new invite repository
func NewInviteRepository(db *sqlx.DB, log zerolog.Logger) *inviteRepository {
	return &inviteRepository{
		db:  db,
		log: log,
	}
}

// Create inserts a new invite record
func (r *inviteRepository) Create(ctx context.Context, invite model.ChatInvite) (model.ChatInvite, error) {
	query := `
		INSERT INTO chat_invites (id, batch_id, inviter_id, invitee_id, session_id, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	now := time.Now()
	invite.CreatedAt = now
	invite.UpdatedAt = now

	_, err := r.db.ExecContext(ctx, query,
		invite.ID,
		invite.BatchID,
		invite.InviterID,
		invite.InviteeID,
		invite.SessionID,
		invite.Status,
		invite.CreatedAt,
		invite.UpdatedAt,
	)
	if err != nil {
		r.log.Error().Err(err).Str("invite_id", invite.ID).Msg("Failed to create invite")
		return model.ChatInvite{}, fmt.Errorf("failed to create invite: %w", err)
	}

	return invite, nil
}

// GetByID returns an invite by its ID, with inviter/invitee usernames joined
func (r *inviteRepository) GetByID(ctx context.Context, id string) (model.ChatInvite, error) {
	query := `
		SELECT
			ci.id, ci.batch_id, ci.inviter_id, ci.invitee_id, ci.session_id,
			ci.status, ci.created_at, ci.updated_at,
			u_inviter.username AS inviter_username,
			u_invitee.username AS invitee_username
		FROM chat_invites ci
		LEFT JOIN users u_inviter ON u_inviter.id = ci.inviter_id
		LEFT JOIN users u_invitee ON u_invitee.id = ci.invitee_id
		WHERE ci.id = ?
	`
	var invite model.ChatInvite
	err := r.db.GetContext(ctx, &invite, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return model.ChatInvite{}, fmt.Errorf("invite not found: %s", id)
		}
		r.log.Error().Err(err).Str("invite_id", id).Msg("Failed to get invite")
		return model.ChatInvite{}, fmt.Errorf("failed to get invite: %w", err)
	}
	return invite, nil
}

// GetPendingByInviteeID returns all pending invites received by a user
func (r *inviteRepository) GetPendingByInviteeID(ctx context.Context, inviteeID uuid.UUID) ([]model.ChatInvite, error) {
	query := `
		SELECT
			ci.id, ci.batch_id, ci.inviter_id, ci.invitee_id, ci.session_id,
			ci.status, ci.created_at, ci.updated_at,
			u_inviter.username AS inviter_username,
			u_invitee.username AS invitee_username
		FROM chat_invites ci
		LEFT JOIN users u_inviter ON u_inviter.id = ci.inviter_id
		LEFT JOIN users u_invitee ON u_invitee.id = ci.invitee_id
		WHERE ci.invitee_id = ?
		  AND ci.status = 'pending'
		ORDER BY ci.created_at DESC
	`
	invites := make([]model.ChatInvite, 0)
	err := r.db.SelectContext(ctx, &invites, query, inviteeID.String())
	if err != nil {
		r.log.Error().Err(err).Str("invitee_id", inviteeID.String()).Msg("Failed to get pending invites")
		return nil, fmt.Errorf("failed to get pending invites: %w", err)
	}
	return invites, nil
}

// GetSentByInviterID returns all invites sent by a user
func (r *inviteRepository) GetSentByInviterID(ctx context.Context, inviterID uuid.UUID) ([]model.ChatInvite, error) {
	query := `
		SELECT
			ci.id, ci.batch_id, ci.inviter_id, ci.invitee_id, ci.session_id,
			ci.status, ci.created_at, ci.updated_at,
			u_inviter.username AS inviter_username,
			u_invitee.username AS invitee_username
		FROM chat_invites ci
		LEFT JOIN users u_inviter ON u_inviter.id = ci.inviter_id
		LEFT JOIN users u_invitee ON u_invitee.id = ci.invitee_id
		WHERE ci.inviter_id = ?
		ORDER BY ci.created_at DESC
	`
	invites := make([]model.ChatInvite, 0)
	err := r.db.SelectContext(ctx, &invites, query, inviterID.String())
	if err != nil {
		r.log.Error().Err(err).Str("inviter_id", inviterID.String()).Msg("Failed to get sent invites")
		return nil, fmt.Errorf("failed to get sent invites: %w", err)
	}
	return invites, nil
}

// GetByBatchID returns all invites sharing the same batch ID
func (r *inviteRepository) GetByBatchID(ctx context.Context, batchID string) ([]model.ChatInvite, error) {
	query := `
		SELECT
			ci.id, ci.batch_id, ci.inviter_id, ci.invitee_id, ci.session_id,
			ci.status, ci.created_at, ci.updated_at,
			u_inviter.username AS inviter_username,
			u_invitee.username AS invitee_username
		FROM chat_invites ci
		LEFT JOIN users u_inviter ON u_inviter.id = ci.inviter_id
		LEFT JOIN users u_invitee ON u_invitee.id = ci.invitee_id
		WHERE ci.batch_id = ?
		ORDER BY ci.created_at ASC
	`
	invites := make([]model.ChatInvite, 0)
	err := r.db.SelectContext(ctx, &invites, query, batchID)
	if err != nil {
		r.log.Error().Err(err).Str("batch_id", batchID).Msg("Failed to get invites by batch")
		return nil, fmt.Errorf("failed to get invites by batch: %w", err)
	}
	return invites, nil
}

// UpdateStatus updates the status of a single invite and refreshes updated_at
func (r *inviteRepository) UpdateStatus(ctx context.Context, id string, status string) error {
	query := `UPDATE chat_invites SET status = ?, updated_at = ? WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, status, time.Now(), id)
	if err != nil {
		r.log.Error().Err(err).Str("invite_id", id).Str("status", status).Msg("Failed to update invite status")
		return fmt.Errorf("failed to update invite status: %w", err)
	}
	return nil
}

// UpdateBatchSessionID sets session_id on all invites in a batch.
// Used when the first acceptance of a new-session invite triggers session creation.
func (r *inviteRepository) UpdateBatchSessionID(ctx context.Context, batchID string, sessionID string) error {
	query := `UPDATE chat_invites SET session_id = ?, updated_at = ? WHERE batch_id = ?`
	_, err := r.db.ExecContext(ctx, query, sessionID, time.Now(), batchID)
	if err != nil {
		r.log.Error().Err(err).Str("batch_id", batchID).Str("session_id", sessionID).Msg("Failed to update batch session ID")
		return fmt.Errorf("failed to update batch session ID: %w", err)
	}
	return nil
}

// HasPendingInvite checks for an existing pending new-session invite from inviter to invitee.
// Scoped to session_id IS NULL so it only matches new-session invites, not existing-session invites.
func (r *inviteRepository) HasPendingInvite(ctx context.Context, inviterID, inviteeID string) (bool, error) {
	query := `
		SELECT COUNT(*)
		FROM chat_invites
		WHERE inviter_id = ?
		  AND invitee_id = ?
		  AND status = 'pending'
		  AND session_id IS NULL
	`
	var count int
	err := r.db.GetContext(ctx, &count, query, inviterID, inviteeID)
	if err != nil {
		r.log.Error().Err(err).Msg("Failed to check pending invite")
		return false, fmt.Errorf("failed to check pending invite: %w", err)
	}
	return count > 0, nil
}

// HasPendingInviteForSession checks for an existing pending invite to a specific session for an invitee.
func (r *inviteRepository) HasPendingInviteForSession(ctx context.Context, sessionID, inviteeID string) (bool, error) {
	query := `
		SELECT COUNT(*)
		FROM chat_invites
		WHERE session_id = ?
		  AND invitee_id = ?
		  AND status = 'pending'
	`
	var count int
	err := r.db.GetContext(ctx, &count, query, sessionID, inviteeID)
	if err != nil {
		r.log.Error().Err(err).Msg("Failed to check pending session invite")
		return false, fmt.Errorf("failed to check pending session invite: %w", err)
	}
	return count > 0, nil
}

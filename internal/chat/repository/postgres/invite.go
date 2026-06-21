package postgres

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

func newInviteRepository(db *sqlx.DB, log zerolog.Logger) *inviteRepository {
	return &inviteRepository{db: db, log: log}
}

func (r *inviteRepository) Create(ctx context.Context, invite model.ChatInvite) (model.ChatInvite, error) {
	now := time.Now()
	invite.CreatedAt = now
	invite.UpdatedAt = now

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO chat_invites (id, batch_id, inviter_id, invitee_id, session_id, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, invite.ID, invite.BatchID, invite.InviterID, invite.InviteeID,
		invite.SessionID, invite.Status, invite.CreatedAt, invite.UpdatedAt)
	if err != nil {
		r.log.Error().Err(err).Str("invite_id", invite.ID).Msg("Failed to create invite")
		return model.ChatInvite{}, fmt.Errorf("failed to create invite: %w", err)
	}

	return invite, nil
}

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
		WHERE ci.id = $1
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
		WHERE ci.invitee_id = $1
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
		WHERE ci.inviter_id = $1
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
		WHERE ci.batch_id = $1
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

func (r *inviteRepository) UpdateStatus(ctx context.Context, id string, status string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE chat_invites SET status = $1, updated_at = $2 WHERE id = $3`, status, time.Now(), id)
	if err != nil {
		r.log.Error().Err(err).Str("invite_id", id).Str("status", status).Msg("Failed to update invite status")
		return fmt.Errorf("failed to update invite status: %w", err)
	}
	return nil
}

func (r *inviteRepository) UpdateBatchSessionID(ctx context.Context, batchID string, sessionID string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE chat_invites SET session_id = $1, updated_at = $2 WHERE batch_id = $3`, sessionID, time.Now(), batchID)
	if err != nil {
		r.log.Error().Err(err).Str("batch_id", batchID).Str("session_id", sessionID).Msg("Failed to update batch session ID")
		return fmt.Errorf("failed to update batch session ID: %w", err)
	}
	return nil
}

func (r *inviteRepository) HasPendingInvite(ctx context.Context, inviterID, inviteeID string) (bool, error) {
	var count int
	err := r.db.GetContext(ctx, &count, `
		SELECT COUNT(*)
		FROM chat_invites
		WHERE inviter_id = $1
		  AND invitee_id = $2
		  AND status = 'pending'
		  AND session_id IS NULL
	`, inviterID, inviteeID)
	if err != nil {
		r.log.Error().Err(err).Msg("Failed to check pending invite")
		return false, fmt.Errorf("failed to check pending invite: %w", err)
	}
	return count > 0, nil
}

func (r *inviteRepository) HasPendingInviteForSession(ctx context.Context, sessionID, inviteeID string) (bool, error) {
	var count int
	err := r.db.GetContext(ctx, &count, `
		SELECT COUNT(*)
		FROM chat_invites
		WHERE session_id = $1
		  AND invitee_id = $2
		  AND status = 'pending'
	`, sessionID, inviteeID)
	if err != nil {
		r.log.Error().Err(err).Msg("Failed to check pending session invite")
		return false, fmt.Errorf("failed to check pending session invite: %w", err)
	}
	return count > 0, nil
}

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

type sessionRepository struct {
	db  *sqlx.DB
	log zerolog.Logger
}

// NewSessionRepository creates a new session repository
func NewSessionRepository(db *sqlx.DB, log zerolog.Logger) *sessionRepository {
	return &sessionRepository{
		db:  db,
		log: log,
	}
}

// GetAllByUserID returns all sessions where the user is a party
func (r *sessionRepository) GetAllByUserID(ctx context.Context, userID uuid.UUID) ([]model.ChatSession, error) {
	query := `
		SELECT id, parties, status, created_at, updated_at, created_by
		FROM chat_sessions
		WHERE status != 'deleted'
		AND (parties LIKE '%' || ? || '%')
		ORDER BY updated_at DESC
	`

	var sessions []model.ChatSession
	err := r.db.SelectContext(ctx, &sessions, query, userID.String())
	if err != nil {
		r.log.Error().Err(err).Str("user_id", userID.String()).Msg("Failed to get sessions by user ID")
		return nil, fmt.Errorf("failed to get sessions: %w", err)
	}

	// Filter to ensure exact match (since LIKE can match partial UUIDs)
	filtered := make([]model.ChatSession, 0)
	for _, session := range sessions {
		if session.HasParty(userID) {
			filtered = append(filtered, session)
		}
	}

	return filtered, nil
}

// Get returns a session by ID
func (r *sessionRepository) Get(ctx context.Context, id string) (model.ChatSession, error) {
	query := `
		SELECT id, parties, status, created_at, updated_at, created_by
		FROM chat_sessions
		WHERE id = ?
	`

	var session model.ChatSession
	err := r.db.GetContext(ctx, &session, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return model.ChatSession{}, fmt.Errorf("session not found")
		}
		r.log.Error().Err(err).Str("session_id", id).Msg("Failed to get session")
		return model.ChatSession{}, fmt.Errorf("failed to get session: %w", err)
	}

	return session, nil
}

// Create creates a new chat session
func (r *sessionRepository) Create(ctx context.Context, session model.ChatSession) (model.ChatSession, error) {
	if session.ID == "" {
		session.ID = uuid.NewString()
	}

	now := time.Now()
	if session.CreatedAt.IsZero() {
		session.CreatedAt = now
	}
	if session.UpdatedAt.IsZero() {
		session.UpdatedAt = now
	}
	if session.Status == "" {
		session.Status = model.SessionStatusActive
	}

	query := `
		INSERT INTO chat_sessions (id, parties, status, created_at, updated_at, created_by)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(ctx, query,
		session.ID,
		session.Parties,
		session.Status,
		session.CreatedAt,
		session.UpdatedAt,
		session.CreatedBy,
	)
	if err != nil {
		r.log.Error().Err(err).Interface("session", session).Msg("Failed to create session")
		return model.ChatSession{}, fmt.Errorf("failed to create session: %w", err)
	}

	r.log.Info().Str("session_id", session.ID).Msg("Session created successfully")
	return session, nil
}

// Update updates an existing chat session
func (r *sessionRepository) Update(ctx context.Context, id string, session model.ChatSession) (model.ChatSession, error) {
	session.UpdatedAt = time.Now()

	query := `
		UPDATE chat_sessions
		SET parties = ?, status = ?, updated_at = ?
		WHERE id = ?
	`

	result, err := r.db.ExecContext(ctx, query,
		session.Parties,
		session.Status,
		session.UpdatedAt,
		id,
	)
	if err != nil {
		r.log.Error().Err(err).Str("session_id", id).Msg("Failed to update session")
		return model.ChatSession{}, fmt.Errorf("failed to update session: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return model.ChatSession{}, fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return model.ChatSession{}, fmt.Errorf("session not found")
	}

	// Fetch the updated session
	return r.Get(ctx, id)
}

// Delete soft deletes a session by setting status to 'deleted'
func (r *sessionRepository) Delete(ctx context.Context, id string) error {
	query := `
		UPDATE chat_sessions
		SET status = ?, updated_at = ?
		WHERE id = ?
	`

	result, err := r.db.ExecContext(ctx, query, model.SessionStatusDeleted, time.Now(), id)
	if err != nil {
		r.log.Error().Err(err).Str("session_id", id).Msg("Failed to delete session")
		return fmt.Errorf("failed to delete session: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("session not found")
	}

	r.log.Info().Str("session_id", id).Msg("Session deleted successfully")
	return nil
}

// AddParty adds a user to the session's parties list
func (r *sessionRepository) AddParty(ctx context.Context, sessionID string, userID uuid.UUID) error {
	// Get the current session
	session, err := r.Get(ctx, sessionID)
	if err != nil {
		return err
	}

	// Check if user is already a party
	if session.HasParty(userID) {
		return nil // Already a party, no-op
	}

	// Add the party
	session.AddParty(userID)

	// Update the session
	_, err = r.Update(ctx, sessionID, session)
	if err != nil {
		r.log.Error().Err(err).Str("session_id", sessionID).Str("user_id", userID.String()).Msg("Failed to add party")
		return fmt.Errorf("failed to add party: %w", err)
	}

	r.log.Info().Str("session_id", sessionID).Str("user_id", userID.String()).Msg("Party added successfully")
	return nil
}

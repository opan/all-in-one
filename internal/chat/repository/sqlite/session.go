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

// GetAllByUserID returns all sessions where the user is an active participant
func (r *sessionRepository) GetAllByUserID(ctx context.Context, userID uuid.UUID) ([]model.ChatSession, error) {
	query := `
		SELECT DISTINCT 
			cs.id, 
			cs.participant_hash, 
			cs.status, 
			cs.created_at, 
			cs.updated_at, 
			cs.created_by
		FROM chat_sessions cs
		INNER JOIN chat_session_participants csp ON cs.id = csp.session_id
		WHERE cs.status != 'deleted'
		  AND csp.user_id = ?
		  AND csp.left_at IS NULL
		ORDER BY cs.updated_at DESC
	`

	var sessions []model.ChatSession
	err := r.db.SelectContext(ctx, &sessions, query, userID.String())
	if err != nil {
		r.log.Error().Err(err).Str("user_id", userID.String()).Msg("Failed to get sessions by user ID")
		return nil, fmt.Errorf("failed to get sessions: %w", err)
	}

	// Load participants for each session
	for i := range sessions {
		participants, err := r.GetParticipants(ctx, sessions[i].ID)
		if err != nil {
			r.log.Error().Err(err).Str("session_id", sessions[i].ID).Msg("Failed to load participants")
			continue
		}
		sessions[i].Participants = participants

		// Load last message for each session
		lastMsg, err := r.getLastMessage(ctx, sessions[i].ID)
		if err == nil && lastMsg != nil {
			sessions[i].LastMessage = lastMsg
		}
	}

	return sessions, nil
}

// GetAllByUserIDWithPagination returns sessions where the user is an active participant with pagination
func (r *sessionRepository) GetAllByUserIDWithPagination(ctx context.Context, userID uuid.UUID, limit int, offset int) ([]model.ChatSession, error) {
	// Validate and set defaults
	if limit <= 0 {
		limit = 20 // default
	}
	if limit > 100 {
		limit = 100 // max
	}
	if offset < 0 {
		offset = 0
	}

	query := `
		SELECT DISTINCT 
			cs.id, 
			cs.participant_hash, 
			cs.status, 
			cs.created_at, 
			cs.updated_at, 
			cs.created_by
		FROM chat_sessions cs
		INNER JOIN chat_session_participants csp ON cs.id = csp.session_id
		WHERE cs.status != 'deleted'
		  AND csp.user_id = ?
		  AND csp.left_at IS NULL
		ORDER BY cs.updated_at DESC
		LIMIT ? OFFSET ?
	`

	var sessions []model.ChatSession
	err := r.db.SelectContext(ctx, &sessions, query, userID.String(), limit, offset)
	if err != nil {
		r.log.Error().Err(err).Str("user_id", userID.String()).Msg("Failed to get sessions by user ID with pagination")
		return nil, fmt.Errorf("failed to get sessions: %w", err)
	}

	// Load participants for each session
	for i := range sessions {
		participants, err := r.GetParticipants(ctx, sessions[i].ID)
		if err != nil {
			r.log.Error().Err(err).Str("session_id", sessions[i].ID).Msg("Failed to load participants")
			continue
		}
		sessions[i].Participants = participants

		// Load last message for each session
		lastMsg, err := r.getLastMessage(ctx, sessions[i].ID)
		if err == nil && lastMsg != nil {
			sessions[i].LastMessage = lastMsg
		}
	}

	return sessions, nil
}

// Get returns a session by ID
func (r *sessionRepository) Get(ctx context.Context, id string) (model.ChatSession, error) {
	query := `
		SELECT id, participant_hash, status, created_at, updated_at, created_by
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

	// Load participants
	participants, err := r.GetParticipants(ctx, id)
	if err != nil {
		r.log.Error().Err(err).Str("session_id", id).Msg("Failed to load participants")
	} else {
		session.Participants = participants
	}

	return session, nil
}

// GetByParticipantHash finds a session by participant hash
func (r *sessionRepository) GetByParticipantHash(ctx context.Context, hash string) (model.ChatSession, error) {
	query := `
		SELECT id, participant_hash, status, created_at, updated_at, created_by
		FROM chat_sessions
		WHERE participant_hash = ?
	`

	var session model.ChatSession
	err := r.db.GetContext(ctx, &session, query, hash)
	if err != nil {
		if err == sql.ErrNoRows {
			return model.ChatSession{}, fmt.Errorf("session not found")
		}
		r.log.Error().Err(err).Str("hash", hash).Msg("Failed to get session by hash")
		return model.ChatSession{}, fmt.Errorf("failed to get session: %w", err)
	}

	// Load participants
	participants, err := r.GetParticipants(ctx, session.ID)
	if err != nil {
		r.log.Error().Err(err).Str("session_id", session.ID).Msg("Failed to load participants")
	} else {
		session.Participants = participants
	}

	return session, nil
}

// GetParticipants returns all participants for a session
func (r *sessionRepository) GetParticipants(ctx context.Context, sessionID string) ([]model.SessionParticipant, error) {
	query := `
		SELECT 
			csp.session_id,
			csp.user_id,
			csp.joined_at,
			csp.left_at,
			u.username
		FROM chat_session_participants csp
		LEFT JOIN users u ON csp.user_id = u.id
		WHERE csp.session_id = ?
		ORDER BY csp.joined_at ASC
	`

	var participants []model.SessionParticipant
	err := r.db.SelectContext(ctx, &participants, query, sessionID)
	if err != nil {
		r.log.Error().Err(err).Str("session_id", sessionID).Msg("Failed to get participants")
		return nil, fmt.Errorf("failed to get participants: %w", err)
	}

	return participants, nil
}

// Create creates a new chat session with participants
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

	// Start transaction
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return model.ChatSession{}, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert session
	query := `
		INSERT INTO chat_sessions (id, participant_hash, status, created_at, updated_at, created_by)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	_, err = tx.ExecContext(ctx, query,
		session.ID,
		session.ParticipantHash,
		session.Status,
		session.CreatedAt,
		session.UpdatedAt,
		session.CreatedBy,
	)
	if err != nil {
		r.log.Error().Err(err).Interface("session", session).Msg("Failed to create session")
		return model.ChatSession{}, fmt.Errorf("failed to create session: %w", err)
	}

	// Insert participants
	if len(session.Participants) > 0 {
		participantQuery := `
			INSERT INTO chat_session_participants (session_id, user_id, joined_at)
			VALUES (?, ?, ?)
		`
		for _, p := range session.Participants {
			joinedAt := p.JoinedAt
			if joinedAt.IsZero() {
				joinedAt = now
			}
			_, err = tx.ExecContext(ctx, participantQuery, session.ID, p.UserID, joinedAt)
			if err != nil {
				r.log.Error().Err(err).Str("session_id", session.ID).Str("user_id", p.UserID).Msg("Failed to add participant")
				return model.ChatSession{}, fmt.Errorf("failed to add participant: %w", err)
			}
		}
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		return model.ChatSession{}, fmt.Errorf("failed to commit transaction: %w", err)
	}

	r.log.Info().Str("session_id", session.ID).Msg("Session created successfully")
	return r.Get(ctx, session.ID)
}

// Update updates an existing chat session
func (r *sessionRepository) Update(ctx context.Context, id string, session model.ChatSession) (model.ChatSession, error) {
	session.UpdatedAt = time.Now()

	query := `
		UPDATE chat_sessions
		SET status = ?, updated_at = ?
		WHERE id = ?
	`

	result, err := r.db.ExecContext(ctx, query,
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

// AddParticipant adds a user to the session (or reactivates if they left)
func (r *sessionRepository) AddParticipant(ctx context.Context, sessionID string, userID uuid.UUID) error {
	// Check if participant exists
	var existing model.SessionParticipant
	query := `
		SELECT session_id, user_id, joined_at, left_at
		FROM chat_session_participants
		WHERE session_id = ? AND user_id = ?
	`
	err := r.db.GetContext(ctx, &existing, query, sessionID, userID.String())

	if err == sql.ErrNoRows {
		// New participant - insert
		insertQuery := `
			INSERT INTO chat_session_participants (session_id, user_id, joined_at)
			VALUES (?, ?, ?)
		`
		_, err = r.db.ExecContext(ctx, insertQuery, sessionID, userID.String(), time.Now())
		if err != nil {
			r.log.Error().Err(err).Str("session_id", sessionID).Str("user_id", userID.String()).Msg("Failed to add participant")
			return fmt.Errorf("failed to add participant: %w", err)
		}
		r.log.Info().Str("session_id", sessionID).Str("user_id", userID.String()).Msg("Participant added")
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to check existing participant: %w", err)
	}

	// Participant exists - check if they left
	if existing.LeftAt != nil {
		// Rejoin - set left_at to NULL
		updateQuery := `
			UPDATE chat_session_participants
			SET left_at = NULL, joined_at = ?
			WHERE session_id = ? AND user_id = ?
		`
		_, err = r.db.ExecContext(ctx, updateQuery, time.Now(), sessionID, userID.String())
		if err != nil {
			r.log.Error().Err(err).Str("session_id", sessionID).Str("user_id", userID.String()).Msg("Failed to rejoin participant")
			return fmt.Errorf("failed to rejoin participant: %w", err)
		}
		r.log.Info().Str("session_id", sessionID).Str("user_id", userID.String()).Msg("Participant rejoined")
	}
	// else: participant is already active, no-op

	return nil
}

// RemoveParticipant marks a participant as having left the session
func (r *sessionRepository) RemoveParticipant(ctx context.Context, sessionID string, userID uuid.UUID) error {
	query := `
		UPDATE chat_session_participants
		SET left_at = ?
		WHERE session_id = ? AND user_id = ? AND left_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, time.Now(), sessionID, userID.String())
	if err != nil {
		r.log.Error().Err(err).Str("session_id", sessionID).Str("user_id", userID.String()).Msg("Failed to remove participant")
		return fmt.Errorf("failed to remove participant: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("participant not found or already left")
	}

	r.log.Info().Str("session_id", sessionID).Str("user_id", userID.String()).Msg("Participant removed")

	// Check if we should auto-delete the session (less than 2 active participants)
	activeCount, err := r.GetActiveParticipantCount(ctx, sessionID)
	if err != nil {
		r.log.Error().Err(err).Str("session_id", sessionID).Msg("Failed to get active participant count")
		return nil // Don't fail the remove operation
	}

	if activeCount < 2 {
		r.log.Info().Str("session_id", sessionID).Int("active_count", activeCount).Msg("Auto-deleting session with less than 2 participants")
		return r.Delete(ctx, sessionID)
	}

	return nil
}

// GetActiveParticipantCount returns the number of active participants in a session
func (r *sessionRepository) GetActiveParticipantCount(ctx context.Context, sessionID string) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM chat_session_participants
		WHERE session_id = ? AND left_at IS NULL
	`

	var count int
	err := r.db.GetContext(ctx, &count, query, sessionID)
	if err != nil {
		return 0, fmt.Errorf("failed to count active participants: %w", err)
	}

	return count, nil
}

// getLastMessage retrieves the most recent message for a session
func (r *sessionRepository) getLastMessage(ctx context.Context, sessionID string) (*model.ChatMessage, error) {
	query := `
		SELECT 
			cm.id,
			cm.chat_session_id,
			cm.user_id,
			cm.message,
			cm.created_at,
			cm.sent_at,
			u.username
		FROM chat_messages cm
		LEFT JOIN users u ON cm.user_id = u.id
		WHERE cm.chat_session_id = ?
		ORDER BY cm.created_at DESC
		LIMIT 1
	`

	var msg model.ChatMessage
	err := r.db.GetContext(ctx, &msg, query, sessionID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No messages yet, not an error
		}
		return nil, fmt.Errorf("failed to get last message: %w", err)
	}

	return &msg, nil
}

// AddParty is deprecated - use AddParticipant instead
func (r *sessionRepository) AddParty(ctx context.Context, sessionID string, userID uuid.UUID) error {
	return r.AddParticipant(ctx, sessionID, userID)
}

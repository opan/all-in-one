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

type messageRepository struct {
	db  *sqlx.DB
	log zerolog.Logger
}

// NewMessageRepository creates a new message repository
func NewMessageRepository(db *sqlx.DB, log zerolog.Logger) *messageRepository {
	return &messageRepository{
		db:  db,
		log: log,
	}
}

// GetBySessionID returns all messages for a session with optional limit
func (r *messageRepository) GetBySessionID(ctx context.Context, sessionID string, limit int) ([]model.Chat, error) {
	if limit <= 0 {
		limit = 100 // Default limit
	}

	query := `
		SELECT 
			c.id,
			c.chat_session_id,
			c.user_id,
			c.message,
			c.created_at,
			u.username
		FROM chats c
		LEFT JOIN users u ON c.user_id = u.id
		WHERE c.chat_session_id = ?
		ORDER BY c.created_at DESC
		LIMIT ?
	`

	var messages []model.Chat
	err := r.db.SelectContext(ctx, &messages, query, sessionID, limit)
	if err != nil {
		r.log.Error().Err(err).Str("session_id", sessionID).Msg("Failed to get messages")
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}

	// Reverse to get chronological order (oldest first)
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nil
}

// Create creates a new message
func (r *messageRepository) Create(ctx context.Context, chat model.Chat) (model.Chat, error) {
	if chat.ID == "" {
		chat.ID = uuid.NewString()
	}

	if chat.CreatedAt.IsZero() {
		chat.CreatedAt = time.Now()
	}

	query := `
		INSERT INTO chats (id, chat_session_id, user_id, message, created_at)
		VALUES (?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(ctx, query,
		chat.ID,
		chat.ChatSessionID,
		chat.UserID,
		chat.Message,
		chat.CreatedAt,
	)
	if err != nil {
		r.log.Error().Err(err).Interface("chat", chat).Msg("Failed to create message")
		return model.Chat{}, fmt.Errorf("failed to create message: %w", err)
	}

	// Fetch the created message with username
	return r.Get(ctx, chat.ID)
}

// Get returns a message by ID
func (r *messageRepository) Get(ctx context.Context, id string) (model.Chat, error) {
	query := `
		SELECT 
			c.id,
			c.chat_session_id,
			c.user_id,
			c.message,
			c.created_at,
			u.username
		FROM chats c
		LEFT JOIN users u ON c.user_id = u.id
		WHERE c.id = ?
	`

	var chat model.Chat
	err := r.db.GetContext(ctx, &chat, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return model.Chat{}, fmt.Errorf("message not found")
		}
		r.log.Error().Err(err).Str("message_id", id).Msg("Failed to get message")
		return model.Chat{}, fmt.Errorf("failed to get message: %w", err)
	}

	return chat, nil
}

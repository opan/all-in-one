package repository

import (
	"context"
	"fmt"

	"github.com/all-in-one/internal/chat/repository/sqlite"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"
)

// sqliteStorage implements the Storage interface for SQLite
type sqliteStorage struct {
	db          *sqlx.DB
	sessionRepo SessionRepository
	messageRepo MessageRepository
	inviteRepo  InviteRepository
	log         zerolog.Logger
}

// NewSQLiteStorage creates a new SQLite storage instance from a shared DB connection.
func NewSQLiteStorage(db *sqlx.DB, log zerolog.Logger) Storage {
	return &sqliteStorage{
		db:          db,
		sessionRepo: sqlite.NewSessionRepository(db, log),
		messageRepo: sqlite.NewMessageRepository(db, log),
		inviteRepo:  sqlite.NewInviteRepository(db, log),
		log:         log,
	}
}

// SessionRepo returns the session repository
func (s *sqliteStorage) SessionRepo() SessionRepository {
	return s.sessionRepo
}

// MessageRepo returns the message repository
func (s *sqliteStorage) MessageRepo() MessageRepository {
	return s.messageRepo
}

// InviteRepo returns the invite repository
func (s *sqliteStorage) InviteRepo() InviteRepository {
	return s.inviteRepo
}

// SearchUsers searches for users by username or name (excluding the current user)
func (s *sqliteStorage) SearchUsers(ctx context.Context, query string, excludeUserID uuid.UUID, limit int) ([]UserSearchResult, error) {
	s.log.Info().Str("query", query).Str("exclude_user_id", excludeUserID.String()).Msg("Searching users")

	var users []UserSearchResult

	searchPattern := "%" + query + "%"

	sqlQuery := `
		SELECT id, username, email, name
		FROM users
		WHERE id != ?
		  AND (username LIKE ? OR name LIKE ? OR email LIKE ?)
		ORDER BY username
		LIMIT ?
	`

	err := s.db.SelectContext(ctx, &users, sqlQuery, excludeUserID.String(), searchPattern, searchPattern, searchPattern, limit)
	if err != nil {
		s.log.Error().Err(err).Msg("Failed to search users")
		return nil, fmt.Errorf("failed to search users: %w", err)
	}

	s.log.Info().Int("count", len(users)).Msg("Users found")
	return users, nil
}

// Close is a no-op; the DB lifetime is managed by central storage.
func (s *sqliteStorage) Close() error { return nil }

package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/all-in-one/internal/chat/repository/sqlite"
	"github.com/all-in-one/internal/config"
	"github.com/all-in-one/internal/storage"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"
)

// sqliteStorage implements the Storage interface for SQLite
type sqliteStorage struct {
	db             *sqlx.DB
	sessionRepo    SessionRepository
	messageRepo    MessageRepository
	inviteRepo     InviteRepository
	log            zerolog.Logger
	closeOnCleanup bool
}

// NewSQLiteStorage creates a new SQLite storage instance
func NewSQLiteStorage(ctx context.Context, config config.Config, log zerolog.Logger) (Storage, error) {
	store, err := storage.NewStorage(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage: %w", err)
	}

	db := store.DB()

	return &sqliteStorage{
		db:             db,
		sessionRepo:    sqlite.NewSessionRepository(db, log),
		messageRepo:    sqlite.NewMessageRepository(db, log),
		inviteRepo:     sqlite.NewInviteRepository(db, log),
		log:            log,
		closeOnCleanup: false, // Don't close DB as it's managed by the main storage
	}, nil
}

// NewSQLiteStorageWithDB creates a new SQLite storage instance with an existing DB connection
func NewSQLiteStorageWithDB(db *sql.DB, log zerolog.Logger) Storage {
	sqlxDB := sqlx.NewDb(db, "sqlite3")

	return &sqliteStorage{
		db:             sqlxDB,
		sessionRepo:    sqlite.NewSessionRepository(sqlxDB, log),
		messageRepo:    sqlite.NewMessageRepository(sqlxDB, log),
		inviteRepo:     sqlite.NewInviteRepository(sqlxDB, log),
		log:            log,
		closeOnCleanup: false,
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

// Close closes the storage connection
func (s *sqliteStorage) Close() error {
	if s.closeOnCleanup && s.db != nil {
		return s.db.Close()
	}
	return nil
}

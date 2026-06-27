package repository

import (
	"context"
	"fmt"

	"github.com/all-in-one/internal/chat/repository/postgres"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"
)

type postgresStorage struct {
	db          *sqlx.DB
	sessionRepo SessionRepository
	messageRepo MessageRepository
	inviteRepo  InviteRepository
	inner       *postgres.Storage
	log         zerolog.Logger
}

func NewPostgresStorage(db *sqlx.DB, log zerolog.Logger) Storage {
	s := postgres.NewStorage(db, log)
	return &postgresStorage{
		db:          db,
		sessionRepo: s.SessionRepo,
		messageRepo: s.MessageRepo,
		inviteRepo:  s.InviteRepo,
		inner:       s,
		log:         log,
	}
}

func (s *postgresStorage) SessionRepo() SessionRepository { return s.sessionRepo }
func (s *postgresStorage) MessageRepo() MessageRepository { return s.messageRepo }
func (s *postgresStorage) InviteRepo() InviteRepository   { return s.inviteRepo }
func (s *postgresStorage) Close() error                   { return nil }

func (s *postgresStorage) SearchUsers(ctx context.Context, query string, excludeUserID uuid.UUID, limit int) ([]UserSearchResult, error) {
	s.log.Info().Str("query", query).Str("exclude_user_id", excludeUserID.String()).Msg("Searching users")

	var users []UserSearchResult
	searchPattern := "%" + query + "%"

	err := s.db.SelectContext(ctx, &users, `
		SELECT id, username, email, name
		FROM users
		WHERE id != $1
		  AND (username LIKE $2 OR name LIKE $3 OR email LIKE $4)
		ORDER BY username
		LIMIT $5
	`, excludeUserID.String(), searchPattern, searchPattern, searchPattern, limit)
	if err != nil {
		s.log.Error().Err(err).Msg("Failed to search users")
		return nil, fmt.Errorf("failed to search users: %w", err)
	}

	s.log.Info().Int("count", len(users)).Msg("Users found")
	return users, nil
}

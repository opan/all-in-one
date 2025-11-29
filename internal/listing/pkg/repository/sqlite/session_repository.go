package sqlite

import (
	"context"
	"fmt"
	"time"

	"github.com/all-in-one/internal/logging"

	"github.com/all-in-one/internal/listing/pkg/model"
	"github.com/all-in-one/internal/listing/query"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type sessionRepository struct {
	db *sqlx.DB
}

func newSessionRepository(db *sqlx.DB) *sessionRepository {
	return &sessionRepository{
		db: db,
	}
}

func (s *sessionRepository) Create(ctx context.Context, session model.Session, opts ...query.QueryOptions) error {
	log := logging.GetLoggerFromContext(ctx)
	log.Info().Str("store", "SessionRepository").Str("action", "Create").Msg("create session")

	exec := getExecCtx(s.db, opts...)

	_, err := exec.NamedExecContext(ctx,
		`INSERT INTO sessions (id, user_id, created_at, user_agent, access_token_expiry, refresh_token_expiry) 
		VALUES (:id, :user_id, :created_at, :user_agent, :access_token_expiry, :refresh_token_expiry)`,
		session)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	return nil
}

func (s *sessionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	log := logging.GetLoggerFromContext(ctx)
	log.Info().Str("store", "SessionRepository").Str("action", "Delete").Msg("delete session")

	_, err := s.db.ExecContext(ctx,
		`DELETE FROM sessions WHERE id = ?`, id.String())

	if err != nil {
		return fmt.Errorf("unable to delete session by id %s: %w", id, err)
	}

	return nil
}

func (s *sessionRepository) Get(ctx context.Context, id uuid.UUID) (*model.Session, error) {
	log := logging.GetLoggerFromContext(ctx)
	log.Info().Str("store", "SessionRepository").Str("action", "Get").Msg("get session")

	var session model.Session

	err := s.db.QueryRowContext(ctx,
		`SELECT id, user_id, created_at, user_agent 
		FROM sessions WHERE id = ?`, id).Scan(&session)

	if err != nil {
		return nil, fmt.Errorf("unable to get session by id %s: %w", id, err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	session.CreatedAt, _ = time.Parse(time.RFC3339, now)

	return &session, nil
}

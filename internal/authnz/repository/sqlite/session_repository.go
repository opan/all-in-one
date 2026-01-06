package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/all-in-one/internal/logging"

	"github.com/all-in-one/internal/authnz/model"
	"github.com/all-in-one/internal/query"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type queryOptions struct {
	trx *sqlx.Tx
	db  *sqlx.DB
}

func (q *queryOptions) Commit() error {
	if q.trx != nil {
		return q.trx.Commit()
	}
	return nil
}

func (q *queryOptions) Rollback() error {
	if q.trx != nil {
		return q.trx.Rollback()
	}
	return nil
}

type sessionRepository struct {
	db *sqlx.DB
}

func newSessionRepository(db *sqlx.DB) *sessionRepository {
	return &sessionRepository{
		db: db,
	}
}

type Execer interface {
	sqlx.ExtContext
	NamedExecContext(ctx context.Context, query string, arg interface{}) (sql.Result, error)
}

// getExecutor returns the appropriate executor (transaction or database) from options
func getExecCtx(db *sqlx.DB, opts ...query.QueryOptions) Execer {
	for _, opt := range opts {
		if qo, ok := opt.(*queryOptions); ok && qo.trx != nil {
			return qo.trx
		}
	}
	return db
}

func (r *sessionRepository) CreateTrx(ctx context.Context) (query.QueryOptions, error) {
	trx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to begin transaction: %w", err)
	}

	return &queryOptions{trx: trx, db: r.db}, nil
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

	err := s.db.GetContext(ctx, &session, "SELECT * FROM sessions WHERE id = ?", id)
	if err != nil {
		return nil, fmt.Errorf("unable to get session by id %s: %w", id, err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	session.CreatedAt, _ = time.Parse(time.RFC3339, now)

	return &session, nil
}

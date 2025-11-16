package sqlite

import (
	"context"
	"fmt"

	"github.com/all-in-one/internal/listing/pkg/model"
	"github.com/all-in-one/internal/listing/query"
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
	exec := getExecCtx(s.db, opts...)

	_, err := exec.NamedExecContext(ctx,
		`INSERT INTO sessions (id, user_id, created_at, user_agent) 
		VALUES (:id, :user_id, :created_at, :user_agent)`,
		session)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	return nil
}

func (s *sessionRepository) Delete(ctx context.Context, id int) error {
	return nil
}

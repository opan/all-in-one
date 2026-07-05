package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/all-in-one/internal/query"
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

type Execer interface {
	sqlx.ExtContext
	NamedExecContext(ctx context.Context, query string, arg interface{}) (sql.Result, error)
}

// getExecCtx returns the appropriate executor (transaction or database) from options
func getExecCtx(db *sqlx.DB, opts ...query.QueryOptions) Execer {
	for _, opt := range opts {
		if qo, ok := opt.(*queryOptions); ok && qo.trx != nil {
			return qo.trx
		}
	}
	return db
}

func createTrx(ctx context.Context, db *sqlx.DB) (query.QueryOptions, error) {
	trx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to begin transaction: %w", err)
	}
	return &queryOptions{trx: trx, db: db}, nil
}

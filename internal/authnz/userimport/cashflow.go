package userimport

import (
	"context"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// Cashflow is cashflow's database: the source of the accounts to import, and
// where each one is linked to its aio account (users.aio_user_id) afterwards.
// It lives on its own DSN, usually the same Postgres server as aio.
type Cashflow struct {
	db *sqlx.DB
}

func OpenCashflow(ctx context.Context, dsn string) (*Cashflow, error) {
	db, err := sqlx.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open cashflow database: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("connect to cashflow database: %w", err)
	}
	return &Cashflow{db: db}, nil
}

func (c *Cashflow) Close() error { return c.db.Close() }

// Accounts returns the cashflow accounts that still log in with a local
// password and aren't linked to aio yet, oldest first.
func (c *Cashflow) Accounts(ctx context.Context) ([]Record, error) {
	var recs []Record
	err := c.db.SelectContext(ctx, &recs, `SELECT id::text AS id, username, password_hash, created_at
		FROM users WHERE password_hash IS NOT NULL AND aio_user_id IS NULL ORDER BY created_at, username`)
	if err != nil && strings.Contains(err.Error(), "aio_user_id") {
		return nil, fmt.Errorf("cashflow's users table has no aio_user_id column: deploy the cashflow version " +
			"with all-in-one login first (it adds the column on start; AUTH_PROVIDER=local is fine)")
	}
	return recs, err
}

// Link sets users.aio_user_id for every imported account, in one
// transaction. Nothing changes if any account is gone or already linked to a
// different aio account.
func (c *Cashflow) Link(ctx context.Context, results []Result) (int, error) {
	var ids, aioIDs []string
	for _, r := range results {
		if r.Outcome != Skipped {
			ids = append(ids, r.Record.ID)
			aioIDs = append(aioIDs, r.AioID.String())
		}
	}
	if len(ids) == 0 {
		return 0, nil
	}
	tx, err := c.db.BeginTxx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var missing, conflicting int
	if err := tx.GetContext(ctx, &missing, `SELECT count(*) FROM unnest($1::uuid[]) AS l(id)
		LEFT JOIN users u ON u.id = l.id WHERE u.id IS NULL`, pq.Array(ids)); err != nil {
		return 0, err
	}
	if missing > 0 {
		return 0, fmt.Errorf("%d accounts disappeared from cashflow since they were read; nothing was linked", missing)
	}
	if err := tx.GetContext(ctx, &conflicting, `SELECT count(*) FROM unnest($1::uuid[], $2::text[]) AS l(id, aio)
		JOIN users u ON u.id = l.id WHERE u.aio_user_id IS NOT NULL AND u.aio_user_id <> l.aio`,
		pq.Array(ids), pq.Array(aioIDs)); err != nil {
		return 0, err
	}
	if conflicting > 0 {
		return 0, fmt.Errorf("%d accounts are already linked to a different aio account; nothing was linked", conflicting)
	}
	res, err := tx.ExecContext(ctx, `UPDATE users u SET aio_user_id = l.aio
		FROM unnest($1::uuid[], $2::text[]) AS l(id, aio) WHERE u.id = l.id`, pq.Array(ids), pq.Array(aioIDs))
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), tx.Commit()
}

// Unlinked counts accounts that still have only a local password; after a
// complete import it is zero.
func (c *Cashflow) Unlinked(ctx context.Context) (int, error) {
	var n int
	err := c.db.GetContext(ctx, &n, `SELECT count(*) FROM users WHERE password_hash IS NOT NULL AND aio_user_id IS NULL`)
	return n, err
}

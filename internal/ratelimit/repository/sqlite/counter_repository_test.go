package sqlite

import (
	"context"
	"sync"
	"testing"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newCounterTestDB(t *testing.T) *sqlx.DB {
	t.Helper()
	db, err := sqlx.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	// A ":memory:" DSN is per-connection — without this, concurrent writers
	// on the connection pool would each see their own empty database.
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })

	_, err = db.Exec(`
		CREATE TABLE rate_limit_counters (
			target_key TEXT NOT NULL,
			bucket_key TEXT NOT NULL,
			day        TEXT NOT NULL,
			count      INTEGER NOT NULL DEFAULT 0,
			updated_at TIMESTAMP NOT NULL,
			PRIMARY KEY (target_key, bucket_key, day)
		);
	`)
	require.NoError(t, err)
	return db
}

func TestCounterRepository_IncrAndGet(t *testing.T) {
	db := newCounterTestDB(t)
	repo := newCounterRepository(db)
	ctx := context.Background()

	n, err := repo.IncrAndGet(ctx, "auth.signup.ip", "ip:1.2.3.4", "2026-07-09")
	require.NoError(t, err)
	assert.Equal(t, 1, n)

	n, err = repo.IncrAndGet(ctx, "auth.signup.ip", "ip:1.2.3.4", "2026-07-09")
	require.NoError(t, err)
	assert.Equal(t, 2, n)

	// a different bucket starts its own count at 1
	n, err = repo.IncrAndGet(ctx, "auth.signup.ip", "ip:5.6.7.8", "2026-07-09")
	require.NoError(t, err)
	assert.Equal(t, 1, n)
}

func TestCounterRepository_IncrAndGet_Concurrent(t *testing.T) {
	db := newCounterTestDB(t)
	repo := newCounterRepository(db)
	ctx := context.Background()

	const n = 50
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := repo.IncrAndGet(ctx, "listing.item.create", "user:abc", "2026-07-09"); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("IncrAndGet error: %v", err)
	}

	got, err := repo.IncrAndGet(ctx, "listing.item.create", "user:abc", "2026-07-09")
	require.NoError(t, err)
	assert.Equal(t, n+1, got)
}

func TestCounterRepository_DeleteForTargetDay(t *testing.T) {
	db := newCounterTestDB(t)
	repo := newCounterRepository(db)
	ctx := context.Background()

	_, err := repo.IncrAndGet(ctx, "auth.login", "ip:1.2.3.4", "2026-07-09")
	require.NoError(t, err)
	_, err = repo.IncrAndGet(ctx, "auth.login", "ip:1.2.3.4", "2026-07-10")
	require.NoError(t, err)

	require.NoError(t, repo.DeleteForTargetDay(ctx, "auth.login", "2026-07-09"))

	var count int
	require.NoError(t, db.Get(&count, "SELECT COUNT(*) FROM rate_limit_counters WHERE target_key = ? AND day = ?", "auth.login", "2026-07-09"))
	assert.Equal(t, 0, count)

	require.NoError(t, db.Get(&count, "SELECT COUNT(*) FROM rate_limit_counters WHERE target_key = ? AND day = ?", "auth.login", "2026-07-10"))
	assert.Equal(t, 1, count)
}

func TestCounterRepository_DeleteOlderThan(t *testing.T) {
	db := newCounterTestDB(t)
	repo := newCounterRepository(db)
	ctx := context.Background()

	_, err := repo.IncrAndGet(ctx, "auth.login", "ip:1.2.3.4", "2026-07-01")
	require.NoError(t, err)
	_, err = repo.IncrAndGet(ctx, "auth.login", "ip:1.2.3.4", "2026-07-05")
	require.NoError(t, err)
	_, err = repo.IncrAndGet(ctx, "auth.login", "ip:1.2.3.4", "2026-07-09")
	require.NoError(t, err)

	n, err := repo.DeleteOlderThan(ctx, "2026-07-05")
	require.NoError(t, err)
	assert.Equal(t, int64(1), n) // only 07-01 is strictly before 07-05

	var count int
	require.NoError(t, db.Get(&count, "SELECT COUNT(*) FROM rate_limit_counters"))
	assert.Equal(t, 2, count)
}

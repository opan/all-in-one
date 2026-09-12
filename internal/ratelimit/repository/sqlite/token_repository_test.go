package sqlite

import (
	"context"
	"testing"
	"time"

	httpHelper "github.com/all-in-one/internal/http"
	"github.com/all-in-one/internal/ratelimit/model"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTokenTestDB(t *testing.T) *sqlx.DB {
	t.Helper()
	db, err := sqlx.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	_, err = db.Exec(`
		CREATE TABLE app_tokens (
			id           TEXT PRIMARY KEY,
			app          TEXT NOT NULL,
			name         TEXT NOT NULL,
			token_hash   TEXT NOT NULL,
			token_prefix TEXT NOT NULL,
			scope_prefix TEXT NOT NULL,
			created_at   TIMESTAMP NOT NULL,
			created_by   TEXT,
			last_used_at TIMESTAMP,
			revoked_at   TIMESTAMP
		);
		CREATE UNIQUE INDEX idx_app_tokens_hash ON app_tokens(token_hash);
	`)
	require.NoError(t, err)
	return db
}

func sampleToken(id, hash string) model.AppToken {
	return model.AppToken{
		ID:          id,
		App:         "cashflow",
		Name:        "cashflow prod",
		TokenHash:   hash,
		TokenPrefix: "aio_cashflo",
		ScopePrefix: "cashflow.",
		CreatedAt:   time.Now().UTC(),
	}
}

func TestTokenRepository_CreateAndGetByHash(t *testing.T) {
	db := newTokenTestDB(t)
	repo := newTokenRepository(db)
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, sampleToken("t1", "hash-abc")))

	got, err := repo.GetByHash(ctx, "hash-abc")
	require.NoError(t, err)
	assert.Equal(t, "t1", got.ID)
	assert.Equal(t, "cashflow", got.App)
	assert.Equal(t, "cashflow.", got.ScopePrefix)
	assert.Nil(t, got.RevokedAt)
}

func TestTokenRepository_GetByHash_NotFound(t *testing.T) {
	db := newTokenTestDB(t)
	repo := newTokenRepository(db)

	_, err := repo.GetByHash(context.Background(), "nope")
	assert.ErrorIs(t, err, httpHelper.ErrNotFound)
}

func TestTokenRepository_GetByHash_ExcludesRevoked(t *testing.T) {
	db := newTokenTestDB(t)
	repo := newTokenRepository(db)
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, sampleToken("t1", "hash-abc")))
	require.NoError(t, repo.Revoke(ctx, "t1"))

	_, err := repo.GetByHash(ctx, "hash-abc")
	assert.ErrorIs(t, err, httpHelper.ErrNotFound, "revoked token must not be returned by GetByHash")

	// but it remains in the audit list
	all, err := repo.List(ctx)
	require.NoError(t, err)
	require.Len(t, all, 1)
	require.NotNil(t, all[0].RevokedAt)
}

func TestTokenRepository_List(t *testing.T) {
	db := newTokenTestDB(t)
	repo := newTokenRepository(db)
	ctx := context.Background()

	first := sampleToken("t1", "hash-1")
	first.CreatedAt = time.Now().UTC().Add(-time.Hour)
	second := sampleToken("t2", "hash-2")
	require.NoError(t, repo.Create(ctx, first))
	require.NoError(t, repo.Create(ctx, second))

	got, err := repo.List(ctx)
	require.NoError(t, err)
	require.Len(t, got, 2)
	// ordered by created_at DESC — newest first
	assert.Equal(t, "t2", got[0].ID)
	assert.Equal(t, "t1", got[1].ID)
}

func TestTokenRepository_TouchLastUsed(t *testing.T) {
	db := newTokenTestDB(t)
	repo := newTokenRepository(db)
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, sampleToken("t1", "hash-abc")))
	require.Nil(t, mustGet(t, repo, "hash-abc").LastUsedAt)

	at := time.Now().UTC().Truncate(time.Second)
	require.NoError(t, repo.TouchLastUsed(ctx, "t1", at))

	got := mustGet(t, repo, "hash-abc")
	require.NotNil(t, got.LastUsedAt)
	assert.WithinDuration(t, at, *got.LastUsedAt, time.Second)
}

func mustGet(t *testing.T, repo *tokenRepository, hash string) model.AppToken {
	t.Helper()
	got, err := repo.GetByHash(context.Background(), hash)
	require.NoError(t, err)
	return got
}

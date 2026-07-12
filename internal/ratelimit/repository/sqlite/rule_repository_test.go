package sqlite

import (
	"context"
	"testing"

	httpHelper "github.com/all-in-one/internal/http"
	"github.com/all-in-one/internal/ratelimit/model"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestDB(t *testing.T) *sqlx.DB {
	t.Helper()
	db, err := sqlx.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	_, err = db.Exec(`
		CREATE TABLE rate_limit_rules (
			target_key   TEXT PRIMARY KEY,
			enabled      INTEGER NOT NULL DEFAULT 1,
			limit_count  INTEGER NOT NULL,
			window_value INTEGER NOT NULL,
			window_unit  TEXT    NOT NULL,
			updated_at   TIMESTAMP NOT NULL,
			updated_by   TEXT
		);
	`)
	require.NoError(t, err)
	return db
}

func strPtr(s string) *string { return &s }

func TestRuleRepository_SeedAndGet(t *testing.T) {
	db := newTestDB(t)
	repo := newRuleRepository(db)
	ctx := context.Background()

	rule := model.Rule{
		TargetKey: "auth.login", Enabled: true,
		LimitCount: 10, WindowValue: 1, WindowUnit: model.WindowMinute,
	}
	require.NoError(t, repo.Seed(ctx, rule))

	got, err := repo.Get(ctx, "auth.login")
	require.NoError(t, err)
	assert.Equal(t, "auth.login", got.TargetKey)
	assert.True(t, got.Enabled)
	assert.Equal(t, 10, got.LimitCount)
	assert.Equal(t, 1, got.WindowValue)
	assert.Equal(t, model.WindowMinute, got.WindowUnit)
}

func TestRuleRepository_Get_NotFound(t *testing.T) {
	db := newTestDB(t)
	repo := newRuleRepository(db)

	_, err := repo.Get(context.Background(), "does.not.exist")
	assert.ErrorIs(t, err, httpHelper.ErrNotFound)
}

func TestRuleRepository_Seed_DoesNotClobberExistingEdit(t *testing.T) {
	db := newTestDB(t)
	repo := newRuleRepository(db)
	ctx := context.Background()

	require.NoError(t, repo.Seed(ctx, model.Rule{
		TargetKey: "auth.login", Enabled: true,
		LimitCount: 10, WindowValue: 1, WindowUnit: model.WindowMinute,
	}))

	// simulate an admin edit
	require.NoError(t, repo.Update(ctx, model.Rule{
		TargetKey: "auth.login", Enabled: false,
		LimitCount: 999, WindowValue: 5, WindowUnit: model.WindowHour,
		UpdatedBy: strPtr("admin"),
	}))

	// re-seeding (e.g. on next boot) must not clobber the edit
	require.NoError(t, repo.Seed(ctx, model.Rule{
		TargetKey: "auth.login", Enabled: true,
		LimitCount: 10, WindowValue: 1, WindowUnit: model.WindowMinute,
	}))

	got, err := repo.Get(ctx, "auth.login")
	require.NoError(t, err)
	assert.False(t, got.Enabled)
	assert.Equal(t, 999, got.LimitCount)
	assert.Equal(t, 5, got.WindowValue)
	assert.Equal(t, model.WindowHour, got.WindowUnit)
	require.NotNil(t, got.UpdatedBy)
	assert.Equal(t, "admin", *got.UpdatedBy)
}

func TestRuleRepository_List(t *testing.T) {
	db := newTestDB(t)
	repo := newRuleRepository(db)
	ctx := context.Background()

	require.NoError(t, repo.Seed(ctx, model.Rule{
		TargetKey: "b.target", Enabled: true, LimitCount: 1, WindowValue: 1, WindowUnit: model.WindowDay,
	}))
	require.NoError(t, repo.Seed(ctx, model.Rule{
		TargetKey: "a.target", Enabled: true, LimitCount: 1, WindowValue: 1, WindowUnit: model.WindowDay,
	}))

	got, err := repo.List(ctx)
	require.NoError(t, err)
	require.Len(t, got, 2)
	// ordered by target_key
	assert.Equal(t, "a.target", got[0].TargetKey)
	assert.Equal(t, "b.target", got[1].TargetKey)
}

func TestRuleRepository_Update(t *testing.T) {
	db := newTestDB(t)
	repo := newRuleRepository(db)
	ctx := context.Background()

	require.NoError(t, repo.Seed(ctx, model.Rule{
		TargetKey: "auth.login", Enabled: true, LimitCount: 10, WindowValue: 1, WindowUnit: model.WindowMinute,
	}))

	require.NoError(t, repo.Update(ctx, model.Rule{
		TargetKey: "auth.login", Enabled: false, LimitCount: 20, WindowValue: 2, WindowUnit: model.WindowHour,
		UpdatedBy: strPtr("admin"),
	}))

	got, err := repo.Get(ctx, "auth.login")
	require.NoError(t, err)
	assert.False(t, got.Enabled)
	assert.Equal(t, 20, got.LimitCount)
	assert.Equal(t, 2, got.WindowValue)
	assert.Equal(t, model.WindowHour, got.WindowUnit)
	require.NotNil(t, got.UpdatedBy)
	assert.Equal(t, "admin", *got.UpdatedBy)
}

func TestRuleRepository_ResetToDefault(t *testing.T) {
	db := newTestDB(t)
	repo := newRuleRepository(db)
	ctx := context.Background()

	require.NoError(t, repo.Seed(ctx, model.Rule{
		TargetKey: "auth.login", Enabled: true, LimitCount: 10, WindowValue: 1, WindowUnit: model.WindowMinute,
	}))
	require.NoError(t, repo.Update(ctx, model.Rule{
		TargetKey: "auth.login", Enabled: false, LimitCount: 999, WindowValue: 5, WindowUnit: model.WindowHour,
		UpdatedBy: strPtr("admin"),
	}))

	// resetting clears the admin edit (including UpdatedBy) back to the
	// registry defaults passed in by the caller
	require.NoError(t, repo.ResetToDefault(ctx, model.Rule{
		TargetKey: "auth.login", Enabled: true, LimitCount: 10, WindowValue: 1, WindowUnit: model.WindowMinute,
		UpdatedBy: strPtr("admin"), // should be cleared regardless of what's passed in
	}))

	got, err := repo.Get(ctx, "auth.login")
	require.NoError(t, err)
	assert.True(t, got.Enabled)
	assert.Equal(t, 10, got.LimitCount)
	assert.Equal(t, 1, got.WindowValue)
	assert.Equal(t, model.WindowMinute, got.WindowUnit)
	assert.Nil(t, got.UpdatedBy)
}

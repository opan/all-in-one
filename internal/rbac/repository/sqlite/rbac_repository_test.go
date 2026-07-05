package sqlite

import (
	"context"
	"testing"

	"github.com/all-in-one/internal/rbac/model"
	"github.com/google/uuid"
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

	_, err = db.Exec(`PRAGMA foreign_keys = ON`)
	require.NoError(t, err)

	_, err = db.Exec(`
		CREATE TABLE features (
			id TEXT PRIMARY KEY,
			key TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			description TEXT,
			admin_only INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL
		);

		CREATE TABLE groups (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			description TEXT,
			is_builtin INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL
		);

		CREATE TABLE users (
			id TEXT PRIMARY KEY,
			username TEXT NOT NULL UNIQUE,
			email TEXT NOT NULL UNIQUE,
			group_id TEXT REFERENCES groups(id) ON DELETE SET NULL
		);

		CREATE TABLE group_features (
			group_id TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
			feature_id TEXT NOT NULL REFERENCES features(id) ON DELETE CASCADE,
			created_at TIMESTAMP NOT NULL,
			PRIMARY KEY (group_id, feature_id)
		);

		CREATE TABLE user_feature_overrides (
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			feature_id TEXT NOT NULL REFERENCES features(id) ON DELETE CASCADE,
			allow INTEGER NOT NULL,
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL,
			PRIMARY KEY (user_id, feature_id)
		);
	`)
	require.NoError(t, err)
	return db
}

func insertUser(t *testing.T, db *sqlx.DB, id uuid.UUID, username string) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO users (id, username, email) VALUES (?, ?, ?)`, id.String(), username, username+"@example.com")
	require.NoError(t, err)
}

// ---- FeatureRepository ----

func TestFeatureRepository_UpsertAndGet(t *testing.T) {
	db := newTestDB(t)
	repo := newFeatureRepository(db)
	ctx := context.Background()

	f := model.Feature{ID: uuid.New(), Key: "listing", Name: "Listings", AdminOnly: false}
	require.NoError(t, repo.Upsert(ctx, f))

	got, err := repo.GetByKey(ctx, "listing")
	require.NoError(t, err)
	assert.Equal(t, "listing", got.Key)
	assert.Equal(t, "Listings", got.Name)
	assert.False(t, got.AdminOnly)

	all, err := repo.List(ctx)
	require.NoError(t, err)
	assert.Len(t, all, 1)
}

func TestFeatureRepository_UpsertDoesNotOverwriteExisting(t *testing.T) {
	db := newTestDB(t)
	repo := newFeatureRepository(db)
	ctx := context.Background()

	require.NoError(t, repo.Upsert(ctx, model.Feature{ID: uuid.New(), Key: "listing", Name: "Listings", AdminOnly: false}))
	// Second upsert with a different name/admin_only must be a no-op (ADR-007).
	require.NoError(t, repo.Upsert(ctx, model.Feature{ID: uuid.New(), Key: "listing", Name: "Renamed", AdminOnly: true}))

	got, err := repo.GetByKey(ctx, "listing")
	require.NoError(t, err)
	assert.Equal(t, "Listings", got.Name)
	assert.False(t, got.AdminOnly)
}

func TestFeatureRepository_GetByKey_NotFound(t *testing.T) {
	db := newTestDB(t)
	repo := newFeatureRepository(db)

	_, err := repo.GetByKey(context.Background(), "nonexistent")
	assert.Error(t, err)
}

// ---- GroupRepository ----

func TestGroupRepository_CreateUpdateDelete(t *testing.T) {
	db := newTestDB(t)
	repo := newGroupRepository(db)
	ctx := context.Background()

	g := model.Group{ID: uuid.New(), Name: "listing-group", Description: "listing only"}
	require.NoError(t, repo.Create(ctx, g))

	got, err := repo.Get(ctx, g.ID)
	require.NoError(t, err)
	assert.Equal(t, "listing-group", got.Name)

	got.Name = "listing-group-renamed"
	require.NoError(t, repo.Update(ctx, got))

	updated, err := repo.GetByName(ctx, "listing-group-renamed")
	require.NoError(t, err)
	assert.Equal(t, g.ID, updated.ID)

	require.NoError(t, repo.Delete(ctx, g.ID))
	_, err = repo.Get(ctx, g.ID)
	assert.Error(t, err)
}

func TestGroupRepository_EnsureBuiltin_Idempotent(t *testing.T) {
	db := newTestDB(t)
	repo := newGroupRepository(db)
	ctx := context.Background()

	first, err := repo.EnsureBuiltin(ctx, "admin", "Full access")
	require.NoError(t, err)
	assert.True(t, first.IsBuiltin)

	second, err := repo.EnsureBuiltin(ctx, "admin", "Full access")
	require.NoError(t, err)
	assert.Equal(t, first.ID, second.ID)

	all, err := repo.List(ctx)
	require.NoError(t, err)
	assert.Len(t, all, 1)
}

// ---- GroupFeatureRepository ----

func TestGroupFeatureRepository_GrantIsIdempotent(t *testing.T) {
	db := newTestDB(t)
	groupRepo := newGroupRepository(db)
	featureRepo := newFeatureRepository(db)
	repo := newGroupFeatureRepository(db)
	ctx := context.Background()

	group := model.Group{ID: uuid.New(), Name: "regular-user"}
	require.NoError(t, groupRepo.Create(ctx, group))
	feature := model.Feature{ID: uuid.New(), Key: "listing", Name: "Listings"}
	require.NoError(t, featureRepo.Upsert(ctx, feature))

	require.NoError(t, repo.Grant(ctx, group.ID, feature.ID))
	require.NoError(t, repo.Grant(ctx, group.ID, feature.ID)) // must not error or duplicate

	keys, err := repo.ListKeysByGroup(ctx, group.ID)
	require.NoError(t, err)
	assert.Equal(t, []string{"listing"}, keys)

	granted, err := repo.HasGrantByKey(ctx, group.ID, "listing")
	require.NoError(t, err)
	assert.True(t, granted)

	granted, err = repo.HasGrantByKey(ctx, group.ID, "chat")
	require.NoError(t, err)
	assert.False(t, granted)
}

func TestGroupFeatureRepository_ReplaceGrants(t *testing.T) {
	db := newTestDB(t)
	groupRepo := newGroupRepository(db)
	featureRepo := newFeatureRepository(db)
	repo := newGroupFeatureRepository(db)
	ctx := context.Background()

	group := model.Group{ID: uuid.New(), Name: "custom-group"}
	require.NoError(t, groupRepo.Create(ctx, group))

	listing := model.Feature{ID: uuid.New(), Key: "listing", Name: "Listings"}
	chat := model.Feature{ID: uuid.New(), Key: "chat", Name: "Chats"}
	require.NoError(t, featureRepo.Upsert(ctx, listing))
	require.NoError(t, featureRepo.Upsert(ctx, chat))

	require.NoError(t, repo.ReplaceGrants(ctx, group.ID, []uuid.UUID{listing.ID, chat.ID}))
	keys, err := repo.ListKeysByGroup(ctx, group.ID)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"listing", "chat"}, keys)

	// Replacing with a smaller set must remove the previous grants.
	require.NoError(t, repo.ReplaceGrants(ctx, group.ID, []uuid.UUID{listing.ID}))
	keys, err = repo.ListKeysByGroup(ctx, group.ID)
	require.NoError(t, err)
	assert.Equal(t, []string{"listing"}, keys)
}

func TestGroupFeatureRepository_CascadesOnGroupDelete(t *testing.T) {
	db := newTestDB(t)
	groupRepo := newGroupRepository(db)
	featureRepo := newFeatureRepository(db)
	repo := newGroupFeatureRepository(db)
	ctx := context.Background()

	group := model.Group{ID: uuid.New(), Name: "custom-group"}
	require.NoError(t, groupRepo.Create(ctx, group))
	feature := model.Feature{ID: uuid.New(), Key: "listing", Name: "Listings"}
	require.NoError(t, featureRepo.Upsert(ctx, feature))
	require.NoError(t, repo.Grant(ctx, group.ID, feature.ID))

	require.NoError(t, groupRepo.Delete(ctx, group.ID))

	keys, err := repo.ListKeysByGroup(ctx, group.ID)
	require.NoError(t, err)
	assert.Empty(t, keys)
}

// ---- OverrideRepository ----

func TestOverrideRepository_GetByKey_NilWhenAbsent(t *testing.T) {
	db := newTestDB(t)
	featureRepo := newFeatureRepository(db)
	repo := newOverrideRepository(db)
	ctx := context.Background()

	userID := uuid.New()
	insertUser(t, db, userID, "user1")
	feature := model.Feature{ID: uuid.New(), Key: "listing", Name: "Listings"}
	require.NoError(t, featureRepo.Upsert(ctx, feature))

	got, err := repo.GetByKey(ctx, userID, "listing")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestOverrideRepository_ReplaceForUser(t *testing.T) {
	db := newTestDB(t)
	featureRepo := newFeatureRepository(db)
	repo := newOverrideRepository(db)
	ctx := context.Background()

	userID := uuid.New()
	insertUser(t, db, userID, "user1")
	listing := model.Feature{ID: uuid.New(), Key: "listing", Name: "Listings"}
	chat := model.Feature{ID: uuid.New(), Key: "chat", Name: "Chats"}
	require.NoError(t, featureRepo.Upsert(ctx, listing))
	require.NoError(t, featureRepo.Upsert(ctx, chat))

	require.NoError(t, repo.ReplaceForUser(ctx, userID, []model.UserFeatureOverride{
		{FeatureID: listing.ID, Allow: true},
		{FeatureID: chat.ID, Allow: false},
	}))

	listingOverride, err := repo.GetByKey(ctx, userID, "listing")
	require.NoError(t, err)
	require.NotNil(t, listingOverride)
	assert.True(t, listingOverride.Allow)

	chatOverride, err := repo.GetByKey(ctx, userID, "chat")
	require.NoError(t, err)
	require.NotNil(t, chatOverride)
	assert.False(t, chatOverride.Allow)

	all, err := repo.ListByUser(ctx, userID)
	require.NoError(t, err)
	assert.Len(t, all, 2)

	// Replacing again must fully replace, not accumulate.
	require.NoError(t, repo.ReplaceForUser(ctx, userID, []model.UserFeatureOverride{
		{FeatureID: listing.ID, Allow: false},
	}))
	all, err = repo.ListByUser(ctx, userID)
	require.NoError(t, err)
	assert.Len(t, all, 1)
	assert.False(t, all[0].Allow)
}

func TestOverrideRepository_CascadesOnUserDelete(t *testing.T) {
	db := newTestDB(t)
	featureRepo := newFeatureRepository(db)
	repo := newOverrideRepository(db)
	ctx := context.Background()

	userID := uuid.New()
	insertUser(t, db, userID, "user1")
	feature := model.Feature{ID: uuid.New(), Key: "listing", Name: "Listings"}
	require.NoError(t, featureRepo.Upsert(ctx, feature))
	require.NoError(t, repo.ReplaceForUser(ctx, userID, []model.UserFeatureOverride{{FeatureID: feature.ID, Allow: true}}))

	_, err := db.Exec(`DELETE FROM users WHERE id = ?`, userID.String())
	require.NoError(t, err)

	all, err := repo.ListByUser(ctx, userID)
	require.NoError(t, err)
	assert.Empty(t, all)
}

// ---- UserGroupRepository ----

func TestUserGroupRepository_AssignAndCount(t *testing.T) {
	db := newTestDB(t)
	groupRepo := newGroupRepository(db)
	repo := newUserGroupRepository(db)
	ctx := context.Background()

	group := model.Group{ID: uuid.New(), Name: "admin"}
	require.NoError(t, groupRepo.Create(ctx, group))

	user1, user2 := uuid.New(), uuid.New()
	insertUser(t, db, user1, "user1")
	insertUser(t, db, user2, "user2")

	// Unassigned initially.
	groupID, err := repo.GetGroupID(ctx, user1)
	require.NoError(t, err)
	assert.Nil(t, groupID)

	require.NoError(t, repo.AssignGroup(ctx, user1, &group.ID))
	require.NoError(t, repo.AssignGroup(ctx, user2, &group.ID))

	got, err := repo.GetGroupID(ctx, user1)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, group.ID, *got)

	count, err := repo.CountByGroup(ctx, group.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	// Unassign.
	require.NoError(t, repo.AssignGroup(ctx, user1, nil))
	groupID, err = repo.GetGroupID(ctx, user1)
	require.NoError(t, err)
	assert.Nil(t, groupID)

	count, err = repo.CountByGroup(ctx, group.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestUserGroupRepository_ListUsersWithGroup(t *testing.T) {
	db := newTestDB(t)
	groupRepo := newGroupRepository(db)
	repo := newUserGroupRepository(db)
	ctx := context.Background()

	group := model.Group{ID: uuid.New(), Name: "regular-user"}
	require.NoError(t, groupRepo.Create(ctx, group))

	assigned, unassigned := uuid.New(), uuid.New()
	insertUser(t, db, assigned, "assigned-user")
	insertUser(t, db, unassigned, "unassigned-user")
	require.NoError(t, repo.AssignGroup(ctx, assigned, &group.ID))

	rows, err := repo.ListUsersWithGroup(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 2)

	byUsername := map[string]model.UserAccessRow{}
	for _, row := range rows {
		byUsername[row.Username] = row
	}

	require.NotNil(t, byUsername["assigned-user"].GroupName)
	assert.Equal(t, "regular-user", *byUsername["assigned-user"].GroupName)
	assert.Nil(t, byUsername["unassigned-user"].GroupName)
}

func TestUserGroupRepository_GroupDeleteSetsNullOnUsers(t *testing.T) {
	db := newTestDB(t)
	groupRepo := newGroupRepository(db)
	repo := newUserGroupRepository(db)
	ctx := context.Background()

	group := model.Group{ID: uuid.New(), Name: "custom-group"}
	require.NoError(t, groupRepo.Create(ctx, group))

	userID := uuid.New()
	insertUser(t, db, userID, "user1")
	require.NoError(t, repo.AssignGroup(ctx, userID, &group.ID))

	require.NoError(t, groupRepo.Delete(ctx, group.ID))

	groupID, err := repo.GetGroupID(ctx, userID)
	require.NoError(t, err)
	assert.Nil(t, groupID, "user's group_id should revert to NULL (regular-user default) after their group is deleted")
}

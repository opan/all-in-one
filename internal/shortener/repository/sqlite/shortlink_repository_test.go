package sqlite

import (
	"context"
	"testing"
	"time"

	httpHelper "github.com/all-in-one/internal/http"
	"github.com/all-in-one/internal/shortener/model"
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
		CREATE TABLE short_links (
			id               TEXT PRIMARY KEY,
			code             TEXT NOT NULL UNIQUE,
			target_url       TEXT NOT NULL,
			created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			expires_at       DATETIME,
			is_active        BOOLEAN NOT NULL DEFAULT 1,
			click_count      INTEGER NOT NULL DEFAULT 0,
			last_accessed_at DATETIME
		);
		CREATE TABLE short_link_owners (
			code    TEXT NOT NULL PRIMARY KEY REFERENCES short_links(code) ON DELETE CASCADE,
			user_id TEXT NOT NULL
		);
		CREATE INDEX idx_short_link_owners_user_id ON short_link_owners(user_id);
		CREATE TABLE users (
			id       TEXT PRIMARY KEY,
			username TEXT NOT NULL UNIQUE
		);
	`)
	require.NoError(t, err)
	return db
}

func seedUser(t *testing.T, db *sqlx.DB, id, username string) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO users (id, username) VALUES (?, ?)`, id, username)
	require.NoError(t, err)
}

func seedLink(t *testing.T, db *sqlx.DB, link model.ShortLink, ownerID string) {
	t.Helper()
	_, err := db.Exec(`
		INSERT INTO short_links (id, code, target_url, created_at, expires_at, is_active, click_count)
		VALUES (?, ?, ?, ?, ?, ?, 0)
	`, link.ID, link.Code, link.TargetURL, link.CreatedAt, link.ExpiresAt, link.IsActive)
	require.NoError(t, err)

	if ownerID != "" {
		_, err = db.Exec(`INSERT INTO short_link_owners (code, user_id) VALUES (?, ?)`, link.Code, ownerID)
		require.NoError(t, err)
	}
}

func baseLink(code string) model.ShortLink {
	return model.ShortLink{
		ID:        "01JT" + code,
		Code:      code,
		TargetURL: "https://example.com/" + code,
		CreatedAt: time.Now().UTC().Truncate(time.Second),
		IsActive:  true,
	}
}

// ---- Create ----

func TestCreate_Success(t *testing.T) {
	repo := newShortLinkRepository(newTestDB(t))
	link := baseLink("abc1234")
	got, err := repo.Create(context.Background(), link, "user-1")
	require.NoError(t, err)
	assert.Equal(t, link.Code, got.Code)
	assert.Equal(t, link.TargetURL, got.TargetURL)
	assert.True(t, got.IsActive)
	assert.Equal(t, uint64(0), got.ClickCount)
}

func TestCreate_AnonymousLink(t *testing.T) {
	db := newTestDB(t)
	repo := newShortLinkRepository(db)
	link := baseLink("anon123")
	got, err := repo.Create(context.Background(), link, "")
	require.NoError(t, err)
	assert.Equal(t, link.Code, got.Code)

	var cnt int
	require.NoError(t, db.Get(&cnt, `SELECT COUNT(*) FROM short_link_owners WHERE code = ?`, link.Code))
	assert.Equal(t, 0, cnt)
}

func TestCreate_DuplicateCode(t *testing.T) {
	repo := newShortLinkRepository(newTestDB(t))
	link := baseLink("dupcode")
	_, err := repo.Create(context.Background(), link, "user-1")
	require.NoError(t, err)

	link2 := link
	link2.ID = "01JT-other-id"
	_, err = repo.Create(context.Background(), link2, "user-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "UNIQUE constraint failed")
}

// ---- GetByCode ----

func TestGetByCode_Found(t *testing.T) {
	db := newTestDB(t)
	repo := newShortLinkRepository(db)
	link := baseLink("findme7")
	seedLink(t, db, link, "user-1")

	got, err := repo.GetByCode(context.Background(), "findme7")
	require.NoError(t, err)
	assert.Equal(t, "findme7", got.Code)
}

func TestGetByCode_NotFound(t *testing.T) {
	repo := newShortLinkRepository(newTestDB(t))
	_, err := repo.GetByCode(context.Background(), "missing")
	assert.ErrorIs(t, err, httpHelper.ErrNotFound)
}

// ---- GetByCodeOwned ----

func TestGetByCodeOwned_Found(t *testing.T) {
	db := newTestDB(t)
	repo := newShortLinkRepository(db)
	link := baseLink("own7777")
	seedLink(t, db, link, "user-x")

	got, err := repo.GetByCodeOwned(context.Background(), "own7777", "user-x")
	require.NoError(t, err)
	assert.Equal(t, "own7777", got.Code)
}

func TestGetByCodeOwned_WrongOwner(t *testing.T) {
	db := newTestDB(t)
	repo := newShortLinkRepository(db)
	link := baseLink("own8888")
	seedLink(t, db, link, "user-x")

	_, err := repo.GetByCodeOwned(context.Background(), "own8888", "user-y")
	assert.ErrorIs(t, err, httpHelper.ErrNotFound)
}

func TestGetByCodeOwned_AnonymousLink(t *testing.T) {
	db := newTestDB(t)
	repo := newShortLinkRepository(db)
	link := baseLink("anon999")
	seedLink(t, db, link, "")

	_, err := repo.GetByCodeOwned(context.Background(), "anon999", "user-x")
	assert.ErrorIs(t, err, httpHelper.ErrNotFound)
}

// ---- ListByOwner ----

func TestListByOwner_Pagination(t *testing.T) {
	db := newTestDB(t)
	repo := newShortLinkRepository(db)
	ownerID := "owner-list"

	codes := []string{"aaa0001", "bbb0002", "ccc0003"}
	for i, code := range codes {
		l := baseLink(code)
		l.CreatedAt = time.Now().Add(-time.Duration(i) * time.Second).UTC()
		seedLink(t, db, l, ownerID)
	}
	// Different owner — must not appear in results
	other := baseLink("zzz9999")
	seedLink(t, db, other, "other-owner")

	links, total, err := repo.ListByOwner(context.Background(), ownerID, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, uint32(3), total)
	assert.Len(t, links, 3)
}

func TestListByOwner_PageSize(t *testing.T) {
	db := newTestDB(t)
	repo := newShortLinkRepository(db)
	ownerID := "owner-page"

	for i := range 5 {
		code := [7]byte{'p', 'g', '0', '0', '0', '0', byte('1' + i)}
		l := baseLink(string(code[:]))
		l.CreatedAt = time.Now().Add(-time.Duration(i) * time.Second).UTC()
		seedLink(t, db, l, ownerID)
	}

	links, total, err := repo.ListByOwner(context.Background(), ownerID, 1, 2)
	require.NoError(t, err)
	assert.Equal(t, uint32(5), total)
	assert.Len(t, links, 2)

	page2, _, err := repo.ListByOwner(context.Background(), ownerID, 2, 2)
	require.NoError(t, err)
	assert.Len(t, page2, 2)
}

func TestListByOwner_Empty(t *testing.T) {
	repo := newShortLinkRepository(newTestDB(t))
	links, total, err := repo.ListByOwner(context.Background(), "nobody", 1, 10)
	require.NoError(t, err)
	assert.Equal(t, uint32(0), total)
	assert.Empty(t, links)
}

func TestListByOwner_ExcludesAnonymous(t *testing.T) {
	db := newTestDB(t)
	repo := newShortLinkRepository(db)

	seedLink(t, db, baseLink("owned11"), "user-1")
	seedLink(t, db, baseLink("anon111"), "")

	links, total, err := repo.ListByOwner(context.Background(), "user-1", 1, 10)
	require.NoError(t, err)
	assert.Equal(t, uint32(1), total)
	assert.Len(t, links, 1)
	assert.Equal(t, "owned11", links[0].Code)
}

// ---- Update ----

func TestUpdate_ToggleActive(t *testing.T) {
	db := newTestDB(t)
	repo := newShortLinkRepository(db)
	link := baseLink("upd1234")
	seedLink(t, db, link, "user-upd")

	link.IsActive = false
	updated, err := repo.Update(context.Background(), link, "user-upd")
	require.NoError(t, err)
	assert.False(t, updated.IsActive)
}

func TestUpdate_SetExpiry(t *testing.T) {
	db := newTestDB(t)
	repo := newShortLinkRepository(db)
	link := baseLink("exp1234")
	seedLink(t, db, link, "user-upd")

	exp := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Second)
	link.ExpiresAt = &exp
	updated, err := repo.Update(context.Background(), link, "user-upd")
	require.NoError(t, err)
	require.NotNil(t, updated.ExpiresAt)
	assert.WithinDuration(t, exp, *updated.ExpiresAt, time.Second)
}

func TestUpdate_WrongOwner(t *testing.T) {
	db := newTestDB(t)
	repo := newShortLinkRepository(db)
	link := baseLink("own1234")
	seedLink(t, db, link, "real-owner")

	_, err := repo.Update(context.Background(), link, "wrong-owner")
	assert.ErrorIs(t, err, httpHelper.ErrNotFound)
}

// ---- Delete ----

func TestDelete_Success(t *testing.T) {
	db := newTestDB(t)
	repo := newShortLinkRepository(db)
	link := baseLink("del1234")
	seedLink(t, db, link, "user-del")

	err := repo.Delete(context.Background(), "del1234", "user-del")
	require.NoError(t, err)

	_, err = repo.GetByCode(context.Background(), "del1234")
	assert.ErrorIs(t, err, httpHelper.ErrNotFound)
}

func TestDelete_WrongOwner(t *testing.T) {
	db := newTestDB(t)
	repo := newShortLinkRepository(db)
	link := baseLink("del5678")
	seedLink(t, db, link, "real-owner")

	err := repo.Delete(context.Background(), "del5678", "wrong-owner")
	assert.ErrorIs(t, err, httpHelper.ErrNotFound)
	// Must still exist for real owner
	_, err = repo.GetByCode(context.Background(), "del5678")
	require.NoError(t, err)
}

func TestDelete_NotFound(t *testing.T) {
	repo := newShortLinkRepository(newTestDB(t))
	err := repo.Delete(context.Background(), "ghost12", "user-x")
	assert.ErrorIs(t, err, httpHelper.ErrNotFound)
}

// ---- IncrementClick ----

func TestIncrementClick_ActiveLink(t *testing.T) {
	db := newTestDB(t)
	repo := newShortLinkRepository(db)
	link := baseLink("clk1234")
	seedLink(t, db, link, "user-clk")

	now := time.Now().UTC().Format(time.RFC3339)
	err := repo.IncrementClick(context.Background(), "clk1234", now)
	require.NoError(t, err)

	got, err := repo.GetByCode(context.Background(), "clk1234")
	require.NoError(t, err)
	assert.Equal(t, uint64(1), got.ClickCount)
	assert.NotNil(t, got.LastAccessedAt)
}

func TestIncrementClick_InactiveLink(t *testing.T) {
	db := newTestDB(t)
	repo := newShortLinkRepository(db)
	link := baseLink("dis1234")
	link.IsActive = false
	seedLink(t, db, link, "user-dis")

	now := time.Now().UTC().Format(time.RFC3339)
	err := repo.IncrementClick(context.Background(), "dis1234", now)
	assert.ErrorIs(t, err, httpHelper.ErrNotFound)
}

func TestIncrementClick_ExpiredLink(t *testing.T) {
	db := newTestDB(t)
	repo := newShortLinkRepository(db)
	link := baseLink("exp5678")
	past := time.Now().Add(-time.Hour).UTC()
	link.ExpiresAt = &past
	seedLink(t, db, link, "user-exp")

	now := time.Now().UTC().Format(time.RFC3339)
	err := repo.IncrementClick(context.Background(), "exp5678", now)
	assert.ErrorIs(t, err, httpHelper.ErrNotFound)
}

func TestIncrementClick_MultipleClicks(t *testing.T) {
	db := newTestDB(t)
	repo := newShortLinkRepository(db)
	link := baseLink("mul1234")
	seedLink(t, db, link, "user-mul")

	now := time.Now().UTC().Format(time.RFC3339)
	for range 5 {
		require.NoError(t, repo.IncrementClick(context.Background(), "mul1234", now))
	}

	got, err := repo.GetByCode(context.Background(), "mul1234")
	require.NoError(t, err)
	assert.Equal(t, uint64(5), got.ClickCount)
}

// ---- ListAll ----

func TestListAll_AcrossOwners(t *testing.T) {
	db := newTestDB(t)
	repo := newShortLinkRepository(db)

	seedUser(t, db, "user-1", "alice")
	seedUser(t, db, "user-2", "bob")
	seedLink(t, db, baseLink("own0001"), "user-1")
	seedLink(t, db, baseLink("own0002"), "user-2")
	seedLink(t, db, baseLink("anon001"), "")

	links, total, err := repo.ListAll(context.Background(), 1, 10)
	require.NoError(t, err)
	assert.Equal(t, uint32(3), total)
	assert.Len(t, links, 3)

	byCode := map[string]model.ShortLinkWithOwner{}
	for _, l := range links {
		byCode[l.Code] = l
	}
	assert.Equal(t, "user-1", byCode["own0001"].OwnerID)
	assert.Equal(t, "alice", byCode["own0001"].OwnerUsername)
	assert.Equal(t, "user-2", byCode["own0002"].OwnerID)
	assert.Equal(t, "bob", byCode["own0002"].OwnerUsername)
	assert.Empty(t, byCode["anon001"].OwnerID)
	assert.Empty(t, byCode["anon001"].OwnerUsername)
}

func TestListAll_Pagination(t *testing.T) {
	db := newTestDB(t)
	repo := newShortLinkRepository(db)

	for i := range 5 {
		code := [7]byte{'p', 'g', 'a', 'l', 'l', '0', byte('1' + i)}
		l := baseLink(string(code[:]))
		l.CreatedAt = time.Now().Add(-time.Duration(i) * time.Second).UTC()
		seedLink(t, db, l, "")
	}

	links, total, err := repo.ListAll(context.Background(), 1, 2)
	require.NoError(t, err)
	assert.Equal(t, uint32(5), total)
	assert.Len(t, links, 2)

	page2, _, err := repo.ListAll(context.Background(), 2, 2)
	require.NoError(t, err)
	assert.Len(t, page2, 2)
}

func TestListAll_Empty(t *testing.T) {
	repo := newShortLinkRepository(newTestDB(t))
	links, total, err := repo.ListAll(context.Background(), 1, 10)
	require.NoError(t, err)
	assert.Equal(t, uint32(0), total)
	assert.Empty(t, links)
}

// ---- DeleteByCode ----

func TestDeleteByCode_RemovesRegardlessOfOwner(t *testing.T) {
	db := newTestDB(t)
	repo := newShortLinkRepository(db)
	link := baseLink("adel123")
	seedLink(t, db, link, "some-owner")

	err := repo.DeleteByCode(context.Background(), "adel123")
	require.NoError(t, err)

	_, err = repo.GetByCode(context.Background(), "adel123")
	assert.ErrorIs(t, err, httpHelper.ErrNotFound)
}

func TestDeleteByCode_Anonymous(t *testing.T) {
	db := newTestDB(t)
	repo := newShortLinkRepository(db)
	link := baseLink("adel456")
	seedLink(t, db, link, "")

	err := repo.DeleteByCode(context.Background(), "adel456")
	require.NoError(t, err)
}

func TestDeleteByCode_NotFound(t *testing.T) {
	repo := newShortLinkRepository(newTestDB(t))
	err := repo.DeleteByCode(context.Background(), "ghost99")
	assert.ErrorIs(t, err, httpHelper.ErrNotFound)
}

// ---- SetActiveByCode ----

func TestSetActiveByCode_Deactivate(t *testing.T) {
	db := newTestDB(t)
	repo := newShortLinkRepository(db)
	link := baseLink("aset123")
	seedLink(t, db, link, "some-owner")

	err := repo.SetActiveByCode(context.Background(), "aset123", false)
	require.NoError(t, err)

	got, err := repo.GetByCode(context.Background(), "aset123")
	require.NoError(t, err)
	assert.False(t, got.IsActive)
}

func TestSetActiveByCode_Reactivate(t *testing.T) {
	db := newTestDB(t)
	repo := newShortLinkRepository(db)
	link := baseLink("aset456")
	link.IsActive = false
	seedLink(t, db, link, "some-owner")

	err := repo.SetActiveByCode(context.Background(), "aset456", true)
	require.NoError(t, err)

	got, err := repo.GetByCode(context.Background(), "aset456")
	require.NoError(t, err)
	assert.True(t, got.IsActive)
}

func TestSetActiveByCode_NotFound(t *testing.T) {
	repo := newShortLinkRepository(newTestDB(t))
	err := repo.SetActiveByCode(context.Background(), "ghost88", false)
	assert.ErrorIs(t, err, httpHelper.ErrNotFound)
}

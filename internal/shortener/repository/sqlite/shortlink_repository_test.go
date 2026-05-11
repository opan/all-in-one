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

	_, err = db.Exec(`
		CREATE TABLE short_links (
			id               TEXT PRIMARY KEY,
			code             TEXT NOT NULL UNIQUE,
			target_url       TEXT NOT NULL,
			owner_id         TEXT,
			created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			expires_at       DATETIME,
			is_active        BOOLEAN NOT NULL DEFAULT 1,
			click_count      INTEGER NOT NULL DEFAULT 0,
			last_accessed_at DATETIME
		);
		CREATE INDEX idx_short_links_owner_id ON short_links(owner_id);
	`)
	require.NoError(t, err)
	return db
}

func strPtr(s string) *string { return &s }

func seedLink(t *testing.T, db *sqlx.DB, link model.ShortLink) {
	t.Helper()
	_, err := db.Exec(`
		INSERT INTO short_links (id, code, target_url, owner_id, created_at, expires_at, is_active, click_count)
		VALUES (?, ?, ?, ?, ?, ?, ?, 0)
	`, link.ID, link.Code, link.TargetURL, link.OwnerID, link.CreatedAt, link.ExpiresAt, link.IsActive)
	require.NoError(t, err)
}

func baseLink(code, ownerID string) model.ShortLink {
	return model.ShortLink{
		ID:        "01JT" + code,
		Code:      code,
		TargetURL: "https://example.com/" + code,
		OwnerID:   strPtr(ownerID),
		CreatedAt: time.Now().UTC().Truncate(time.Second),
		IsActive:  true,
	}
}

// ---- Create ----

func TestCreate_Success(t *testing.T) {
	repo := newShortLinkRepository(newTestDB(t))
	link := baseLink("abc1234", "user-1")
	got, err := repo.Create(context.Background(), link)
	require.NoError(t, err)
	assert.Equal(t, link.Code, got.Code)
	assert.Equal(t, link.TargetURL, got.TargetURL)
	assert.Equal(t, link.OwnerID, got.OwnerID)
	assert.True(t, got.IsActive)
	assert.Equal(t, uint64(0), got.ClickCount)
}

func TestCreate_DuplicateCode(t *testing.T) {
	repo := newShortLinkRepository(newTestDB(t))
	link := baseLink("dupcode", "user-1")
	_, err := repo.Create(context.Background(), link)
	require.NoError(t, err)

	link2 := link
	link2.ID = "01JT-other-id"
	_, err = repo.Create(context.Background(), link2)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "UNIQUE constraint failed")
}

// ---- GetByCode ----

func TestGetByCode_Found(t *testing.T) {
	db := newTestDB(t)
	repo := newShortLinkRepository(db)
	link := baseLink("findme7", "user-1")
	seedLink(t, db, link)

	got, err := repo.GetByCode(context.Background(), "findme7")
	require.NoError(t, err)
	assert.Equal(t, "findme7", got.Code)
}

func TestGetByCode_NotFound(t *testing.T) {
	repo := newShortLinkRepository(newTestDB(t))
	_, err := repo.GetByCode(context.Background(), "missing")
	assert.ErrorIs(t, err, httpHelper.ErrNotFound)
}

// ---- ListByOwner ----

func TestListByOwner_Pagination(t *testing.T) {
	db := newTestDB(t)
	repo := newShortLinkRepository(db)
	ownerID := "owner-list"

	codes := []string{"aaa0001", "bbb0002", "ccc0003"}
	for i, code := range codes {
		l := baseLink(code, ownerID)
		l.CreatedAt = time.Now().Add(-time.Duration(i) * time.Second).UTC()
		seedLink(t, db, l)
	}
	// Different owner — must not appear in results
	other := baseLink("zzz9999", "other-owner")
	seedLink(t, db, other)

	links, total, err := repo.ListByOwner(context.Background(), ownerID, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, uint32(3), total)
	assert.Len(t, links, 3)
	for _, l := range links {
		assert.Equal(t, ownerID, *l.OwnerID)
	}
}

func TestListByOwner_PageSize(t *testing.T) {
	db := newTestDB(t)
	repo := newShortLinkRepository(db)
	ownerID := "owner-page"

	for i := range 5 {
		code := [7]byte{'p', 'g', '0', '0', '0', '0', byte('1' + i)}
		l := baseLink(string(code[:]), ownerID)
		l.CreatedAt = time.Now().Add(-time.Duration(i) * time.Second).UTC()
		seedLink(t, db, l)
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

// ---- Update ----

func TestUpdate_ToggleActive(t *testing.T) {
	db := newTestDB(t)
	repo := newShortLinkRepository(db)
	link := baseLink("upd1234", "user-upd")
	seedLink(t, db, link)

	link.IsActive = false
	updated, err := repo.Update(context.Background(), link)
	require.NoError(t, err)
	assert.False(t, updated.IsActive)
}

func TestUpdate_SetExpiry(t *testing.T) {
	db := newTestDB(t)
	repo := newShortLinkRepository(db)
	link := baseLink("exp1234", "user-upd")
	seedLink(t, db, link)

	exp := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Second)
	link.ExpiresAt = &exp
	updated, err := repo.Update(context.Background(), link)
	require.NoError(t, err)
	require.NotNil(t, updated.ExpiresAt)
	assert.WithinDuration(t, exp, *updated.ExpiresAt, time.Second)
}

func TestUpdate_WrongOwner(t *testing.T) {
	db := newTestDB(t)
	repo := newShortLinkRepository(db)
	link := baseLink("own1234", "real-owner")
	seedLink(t, db, link)

	link.OwnerID = strPtr("wrong-owner")
	_, err := repo.Update(context.Background(), link)
	assert.ErrorIs(t, err, httpHelper.ErrNotFound)
}

// ---- Delete ----

func TestDelete_Success(t *testing.T) {
	db := newTestDB(t)
	repo := newShortLinkRepository(db)
	link := baseLink("del1234", "user-del")
	seedLink(t, db, link)

	err := repo.Delete(context.Background(), "del1234", "user-del")
	require.NoError(t, err)

	_, err = repo.GetByCode(context.Background(), "del1234")
	assert.ErrorIs(t, err, httpHelper.ErrNotFound)
}

func TestDelete_WrongOwner(t *testing.T) {
	db := newTestDB(t)
	repo := newShortLinkRepository(db)
	link := baseLink("del5678", "real-owner")
	seedLink(t, db, link)

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
	link := baseLink("clk1234", "user-clk")
	seedLink(t, db, link)

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
	link := baseLink("dis1234", "user-dis")
	link.IsActive = false
	seedLink(t, db, link)

	now := time.Now().UTC().Format(time.RFC3339)
	err := repo.IncrementClick(context.Background(), "dis1234", now)
	assert.ErrorIs(t, err, httpHelper.ErrNotFound)
}

func TestIncrementClick_ExpiredLink(t *testing.T) {
	db := newTestDB(t)
	repo := newShortLinkRepository(db)
	link := baseLink("exp5678", "user-exp")
	past := time.Now().Add(-time.Hour).UTC()
	link.ExpiresAt = &past
	seedLink(t, db, link)

	now := time.Now().UTC().Format(time.RFC3339)
	err := repo.IncrementClick(context.Background(), "exp5678", now)
	assert.ErrorIs(t, err, httpHelper.ErrNotFound)
}

func TestIncrementClick_MultipleClicks(t *testing.T) {
	db := newTestDB(t)
	repo := newShortLinkRepository(db)
	link := baseLink("mul1234", "user-mul")
	seedLink(t, db, link)

	now := time.Now().UTC().Format(time.RFC3339)
	for range 5 {
		require.NoError(t, repo.IncrementClick(context.Background(), "mul1234", now))
	}

	got, err := repo.GetByCode(context.Background(), "mul1234")
	require.NoError(t, err)
	assert.Equal(t, uint64(5), got.ClickCount)
}

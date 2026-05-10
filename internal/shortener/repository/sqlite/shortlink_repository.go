package sqlite

import (
	"context"

	httpHelper "github.com/all-in-one/internal/http"
	"github.com/all-in-one/internal/shortener/model"
	"github.com/jmoiron/sqlx"
)

type shortLinkRepository struct {
	db *sqlx.DB
}

func newShortLinkRepository(db *sqlx.DB) *shortLinkRepository {
	return &shortLinkRepository{db: db}
}

func (r *shortLinkRepository) Create(ctx context.Context, link model.ShortLink) (model.ShortLink, error) {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO short_links (id, code, target_url, owner_id, created_at, expires_at, is_active, click_count)
		VALUES (?, ?, ?, ?, ?, ?, ?, 0)
	`, link.ID, link.Code, link.TargetURL, link.OwnerID, link.CreatedAt, link.ExpiresAt, link.IsActive)
	if err != nil {
		return model.ShortLink{}, err
	}
	return r.GetByCode(ctx, link.Code)
}

func (r *shortLinkRepository) GetByCode(ctx context.Context, code string) (model.ShortLink, error) {
	var link model.ShortLink
	err := r.db.GetContext(ctx, &link, `
		SELECT id, code, target_url, owner_id, created_at, expires_at, is_active, click_count, last_accessed_at
		FROM short_links
		WHERE code = ?
	`, code)
	if err != nil {
		return model.ShortLink{}, httpHelper.ErrNotFound
	}
	return link, nil
}

func (r *shortLinkRepository) ListByOwner(ctx context.Context, ownerID int64, page, pageSize uint32) ([]model.ShortLink, uint32, error) {
	if page == 0 {
		page = 1
	}
	if pageSize == 0 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var total uint32
	if err := r.db.GetContext(ctx, &total,
		`SELECT COUNT(*) FROM short_links WHERE owner_id = ?`, ownerID); err != nil {
		return nil, 0, err
	}

	var links []model.ShortLink
	err := r.db.SelectContext(ctx, &links, `
		SELECT id, code, target_url, owner_id, created_at, expires_at, is_active, click_count, last_accessed_at
		FROM short_links
		WHERE owner_id = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, ownerID, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	return links, total, nil
}

func (r *shortLinkRepository) Update(ctx context.Context, link model.ShortLink) (model.ShortLink, error) {
	_, err := r.db.ExecContext(ctx, `
		UPDATE short_links
		SET is_active = ?, expires_at = ?
		WHERE code = ? AND owner_id = ?
	`, link.IsActive, link.ExpiresAt, link.Code, link.OwnerID)
	if err != nil {
		return model.ShortLink{}, err
	}
	return r.GetByCode(ctx, link.Code)
}

func (r *shortLinkRepository) Delete(ctx context.Context, code string, ownerID int64) error {
	result, err := r.db.ExecContext(ctx,
		`DELETE FROM short_links WHERE code = ? AND owner_id = ?`, code, ownerID)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return httpHelper.ErrNotFound
	}
	return nil
}

func (r *shortLinkRepository) IncrementClick(ctx context.Context, code string, now string) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE short_links
		SET click_count = click_count + 1, last_accessed_at = ?
		WHERE code = ?
		  AND is_active = 1
		  AND (expires_at IS NULL OR expires_at > ?)
	`, now, code, now)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return httpHelper.ErrNotFound
	}
	return nil
}

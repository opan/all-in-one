package postgres

import (
	"context"
	"database/sql"
	"time"

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

func (r *shortLinkRepository) Create(ctx context.Context, link model.ShortLink, ownerID string) (model.ShortLink, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return model.ShortLink{}, err
	}
	defer tx.Rollback() //nolint:errcheck

	_, err = tx.ExecContext(ctx, `
		INSERT INTO short_links (id, code, target_url, created_at, expires_at, is_active, click_count)
		VALUES ($1, $2, $3, $4, $5, $6, 0)
	`, link.ID, link.Code, link.TargetURL, link.CreatedAt, link.ExpiresAt, link.IsActive)
	if err != nil {
		return model.ShortLink{}, err
	}

	if ownerID != "" {
		_, err = tx.ExecContext(ctx,
			`INSERT INTO short_link_owners (code, user_id) VALUES ($1, $2)`,
			link.Code, ownerID)
		if err != nil {
			return model.ShortLink{}, err
		}
	}

	if err = tx.Commit(); err != nil {
		return model.ShortLink{}, err
	}

	return r.GetByCode(ctx, link.Code)
}

func (r *shortLinkRepository) GetByCode(ctx context.Context, code string) (model.ShortLink, error) {
	var link model.ShortLink
	err := r.db.GetContext(ctx, &link, `
		SELECT id, code, target_url, created_at, expires_at, is_active, click_count, last_accessed_at
		FROM short_links
		WHERE code = $1
	`, code)
	if err != nil {
		if err == sql.ErrNoRows {
			return model.ShortLink{}, httpHelper.ErrNotFound
		}
		return model.ShortLink{}, err
	}
	return link, nil
}

func (r *shortLinkRepository) GetByCodeOwned(ctx context.Context, code, ownerID string) (model.ShortLink, error) {
	var link model.ShortLink
	err := r.db.GetContext(ctx, &link, `
		SELECT sl.id, sl.code, sl.target_url, sl.created_at, sl.expires_at, sl.is_active, sl.click_count, sl.last_accessed_at
		FROM short_links sl
		JOIN short_link_owners slo ON sl.code = slo.code
		WHERE sl.code = $1 AND slo.user_id = $2
	`, code, ownerID)
	if err != nil {
		if err == sql.ErrNoRows {
			return model.ShortLink{}, httpHelper.ErrNotFound
		}
		return model.ShortLink{}, err
	}
	return link, nil
}

func (r *shortLinkRepository) ListByOwner(ctx context.Context, ownerID string, page, pageSize uint32) ([]model.ShortLink, uint32, error) {
	if page == 0 {
		page = 1
	}
	if pageSize == 0 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var total uint32
	if err := r.db.GetContext(ctx, &total, `
		SELECT COUNT(*) FROM short_links sl
		JOIN short_link_owners slo ON sl.code = slo.code
		WHERE slo.user_id = $1
	`, ownerID); err != nil {
		return nil, 0, err
	}

	var links []model.ShortLink
	err := r.db.SelectContext(ctx, &links, `
		SELECT sl.id, sl.code, sl.target_url, sl.created_at, sl.expires_at, sl.is_active, sl.click_count, sl.last_accessed_at
		FROM short_links sl
		JOIN short_link_owners slo ON sl.code = slo.code
		WHERE slo.user_id = $1
		ORDER BY sl.created_at DESC
		LIMIT $2 OFFSET $3
	`, ownerID, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	return links, total, nil
}

func (r *shortLinkRepository) Update(ctx context.Context, link model.ShortLink, ownerID string) (model.ShortLink, error) {
	result, err := r.db.ExecContext(ctx, `
		UPDATE short_links
		SET is_active = $1, expires_at = $2
		WHERE code = $3
		  AND EXISTS (SELECT 1 FROM short_link_owners WHERE code = $4 AND user_id = $5)
	`, link.IsActive, link.ExpiresAt, link.Code, link.Code, ownerID)
	if err != nil {
		return model.ShortLink{}, err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return model.ShortLink{}, httpHelper.ErrNotFound
	}
	return r.GetByCode(ctx, link.Code)
}

func (r *shortLinkRepository) Delete(ctx context.Context, code, ownerID string) error {
	result, err := r.db.ExecContext(ctx, `
		DELETE FROM short_links
		WHERE code = $1
		  AND EXISTS (SELECT 1 FROM short_link_owners WHERE code = $2 AND user_id = $3)
	`, code, code, ownerID)
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
	ts, err := time.Parse(time.RFC3339, now)
	if err != nil {
		return err
	}
	result, err := r.db.ExecContext(ctx, `
		UPDATE short_links
		SET click_count = click_count + 1, last_accessed_at = $1
		WHERE code = $2
		  AND is_active = TRUE
		  AND (expires_at IS NULL OR expires_at > $3)
	`, ts, code, ts)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return httpHelper.ErrNotFound
	}
	return nil
}

func (r *shortLinkRepository) ListAll(ctx context.Context, page, pageSize uint32) ([]model.ShortLinkWithOwner, uint32, error) {
	if page == 0 {
		page = 1
	}
	if pageSize == 0 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var total uint32
	if err := r.db.GetContext(ctx, &total, `SELECT COUNT(*) FROM short_links`); err != nil {
		return nil, 0, err
	}

	links := make([]model.ShortLinkWithOwner, 0)
	err := r.db.SelectContext(ctx, &links, `
		SELECT sl.id, sl.code, sl.target_url, sl.created_at, sl.expires_at, sl.is_active, sl.click_count, sl.last_accessed_at,
		       COALESCE(slo.user_id, '') AS owner_id, COALESCE(u.username, '') AS owner_username
		FROM short_links sl
		LEFT JOIN short_link_owners slo ON sl.code = slo.code
		LEFT JOIN users u ON slo.user_id = u.id
		ORDER BY sl.created_at DESC
		LIMIT $1 OFFSET $2
	`, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	return links, total, nil
}

func (r *shortLinkRepository) DeleteByCode(ctx context.Context, code string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM short_links WHERE code = $1`, code)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return httpHelper.ErrNotFound
	}
	return nil
}

func (r *shortLinkRepository) SetActiveByCode(ctx context.Context, code string, active bool) error {
	result, err := r.db.ExecContext(ctx, `UPDATE short_links SET is_active = $1 WHERE code = $2`, active, code)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return httpHelper.ErrNotFound
	}
	return nil
}

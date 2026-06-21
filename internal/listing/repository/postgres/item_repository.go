package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	httpHelper "github.com/all-in-one/internal/http"
	"github.com/all-in-one/internal/listing/model"
	"github.com/all-in-one/internal/query"
	"github.com/jmoiron/sqlx"
)

type itemRepository struct {
	db *sqlx.DB
}

func newItemRepository(db *sqlx.DB) *itemRepository {
	return &itemRepository{db: db}
}

func (r *itemRepository) GetAll(ctx context.Context) ([]model.Item, error) {
	var items []model.Item
	err := r.db.SelectContext(ctx, &items, `
		SELECT id, topic_id, title, description, created_at, updated_at, form_schema_values
		FROM items
		ORDER BY id
	`)
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (r *itemRepository) GetByTopicID(ctx context.Context, topicID int) ([]model.Item, error) {
	var items []model.Item
	err := r.db.SelectContext(ctx, &items, `
		SELECT id, topic_id, title, description, created_at, updated_at, form_schema_values
		FROM items
		WHERE topic_id = $1
		ORDER BY id
	`, topicID)
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (r *itemRepository) Get(ctx context.Context, id int) (model.Item, error) {
	var item model.Item
	err := r.db.GetContext(ctx, &item, `
		SELECT *
		FROM items
		WHERE id = $1
	`, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return model.Item{}, httpHelper.ErrNotFound
		}
		return model.Item{}, err
	}
	return item, nil
}

func (r *itemRepository) Create(ctx context.Context, item model.Item) (model.Item, error) {
	now := time.Now().UTC()
	var id int
	err := r.db.QueryRowxContext(ctx, `
		INSERT INTO items (topic_id, title, description, created_at, updated_at, form_schema_values)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`, item.TopicID, item.Title, item.Description, now, now, item.FormSchemaValues).Scan(&id)
	if err != nil {
		return model.Item{}, err
	}
	item.ID = id
	item.CreatedAt = now
	item.UpdatedAt = now
	return item, nil
}

func (r *itemRepository) Update(ctx context.Context, id int, item model.Item) (model.Item, error) {
	existingItem, err := r.Get(ctx, id)
	if err != nil {
		return model.Item{}, err
	}

	now := time.Now().UTC()
	_, err = r.db.ExecContext(ctx, `
		UPDATE items
		SET topic_id = $1, title = $2, description = $3, updated_at = $4, form_schema_values = $5
		WHERE id = $6
	`, item.TopicID, item.Title, item.Description, now, item.FormSchemaValues, id)
	if err != nil {
		return model.Item{}, err
	}

	item.ID = id
	item.CreatedAt = existingItem.CreatedAt
	item.UpdatedAt = now
	return item, nil
}

func (r *itemRepository) Delete(ctx context.Context, id int) error {
	_, err := r.Get(ctx, id)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, "DELETE FROM items WHERE id = $1", id)
	return err
}

func (r *itemRepository) DeleteByTopicID(ctx context.Context, topicID int, opts ...query.QueryOptions) error {
	exec := getExecCtx(r.db, opts...)
	_, err := exec.ExecContext(ctx, "DELETE FROM items WHERE topic_id = $1", topicID)
	if err != nil {
		return fmt.Errorf("unable to delete items by topic ID: %w", err)
	}
	return nil
}

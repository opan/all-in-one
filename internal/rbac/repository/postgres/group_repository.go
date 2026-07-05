package postgres

import (
	"context"
	"database/sql"
	"time"

	httpHelper "github.com/all-in-one/internal/http"
	"github.com/all-in-one/internal/logging"
	"github.com/all-in-one/internal/query"
	"github.com/all-in-one/internal/rbac/model"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type groupRepository struct {
	db *sqlx.DB
}

func newGroupRepository(db *sqlx.DB) *groupRepository {
	return &groupRepository{db: db}
}

func (r *groupRepository) List(ctx context.Context) ([]model.Group, error) {
	var groups []model.Group
	if err := r.db.SelectContext(ctx, &groups, "SELECT * FROM groups ORDER BY name"); err != nil {
		return nil, err
	}
	return groups, nil
}

func (r *groupRepository) Get(ctx context.Context, id uuid.UUID) (model.Group, error) {
	var group model.Group
	if err := r.db.GetContext(ctx, &group, "SELECT * FROM groups WHERE id = $1", id.String()); err != nil {
		if err == sql.ErrNoRows {
			return model.Group{}, httpHelper.ErrNotFound
		}
		return model.Group{}, err
	}
	return group, nil
}

func (r *groupRepository) GetByName(ctx context.Context, name string) (model.Group, error) {
	var group model.Group
	if err := r.db.GetContext(ctx, &group, "SELECT * FROM groups WHERE name = $1", name); err != nil {
		if err == sql.ErrNoRows {
			return model.Group{}, httpHelper.ErrNotFound
		}
		return model.Group{}, err
	}
	return group, nil
}

func (r *groupRepository) Create(ctx context.Context, group model.Group, opts ...query.QueryOptions) error {
	log := logging.GetLoggerFromContext(ctx)
	log.Info().Str("entity", "GroupRepo").Str("action", "Create").Str("name", group.Name).Msg("create group")

	exec := getExecCtx(r.db, opts...)

	if group.ID == uuid.Nil {
		group.ID = uuid.New()
	}
	now := time.Now().UTC()
	group.CreatedAt = &now
	group.UpdatedAt = &now

	_, err := exec.NamedExecContext(ctx,
		`INSERT INTO groups (id, name, description, is_builtin, created_at, updated_at)
		VALUES (:id, :name, :description, :is_builtin, :created_at, :updated_at)`,
		group)
	return err
}

func (r *groupRepository) Update(ctx context.Context, group model.Group, opts ...query.QueryOptions) error {
	log := logging.GetLoggerFromContext(ctx)
	log.Info().Str("entity", "GroupRepo").Str("action", "Update").Str("id", group.ID.String()).Msg("update group")

	exec := getExecCtx(r.db, opts...)

	now := time.Now().UTC()
	group.UpdatedAt = &now

	_, err := exec.NamedExecContext(ctx,
		`UPDATE groups SET name = :name, description = :description, updated_at = :updated_at WHERE id = :id`,
		group)
	return err
}

func (r *groupRepository) Delete(ctx context.Context, id uuid.UUID, opts ...query.QueryOptions) error {
	log := logging.GetLoggerFromContext(ctx)
	log.Info().Str("entity", "GroupRepo").Str("action", "Delete").Str("id", id.String()).Msg("delete group")

	exec := getExecCtx(r.db, opts...)
	_, err := exec.ExecContext(ctx, "DELETE FROM groups WHERE id = $1", id.String())
	return err
}

// EnsureBuiltin returns the named group, creating it (is_builtin=true) if it
// doesn't exist yet. Safe to call on every server start.
func (r *groupRepository) EnsureBuiltin(ctx context.Context, name, description string) (model.Group, error) {
	existing, err := r.GetByName(ctx, name)
	if err == nil {
		return existing, nil
	}
	if err != httpHelper.ErrNotFound {
		return model.Group{}, err
	}

	now := time.Now().UTC()
	group := model.Group{
		ID:          uuid.New(),
		Name:        name,
		Description: description,
		IsBuiltin:   true,
		CreatedAt:   &now,
		UpdatedAt:   &now,
	}

	_, err = r.db.NamedExecContext(ctx,
		`INSERT INTO groups (id, name, description, is_builtin, created_at, updated_at)
		VALUES (:id, :name, :description, :is_builtin, :created_at, :updated_at)`,
		group)
	if err != nil {
		return model.Group{}, err
	}
	return group, nil
}

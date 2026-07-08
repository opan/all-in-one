package postgres

import (
	"context"
	"database/sql"

	httpHelper "github.com/all-in-one/internal/http"
	"github.com/all-in-one/internal/logging"
	"github.com/all-in-one/internal/query"
	"github.com/all-in-one/internal/rbac/model"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type userGroupRepository struct {
	db *sqlx.DB
}

func newUserGroupRepository(db *sqlx.DB) *userGroupRepository {
	return &userGroupRepository{db: db}
}

// GetGroupID returns the user's group ID, nil if the user has no group
// assigned (NULL — resolves to regular-user, see resolver), or ErrNotFound if
// the user itself doesn't exist.
func (r *userGroupRepository) GetGroupID(ctx context.Context, userID uuid.UUID) (*uuid.UUID, error) {
	var groupID sql.NullString
	err := r.db.GetContext(ctx, &groupID, "SELECT group_id FROM users WHERE id = $1", userID.String())
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, httpHelper.ErrNotFound
		}
		return nil, err
	}
	if !groupID.Valid {
		return nil, nil
	}
	parsed, err := uuid.Parse(groupID.String)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

// AssignGroup sets the user's group. Pass groupID=nil to unassign (falls
// back to the regular-user default).
func (r *userGroupRepository) AssignGroup(ctx context.Context, userID uuid.UUID, groupID *uuid.UUID, opts ...query.QueryOptions) error {
	log := logging.GetLoggerFromContext(ctx)
	log.Info().Str("entity", "UserGroupRepo").Str("action", "AssignGroup").Str("user_id", userID.String()).Msg("assign user group")

	exec := getExecCtx(r.db, opts...)

	var groupIDValue any
	if groupID != nil {
		groupIDValue = groupID.String()
	}

	_, err := exec.ExecContext(ctx, "UPDATE users SET group_id = $1 WHERE id = $2", groupIDValue, userID.String())
	return err
}

func (r *userGroupRepository) CountByGroup(ctx context.Context, groupID uuid.UUID) (int, error) {
	var count int
	err := r.db.GetContext(ctx, &count, "SELECT COUNT(*) FROM users WHERE group_id = $1", groupID.String())
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (r *userGroupRepository) ListUsersWithGroup(ctx context.Context) ([]model.UserAccessRow, error) {
	var rows []model.UserAccessRow
	err := r.db.SelectContext(ctx, &rows, `
		SELECT users.id AS user_id, users.username, users.email, users.group_id, users.blocked, groups.name AS group_name
		FROM users
		LEFT JOIN groups ON groups.id = users.group_id
		ORDER BY users.username`)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

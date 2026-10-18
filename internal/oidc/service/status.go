package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/all-in-one/internal/oidc"
	"github.com/all-in-one/internal/oidc/model"
	"github.com/google/uuid"
)

// AdminChecker is the subset of the RBAC resolver the status check needs.
type AdminChecker interface {
	IsAdmin(ctx context.Context, userID uuid.UUID) (bool, error)
}

// UserStatus reports whether an aio user is an admin right now (RFC-002).
// Apps ask on every admin request, so nothing here is cached.
func (s *Service) UserStatus(ctx context.Context, sub string) (model.UserStatus, error) {
	id, err := uuid.Parse(sub)
	if err != nil {
		return model.UserStatus{}, oidc.ErrUserNotFound
	}
	u, err := s.users.Find(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return model.UserStatus{}, oidc.ErrUserNotFound
	}
	if err != nil {
		return model.UserStatus{}, fmt.Errorf("load user: %w", err)
	}
	if u.Blocked {
		return model.UserStatus{Blocked: true}, nil
	}
	admin, err := s.admins.IsAdmin(ctx, id)
	if err != nil {
		return model.UserStatus{}, fmt.Errorf("resolve admin status: %w", err)
	}
	return model.UserStatus{Admin: admin}, nil
}

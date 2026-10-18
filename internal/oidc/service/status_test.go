package service

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	authnzModel "github.com/all-in-one/internal/authnz/model"
	"github.com/all-in-one/internal/oidc"
	"github.com/all-in-one/internal/oidc/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type usersByID map[uuid.UUID]authnzModel.User

func (u usersByID) Find(_ context.Context, id uuid.UUID) (authnzModel.User, error) {
	if user, ok := u[id]; ok {
		return user, nil
	}
	return authnzModel.User{}, sql.ErrNoRows
}

type adminCheck struct {
	admins map[uuid.UUID]bool
	err    error
}

func (a adminCheck) IsAdmin(_ context.Context, id uuid.UUID) (bool, error) {
	return a.admins[id], a.err
}

type brokenUsers struct{}

func (brokenUsers) Find(context.Context, uuid.UUID) (authnzModel.User, error) {
	return authnzModel.User{}, errors.New("db down")
}

func TestUserStatus(t *testing.T) {
	admin := authnzModel.User{ID: uuid.New(), Username: "admin"}
	regular := authnzModel.User{ID: uuid.New(), Username: "budi"}
	blockedAdmin := authnzModel.User{ID: uuid.New(), Username: "eks", Blocked: true}
	users := usersByID{admin.ID: admin, regular.ID: regular, blockedAdmin.ID: blockedAdmin}
	admins := map[uuid.UUID]bool{admin.ID: true, blockedAdmin.ID: true}

	cases := []struct {
		name    string
		users   UserLookup
		check   AdminChecker
		sub     string
		want    model.UserStatus
		wantErr error
		anyErr  bool
	}{
		{name: "admin", users: users, check: adminCheck{admins: admins}, sub: admin.ID.String(),
			want: model.UserStatus{Admin: true}},
		{name: "regular user", users: users, check: adminCheck{admins: admins}, sub: regular.ID.String(),
			want: model.UserStatus{}},
		{name: "blocked admin is not an admin", users: users, check: adminCheck{admins: admins},
			sub: blockedAdmin.ID.String(), want: model.UserStatus{Blocked: true}},
		{name: "unknown user", users: users, check: adminCheck{admins: admins}, sub: uuid.NewString(),
			wantErr: oidc.ErrUserNotFound},
		{name: "sub is not a uuid", users: users, check: adminCheck{admins: admins}, sub: "budi",
			wantErr: oidc.ErrUserNotFound},
		{name: "user lookup fails", users: brokenUsers{}, check: adminCheck{admins: admins},
			sub: admin.ID.String(), anyErr: true},
		{name: "admin check fails", users: users, check: adminCheck{err: errors.New("db down")},
			sub: admin.ID.String(), anyErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := &Service{users: tc.users, admins: tc.check}
			got, err := svc.UserStatus(context.Background(), tc.sub)
			switch {
			case tc.wantErr != nil:
				assert.ErrorIs(t, err, tc.wantErr)
			case tc.anyErr:
				require.Error(t, err)
				assert.NotErrorIs(t, err, oidc.ErrUserNotFound)
				assert.Equal(t, model.UserStatus{}, got)
			default:
				require.NoError(t, err)
				assert.Equal(t, tc.want, got)
			}
		})
	}
}

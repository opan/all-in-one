package tester

import (
	"context"

	"github.com/all-in-one/internal/auth"
)

func ContextWithUser(userID, username, email, sessionID string) context.Context {
	ctx := ContextWithLogger()
	claims := auth.UserClaims{
		UserID:    userID,
		Username:  username,
		Email:     email,
		SessionID: sessionID,
	}
	return context.WithValue(ctx, auth.UserContextKey, claims)
}

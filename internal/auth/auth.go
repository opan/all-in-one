package auth

import (
	"context"

	httpHelper "github.com/all-in-one/internal/http"
	"golang.org/x/crypto/bcrypt"
)

func CheckPassword(password, hash string) (bool, error) {
	// Implement password hash checking logic here
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return (err == nil), err
}

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	// Implement password hashing logic here
	return string(hash), nil
}

func GetUserFromContext(ctx context.Context) (httpHelper.UserClaims, bool) {
	user, ok := ctx.Value(httpHelper.UserContextKey).(httpHelper.UserClaims)
	return user, ok
}

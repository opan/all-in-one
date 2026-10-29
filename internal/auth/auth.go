package auth

import (
	"context"
	"errors"
	"fmt"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

type contextKey string

const (
	UserContextKey contextKey = "user"
)

type UserClaims struct {
	SessionID string `json:"sub"`
	Email     string `json:"email"`
	Username  string `json:"username"`
	UserID    string `json:"user_id"`
	// PasswordChangeRequired is copied from the session by the JWT middleware.
	PasswordChangeRequired bool `json:"-"`
}

// Rules for new passwords (signup and password change). bcrypt refuses input
// longer than 72 bytes, so that is the upper bound.
const (
	MinPasswordLength = 5
	MaxPasswordBytes  = 72
)

var (
	ErrPasswordTooShort = fmt.Errorf("password must be at least %d characters", MinPasswordLength)
	ErrPasswordTooLong  = errors.New("password is too long")
)

// ValidatePassword checks a new password against aio's rules.
func ValidatePassword(password string) error {
	if utf8.RuneCountInString(password) < MinPasswordLength {
		return ErrPasswordTooShort
	}
	if len(password) > MaxPasswordBytes {
		return ErrPasswordTooLong
	}
	return nil
}

func CheckPassword(password, hash string) (bool, error) {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return (err == nil), err
}

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func GetUserFromContext(ctx context.Context) (UserClaims, bool) {
	user, ok := ctx.Value(UserContextKey).(UserClaims)
	return user, ok
}

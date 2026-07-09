package model

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID        uuid.UUID  `json:"id" db:"id"`
	Username  string     `json:"username" db:"username"`
	Email     string     `json:"email" db:"email"`
	Name      string     `json:"name" db:"name"`
	LastLogin *time.Time `json:"last_login,omitempty" db:"last_login"`

	PasswordHash        string     `json:"-" db:"password_hash"`
	TOTPEnabled         bool       `json:"totp_enabled" db:"totp_enabled"`
	TOTPSecretEncrypted *string    `json:"-" db:"totp_secret_encrypted"`
	TOTPVerifiedAt      *time.Time `json:"-" db:"totp_verified_at"`
	GroupID             *uuid.UUID `json:"group_id,omitempty" db:"group_id"`
	Blocked             bool       `json:"blocked" db:"blocked"`
	CreatedAt           *time.Time `json:"created_at,omitempty" db:"created_at"`
	UpdatedAt           *time.Time `json:"updated_at,omitempty" db:"updated_at"`
}

type UserPasswordReset struct {
	CurrentPassword string `json:"current_password"`
	Password        string `json:"password"`
}

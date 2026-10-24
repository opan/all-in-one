package model

import (
	"time"

	"github.com/google/uuid"
)

// PasswordReset is a temporary password emailed to a user (RFC-003 §6.4). It
// works alongside the current password until it is used, voided or expires,
// so requesting one never locks the owner out.
type PasswordReset struct {
	ID               uuid.UUID  `db:"id"`
	UserID           uuid.UUID  `db:"user_id"`
	TempPasswordHash string     `db:"temp_password_hash"`
	ExpiresAt        time.Time  `db:"expires_at"`
	Attempts         int        `db:"attempts"`
	UsedAt           *time.Time `db:"used_at"`
	VoidedAt         *time.Time `db:"voided_at"`
	IssuedBy         *string    `db:"issued_by"` // admin user id; nil when the user asked for it
	CreatedAt        time.Time  `db:"created_at"`
}

// EmailVerification is a pending email address waiting for its owner to
// open the link sent to it (RFC-003 §6.2). TokenHash is the SHA-256 of the
// 256-bit token in the link.
type EmailVerification struct {
	ID        uuid.UUID  `db:"id"`
	UserID    uuid.UUID  `db:"user_id"`
	Email     string     `db:"email"`
	TokenHash string     `db:"token_hash"`
	ExpiresAt time.Time  `db:"expires_at"`
	UsedAt    *time.Time `db:"used_at"`
	CreatedAt time.Time  `db:"created_at"`
}

package model

import (
	"time"

	"github.com/google/uuid"
)

type TOTPChallenge struct {
	ID        uuid.UUID `json:"id" db:"id"`
	UserID    uuid.UUID `json:"user_id" db:"user_id"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	ExpiresAt time.Time `json:"expires_at" db:"expires_at"`
	Attempts  int       `json:"attempts" db:"attempts"`
	UserAgent string    `json:"user_agent" db:"user_agent"`
}

type RecoveryCode struct {
	ID        uuid.UUID  `json:"id" db:"id"`
	UserID    uuid.UUID  `json:"user_id" db:"user_id"`
	CodeHash  string     `json:"-" db:"code_hash"`
	UsedAt    *time.Time `json:"used_at,omitempty" db:"used_at"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
}

type TOTPSetupResponse struct {
	QRCode        string   `json:"qr_code"`
	Secret        string   `json:"secret"`
	RecoveryCodes []string `json:"recovery_codes"`
}

type TOTPVerifyRequest struct {
	Code string `json:"code"`
}

type TOTPChallengeResponse struct {
	Requires2FA    bool   `json:"requires_2fa"`
	ChallengeToken string `json:"challenge_token"`
	ExpiresAt      int64  `json:"expires_at"`
}

type TOTPLoginRequest struct {
	ChallengeToken string `json:"challenge_token"`
	Code           string `json:"code"`
}

type RecoveryCodeLoginRequest struct {
	ChallengeToken string `json:"challenge_token"`
	RecoveryCode   string `json:"recovery_code"`
}

type TOTPStatusResponse struct {
	Enabled                 bool `json:"enabled"`
	RecoveryCodesRemaining int  `json:"recovery_codes_remaining"`
}

type DisableTOTPRequest struct {
	CurrentPassword string `json:"current_password"`
}

type RegenerateRecoveryCodesRequest struct {
	CurrentPassword string `json:"current_password"`
}

type RegenerateRecoveryCodesResponse struct {
	RecoveryCodes []string `json:"recovery_codes"`
}

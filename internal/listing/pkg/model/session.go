package model

import (
	"time"

	"github.com/google/uuid"
)

type Session struct {
	ID                 uuid.UUID `json:"id"`
	UserID             uuid.UUID `json:"user_id" db:"user_id"`
	CreatedAt          time.Time `json:"created_at" db:"created_at"`
	UserAgent          string    `json:"user_agent" db:"user_agent"`
	AccessTokenExpiry  int       `json:"access_token_expiry" db:"access_token_expiry"`
	RefreshTokenExpiry int       `json:"refresh_token_expiry" db:"refresh_token_expiry"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Session) IsAccessTokenExpired() bool {
	return s.AccessTokenExpiry < int(time.Now().Unix())
}

func (s *Session) IsRefreshTokenExpired() bool {
	return s.RefreshTokenExpiry < int(time.Now().Unix())
}

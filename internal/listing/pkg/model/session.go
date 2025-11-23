package model

import (
	"time"

	"github.com/google/uuid"
)

type Session struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id" db:"user_id"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UserAgent string    `json:"user_agent" db:"user_agent"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

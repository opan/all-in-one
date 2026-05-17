package model

import "time"

type ShortLink struct {
	ID             string     `db:"id"               json:"id"`
	Code           string     `db:"code"             json:"code"`
	TargetURL      string     `db:"target_url"       json:"target_url"`
	CreatedAt      time.Time  `db:"created_at"       json:"created_at"`
	ExpiresAt      *time.Time `db:"expires_at"       json:"expires_at,omitempty"`
	IsActive       bool       `db:"is_active"        json:"is_active"`
	ClickCount     uint64     `db:"click_count"      json:"click_count"`
	LastAccessedAt *time.Time `db:"last_accessed_at" json:"last_accessed_at,omitempty"`
}

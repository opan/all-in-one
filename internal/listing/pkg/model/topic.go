package model

import "time"

// Item represents a listing item
type Topic struct {
	ID          int       `json:"id"`
	Name        string    `json:"title"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

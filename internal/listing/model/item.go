package model

import (
	"encoding/json"
	"time"
)

// Item represents a listing item
type Item struct {
	ID               int             `json:"id"`
	TopicID          int             `json:"topic_id" db:"topic_id"`
	Title            string          `json:"title"`
	FormSchemaValues json.RawMessage `json:"form_schema_values" db:"form_schema_values"`
	Description      string          `json:"description"`
	CreatedAt        time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at" db:"updated_at"`
}

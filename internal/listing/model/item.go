package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// JSONText is a raw JSON value stored as TEXT. It implements sql.Scanner and
// driver.Valuer so it can be scanned from either []byte (SQLite) or string
// (lib/pq/PostgreSQL), and marshals/unmarshals as raw JSON like json.RawMessage.
type JSONText json.RawMessage

// Scan implements sql.Scanner, accepting []byte, string, or nil.
func (j *JSONText) Scan(value any) error {
	if value == nil {
		*j = nil
		return nil
	}
	switch v := value.(type) {
	case []byte:
		*j = append((*j)[0:0], v...)
	case string:
		*j = append((*j)[0:0], v...)
	default:
		return fmt.Errorf("unsupported Scan type %T into JSONText", value)
	}
	return nil
}

// Value implements driver.Valuer.
func (j JSONText) Value() (driver.Value, error) {
	if len(j) == 0 {
		return nil, nil
	}
	return []byte(j), nil
}

// MarshalJSON returns the raw JSON.
func (j JSONText) MarshalJSON() ([]byte, error) {
	if len(j) == 0 {
		return []byte("null"), nil
	}
	return j, nil
}

// UnmarshalJSON stores the raw JSON.
func (j *JSONText) UnmarshalJSON(data []byte) error {
	if j == nil {
		return fmt.Errorf("model.JSONText: UnmarshalJSON on nil pointer")
	}
	*j = append((*j)[0:0], data...)
	return nil
}

// Item represents a listing item
type Item struct {
	ID               int       `json:"id"`
	TopicID          int       `json:"topic_id" db:"topic_id"`
	Title            string    `json:"title"`
	FormSchemaValues JSONText  `json:"form_schema_values" db:"form_schema_values"`
	Description      string    `json:"description"`
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time `json:"updated_at" db:"updated_at"`
}

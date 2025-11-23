package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// FieldType represents the type of custom field
type FieldType string

const (
	FieldTypeText        FieldType = "text"
	FieldTypeNumber      FieldType = "number"
	FieldTypeSelect      FieldType = "select"
	FieldTypeMultiSelect FieldType = "multi_select"
	FieldTypeCheckbox    FieldType = "checkbox"
	FieldTypeDate        FieldType = "date"
)

// CustomField defines a single custom field in the schema
type CustomField struct {
	Key          string      `json:"key"`
	Label        string      `json:"label"`
	Type         FieldType   `json:"type"`
	Required     bool        `json:"required"`
	Options      []string    `json:"options,omitempty"`
	DefaultValue interface{} `json:"default_value,omitempty"`
	Description  string      `json:"description,omitempty"`
}

// FormSchema defines the structure of custom fields for a topic
type FormSchema struct {
	Fields []CustomField `json:"fields"`
}

// Value implements driver.Valuer for database storage
func (fs FormSchema) Value() (driver.Value, error) {
	if len(fs.Fields) == 0 {
		return nil, nil
	}
	return json.Marshal(fs)
}

// Scan implements sql.Scanner for database retrieval
func (fs *FormSchema) Scan(value interface{}) error {
	if value == nil {
		fs.Fields = []CustomField{}
		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("unsupported type: %T", value)
	}

	return json.Unmarshal(bytes, fs)
}

type Topic struct {
	ID          int        `json:"id" db:"id"`
	Name        string     `json:"name" db:"name"`
	Description string     `json:"description" db:"description"`
	FormSchema  FormSchema `json:"form_schema" db:"form_schema"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at" db:"updated_at"`
}

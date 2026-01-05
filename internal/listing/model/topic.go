package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// JSONSchemaProperty defines a property in the JSON Schema
type JSONSchemaProperty struct {
	Type        string                   `json:"type"`
	Title       string                   `json:"title,omitempty"`
	Description string                   `json:"description,omitempty"`
	Enum        []string                 `json:"enum,omitempty"`
	Format      string                   `json:"format,omitempty"`
	Default     interface{}              `json:"default,omitempty"`
	Items       *JSONSchemaPropertyItems `json:"items,omitempty"`
}

// JSONSchemaPropertyItems defines items for array types
type JSONSchemaPropertyItems struct {
	Type string   `json:"type,omitempty"`
	Enum []string `json:"enum,omitempty"`
}

// JSONSchema defines the data schema (JSON Schema format)
type JSONSchema struct {
	Type       string                        `json:"type"`
	Properties map[string]JSONSchemaProperty `json:"properties"`
	Required   []string                      `json:"required,omitempty"`
}

// UISchemaElement defines a UI schema element
type UISchemaElement struct {
	Type     string                 `json:"type"`
	Scope    string                 `json:"scope,omitempty"`
	Label    interface{}            `json:"label,omitempty"` // string or bool
	Options  map[string]interface{} `json:"options,omitempty"`
	Elements []UISchemaElement      `json:"elements,omitempty"`
}

// UISchema defines the UI schema for JSONForms
type UISchema struct {
	Type     string                 `json:"type"`
	Elements []UISchemaElement      `json:"elements,omitempty"`
	Scope    string                 `json:"scope,omitempty"`
	Label    interface{}            `json:"label,omitempty"`
	Options  map[string]interface{} `json:"options,omitempty"`
}

// FormSchema defines the JSONForms.io compliant schema structure.
// It consists of two parts:
// 1. Schema (JSON Schema) - defines the data structure and validation rules
// 2. UISchema - defines how the form should be rendered
//
// Example:
//
//	{
//	  "schema": {
//	    "type": "object",
//	    "properties": {
//	      "title": {
//	        "type": "string",
//	        "title": "Title",
//	        "description": "Item title"
//	      },
//	      "price": {
//	        "type": "number",
//	        "title": "Price"
//	      },
//	      "category": {
//	        "type": "string",
//	        "title": "Category",
//	        "enum": ["electronics", "furniture", "clothing"]
//	      },
//	      "inStock": {
//	        "type": "boolean",
//	        "title": "In Stock"
//	      },
//	      "publishDate": {
//	        "type": "string",
//	        "format": "date",
//	        "title": "Publish Date"
//	      }
//	    },
//	    "required": ["title", "price"]
//	  },
//	  "uischema": {
//	    "type": "VerticalLayout",
//	    "elements": [
//	      {
//	        "type": "Control",
//	        "scope": "#/properties/title"
//	      },
//	      {
//	        "type": "Control",
//	        "scope": "#/properties/price"
//	      },
//	      {
//	        "type": "Control",
//	        "scope": "#/properties/category"
//	      },
//	      {
//	        "type": "Control",
//	        "scope": "#/properties/inStock"
//	      },
//	      {
//	        "type": "Control",
//	        "scope": "#/properties/publishDate"
//	      }
//	    ]
//	  }
//	}
//
// For more information, see https://jsonforms.io/
type FormSchema struct {
	Schema   JSONSchema `json:"schema"`
	UISchema UISchema   `json:"uischema"`
}

// Value implements driver.Valuer for database storage
func (fs FormSchema) Value() (driver.Value, error) {
	if len(fs.Schema.Properties) == 0 {
		return nil, nil
	}
	return json.Marshal(fs)
}

// Scan implements sql.Scanner for database retrieval
func (fs *FormSchema) Scan(value interface{}) error {
	if value == nil {
		fs.Schema.Properties = make(map[string]JSONSchemaProperty)
		fs.UISchema.Elements = []UISchemaElement{}
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
	UserID      uuid.UUID  `json:"user_id" db:"user_id"`
	Name        string     `json:"name" db:"name"`
	Description string     `json:"description" db:"description"`
	FormSchema  FormSchema `json:"form_schema" db:"form_schema"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at" db:"updated_at"`
}

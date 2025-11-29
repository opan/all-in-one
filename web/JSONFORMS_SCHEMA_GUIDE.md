# JSONForms Schema Guide

This guide explains how to create form schemas for topics using the JSONForms.io format.

## Overview

Topics in the listing app use [JSONForms](https://jsonforms.io/) to define custom fields for items. A JSONForms schema consists of two parts:

1. **JSON Schema** - Defines the data structure and validation rules
2. **UI Schema** - Defines how the form should be rendered

## JSON Schema

The JSON Schema follows the [JSON Schema specification](https://json-schema.org/). It defines:
- Property names and types
- Validation rules (required fields, enums, formats)
- Default values
- Descriptions

### Supported Types

- `string` - Text input
- `number` - Numeric input (decimals allowed)
- `integer` - Whole numbers only
- `boolean` - Checkbox
- `array` - List of items

### Supported Formats

For string types, you can use these formats:
- `date` - Date picker
- `time` - Time picker
- `date-time` - Date and time picker
- `email` - Email input with validation
- `uri` - URL input

## UI Schema

The UI Schema defines how controls are laid out and rendered. 

### Layout Types

- `VerticalLayout` - Stack controls vertically
- `HorizontalLayout` - Arrange controls horizontally
- `Group` - Group controls with a label

### Control Element

Each control must specify:
- `type: "Control"` - Indicates this is a form control
- `scope` - JSON pointer to the property in the format `#/properties/propertyName`

Optional attributes:
- `label` - Custom label (string) or `false` to hide label
- `options` - Renderer-specific options

## Complete Example

```json
{
  "schema": {
    "type": "object",
    "properties": {
      "title": {
        "type": "string",
        "title": "Title",
        "description": "The item's title"
      },
      "price": {
        "type": "number",
        "title": "Price",
        "description": "Price in USD"
      },
      "category": {
        "type": "string",
        "title": "Category",
        "enum": ["electronics", "furniture", "clothing", "other"]
      },
      "condition": {
        "type": "string",
        "title": "Condition",
        "enum": ["new", "like-new", "used", "for-parts"],
        "default": "used"
      },
      "inStock": {
        "type": "boolean",
        "title": "In Stock",
        "default": true
      },
      "publishDate": {
        "type": "string",
        "format": "date",
        "title": "Publish Date"
      },
      "tags": {
        "type": "array",
        "title": "Tags",
        "items": {
          "type": "string"
        }
      },
      "description": {
        "type": "string",
        "title": "Description",
        "description": "Detailed item description"
      }
    },
    "required": ["title", "price", "category"]
  },
  "uischema": {
    "type": "VerticalLayout",
    "elements": [
      {
        "type": "Control",
        "scope": "#/properties/title"
      },
      {
        "type": "HorizontalLayout",
        "elements": [
          {
            "type": "Control",
            "scope": "#/properties/price"
          },
          {
            "type": "Control",
            "scope": "#/properties/category"
          }
        ]
      },
      {
        "type": "Control",
        "scope": "#/properties/condition"
      },
      {
        "type": "Control",
        "scope": "#/properties/inStock"
      },
      {
        "type": "Control",
        "scope": "#/properties/publishDate"
      },
      {
        "type": "Control",
        "scope": "#/properties/tags"
      },
      {
        "type": "Control",
        "scope": "#/properties/description",
        "options": {
          "multi": true
        }
      }
    ]
  }
}
```

## Simple Example (Minimal)

```json
{
  "schema": {
    "type": "object",
    "properties": {
      "name": {
        "type": "string",
        "title": "Name"
      },
      "email": {
        "type": "string",
        "format": "email",
        "title": "Email"
      }
    },
    "required": ["name"]
  },
  "uischema": {
    "type": "VerticalLayout",
    "elements": [
      {
        "type": "Control",
        "scope": "#/properties/name"
      },
      {
        "type": "Control",
        "scope": "#/properties/email"
      }
    ]
  }
}
```

## Advanced Features

### Enum with Radio Buttons

```json
{
  "schema": {
    "type": "object",
    "properties": {
      "size": {
        "type": "string",
        "title": "Size",
        "enum": ["small", "medium", "large"]
      }
    }
  },
  "uischema": {
    "type": "VerticalLayout",
    "elements": [
      {
        "type": "Control",
        "scope": "#/properties/size",
        "options": {
          "format": "radio"
        }
      }
    ]
  }
}
```

### Readonly Fields

```json
{
  "type": "Control",
  "scope": "#/properties/createdAt",
  "options": {
    "readonly": true
  }
}
```

### Custom Labels

```json
{
  "type": "Control",
  "scope": "#/properties/firstName",
  "label": "First Name"
}
```

### Hide Label

```json
{
  "type": "Control",
  "scope": "#/properties/search",
  "label": false
}
```

## Tips

1. **Keep it simple** - Start with a basic schema and add complexity as needed
2. **Required fields** - Add frequently-needed fields to the `required` array
3. **Default values** - Provide sensible defaults to improve UX
4. **Descriptions** - Add helpful descriptions for complex fields
5. **Enums** - Use enums for fields with a fixed set of options
6. **Validation** - JSON Schema provides rich validation capabilities

## Resources

- [JSONForms Documentation](https://jsonforms.io/docs/)
- [JSON Schema Guide](https://json-schema.org/understanding-json-schema/)
- [JSONForms Examples](https://jsonforms.io/examples/)

## Testing Your Schema

When creating a topic, paste your schema JSON into the "Form Schema (JSON)" textarea. The preview section will show:
1. All properties defined in the schema
2. Their types, formats, and requirements
3. The UI layout structure

If there are validation errors, they will be displayed below the textarea.

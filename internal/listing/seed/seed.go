package seed

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/all-in-one/internal/listing/model"
	"github.com/all-in-one/internal/listing/repository"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// SeedTopicsAndItems initializes sample topics and items in the database
func SeedTopicsAndItems(ctx context.Context, storage repository.Storage, userID uuid.UUID, log zerolog.Logger) error {
	topicRepo := storage.TopicRepo()
	itemRepo := storage.ItemRepo()

	// Check if topics already exist
	existingTopics, err := topicRepo.GetAll(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to check existing topics: %w", err)
	}

	if len(existingTopics) > 0 {
		log.Info().Msg("Topics already exist, skipping seed")
		return nil
	}

	// Define sample topics
	topics := []model.Topic{
		{
			UserID:      userID,
			Name:        "Electronics",
			Description: "Electronic devices and gadgets",
			FormSchema: model.FormSchema{
				Schema: model.JSONSchema{
					Type: "object",
					Properties: map[string]model.JSONSchemaProperty{
						"brand": {
							Type:  "string",
							Title: "Brand",
						},
						"model": {
							Type:  "string",
							Title: "Model",
						},
						"condition": {
							Type:  "string",
							Title: "Condition",
							Enum:  []string{"new", "like-new", "good", "fair", "poor"},
						},
						"price": {
							Type:   "number",
							Title:  "Price",
							Format: "currency",
						},
						"warranty": {
							Type:  "boolean",
							Title: "Has Warranty",
						},
					},
					Required: []string{"brand", "model", "condition", "price"},
				},
				UISchema: model.UISchema{
					Type: "VerticalLayout",
					Elements: []model.UISchemaElement{
						{
							Type:  "Control",
							Scope: "#/properties/brand",
						},
						{
							Type:  "Control",
							Scope: "#/properties/model",
						},
						{
							Type:  "Control",
							Scope: "#/properties/condition",
						},
						{
							Type:  "Control",
							Scope: "#/properties/price",
						},
						{
							Type:  "Control",
							Scope: "#/properties/warranty",
						},
					},
				},
			},
		},
		{
			UserID:      userID,
			Name:        "Books",
			Description: "Books and reading materials",
			FormSchema: model.FormSchema{
				Schema: model.JSONSchema{
					Type: "object",
					Properties: map[string]model.JSONSchemaProperty{
						"title": {
							Type:  "string",
							Title: "Book Title",
						},
						"author": {
							Type:  "string",
							Title: "Author",
						},
						"isbn": {
							Type:  "string",
							Title: "ISBN",
						},
						"genre": {
							Type:  "string",
							Title: "Genre",
							Enum:  []string{"fiction", "non-fiction", "science", "history", "biography", "other"},
						},
						"condition": {
							Type:  "string",
							Title: "Condition",
							Enum:  []string{"new", "like-new", "good", "acceptable"},
						},
						"price": {
							Type:   "number",
							Title:  "Price",
							Format: "currency",
						},
					},
					Required: []string{"title", "author", "genre", "condition", "price"},
				},
				UISchema: model.UISchema{
					Type: "VerticalLayout",
					Elements: []model.UISchemaElement{
						{
							Type:  "Control",
							Scope: "#/properties/title",
						},
						{
							Type:  "Control",
							Scope: "#/properties/author",
						},
						{
							Type:  "Control",
							Scope: "#/properties/isbn",
						},
						{
							Type:  "Control",
							Scope: "#/properties/genre",
						},
						{
							Type:  "Control",
							Scope: "#/properties/condition",
						},
						{
							Type:  "Control",
							Scope: "#/properties/price",
						},
					},
				},
			},
		},
		{
			UserID:      userID,
			Name:        "Vehicles",
			Description: "Cars, motorcycles, and other vehicles",
			FormSchema: model.FormSchema{
				Schema: model.JSONSchema{
					Type: "object",
					Properties: map[string]model.JSONSchemaProperty{
						"make": {
							Type:  "string",
							Title: "Make",
						},
						"model": {
							Type:  "string",
							Title: "Model",
						},
						"year": {
							Type:  "integer",
							Title: "Year",
						},
						"mileage": {
							Type:  "integer",
							Title: "Mileage",
						},
						"condition": {
							Type:  "string",
							Title: "Condition",
							Enum:  []string{"excellent", "good", "fair", "poor"},
						},
						"price": {
							Type:   "number",
							Title:  "Price",
							Format: "currency",
						},
					},
					Required: []string{"make", "model", "year", "price"},
				},
				UISchema: model.UISchema{
					Type: "VerticalLayout",
					Elements: []model.UISchemaElement{
						{
							Type:  "Control",
							Scope: "#/properties/make",
						},
						{
							Type:  "Control",
							Scope: "#/properties/model",
						},
						{
							Type:  "Control",
							Scope: "#/properties/year",
						},
						{
							Type:  "Control",
							Scope: "#/properties/mileage",
						},
						{
							Type:  "Control",
							Scope: "#/properties/condition",
						},
						{
							Type:  "Control",
							Scope: "#/properties/price",
						},
					},
				},
			},
		},
	}

	// Create topics and store their IDs
	createdTopics := make([]model.Topic, 0, len(topics))
	for _, topic := range topics {
		created, err := topicRepo.Create(ctx, topic)
		if err != nil {
			return fmt.Errorf("failed to create topic %s: %w", topic.Name, err)
		}
		createdTopics = append(createdTopics, created)
		log.Info().
			Str("name", topic.Name).
			Int("id", created.ID).
			Msg("Topic created successfully")
	}

	// Define sample items for each topic
	sampleItems := []struct {
		topicName string
		items     []struct {
			title       string
			description string
			values      map[string]interface{}
		}
	}{
		{
			topicName: "Electronics",
			items: []struct {
				title       string
				description string
				values      map[string]interface{}
			}{
				{
					title:       "iPhone 15 Pro",
					description: "Latest iPhone model in excellent condition",
					values: map[string]interface{}{
						"brand":     "Apple",
						"model":     "iPhone 15 Pro",
						"condition": "new",
						"price":     999.99,
						"warranty":  true,
					},
				},
				{
					title:       "Samsung Galaxy S24",
					description: "Flagship Android phone",
					values: map[string]interface{}{
						"brand":     "Samsung",
						"model":     "Galaxy S24",
						"condition": "like-new",
						"price":     849.99,
						"warranty":  true,
					},
				},
				{
					title:       "MacBook Pro 16",
					description: "Powerful laptop for professionals",
					values: map[string]interface{}{
						"brand":     "Apple",
						"model":     "MacBook Pro 16-inch",
						"condition": "good",
						"price":     2299.99,
						"warranty":  false,
					},
				},
			},
		},
		{
			topicName: "Books",
			items: []struct {
				title       string
				description string
				values      map[string]interface{}
			}{
				{
					title:       "The Go Programming Language",
					description: "Comprehensive guide to Go",
					values: map[string]interface{}{
						"title":     "The Go Programming Language",
						"author":    "Alan Donovan & Brian Kernighan",
						"isbn":      "978-0134190440",
						"genre":     "science",
						"condition": "like-new",
						"price":     45.99,
					},
				},
				{
					title:       "Clean Code",
					description: "A handbook of agile software craftsmanship",
					values: map[string]interface{}{
						"title":     "Clean Code",
						"author":    "Robert C. Martin",
						"isbn":      "978-0132350884",
						"genre":     "science",
						"condition": "good",
						"price":     39.99,
					},
				},
				{
					title:       "Sapiens",
					description: "A brief history of humankind",
					values: map[string]interface{}{
						"title":     "Sapiens: A Brief History of Humankind",
						"author":    "Yuval Noah Harari",
						"isbn":      "978-0062316097",
						"genre":     "history",
						"condition": "new",
						"price":     24.99,
					},
				},
			},
		},
		{
			topicName: "Vehicles",
			items: []struct {
				title       string
				description string
				values      map[string]interface{}
			}{
				{
					title:       "2022 Toyota Camry",
					description: "Reliable sedan with low mileage",
					values: map[string]interface{}{
						"make":      "Toyota",
						"model":     "Camry",
						"year":      2022,
						"mileage":   15000,
						"condition": "excellent",
						"price":     28999.99,
					},
				},
				{
					title:       "2020 Honda Civic",
					description: "Compact car, fuel efficient",
					values: map[string]interface{}{
						"make":      "Honda",
						"model":     "Civic",
						"year":      2020,
						"mileage":   35000,
						"condition": "good",
						"price":     19999.99,
					},
				},
			},
		},
	}

	// Create items
	totalItems := 0
	for _, sampleGroup := range sampleItems {
		// Find the topic ID for this group
		var topicID int
		for _, t := range createdTopics {
			if t.Name == sampleGroup.topicName {
				topicID = t.ID
				break
			}
		}

		if topicID == 0 {
			log.Warn().Str("topic", sampleGroup.topicName).Msg("Topic not found, skipping items")
			continue
		}

		for _, itemData := range sampleGroup.items {
			// Marshal form schema values to JSON
			valuesJSON, err := json.Marshal(itemData.values)
			if err != nil {
				return fmt.Errorf("failed to marshal form values for item %s: %w", itemData.title, err)
			}

			item := model.Item{
				TopicID:          topicID,
				Title:            itemData.title,
				Description:      itemData.description,
				FormSchemaValues: valuesJSON,
			}

			created, err := itemRepo.Create(ctx, item)
			if err != nil {
				return fmt.Errorf("failed to create item %s: %w", itemData.title, err)
			}

			totalItems++
			log.Info().
				Str("title", itemData.title).
				Int("id", created.ID).
				Int("topic_id", topicID).
				Msg("Item created successfully")
		}
	}

	log.Info().
		Int("topics", len(createdTopics)).
		Int("items", totalItems).
		Msg("Successfully seeded topics and items")

	return nil
}

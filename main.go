package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/all-in-one/pkg/listing"
	"github.com/all-in-one/pkg/models"
	"github.com/all-in-one/pkg/storage"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
)

// Health check endpoint
func healthCheck(w http.ResponseWriter, r *http.Request) {
	response := models.Response{
		Success: true,
		Message: "API is running",
		Data: map[string]interface{}{
			"timestamp": time.Now(),
			"version":   "1.0.0",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func main() {
	// Initialize storage
	itemStore := storage.NewMemoryStorage()

	// Initialize sample data
	itemCount := itemStore.InitializeSampleData()
	fmt.Printf("✅ Initialized with %d sample items\n", itemCount)

	// Initialize router
	r := mux.NewRouter()

	// API routes
	api := r.PathPrefix("/api/v1").Subrouter()

	// Initialize and register listing handler
	listingHandler := listing.NewHandler(itemStore)
	listingHandler.RegisterRoutes(api)

	// Health check
	api.HandleFunc("/health", healthCheck).Methods("GET")

	// Setup CORS for frontend integration
	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"}, // In production, specify your frontend domain
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"*"},
	})

	// Wrap router with CORS
	handler := c.Handler(r)

	// Start server
	port := ":8080"
	fmt.Printf("🚀 Server starting on port %s\n", port)
	fmt.Println("📋 Available endpoints:")
	fmt.Println("  GET    /api/v1/health      - Health check")
	fmt.Println("  GET    /api/v1/items       - Get all items")
	fmt.Println("  POST   /api/v1/items       - Create new item")
	fmt.Println("  GET    /api/v1/items/{id}  - Get item by ID")
	fmt.Println("  PUT    /api/v1/items/{id}  - Update item")
	fmt.Println("  DELETE /api/v1/items/{id}  - Delete item")
	fmt.Println()

	log.Fatal(http.ListenAndServe(port, handler))
}

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/all-in-one/internal/common"
	"github.com/all-in-one/internal/config"
	"github.com/all-in-one/internal/listing"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
)

// Health check endpoint
func healthCheck(w http.ResponseWriter, r *http.Request) {
	response := common.Response{
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
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	fmt.Printf("🔧 Using %s storage\n", cfg.Storage.Type)

	// Initialize listing service based on configuration
	var listingService *listing.Service

	switch cfg.Storage.Type {
	case "sqlite":
		listingService, err = listing.NewSQLiteService(cfg.Storage.Path)
		if err != nil {
			log.Fatalf("Failed to initialize SQLite storage: %v", err)
		}
		defer func() {
			if err := listingService.Close(); err != nil {
				log.Printf("Error closing SQLite storage: %v", err)
			}
		}()
	case "memory":
		listingService = listing.NewMemoryService()
	default:
		log.Fatalf("Unknown storage type: %s. Supported types: memory, sqlite", cfg.Storage.Type)
	}

	// Initialize sample data
	listingCount := listingService.InitializeSampleData()
	fmt.Printf("✅ Initialized with %d sample listings\n", listingCount)

	// Initialize router
	r := mux.NewRouter()

	// API routes
	api := r.PathPrefix("/api/v1").Subrouter()

	// Register listing routes
	listingService.RegisterRoutes(api)

	// Health check
	api.HandleFunc("/health", healthCheck).Methods("GET") // Setup CORS for frontend integration
	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"}, // In production, specify your frontend domain
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"*"},
	})

	// Wrap router with CORS
	handler := c.Handler(r)

	// Start server
	port := cfg.Server.Port
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

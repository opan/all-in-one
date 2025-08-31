package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/all-in-one/pkg/common"
	"github.com/all-in-one/pkg/listing"
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
	// Initialize listing storage
	// Option 1: In-memory storage
	listingStore := listing.NewMemoryStorage()

	// Option 2: SQLite storage (uncomment to use)
	// listingStore, err := listing.NewSQLiteStorage("./data/listings.db")
	// if err != nil {
	//     log.Fatalf("Failed to initialize SQLite storage: %v", err)
	// }
	// defer listingStore.(*listing.SQLiteStorage).Close()

	// Initialize sample data
	listingCount := listingStore.InitializeSampleData()
	fmt.Printf("✅ Initialized with %d sample listings\n", listingCount)

	// Initialize router
	r := mux.NewRouter()

	// API routes
	api := r.PathPrefix("/api/v1").Subrouter()

	// Initialize and register listing handler
	listingHandler := listing.NewHandler(listingStore)
	listingHandler.RegisterRoutes(api)

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

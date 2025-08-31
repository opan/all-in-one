package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/all-in-one/pkg/book"
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

	// Initialize book storage
	// Option 1: In-memory storage
	bookStore := book.NewMemoryStorage()

	// Option 2: SQLite storage (uncomment to use)
	// bookStore, err := book.NewSQLiteStorage("./data/books.db")
	// if err != nil {
	//     log.Fatalf("Failed to initialize SQLite storage: %v", err)
	// }
	// defer bookStore.(*book.SQLiteStorage).Close()

	// Initialize sample data
	listingCount := listingStore.InitializeSampleData()
	bookCount := bookStore.InitializeSampleData()
	fmt.Printf("✅ Initialized with %d sample listings and %d sample books\n", listingCount, bookCount)

	// Initialize router
	r := mux.NewRouter()

	// API routes
	api := r.PathPrefix("/api/v1").Subrouter()

	// Initialize and register listing handler
	listingHandler := listing.NewHandler(listingStore)
	listingHandler.RegisterRoutes(api)

	// Initialize and register book handler
	bookHandler := book.NewHandler(bookStore)
	bookHandler.RegisterRoutes(api)

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
	fmt.Println("  GET    /api/v1/books       - Get all books")
	fmt.Println("  POST   /api/v1/books       - Create new book")
	fmt.Println("  GET    /api/v1/books/{id}  - Get book by ID")
	fmt.Println("  PUT    /api/v1/books/{id}  - Update book")
	fmt.Println("  DELETE /api/v1/books/{id}  - Delete book")
	fmt.Println()

	log.Fatal(http.ListenAndServe(port, handler))
}

package listing

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/all-in-one/internal/common"
	"github.com/all-in-one/internal/config"
	"github.com/all-in-one/internal/listing"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
	"github.com/sirupsen/logrus"
)

// Health check endpoint
func healthCheck(w http.ResponseWriter, r *http.Request) {
	logrus.WithFields(logrus.Fields{
		"method": r.Method,
		"path":   r.URL.Path,
		"ip":     r.RemoteAddr,
	}).Info("Health check requested")

	response := common.Response{
		Success: true,
		Message: "Listing API is running",
		Data: map[string]interface{}{
			"timestamp": time.Now(),
			"version":   "1.0.0",
			"service":   "listing",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// LoggingMiddleware logs all HTTP requests
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		logrus.WithFields(logrus.Fields{
			"method": r.Method,
			"path":   r.URL.Path,
			"ip":     r.RemoteAddr,
		}).Info("Request started")

		next.ServeHTTP(w, r)

		logrus.WithFields(logrus.Fields{
			"method":   r.Method,
			"path":     r.URL.Path,
			"ip":       r.RemoteAddr,
			"duration": time.Since(start),
		}).Info("Request completed")
	})
}

// Run starts the listing service
func Run() {
	// Setup logging
	logrus.SetFormatter(&logrus.JSONFormatter{})
	logrus.SetLevel(logrus.InfoLevel)

	fmt.Println("🏷️  Starting Listing Service...")
	logrus.Info("Initializing Listing Service")

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		logrus.WithError(err).Fatal("Failed to load config")
	}

	logrus.WithField("storage_type", cfg.Storage.Type).Info("Configuration loaded")
	fmt.Printf("🔧 Using %s storage\n", cfg.Storage.Type)

	// Initialize listing service based on configuration
	var listingService *listing.Service

	switch cfg.Storage.Type {
	case "sqlite":
		logrus.WithField("db_path", cfg.Storage.Path).Info("Initializing SQLite storage")
		listingService, err = listing.NewSQLiteService(cfg.Storage.Path)
		if err != nil {
			logrus.WithError(err).Fatal("Failed to initialize SQLite storage")
		}
		defer func() {
			if err := listingService.Close(); err != nil {
				logrus.WithError(err).Error("Error closing SQLite storage")
			}
		}()
	case "memory":
		logrus.Info("Initializing in-memory storage")
		listingService = listing.NewMemoryService()
	default:
		logrus.WithField("storage_type", cfg.Storage.Type).Fatal("Unknown storage type. Supported types: memory, sqlite")
	}

	// Initialize sample data
	listingCount := listingService.InitializeSampleData()
	logrus.WithField("count", listingCount).Info("Sample data initialized")
	fmt.Printf("✅ Initialized with %d sample listings\n", listingCount)

	// Initialize router
	r := mux.NewRouter()

	// Add logging middleware
	r.Use(loggingMiddleware)

	// API routes
	api := r.PathPrefix("/api/v1").Subrouter()

	// Register listing routes
	listingService.RegisterRoutes(api)

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
	port := cfg.Server.Port
	logrus.WithField("port", port).Info("Starting HTTP server")
	fmt.Printf("🚀 Listing Service starting on port %s\n", port)
	fmt.Println("📋 Available endpoints:")
	fmt.Println("  GET    /api/v1/health      - Health check")
	fmt.Println("  GET    /api/v1/items       - Get all items")
	fmt.Println("  POST   /api/v1/items       - Create new item")
	fmt.Println("  GET    /api/v1/items/{id}  - Get item by ID")
	fmt.Println("  PUT    /api/v1/items/{id}  - Update item")
	fmt.Println("  DELETE /api/v1/items/{id}  - Delete item")
	fmt.Println()

	logrus.Fatal(http.ListenAndServe(port, handler))
}

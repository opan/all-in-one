package server

import (
	"net/http"

	"github.com/all-in-one/internal/config"
	httpHelper "github.com/all-in-one/internal/http"
	"github.com/all-in-one/internal/storage"
	"github.com/rs/cors"
	"github.com/rs/zerolog"

	"github.com/gorilla/mux"
)

type server struct {
	config config.Config
	log    zerolog.Logger
}

type Opts struct {
	Config config.Config
	Logger zerolog.Logger
}

func New(opts Opts) *server {
	return &server{
		config: opts.Config,
		log:    opts.Logger,
	}
}

func (s *server) Start() error {
	// Implementation of server start logic

	s.log.Info().Msg("Initiating server start...")

	storage := storage.NewStorage(s.config, s.log)
	svc, err := storage.CreateService()
	if err != nil {
		s.log.Error().Err(err).Msg("Failed to create listing service")
		return err
	}

	s.log.Info().Msg("Initializing sample data...")
	dataCount := svc.InitializeSampleData()
	s.log.Info().Msgf("Initialized %d sample data items", dataCount)

	// Initialize HTTP helper
	h := httpHelper.NewHTTP(s.log)

	// Initialize router
	r := mux.NewRouter()

	// Add logging middleware
	r.Use(h.LoggingMiddleware)

	// API routes
	api := r.PathPrefix("/api/v1").Subrouter()

	// Register listing routes
	svc.RegisterRoutes(api)

	// Health check
	api.HandleFunc("/health", h.HealthCheck).Methods("GET")

	// Setup CORS for frontend integration
	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"}, // In production, specify your frontend domain
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"*"},
	})

	// Wrap router with CORS
	handler := c.Handler(r)
	port := s.config.Server.Port
	s.log.Info().Msgf("Starting server on port %s...", port)
	if err := http.ListenAndServe(":"+port, handler); err != nil {
		s.log.Error().Err(err).Msg("Server failed to start")
		return err
	}

	return nil
}

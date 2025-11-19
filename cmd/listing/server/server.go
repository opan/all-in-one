package server

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/all-in-one/internal/config"
	httpHelper "github.com/all-in-one/internal/http"
	"github.com/all-in-one/internal/listing/service"
	"github.com/rs/cors"
	"github.com/rs/zerolog"

	"github.com/gorilla/mux"
	httpSwagger "github.com/swaggo/http-swagger/v2"

	_ "github.com/all-in-one/docs" // Import generated docs
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
	// Create context for server lifetime
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.log.Info().Msg("Initiating server start...")

	svc, err := service.NewService(ctx, s.config, s.log)
	if err != nil {
		s.log.Error().Err(err).Msg("Failed to create listing service")
		return err
	}

	s.log.Info().Msg("Initializing sample data...")
	dataCount := svc.InitializeSampleData(ctx)
	s.log.Info().Msgf("Initialized %d sample data items", dataCount)

	// Initialize HTTP helper
	h := httpHelper.NewHTTP(s.log, s.config)

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

	// Swagger documentation
	s.log.Info().Msg("Register swagger...")
	r.PathPrefix("/swagger/").Handler(httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
		httpSwagger.DeepLinking(true),
		httpSwagger.DocExpansion("list"),
		httpSwagger.DomID("swagger-ui"),
	)).Methods("GET")

	// Setup CORS for frontend integration
	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"}, // In production, specify your frontend domain
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"*"},
	})

	// Wrap router with CORS
	handler := c.Handler(r)
	port := s.config.Server.Port

	// Create HTTP server
	httpServer := &http.Server{
		Addr:    ":" + port,
		Handler: handler,
	}

	// Channel to listen for errors coming from the server
	serverErrors := make(chan error, 1)

	// Start the server in a goroutine
	go func() {
		s.log.Info().Msgf("Starting server on port %s...", port)
		serverErrors <- httpServer.ListenAndServe()
	}()

	// Channel to listen for interrupt or terminate signals
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	// Block until we receive a signal or an error
	select {
	case err := <-serverErrors:
		if err != nil && err != http.ErrServerClosed {
			s.log.Error().Err(err).Msg("Server failed to start")
			return err
		}

	case sig := <-shutdown:
		s.log.Info().Msgf("Shutdown signal received: %v", sig)

		// Cancel the server context
		cancel()

		// Create a deadline for graceful shutdown
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		// Attempt graceful shutdown
		s.log.Info().Msg("Initiating graceful shutdown...")
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			s.log.Error().Err(err).Msg("Error during graceful shutdown, forcing close")
			httpServer.Close()
			return err
		}

		s.log.Info().Msg("Server shutdown completed successfully")
	}

	return nil
}

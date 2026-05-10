package server

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/all-in-one/internal/authnz/middleware"
	authnzSvc "github.com/all-in-one/internal/authnz/service"
	chatSvc "github.com/all-in-one/internal/chat/service"
	"github.com/all-in-one/internal/config"
	httpHelper "github.com/all-in-one/internal/http"
	listingSvc "github.com/all-in-one/internal/listing/service"
	shortenerSvc "github.com/all-in-one/internal/shortener/service"
	"github.com/all-in-one/internal/storage"
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

	s.log.Info().Msg("Initiating database connection...")
	store, err := storage.NewStorage(s.config)
	if err != nil {
		s.log.Error().Err(err).Msg("Database connection failed")
		return err
	}

	db := store.DB()
	s.log.Info().Msg("Running database migrations...")
	if err := store.MigrateUp(); err != nil {
		s.log.Error().Err(err).Msg("Database migration failed")
		return err
	}
	s.log.Info().Msg("Database connection established")

	asvc, err := authnzSvc.NewService(ctx, db, s.config, s.log)
	if err != nil {
		s.log.Error().Err(err).Msg("Failed to create authnz service")
		return err
	}

	lsvc, err := listingSvc.NewService(ctx, s.config, s.log)
	if err != nil {
		s.log.Error().Err(err).Msg("Failed to create listing service")
		return err
	}

	csvc, err := chatSvc.NewService(ctx, s.config, s.log)
	if err != nil {
		s.log.Error().Err(err).Msg("Failed to create chat service")
		return err
	}
	defer csvc.Close()

	ssvc, err := shortenerSvc.NewService(ctx, s.config, s.log)
	if err != nil {
		s.log.Error().Err(err).Msg("Failed to create shortener service")
		return err
	}
	defer ssvc.Close()

	// Initialize HTTP helper
	h := httpHelper.NewHTTP(s.log, s.config)

	// Initialize router
	r := mux.NewRouter()

	// Add logging middleware
	r.Use(h.LoggingMiddleware)

	// API routes
	api := r.PathPrefix("/api/v1").Subrouter()

	// Public routes (no authentication required)
	publicRoutes := api.NewRoute().Subrouter()
	lsvc.RegisterRoutes(publicRoutes)
	asvc.RegisterPublicRoutes(publicRoutes)
	ssvc.RegisterPublicRoutes(publicRoutes)

	// Authenticated routes (JWT required)
	jwtMiddleware := middleware.NewJWTMiddleware(s.config)
	authenticatedRoutes := api.NewRoute().Subrouter()
	authenticatedRoutes.Use(jwtMiddleware.JWTAuth)
	lsvc.RegisterAuthenticatedRoutes(authenticatedRoutes)
	asvc.RegisterAuthenticatedRoutes(authenticatedRoutes)
	csvc.RegisterAuthenticatedRoutes(authenticatedRoutes)
	ssvc.RegisterAuthenticatedRoutes(authenticatedRoutes)

	// Shortener public redirect: /r/{code} — lives outside /api/v1
	ssvc.Handler.RegisterRedirectRoute(r)

	// Health check (public)
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
	// Serve frontend static files — must be registered last as a catch-all.
	// Requests for known static assets are served directly; all other paths
	// fall back to index.html so SvelteKit's client-side router takes over.
	r.PathPrefix("/").Handler(spaFileServer("./web/build"))

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

// spaFileServer serves static files from dir. If the requested path does not
// exist on disk it falls back to index.html, allowing SvelteKit's client-side
// router to handle the route in the browser.
func spaFileServer(dir string) http.Handler {
	fs := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(dir, filepath.Clean("/"+r.URL.Path))
		if _, err := os.Stat(path); os.IsNotExist(err) {
			http.ServeFile(w, r, filepath.Join(dir, "index.html"))
			return
		}
		fs.ServeHTTP(w, r)
	})
}

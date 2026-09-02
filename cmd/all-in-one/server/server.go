package server

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/all-in-one/internal/authnz/middleware"
	authnzSvc "github.com/all-in-one/internal/authnz/service"
	chatSvc "github.com/all-in-one/internal/chat/service"
	"github.com/all-in-one/internal/config"
	dashboardSvc "github.com/all-in-one/internal/dashboard/service"
	httpHelper "github.com/all-in-one/internal/http"
	listingSvc "github.com/all-in-one/internal/listing/service"
	"github.com/all-in-one/internal/logging"
	"github.com/all-in-one/internal/observability"
	"github.com/all-in-one/internal/ratelimit"
	ratelimitSvc "github.com/all-in-one/internal/ratelimit/service"
	"github.com/all-in-one/internal/rbac"
	rbacMw "github.com/all-in-one/internal/rbac/middleware"
	rbacSvc "github.com/all-in-one/internal/rbac/service"
	shortenerSvc "github.com/all-in-one/internal/shortener/service"
	"github.com/all-in-one/internal/storage"
	"github.com/rs/cors"
	"github.com/rs/zerolog"

	"github.com/gorilla/mux"
	httpSwagger "github.com/swaggo/http-swagger/v2"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux"

	_ "github.com/all-in-one/docs"
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

	// Attach the logger so any code that pulls a logger from context (e.g.
	// logging.GetLoggerFromContext, used by RBAC bootstrap) logs to the real
	// output instead of silently falling back to a no-op logger — mirrors
	// what h.LoggingMiddleware does per-request (internal/http/http.go).
	ctx = context.WithValue(ctx, logging.LoggerKey, &s.log)

	s.log.Info().Msg("Initiating server start...")

	otelShutdown, err := observability.Init(ctx, s.config.Telemetry)
	if err != nil {
		if s.config.Telemetry.Enabled {
			s.log.Error().Err(err).Msg("OpenTelemetry initialization failed")
			return err
		}
		s.log.Warn().Err(err).Msg("OpenTelemetry initialization failed; continuing without telemetry")
	} else if s.config.Telemetry.Enabled {
		s.log.Info().
			Str("otlp_endpoint", s.config.Telemetry.OTLPEndpoint).
			Float64("sample_ratio", s.config.Telemetry.SampleRatio).
			Msg("OpenTelemetry tracing enabled")
	}
	defer func() {
		if err := otelShutdown(context.Background()); err != nil {
			s.log.Warn().Err(err).Msg("OpenTelemetry shutdown returned error")
		}
	}()

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

	rsvc, err := rbacSvc.NewService(ctx, db, s.config, s.log)
	if err != nil {
		s.log.Error().Err(err).Msg("Failed to create rbac service")
		return err
	}
	if err := rsvc.Bootstrap(ctx, asvc.Store.UserRepo()); err != nil {
		s.log.Error().Err(err).Msg("RBAC bootstrap failed")
		return err
	}
	asvc.Handler.SetAccessResolver(rsvc.Resolver)

	rlsvc, err := ratelimitSvc.NewService(ctx, db, s.config, s.log)
	if err != nil {
		s.log.Error().Err(err).Msg("Failed to create ratelimit service")
		return err
	}
	defer rlsvc.Close()

	lsvc, err := listingSvc.NewService(ctx, db, s.config, s.log)
	if err != nil {
		s.log.Error().Err(err).Msg("Failed to create listing service")
		return err
	}

	csvc, err := chatSvc.NewService(ctx, db, s.config, s.log)
	if err != nil {
		s.log.Error().Err(err).Msg("Failed to create chat service")
		return err
	}
	defer csvc.Close()

	ssvc, err := shortenerSvc.NewService(ctx, db, s.config, s.log)
	if err != nil {
		s.log.Error().Err(err).Msg("Failed to create shortener service")
		return err
	}
	defer ssvc.Close()

	// Dashboard is a pure aggregator over the app storages built above plus the
	// RBAC resolver — it opens no DB handle of its own and only reads.
	dsvc := dashboardSvc.NewService(lsvc.Storage, csvc.Storage, ssvc.Storage, rsvc.Resolver, s.log)

	// Initialize HTTP helper
	h := httpHelper.NewHTTP(s.log, s.config)

	// Initialize router
	r := mux.NewRouter()

	// OTel HTTP tracing — registered before the logging middleware so that
	// the per-request logger can pick up trace_id/span_id from the context
	// (added in phase 2). Static assets, health, and swagger are filtered
	// out to keep trace volume sane.
	r.Use(otelmux.Middleware(
		s.config.Telemetry.ServiceName,
		otelmux.WithFilter(shouldTrace),
	))

	// Add logging middleware
	r.Use(h.LoggingMiddleware)

	// API routes
	api := r.PathPrefix("/api/v1").Subrouter()

	// Shared rate limit enforcement middleware — one *middleware.Limiter (one
	// in-memory throttle store) reused across every subrouter it's attached
	// to (docs/adr/RATE_LIMITING_ADR.md ADR-004).
	rlMw := rlsvc.LimiterMiddleware()

	// Public routes (no authentication required)
	publicRoutes := api.NewRoute().Subrouter()
	publicRoutes.Use(rlMw)
	lsvc.RegisterRoutes(publicRoutes)
	asvc.RegisterPublicRoutes(publicRoutes)
	ssvc.RegisterPublicRoutes(publicRoutes)

	// Authenticated routes (JWT required), split into RBAC-gated siblings —
	// per-app subrouters are used instead of a single shared one because
	// routes are not cleanly path-prefixed (e.g. chat's /users/search vs
	// authnz's /users/me); see docs/adr/ACCESS_MANAGEMENT_ADR.md ADR-006.
	jwtMiddleware := middleware.NewJWTMiddleware(s.config, asvc.Store.SessionRepo())
	authz := rbacMw.NewAuthz(rsvc.Resolver, s.config)

	// authnz self-service (profile, password reset, 2FA, session management)
	// — authenticated but not feature-gated.
	selfRoutes := api.NewRoute().Subrouter()
	selfRoutes.Use(jwtMiddleware.JWTAuth)
	asvc.RegisterAuthenticatedRoutes(selfRoutes)
	// Home dashboard summary — authenticated but not feature-gated, so users
	// with a subset of features still get their accessible sections.
	dsvc.RegisterAuthenticatedRoutes(selfRoutes)

	// mkGated builds a subrouter gated by JWT auth plus the named feature.
	// rlMw runs right after JWTAuth so user-scoped targets can key by the
	// authenticated user id (auth.GetUserFromContext) — ADR-004.
	mkGated := func(feature string) *mux.Router {
		sr := api.NewRoute().Subrouter()
		sr.Use(jwtMiddleware.JWTAuth)
		sr.Use(rlMw)
		sr.Use(authz.RequireFeature(feature))
		return sr
	}
	lsvc.RegisterAuthenticatedRoutes(mkGated(rbac.FeatureListing))
	csvc.RegisterAuthenticatedRoutes(mkGated(rbac.FeatureChat))
	ssvc.RegisterAuthenticatedRoutes(mkGated(rbac.FeatureShortener))

	// Admin-only management APIs: RBAC under /api/v1/access/*, user management
	// under /api/v1/admin/users/*, shortener moderation under
	// /api/v1/admin/shortener/*, rate limit config under
	// /api/v1/ratelimit/*. All share the RequireAdmin subrouter.
	adminRoutes := api.NewRoute().Subrouter()
	adminRoutes.Use(jwtMiddleware.JWTAuth)
	adminRoutes.Use(authz.RequireAdmin)
	rsvc.RegisterAdminRoutes(adminRoutes)
	asvc.Handler.RegisterAdminRoutes(adminRoutes)
	ssvc.RegisterAdminRoutes(adminRoutes)
	rlsvc.RegisterAdminRoutes(adminRoutes)

	// Shortener public redirect: /r/{code} — lives outside /api/v1, on its
	// own subrouter carrying rlMw so the ratelimit app-feature enforces the
	// ip-scoped shortener.link.resolve target on it (ADR-011). Registered
	// before the boot-time validation below so that target's binding is seen.
	redirectRoutes := r.NewRoute().Subrouter()
	redirectRoutes.Use(rlMw)
	ssvc.Handler.RegisterRedirectRoute(redirectRoutes)

	// Boot-time drift protection: every rate limit Registry target must be
	// bound to a route that's actually registered, or enforcement for it
	// would silently no-op (e.g. after a route rename) — ADR-004. Runs after
	// every rate-limited route (incl. the redirect above) is registered.
	if missing := validateRateLimitBindings(r); len(missing) > 0 {
		s.log.Fatal().Strs("missing", missing).
			Msg("ratelimit: registry targets have no matching registered route — enforcement would silently no-op")
	} else {
		s.log.Info().Int("targets", len(ratelimit.Registered())).
			Msg("ratelimit: all targets bound to a registered route")
	}

	// Health check (public)
	api.HandleFunc("/health", h.HealthCheck).Methods("GET")

	// Public runtime config the SPA reads before authentication (demo-account
	// flag). Public and unauthenticated by design.
	api.HandleFunc("/config", h.PublicConfig).Methods("GET")

	if s.config.Server.SwaggerEnabled {
		s.log.Info().Msg("Register swagger...")
		r.PathPrefix("/swagger/").Handler(httpSwagger.Handler(
			httpSwagger.URL("/swagger/doc.json"),
			httpSwagger.DeepLinking(true),
			httpSwagger.DocExpansion("list"),
			httpSwagger.DomID("swagger-ui"),
		)).Methods("GET")
	}

	// Setup CORS — allowed origins configured via server.allowed_origins in config.yml
	allowedOrigins := s.config.Server.AllowedOrigins
	if len(allowedOrigins) == 0 {
		allowedOrigins = []string{"*"}
	}
	c := cors.New(cors.Options{
		AllowedOrigins: allowedOrigins,
		AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
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

// validateRateLimitBindings walks every registered mux route and returns a
// human-readable entry for each ratelimit.Registry target whose (method,
// path template) doesn't match an actually-registered route. Compares
// against the full template gorilla/mux exposes via GetPathTemplate
// (including the /api/v1 prefix and any {var} placeholders), matching
// exactly what the limiter middleware resolves at request time.
func validateRateLimitBindings(r *mux.Router) []string {
	registered := make(map[string]bool)
	r.Walk(func(route *mux.Route, router *mux.Router, ancestors []*mux.Route) error {
		methods, err := route.GetMethods()
		if err != nil {
			return nil // no Methods() matcher (e.g. the SPA catch-all) — not a rate limit target
		}
		tmpl, err := route.GetPathTemplate()
		if err != nil {
			return nil
		}
		for _, m := range methods {
			registered[m+" "+tmpl] = true
		}
		return nil
	})

	var missing []string
	for _, t := range ratelimit.Registered() {
		if !registered[t.Method+" "+t.Path] {
			missing = append(missing, t.Key+" ("+t.Method+" "+t.Path+")")
		}
	}
	return missing
}

// shouldTrace returns true for requests we want to record as spans.
// Filters out the health check and any non-API path (SPA assets, swagger UI),
// keeping traces focused on the REST API and the shortener redirect.
func shouldTrace(r *http.Request) bool {
	p := r.URL.Path
	if p == "/api/v1/health" {
		return false
	}
	return strings.HasPrefix(p, "/api/v1/") || strings.HasPrefix(p, "/r/")
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

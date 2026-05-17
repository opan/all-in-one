package handler

import (
	"net/http"
	"time"

	"github.com/all-in-one/internal/config"
	"github.com/all-in-one/internal/shortener/middleware"
	"github.com/all-in-one/internal/shortener/repository"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
)

type Handler struct {
	storage        repository.Storage
	config         config.Config
	log            zerolog.Logger
	rateLimiter    *middleware.RateLimiter
	resolveLimiter *middleware.RateLimiter
}

func NewHandler(storage repository.Storage, cfg config.Config, log zerolog.Logger) *Handler {
	rl := cfg.Shortener.RateLimit

	createLimit := rl.CreatesPerWindow
	if createLimit == 0 {
		createLimit = 100
	}
	createWindow := rl.WindowMinutes
	if createWindow == 0 {
		createWindow = 15
	}

	resolveLimit := rl.ResolvePerWindow
	if resolveLimit == 0 {
		resolveLimit = 300
	}
	resolveWindow := rl.ResolveWindowMinutes
	if resolveWindow == 0 {
		resolveWindow = 1
	}

	return &Handler{
		storage:        storage,
		config:         cfg,
		log:            log,
		rateLimiter:    middleware.NewRateLimiter(createLimit, time.Duration(createWindow)*time.Minute),
		resolveLimiter: middleware.NewRateLimiter(resolveLimit, time.Duration(resolveWindow)*time.Minute),
	}
}

func (h *Handler) RegisterPublicRoutes(router *mux.Router) {}

func (h *Handler) RegisterAuthenticatedRoutes(router *mux.Router) {
	router.Handle("/shortener/links", h.rateLimiter.Wrap(h.CreateShortLink)).Methods(http.MethodPost)
	router.HandleFunc("/shortener/links", h.ListShortLinks).Methods(http.MethodGet)
	router.HandleFunc("/shortener/links/{code}", h.GetShortLink).Methods(http.MethodGet)
	router.HandleFunc("/shortener/links/{code}", h.UpdateShortLink).Methods(http.MethodPatch)
	router.HandleFunc("/shortener/links/{code}", h.DeleteShortLink).Methods(http.MethodDelete)
}

func (h *Handler) RegisterRedirectRoute(router *mux.Router) {
	router.Handle("/r/{code}", h.resolveLimiter.WrapWithKey(h.ResolveShortLink, resolveKey)).Methods(http.MethodGet)
}

// resolveKey keys the resolve rate limiter by short code so each link has its own bucket.
func resolveKey(r *http.Request) string {
	return "resolve:" + mux.Vars(r)["code"]
}

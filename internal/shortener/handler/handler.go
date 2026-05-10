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
	storage     repository.Storage
	config      config.Config
	log         zerolog.Logger
	rateLimiter *middleware.RateLimiter
}

func NewHandler(storage repository.Storage, cfg config.Config, log zerolog.Logger) *Handler {
	rl := cfg.Shortener.RateLimit
	limit := rl.CreatesPerWindow
	if limit == 0 {
		limit = 100
	}
	windowMins := rl.WindowMinutes
	if windowMins == 0 {
		windowMins = 15
	}

	return &Handler{
		storage:     storage,
		config:      cfg,
		log:         log,
		rateLimiter: middleware.NewRateLimiter(limit, time.Duration(windowMins)*time.Minute),
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
	router.HandleFunc("/r/{code}", h.ResolveShortLink).Methods(http.MethodGet)
}

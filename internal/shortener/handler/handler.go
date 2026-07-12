package handler

import (
	"net/http"

	"github.com/all-in-one/internal/config"
	"github.com/all-in-one/internal/shortener/repository"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
)

type Handler struct {
	storage repository.Storage
	config  config.Config
	log     zerolog.Logger
	metrics *handlerMetrics
}

func NewHandler(storage repository.Storage, cfg config.Config, log zerolog.Logger) *Handler {
	return &Handler{
		storage: storage,
		config:  cfg,
		log:     log,
		metrics: newHandlerMetrics(),
	}
}

func (h *Handler) RegisterPublicRoutes(router *mux.Router) {}

func (h *Handler) RegisterAuthenticatedRoutes(router *mux.Router) {
	// Create is rate-limited by the ratelimit app-feature via the
	// shortener.link.create target — this subrouter already carries rlMw
	// (mkGated(FeatureShortener) in server.go). See ADR-011.
	router.HandleFunc("/shortener/links", h.CreateShortLink).Methods(http.MethodPost)
	router.HandleFunc("/shortener/links", h.ListShortLinks).Methods(http.MethodGet)
	router.HandleFunc("/shortener/links/{code}", h.GetShortLink).Methods(http.MethodGet)
	router.HandleFunc("/shortener/links/{code}", h.UpdateShortLink).Methods(http.MethodPatch)
	router.HandleFunc("/shortener/links/{code}", h.DeleteShortLink).Methods(http.MethodDelete)
}

func (h *Handler) RegisterRedirectRoute(router *mux.Router) {
	// Resolve is rate-limited by the ratelimit app-feature via the
	// shortener.link.resolve target (ip-scoped) — router must carry rlMw.
	// See ADR-011.
	router.HandleFunc("/r/{code}", h.ResolveShortLink).Methods(http.MethodGet)
}

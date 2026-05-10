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
}

func NewHandler(storage repository.Storage, config config.Config, log zerolog.Logger) *Handler {
	return &Handler{
		storage: storage,
		config:  config,
		log:     log,
	}
}

// RegisterPublicRoutes mounts routes that do not require authentication.
// The redirect route /r/{code} is registered on the root router in server.go.
func (h *Handler) RegisterPublicRoutes(router *mux.Router) {}

// RegisterAuthenticatedRoutes mounts management routes behind JWT auth.
func (h *Handler) RegisterAuthenticatedRoutes(router *mux.Router) {
	router.HandleFunc("/shortener/links", h.CreateShortLink).Methods(http.MethodPost)
	router.HandleFunc("/shortener/links", h.ListShortLinks).Methods(http.MethodGet)
	router.HandleFunc("/shortener/links/{code}", h.GetShortLink).Methods(http.MethodGet)
	router.HandleFunc("/shortener/links/{code}", h.UpdateShortLink).Methods(http.MethodPatch)
	router.HandleFunc("/shortener/links/{code}", h.DeleteShortLink).Methods(http.MethodDelete)
}

// RegisterRedirectRoute mounts the public redirect on the root router.
func (h *Handler) RegisterRedirectRoute(router *mux.Router) {
	router.HandleFunc("/r/{code}", h.ResolveShortLink).Methods(http.MethodGet)
}

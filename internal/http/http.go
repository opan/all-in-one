package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/all-in-one/internal/config"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// Common storage errors
var (
	ErrNotFound = errors.New("resource not found")
)

// Response is a standard API response structure
type Response struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

type HTTP struct {
	log    zerolog.Logger
	config config.Config
}

type requestID string

const requestIDKey requestID = "request_id"

func NewHTTP(log zerolog.Logger, config config.Config) *HTTP {
	return &HTTP{
		log:    log,
		config: config,
	}
}

func (h *HTTP) LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create context with request ID and timeout
		ctx := r.Context()
		reqID := generateRequestID()
		ctx = context.WithValue(ctx, requestIDKey, reqID)

		timeout := h.config.Http.Timeout
		ctx, cancel := context.WithTimeout(ctx, timeout*time.Second)
		defer cancel()

		// Create logger with request_id
		logger := h.log.With().Str("request_id", reqID).Logger()

		// Store logger in the context for downstream
		ctx = context.WithValue(ctx, "logger", &logger)

		h.log.Info().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("ip", r.RemoteAddr).
			Msg("Request started")

		next.ServeHTTP(w, r.WithContext(ctx))

		h.log.Info().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("ip", r.RemoteAddr).
			Dur("duration", time.Since(start)).
			Msg("Request completed")
	})
}

func (h *HTTP) HealthCheck(w http.ResponseWriter, r *http.Request) {
	h.log.Info().
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("ip", r.RemoteAddr).
		Msg("Health check requested")

	response := Response{
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

func generateRequestID() string {
	return uuid.NewString()
}

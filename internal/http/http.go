package http

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/all-in-one/internal/config"
	"github.com/all-in-one/internal/logging"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/trace"
)

// Common storage errors
var (
	ErrNotFound = errors.New("resource not found")
)

// Response is a standard API response structure
type Response struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
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

// statusResponseWriter wraps http.ResponseWriter to capture the response status code.
type statusResponseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *statusResponseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// Hijack implements http.Hijacker so that WebSocket upgrades work through this wrapper.
func (rw *statusResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := rw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("underlying ResponseWriter does not implement http.Hijacker")
	}
	return hijacker.Hijack()
}

func (h *HTTP) LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		ctx := r.Context()
		reqID := uuid.NewString()
		ctx = context.WithValue(ctx, requestIDKey, reqID)

		timeout := h.config.Http.Timeout
		ctx, cancel := context.WithTimeout(ctx, timeout*time.Second)
		defer cancel()

		// Build per-request logger with request_id; add trace/span IDs when
		// otelmux has already attached a span to the context (registered before
		// this middleware in server.go).
		logCtx := h.log.With().Str("request_id", reqID)
		if spanCtx := trace.SpanContextFromContext(ctx); spanCtx.IsValid() {
			logCtx = logCtx.
				Str("trace_id", spanCtx.TraceID().String()).
				Str("span_id", spanCtx.SpanID().String())
		}
		logger := logCtx.Logger()

		ctx = context.WithValue(ctx, logging.LoggerKey, &logger)

		srw := &statusResponseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(srw, r.WithContext(ctx))

		logger.Info().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("ip", r.RemoteAddr).
			Int("status_code", srw.status).
			Dur("duration_ms", time.Since(start)).
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

// SendJSON sends a JSON response
func SendJSON(w http.ResponseWriter, data interface{}, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// SendError sends an error response
func SendError(w http.ResponseWriter, message string, statusCode int) {
	response := Response{
		Success: false,
		Error:   message,
	}
	SendJSON(w, response, statusCode)
}

func SendUnauthorized(w http.ResponseWriter, message string) {
	response := Response{
		Success: false,
		Error:   message,
	}
	SendJSON(w, response, http.StatusUnauthorized)
}

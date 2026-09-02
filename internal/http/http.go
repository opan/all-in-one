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

// demoModePublic is the client-facing view of the demo_mode flag. Credentials
// are included only when the flag is enabled so a disabled demo never leaks a
// username/password to callers.
type demoModePublic struct {
	Enabled  bool   `json:"enabled"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// PublicConfig godoc
// @Summary      Public runtime config
// @Description  Client-facing configuration the SPA needs before authentication (currently the demo-account flag). No secrets beyond the intentionally public demo credentials.
// @Tags         config
// @Produce      json
// @Success      200  {object}  Response  "Public config"
// @Router       /config [get]
func (h *HTTP) PublicConfig(w http.ResponseWriter, r *http.Request) {
	demo := demoModePublic{Enabled: h.config.DemoMode.Enabled}
	if demo.Enabled {
		demo.Username = h.config.DemoMode.Username
		demo.Password = h.config.DemoMode.Password
	}

	response := Response{
		Success: true,
		Data: map[string]any{
			"demo_mode": demo,
		},
	}

	SendJSON(w, response, http.StatusOK)
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

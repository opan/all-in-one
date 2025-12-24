package http

import (
	"context"
	"fmt"
	"net/http"

	"github.com/all-in-one/internal/logging"
	jwt "github.com/golang-jwt/jwt/v5"
)

type contextKey string

const (
	UserContextKey contextKey = "user"
)

type UserClaims struct {
	SessionID string `json:"sub"`
	Email     string `json:"email"`
	Username  string `json:"username"`
	UserID    string `json:"user_id"`
}

func (h *HTTP) JWTAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		log := logging.GetLoggerFromContext(ctx)

		if userClaims, ok := h.tryDirectAuth(ctx, r); ok {
			ctx = context.WithValue(ctx, UserContextKey, userClaims)

			log.Info().Str("username", userClaims.Username).Msg("direct authentication bypass used")
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		userClaims, err := h.validateJWT(ctx, r)
		if err != nil {
			log.Error().Err(err).Str("username", userClaims.Username).Msg("unauthorized access")
			SendUnauthorized(w, err.Error())
			return
		}

		ctx = context.WithValue(ctx, UserContextKey, userClaims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *HTTP) tryDirectAuth(ctx context.Context, r *http.Request) (UserClaims, bool) {
	log := logging.GetLoggerFromContext(ctx)

	if !h.config.Auth.DirectAuthEnabled {
		return UserClaims{}, false
	}

	username := r.Header.Get("x-direct-auth-username")
	if username == "" {
		log.Warn().Msg("empty username in the header for direct auth method")
		return UserClaims{}, false
	}

	return UserClaims{
		SessionID: "direct-auth",
		Email:     username,
	}, true
}

func (h *HTTP) validateJWT(ctx context.Context, r *http.Request) (UserClaims, error) {
	log := logging.GetLoggerFromContext(ctx)

	cookie, err := r.Cookie("access_token")
	if err != nil {
		log.Error().Err(err).Msg("cannot find JWT token")
		return UserClaims{}, fmt.Errorf("Unauthorized: missing or invalid token: %v", err)
	}

	token, err := jwt.Parse(cookie.Value, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			log.Error().Err(jwt.ErrSignatureInvalid).Msg("invalid JWT signature")
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(h.config.Auth.JWTSecret), nil
	})

	if err != nil {
		log.Error().Err(err).Msg("failed to parse JWT token")
		return UserClaims{}, fmt.Errorf("Unauthorized: invalid token: %v", err)
	}

	if !token.Valid {
		log.Error().Err(err).Msg("invalid JWT token")
		return UserClaims{}, fmt.Errorf("Unauthorized: invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		log.Error().Msg("invalid JWT claims")
		return UserClaims{}, fmt.Errorf("Unauthorized: invalid token claims")
	}

	return UserClaims{
		SessionID: claims["sub"].(string),
		Email:     claims["email"].(string),
	}, nil
}

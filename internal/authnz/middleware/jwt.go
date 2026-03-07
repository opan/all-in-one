package middleware

import (
	"context"
	"fmt"
	"net/http"

	"github.com/all-in-one/internal/auth"
	"github.com/all-in-one/internal/config"
	"github.com/all-in-one/internal/logging"
	jwt "github.com/golang-jwt/jwt/v5"
)

type JWTMiddleware struct {
	config config.Config
}

func NewJWTMiddleware(config config.Config) *JWTMiddleware {
	return &JWTMiddleware{
		config: config,
	}
}

func (m *JWTMiddleware) JWTAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		log := logging.GetLoggerFromContext(ctx)

		if userClaims, ok := m.tryDirectAuth(ctx, r); ok {
			ctx = context.WithValue(ctx, auth.UserContextKey, userClaims)

			log.Info().Str("username", userClaims.Username).Msg("direct authentication bypass used")
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		userClaims, err := m.validateJWT(ctx, r)
		if err != nil {
			log.Error().Err(err).Str("username", userClaims.Username).Msg("unauthorized access")
			sendUnauthorized(w, err.Error())
			return
		}

		ctx = context.WithValue(ctx, auth.UserContextKey, userClaims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (m *JWTMiddleware) tryDirectAuth(ctx context.Context, r *http.Request) (auth.UserClaims, bool) {
	log := logging.GetLoggerFromContext(ctx)

	if !m.config.Auth.DirectAuthEnabled {
		return auth.UserClaims{}, false
	}

	username := r.Header.Get("x-direct-auth-username")
	if username == "" {
		log.Warn().Msg("empty username in the header for direct auth method")
		return auth.UserClaims{}, false
	}

	return auth.UserClaims{
		SessionID: "direct-auth",
		Email:     username,
		// TODO: Use a better way to generate user IDs for direct auth
		UserID:   username,
		Username: username,
	}, true
}

func (m *JWTMiddleware) validateJWT(ctx context.Context, r *http.Request) (auth.UserClaims, error) {
	log := logging.GetLoggerFromContext(ctx)

	// Try to get token from cookie first
	var tokenString string
	cookie, err := r.Cookie("access_token")
	if err != nil {
		// If cookie not found, try query parameter (for WebSocket connections)
		tokenString = r.URL.Query().Get("token")
		if tokenString == "" {
			log.Error().Err(err).Msg("cannot find JWT token in cookie or query")
			return auth.UserClaims{}, fmt.Errorf("Unauthorized: missing or invalid token: %v", err)
		}
	} else {
		tokenString = cookie.Value
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			log.Error().Err(jwt.ErrSignatureInvalid).Msg("invalid JWT signature")
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(m.config.Auth.JWTSecret), nil
	})

	if err != nil {
		log.Error().Err(err).Msg("failed to parse JWT token")
		return auth.UserClaims{}, fmt.Errorf("Unauthorized: invalid token: %v", err)
	}

	if !token.Valid {
		log.Error().Err(err).Msg("invalid JWT token")
		return auth.UserClaims{}, fmt.Errorf("Unauthorized: invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		log.Error().Msg("invalid JWT claims")
		return auth.UserClaims{}, fmt.Errorf("Unauthorized: invalid token claims")
	}

	return auth.UserClaims{
		SessionID: claims["sub"].(string),
		Email:     claims["email"].(string),
		UserID:    claims["user_id"].(string),
		Username:  claims["username"].(string),
	}, nil
}

func sendUnauthorized(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	fmt.Fprintf(w, `{"success":false,"message":null,"data":null,"error":"%s"}`, message)
}

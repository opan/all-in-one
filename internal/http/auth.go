package http

import (
	"context"
	"fmt"
	"net/http"

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

		if userClaims, ok := h.tryDirectAuth(ctx, r); ok {
			ctx = context.WithValue(ctx, UserContextKey, userClaims)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		userClaims, err := h.validateJWT(ctx, r)
		if err != nil {
			SendUnauthorized(w, err.Error())
			return
		}

		ctx = context.WithValue(ctx, UserContextKey, userClaims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *HTTP) tryDirectAuth(ctx context.Context, r *http.Request) (UserClaims, bool) {
	if !h.config.Auth.DirectAuthEnabled {
		return UserClaims{}, false
	}

	username := r.Header.Get("x-direct-auth-username")
	if username == "" {
		return UserClaims{}, false
	}

	h.log.Warn().
		Ctx(ctx).
		Str("username", username).
		Msg("direct authentication bypass used")

	return UserClaims{
		SessionID: "direct-auth",
		Email:     username,
	}, true
}

func (h *HTTP) validateJWT(ctx context.Context, r *http.Request) (UserClaims, error) {
	cookie, err := r.Cookie("access_token")
	if err != nil {
		h.log.Error().Ctx(ctx).Msg("cannot find JWT token")
		return UserClaims{}, fmt.Errorf("Unauthorized: missing or invalid token: %v", err)
	}

	token, err := jwt.Parse(cookie.Value, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(h.config.Auth.JWTSecret), nil
	})

	if err != nil {
		h.log.Error().Ctx(ctx).Msg("failed to parse JWT token")
		return UserClaims{}, fmt.Errorf("Unauthorized: invalid token: %v", err)
	}

	if !token.Valid {
		h.log.Error().Ctx(ctx).Msg("invalid JWT token")
		return UserClaims{}, fmt.Errorf("Unauthorized: invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		h.log.Error().Ctx(ctx).Msg("invalid JWT claims")
		return UserClaims{}, fmt.Errorf("Unauthorized: invalid token claims")
	}

	return UserClaims{
		SessionID: claims["sub"].(string),
		Email:     claims["email"].(string),
	}, nil
}

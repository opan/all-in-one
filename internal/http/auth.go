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
}

func (h *HTTP) JWTAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		cookie, err := r.Cookie("jwt_token")
		if err != nil {
			h.log.Error().Ctx(ctx).Msg("cannot find JWT token")
			SendUnauthorized(w, fmt.Sprintf("Unauthorized: missing or invalid token: %v", err))
			return
		}

		tokenStr := cookie.Value
		token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}

			return []byte("your-secret-key"), nil
		})

		if err != nil {
			h.log.Error().Ctx(ctx).Msg("failed to parse JWT token")
			SendUnauthorized(w, fmt.Sprintf("Unauthorized: invalid token: %v", err))
			return
		}

		if !token.Valid {
			h.log.Error().Ctx(ctx).Msg("invalid JWT token")
			SendUnauthorized(w, "Unauthorized: invalid token")
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			h.log.Error().Ctx(ctx).Msg("invalid JWT claims")
			SendUnauthorized(w, "Unauthorized: invalid token claims")
			return
		}

		uc := UserClaims{
			SessionID: claims["sub"].(string),
			Email:     claims["email"].(string),
		}

		ctx = context.WithValue(ctx, UserContextKey, uc)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetUserClaims(ctx context.Context) (UserClaims, bool) {
	uc, ok := ctx.Value(UserContextKey).(UserClaims)
	return uc, ok
}

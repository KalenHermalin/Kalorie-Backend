package middlewares

import (
	"context"
	"main/internal/apperrors"
	"main/internal/models"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const (
	UserIDKey    contextKey = "userID"
	IsPremiumKey contextKey = "isPremium"
)

func Auth(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 1. Get the Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				apperrors.WriteError(w, *apperrors.ErrUnauthoirized)
				return
			}

			// 2. Parse the Bearer token
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				apperrors.WriteError(w, *apperrors.ErrInvalidTokenFormat)
				return
			}

			// 3. Validate the token
			var claims models.CustomClaimsAccess
			token, err := jwt.ParseWithClaims(parts[1], &claims, func(token *jwt.Token) (any, error) {
				return []byte(jwtSecret), nil
			})
			if err != nil || !token.Valid {
				apperrors.WriteError(w, *apperrors.ErrInvalidToken)
				return
			}

			// 4. Inject UserID into Context for your handlers
			ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
			ctx = context.WithValue(ctx, IsPremiumKey, claims.IsPremium) // Chain the context
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
func GetUserID(ctx context.Context) int {
	id, _ := ctx.Value(UserIDKey).(int)
	return id
}

func GetIsPremium(ctx context.Context) bool {
	premium, _ := ctx.Value(IsPremiumKey).(bool)
	return premium
}

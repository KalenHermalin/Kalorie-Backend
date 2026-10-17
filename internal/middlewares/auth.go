package middlewares

import (
	"context"
	"errors"
	"main/internal/apperrors"
	"main/internal/loggerx"
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
			log := loggerx.Log(r.Context())
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				kerr := apperrors.New(apperrors.Unauthorized, nil)
				apperrors.Write(w, kerr)
				log.Warn("no Auth header", "code", kerr.Kind.Code)
				return
			}

			// 2. Parse the Bearer token
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				kerr := apperrors.New(apperrors.MalformedAuthHeader, nil)
				apperrors.Write(w, kerr)
				log.Warn("malformed auth header", "code", kerr.Kind.Code)
				return
			}

			// 3. Validate the token
			var claims models.CustomClaimsAccess
			token, err := jwt.ParseWithClaims(parts[1], &claims, func(token *jwt.Token) (any, error) {
				return []byte(jwtSecret), nil
			})
			if err != nil || !token.Valid {
				kerr := apperrors.New(apperrors.InvalidToken, err)
				if errors.Is(err, jwt.ErrTokenExpired) {
					log.Info("token expired", "code", kerr.Kind.Code, "err", err)
				} else {
					log.Warn("token rejected", "code", kerr.Kind.Code, "err", err)
				}
				apperrors.Write(w, kerr)
				return
			}

			// 4. Inject UserID into Context for your handlers
			ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
			ctx = context.WithValue(ctx, IsPremiumKey, claims.IsPremium) // Chain the context
			new_logger := log.With("user_id", claims.UserID)
			ctx = loggerx.With(ctx, new_logger)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
func GetUserID(ctx context.Context) string {
	id, _ := ctx.Value(UserIDKey).(string)
	return id
}

func GetIsPremium(ctx context.Context) bool {
	premium, _ := ctx.Value(IsPremiumKey).(bool)
	return premium
}

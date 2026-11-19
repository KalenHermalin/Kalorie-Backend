package middlewares

import (
	"context"
	"errors"
	"main/internal/apperrors"
	"main/internal/loggerx"
	"main/internal/models"
	"net/http"
	"strings"
	"time"

	"firebase.google.com/go/v4/auth"
)

type contextKey string

const (
	UserIDKey    contextKey = "userID"
	IsPremiumKey contextKey = "isPremium"
)

func Auth(client models.AuthClient, userLookup models.AuthLookUp) func(http.Handler) http.Handler {
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

			// 3. Validate firebase token
			ctx, cancel := context.WithTimeout(r.Context(), time.Second*2)

			token, err := client.VerifyIDToken(ctx, parts[1])

			if err != nil {
				kerr := apperrors.New(apperrors.InvalidToken, nil)
				apperrors.Write(w, kerr)
				if auth.IsIDTokenExpired(err) {
					log.Info("Token was expired", "error", err, "code", kerr.Kind.Code)
				} else {
					log.Warn("Token was invalid", "error", err, "code", kerr.Kind.Code)
				}
				cancel()
				return

			}
			cancel()
			ctx, cancel = context.WithTimeout(r.Context(), time.Second*5)
			defer cancel()
			userId, err := userLookup.FirebaseToUUID(ctx, token.UID)
			if err != nil {
				var kerr *apperrors.KError
				if !errors.As(err, &kerr) { // Search error chain and fills kerr with a KError found in the chain
					kerr = apperrors.New(apperrors.Internal, err)
				}
				if kerr.Kind.Status >= 500 {
					log.Error("error looking up firebase id", "code", kerr.Kind.Code, "err", err)
				} else {
					log.Warn("no firebaseId in database", "code", kerr.Kind.Code, "err", err)
				}
				apperrors.Write(w, kerr)
				return
			}
			// 4. Inject UserID into Context for your handlers
			ctx = context.WithValue(r.Context(), UserIDKey, userId)
			new_logger := log.With("user_id", userId)
			ctx = loggerx.With(ctx, new_logger)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
func GetUserID(ctx context.Context) string {
	id, _ := ctx.Value(UserIDKey).(string)
	return id
}

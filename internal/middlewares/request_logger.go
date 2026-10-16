package middlewares

import "net/http"

func RequestLogger() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
	}
}

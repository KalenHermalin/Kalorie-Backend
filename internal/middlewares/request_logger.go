package middlewares

import (
	"log/slog"
	"main/internal/loggerx"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

func RequestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := middleware.GetReqID(r.Context())
			reqLog := logger.With("request_id", id, "path", r.URL.Path, "method", r.Method)
			ctx := loggerx.With(r.Context(), reqLog)
			startTime := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r.WithContext(ctx))
			elapsed := time.Since(startTime)
			reqLog.Info("request_completed", "status", ww.Status(), "duration_ms", elapsed.Milliseconds())

		})
	}
}

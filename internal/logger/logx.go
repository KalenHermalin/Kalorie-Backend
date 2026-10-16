package logger

import (
	"context"
	"log/slog"
)

type ctxKey struct{}

func With(ctx context.Context, slogger *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, slogger)
}

func Log(ctx context.Context) *slog.Logger {
	logger, ok := ctx.Value(ctxKey{}).(*slog.Logger)
	if !ok || logger == nil {
		return slog.Default()
	}
	return logger
}

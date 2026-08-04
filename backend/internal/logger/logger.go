// Package logger настраивает структурированное логирование (MNT6).
//
// В логи не должны попадать пароли, токены и тела запросов аутентификации
// (SEC7): логируйте идентификаторы, а не полезную нагрузку.
package logger

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/config"
)

type contextKey struct{}

var requestIDKey contextKey

func New(cfg config.LogConfig) *slog.Logger {
	options := &slog.HandlerOptions{Level: parseLevel(cfg.Level)}

	var handler slog.Handler
	if strings.EqualFold(cfg.Format, "text") {
		handler = slog.NewTextHandler(os.Stdout, options)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, options)
	}

	return slog.New(handler)
}

func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

func RequestID(ctx context.Context) string {
	value, _ := ctx.Value(requestIDKey).(string)
	return value
}

// FromContext возвращает логгер с подставленным request_id.
func FromContext(ctx context.Context, base *slog.Logger) *slog.Logger {
	if base == nil {
		base = slog.Default()
	}

	if id := RequestID(ctx); id != "" {
		return base.With(slog.String("request_id", id))
	}

	return base
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

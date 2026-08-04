// Package postgres отвечает за подключение к БД и применение миграций.
package postgres

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/config"
)

// Connect открывает пул и дожидается готовности БД: в docker compose
// контейнер БД может быть ещё не готов принимать соединения.
func Connect(ctx context.Context, cfg config.PostgresConfig, log *slog.Logger) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("разбор строки подключения: %w", err)
	}

	poolConfig.MaxConns = cfg.MaxConns
	poolConfig.ConnConfig.ConnectTimeout = cfg.ConnectTimeout

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("создание пула соединений: %w", err)
	}

	if err := ping(ctx, pool, cfg.ConnectRetries, log); err != nil {
		pool.Close()
		return nil, err
	}

	return pool, nil
}

func ping(ctx context.Context, pool *pgxpool.Pool, retries int, log *slog.Logger) error {
	var lastErr error

	for attempt := 1; attempt <= retries; attempt++ {
		err := pool.Ping(ctx)
		if err == nil {
			return nil
		}

		lastErr = err

		log.Warn("база данных ещё не готова, повтор подключения",
			slog.Int("attempt", attempt),
			slog.Int("retries", retries),
		)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}

	return fmt.Errorf("база данных недоступна после %d попыток: %w", retries, lastErr)
}

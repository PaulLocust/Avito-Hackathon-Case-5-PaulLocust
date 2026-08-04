// Command api — HTTP-сервис «Антискам тренажёр».
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/config"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/logger"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/repository"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/service"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/storage/postgres"
	httptransport "github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/transport/http"
)

// version подставляется линковщиком: -ldflags "-X main.version=..."
var version = "dev"

func main() {
	if err := run(); err != nil {
		slog.Error("сервис остановлен с ошибкой", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	log := logger.New(cfg.Log)
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Info("запуск сервиса",
		slog.String("version", version),
		slog.String("env", cfg.Env),
	)

	// Трафик не принимается до готовности БД и применения миграций (REL1).
	pool, err := postgres.Connect(ctx, cfg.Postgres, log)
	if err != nil {
		return fmt.Errorf("подключение к базе данных: %w", err)
	}
	defer pool.Close()

	if migrateErr := postgres.Migrate(ctx, cfg.Postgres); migrateErr != nil {
		return migrateErr
	}

	log.Info("миграции применены")

	repos := repository.New(pool)
	services := service.New(repos, cfg)
	handler := httptransport.NewHandler(services, cfg, log, pool, version)

	server := &http.Server{
		Addr:              net.JoinHostPort("", fmt.Sprintf("%d", cfg.HTTP.Port)),
		Handler:           httptransport.NewRouter(handler, cfg, log),
		ReadHeaderTimeout: cfg.HTTP.ReadTimeout,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
	}

	serverErrors := make(chan error, 1)

	go func() {
		log.Info("http-сервер слушает", slog.Int("port", cfg.HTTP.Port))

		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()

	select {
	case err := <-serverErrors:
		return fmt.Errorf("http-сервер: %w", err)

	case <-ctx.Done():
		log.Info("получен сигнал остановки, завершаем текущие запросы")
	}

	// Сервер дожидается текущих запросов, но не дольше таймаута (REL4).
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("остановка http-сервера: %w", err)
	}

	log.Info("сервис остановлен")

	return nil
}

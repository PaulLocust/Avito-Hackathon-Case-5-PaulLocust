// Command seed применяет миграции и загружает контент в БД (make seed).
// Запуск идемпотентен: на неизменном контенте новых версий не создаётся.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/config"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/logger"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/repository"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/service"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/storage/postgres"
)

func main() {
	if err := run(); err != nil {
		slog.Error("загрузка контента не выполнена", slog.String("error", err.Error()))
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

	pool, err := postgres.Connect(ctx, cfg.Postgres, log)
	if err != nil {
		return fmt.Errorf("подключение к базе данных: %w", err)
	}
	defer pool.Close()

	if migrateErr := postgres.Migrate(ctx, cfg.Postgres); migrateErr != nil {
		return migrateErr
	}

	repos := repository.New(pool)
	services := service.New(repos, cfg)

	report, err := services.Content.LoadFromDir(ctx, cfg.Content.SeedDir)
	if err != nil {
		return err
	}

	printReport(log, report)

	// Сценарий, не прошедший валидацию, не должен молча выпасть из витрины.
	if report.Failed() {
		return fmt.Errorf("контент не прошёл валидацию: файлов с ошибками — %d", len(report.Issues))
	}

	return nil
}

func printReport(log *slog.Logger, report service.LoadReport) {
	log.Info("загрузка контента завершена",
		slog.Int("signals", report.SignalsLoaded),
		slog.Int("created", len(report.ScenariosCreated)),
		slog.Int("updated", len(report.ScenariosUpdated)),
		slog.Int("skipped", len(report.ScenariosSkipped)),
	)

	for file, issues := range report.Issues {
		for _, issue := range issues {
			log.Error("нарушение структуры сценария",
				slog.String("file", file),
				slog.String("path", issue.Path),
				slog.String("message", issue.Message),
			)
		}
	}
}

package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/domain"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/repository"
)

type catalogService struct {
	scenarios  repository.ScenarioRepository
	progress   repository.ProgressRepository
	sessions   repository.SessionRepository
	thresholds domain.Thresholds
}

var _ CatalogService = (*catalogService)(nil)

// List возвращает витрину для выбранной роли. Гостю карточки отдаются без
// блока прогресса (FR4), авторизованному — со статистикой по каждому
// сценарию (FR6, FR7), которая забирается одним запросом на весь список.
func (s *catalogService) List(
	ctx context.Context,
	userID *uuid.UUID,
	role *domain.Role,
) ([]domain.ScenarioCard, error) {
	scenarios, err := s.scenarios.ListActive(ctx, role)
	if err != nil {
		return nil, fmt.Errorf("список сценариев: %w", err)
	}

	stats, err := s.scenarioStats(ctx, userID)
	if err != nil {
		return nil, err
	}

	cards := make([]domain.ScenarioCard, 0, len(scenarios))
	for _, scenario := range scenarios {
		cards = append(cards, s.card(scenario, stats))
	}

	return cards, nil
}

// Get отдаёт карточку сценария для экрана подтверждения старта. Для
// авторизованного пользователя дополнительно проставляется незавершённая
// сессия, чтобы экран предложил продолжить, а не начать заново (FR12).
func (s *catalogService) Get(
	ctx context.Context,
	userID *uuid.UUID,
	code string,
) (domain.ScenarioCard, error) {
	scenario, err := s.scenarios.GetActiveByCode(ctx, code)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.ScenarioCard{}, err
		}

		return domain.ScenarioCard{}, fmt.Errorf("сценарий %s: %w", code, err)
	}

	// Шаги нужны движку прохождения, а не витрине: карточка не должна
	// раскрывать содержание диалога до старта.
	scenario.Steps = nil

	stats, err := s.scenarioStats(ctx, userID)
	if err != nil {
		return domain.ScenarioCard{}, err
	}

	card := s.card(scenario, stats)

	if userID != nil {
		active, err := s.sessions.GetActiveByOwnerScenario(ctx, domain.UserOwner(*userID), code)

		switch {
		case err == nil:
			sessionID := active.ID
			card.ActiveSessionID = &sessionID
		case errors.Is(err, domain.ErrNotFound):
			// Незавершённой сессии нет — обычная ситуация.
		default:
			return domain.ScenarioCard{}, fmt.Errorf("активная сессия по сценарию %s: %w", code, err)
		}
	}

	return card, nil
}

// scenarioStats возвращает статистику пользователя по сценариям; для гостя —
// nil, и карточки собираются без прогресса.
func (s *catalogService) scenarioStats(
	ctx context.Context,
	userID *uuid.UUID,
) (map[string]domain.UserScenarioStats, error) {
	if userID == nil {
		return nil, nil
	}

	stats, err := s.progress.ScenarioStats(ctx, *userID)
	if err != nil {
		return nil, fmt.Errorf("статистика по сценариям: %w", err)
	}

	return stats, nil
}

// card собирает карточку и досчитывает процент и уровень лучшей попытки:
// репозиторий отдаёт сырой агрегат, пороги приходят из конфигурации (FR21).
func (s *catalogService) card(
	scenario domain.Scenario,
	stats map[string]domain.UserScenarioStats,
) domain.ScenarioCard {
	card := domain.ScenarioCard{Scenario: scenario}

	if stats == nil {
		return card
	}

	// Сценарий без попыток тоже получает блок прогресса: авторизованному
	// пользователю карточка сообщает «не проходили», а не молчит.
	scenarioStats := stats[scenario.Code]

	if scenarioStats.Attempted {
		result := domain.EvaluateTotals(scenarioStats.BestScore, scenarioStats.BestAnswers, s.thresholds)
		percent, level := result.Percent, result.Level
		scenarioStats.BestPercent = &percent
		scenarioStats.BestLevel = &level
	}

	card.Stats = &scenarioStats

	return card
}

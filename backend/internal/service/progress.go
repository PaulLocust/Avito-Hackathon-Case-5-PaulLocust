package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/domain"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/repository"
)

// progressService принимает domain.Owner: репозиторий умеет считать данные
// и по гостю (guest_session_id), но кто реально сюда попадёт — решает
// вызывающий транспортный слой. HTTP-хендлеры (handler_progress.go,
// handler_catalog.go) висят за requireAuth и передают только
// domain.UserOwner — до регистрации гость свой прогресс не видит, хотя он
// уже копится в БД и переносится на аккаунт при ClaimGuest.
type progressService struct {
	progress   repository.ProgressRepository
	sessions   repository.SessionRepository
	scenarios  repository.ScenarioRepository
	signals    repository.RiskSignalRepository
	thresholds domain.Thresholds
}

var _ ProgressService = (*progressService)(nil)

// Overview собирает верхнюю часть главной: сколько сценариев пройдено,
// лучший/средний результат, динамику последней попытки, незавершённую
// тренировку и предложение следующего шага (FR12, FR25, FR27).
func (s *progressService) Overview(ctx context.Context, owner domain.Owner) (domain.Progress, error) {
	attempts, err := s.progress.ScoredAttempts(ctx, owner)
	if err != nil {
		return domain.Progress{}, fmt.Errorf("попытки владельца: %w", err)
	}

	total, err := s.scenarios.CountActive(ctx)
	if err != nil {
		return domain.Progress{}, fmt.Errorf("число активных сценариев: %w", err)
	}

	progress := domain.Progress{TotalScenarios: total, AttemptsCount: len(attempts)}

	if len(attempts) > 0 {
		completed := make(map[string]struct{})
		bestPercent, sumPercent := -1, 0
		var bestLevel domain.SecurityLevel

		for _, attempt := range attempts {
			completed[attempt.ScenarioCode] = struct{}{}

			result := domain.EvaluateTotals(attempt.Score, attempt.AnswersCount, s.thresholds)
			sumPercent += result.Percent

			if result.Percent > bestPercent {
				bestPercent, bestLevel = result.Percent, result.Level
			}
		}

		progress.CompletedScenarios = len(completed)

		average := sumPercent / len(attempts)
		progress.BestPercent = &bestPercent
		progress.BestLevel = &bestLevel
		progress.AveragePercent = &average
		progress.LastDeltaPercent = lastDeltaPercent(attempts, s.thresholds)
	}

	active, err := s.activeSession(ctx, owner)
	if err != nil {
		return domain.Progress{}, err
	}
	progress.ActiveSession = active

	next, err := s.suggestNextStep(ctx, attempts)
	if err != nil {
		return domain.Progress{}, err
	}
	progress.NextStep = next

	return progress, nil
}

// lastDeltaPercent — динамика последней попытки относительно предыдущей ПО
// ТОМУ ЖЕ сценарию (FR23, распространено на общий прогресс). Первой
// попытке по сценарию сравнивать не с чем — nil.
func lastDeltaPercent(attempts []domain.ScoredAttempt, thresholds domain.Thresholds) *int {
	if len(attempts) == 0 {
		return nil
	}

	last := attempts[len(attempts)-1]
	lastPercent := domain.EvaluateTotals(last.Score, last.AnswersCount, thresholds).Percent

	for i := len(attempts) - 2; i >= 0; i-- {
		if attempts[i].ScenarioCode != last.ScenarioCode {
			continue
		}

		delta := lastPercent - domain.EvaluateTotals(attempts[i].Score, attempts[i].AnswersCount, thresholds).Percent
		return &delta
	}

	return nil
}

func (s *progressService) activeSession(ctx context.Context, owner domain.Owner) (*domain.ActiveSession, error) {
	session, err := s.sessions.GetActiveByOwner(ctx, owner)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("активная сессия владельца: %w", err)
	}

	scenario, err := s.scenarios.GetByCodeVersion(ctx, session.ScenarioCode, session.ScenarioVersion)
	if err != nil {
		return nil, fmt.Errorf("сценарий активной сессии %s: %w", session.ScenarioCode, err)
	}

	answers, err := s.sessions.ListAnswers(ctx, session.ID)
	if err != nil {
		return nil, fmt.Errorf("ответы активной сессии: %w", err)
	}

	return &domain.ActiveSession{
		SessionID:    session.ID,
		Scenario:     scenario,
		AnswersCount: len(answers),
		StepsTotal:   scenario.StepsCount,
		StartedAt:    session.StartedAt,
	}, nil
}

// suggestNextStep — общий вариант той же логики, что в trainingService
// (FR27): сначала непройденный сценарий, иначе — повтор при слабом
// последнем результате, иначе — «всё пройдено».
func (s *progressService) suggestNextStep(ctx context.Context, attempts []domain.ScoredAttempt) (domain.NextStep, error) {
	completed := make(map[string]struct{}, len(attempts))
	for _, attempt := range attempts {
		completed[attempt.ScenarioCode] = struct{}{}
	}

	active, err := s.scenarios.ListActive(ctx, nil)
	if err != nil {
		return domain.NextStep{}, fmt.Errorf("список активных сценариев: %w", err)
	}

	for _, scenario := range active {
		if _, ok := completed[scenario.Code]; ok {
			continue
		}

		candidate := scenario
		return domain.NextStep{
			Type:     domain.NextStepNewScenario,
			Scenario: &candidate,
			Reason:   fmt.Sprintf("Ещё не пройден сценарий «%s»", scenario.Title),
		}, nil
	}

	if len(attempts) > 0 {
		last := attempts[len(attempts)-1]
		result := domain.EvaluateTotals(last.Score, last.AnswersCount, s.thresholds)

		if result.Percent < s.thresholds.Attentive {
			if scenario, err := s.scenarios.GetActiveByCode(ctx, last.ScenarioCode); err == nil {
				return domain.NextStep{
					Type:     domain.NextStepRetryScenario,
					Scenario: &scenario,
					Reason:   "Пройдите сценарий ещё раз, чтобы отработать пропущенные признаки",
				}, nil
			}
		}
	}

	return domain.NextStep{
		Type:   domain.NextStepAllDone,
		Reason: "Все сценарии пройдены. Загляните в справочник признаков риска",
	}, nil
}

// Signals — статистика по всем признакам каталога (FR26). Встретившиеся
// владельцу признаки получают mastered/weak, невстретившиеся — unknown, а не
// пропускаются.
func (s *progressService) Signals(ctx context.Context, owner domain.Owner) ([]domain.SignalStat, error) {
	encountered, err := s.progress.SignalStats(ctx, owner)
	if err != nil {
		return nil, fmt.Errorf("статистика по признакам риска: %w", err)
	}

	catalog, err := s.signals.List(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("каталог признаков риска: %w", err)
	}

	byCode := make(map[string]domain.SignalStat, len(encountered))
	for _, stat := range encountered {
		byCode[stat.Signal.Code] = stat
	}

	result := make([]domain.SignalStat, 0, len(catalog))
	for _, signal := range catalog {
		if stat, ok := byCode[signal.Code]; ok {
			result = append(result, stat)
			continue
		}

		result = append(result, domain.SignalStat{Signal: signal, Status: domain.SignalUnknown})
	}

	return result, nil
}

// Attempts — история завершённых попыток владельца по сценарию, от новых к
// старым (FR24); процент/уровень считаются на лету через EvaluateTotals —
// 100% воспроизводимо при смене порогов (FR22).
func (s *progressService) Attempts(
	ctx context.Context,
	owner domain.Owner,
	scenarioCode string,
) ([]domain.Attempt, error) {
	sessions, err := s.sessions.ListCompleted(ctx, owner, scenarioCode)
	if err != nil {
		return nil, fmt.Errorf("история попыток по сценарию %s: %w", scenarioCode, err)
	}

	attempts := make([]domain.Attempt, 0, len(sessions))
	for _, session := range sessions {
		answers, err := s.sessions.ListAnswers(ctx, session.ID)
		if err != nil {
			return nil, fmt.Errorf("ответы попытки %s: %w", session.ID, err)
		}

		result := domain.EvaluateTotals(session.Score, len(answers), s.thresholds)

		var finishedAt time.Time
		if session.FinishedAt != nil {
			finishedAt = *session.FinishedAt
		}

		attempts = append(attempts, domain.Attempt{
			SessionID:       session.ID,
			ScenarioCode:    session.ScenarioCode,
			ScenarioVersion: session.ScenarioVersion,
			Score:           session.Score,
			Percent:         result.Percent,
			Level:           result.Level,
			FinishedAt:      finishedAt,
		})
	}

	return attempts, nil
}

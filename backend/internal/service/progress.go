package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/domain"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/repository"
)

type progressService struct {
	progress   repository.ProgressRepository
	sessions   repository.SessionRepository
	scenarios  repository.ScenarioRepository
	thresholds domain.Thresholds
}

var _ ProgressService = (*progressService)(nil)

// TODO(M3): агрегаты по завершённым сессиям, общее число сценариев,
// незавершённая сессия для блока продолжения и предложение следующего шага.
func (s *progressService) Overview(ctx context.Context, userID uuid.UUID) (domain.Progress, error) {
	_, _ = ctx, userID
	return domain.Progress{}, domain.ErrNotImplemented
}

// TODO(M3): ни разу не встреченные признаки возвращать со статусом unknown,
// а не пропускать.
func (s *progressService) Signals(ctx context.Context, userID uuid.UUID) ([]domain.SignalStat, error) {
	_, _ = ctx, userID
	return nil, domain.ErrNotImplemented
}

// TODO(M3): процент и уровень считать через domain.Evaluate по ответам, а не
// хранить в БД: оценка остаётся воспроизводимой при изменении порогов.
func (s *progressService) Attempts(
	ctx context.Context,
	userID uuid.UUID,
	scenarioCode string,
) ([]domain.Attempt, error) {
	_, _, _ = ctx, userID, scenarioCode
	return nil, domain.ErrNotImplemented
}

package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/domain"
)

type progressRepository struct {
	pool *pgxpool.Pool
}

var _ ProgressRepository = (*progressRepository)(nil)

// TODO(M3): GROUP BY scenario_code по завершённым сессиям — число попыток,
// лучший и последний результат. Прерванные сессии не учитываются.
func (r *progressRepository) ScenarioStats(
	ctx context.Context,
	userID uuid.UUID,
) (map[string]domain.UserScenarioStats, error) {
	_, _ = ctx, userID
	return nil, domain.ErrNotImplemented
}

// TODO(M3): дельту последней попытки удобно считать оконной функцией lag().
func (r *progressRepository) Summary(ctx context.Context, userID uuid.UUID) (domain.Progress, error) {
	_, _ = ctx, userID
	return domain.Progress{}, domain.ErrNotImplemented
}

// TODO(M3): unnest(answers.risk_signal_codes) с группировкой по коду:
// encountered — сколько раз встретился, recognized — сколько раз выбран safe.
func (r *progressRepository) SignalStats(ctx context.Context, userID uuid.UUID) ([]domain.SignalStat, error) {
	_, _ = ctx, userID
	return nil, domain.ErrNotImplemented
}

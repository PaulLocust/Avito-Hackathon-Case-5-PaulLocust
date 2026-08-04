package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/domain"
)

type scenarioRepository struct {
	pool *pgxpool.Pool
}

var _ ScenarioRepository = (*scenarioRepository)(nil)

// TODO(M2): только is_active, порядок карточек стабильный.
func (r *scenarioRepository) ListActive(ctx context.Context, role *domain.Role) ([]domain.Scenario, error) {
	_, _ = ctx, role
	return nil, domain.ErrNotImplemented
}

// TODO(M2): шаги и варианты забирать одним запросом с JOIN.
func (r *scenarioRepository) GetActiveByCode(ctx context.Context, code string) (domain.Scenario, error) {
	_, _ = ctx, code
	return domain.Scenario{}, domain.ErrNotImplemented
}

// TODO(M2)
func (r *scenarioRepository) GetByCodeVersion(ctx context.Context, code string, version int) (domain.Scenario, error) {
	_, _, _ = ctx, code, version
	return domain.Scenario{}, domain.ErrNotImplemented
}

// TODO(M5): поиск по steps.risk_signal_codes (пересечение массивов).
func (r *scenarioRepository) ListBySignal(ctx context.Context, signalCode string) ([]domain.Scenario, error) {
	_, _ = ctx, signalCode
	return nil, domain.ErrNotImplemented
}

// TODO(M3): знаменатель строки «пройдено X из Y».
func (r *scenarioRepository) CountActive(ctx context.Context) (int, error) {
	_ = ctx
	return 0, domain.ErrNotImplemented
}

// TODO(M5): в одной транзакции — при совпадении content_hash ничего не
// менять, иначе снять is_active со старой версии и записать новую.
func (r *scenarioRepository) Upsert(
	ctx context.Context,
	scenario domain.Scenario,
	contentHash string,
) (int, bool, error) {
	_, _, _ = ctx, scenario, contentHash
	return 0, false, domain.ErrNotImplemented
}

package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/domain"
)

type riskSignalRepository struct {
	pool *pgxpool.Pool
}

var _ RiskSignalRepository = (*riskSignalRepository)(nil)

// TODO(M5): сортировка по side и code.
func (r *riskSignalRepository) List(ctx context.Context, side *domain.Side) ([]domain.RiskSignal, error) {
	_, _ = ctx, side
	return nil, domain.ErrNotImplemented
}

// TODO(M5)
func (r *riskSignalRepository) Get(ctx context.Context, code string) (domain.RiskSignal, error) {
	_, _ = ctx, code
	return domain.RiskSignal{}, domain.ErrNotImplemented
}

// TODO(M5): порядок результата должен повторять порядок codes.
func (r *riskSignalRepository) ListByCodes(ctx context.Context, codes []string) ([]domain.RiskSignal, error) {
	_, _ = ctx, codes
	return nil, domain.ErrNotImplemented
}

// TODO(M5): INSERT ... ON CONFLICT (code) DO UPDATE.
func (r *riskSignalRepository) Upsert(ctx context.Context, signals []domain.RiskSignal) error {
	_, _ = ctx, signals
	return domain.ErrNotImplemented
}

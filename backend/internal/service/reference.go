package service

import (
	"context"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/domain"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/repository"
)

type referenceService struct {
	signals   repository.RiskSignalRepository
	scenarios repository.ScenarioRepository
}

var _ ReferenceService = (*referenceService)(nil)

// TODO(M5)
func (s *referenceService) ListSignals(ctx context.Context, side *domain.Side) ([]domain.RiskSignal, error) {
	_, _ = ctx, side
	return nil, domain.ErrNotImplemented
}

// TODO(M5): карточка признака вместе со сценариями, где он отрабатывается.
func (s *referenceService) GetSignal(ctx context.Context, code string) (domain.RiskSignalDetail, error) {
	_, _ = ctx, code
	return domain.RiskSignalDetail{}, domain.ErrNotImplemented
}

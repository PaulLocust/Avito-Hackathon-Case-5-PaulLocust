package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/domain"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/repository"
)

type referenceService struct {
	signals   repository.RiskSignalRepository
	scenarios repository.ScenarioRepository
}

var _ ReferenceService = (*referenceService)(nil)

// ListSignals возвращает справочник признаков риска, при необходимости —
// только для одной стороны сделки (FR28).
func (s *referenceService) ListSignals(ctx context.Context, side *domain.Side) ([]domain.RiskSignal, error) {
	signals, err := s.signals.List(ctx, side)
	if err != nil {
		return nil, fmt.Errorf("справочник признаков риска: %w", err)
	}

	return signals, nil
}

// GetSignal возвращает карточку признака вместе со сценариями, в которых он
// отрабатывается: из справочника должен быть путь в тренировку (FR29).
func (s *referenceService) GetSignal(ctx context.Context, code string) (domain.RiskSignalDetail, error) {
	signal, err := s.signals.Get(ctx, code)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.RiskSignalDetail{}, err
		}

		return domain.RiskSignalDetail{}, fmt.Errorf("признак риска %s: %w", code, err)
	}

	scenarios, err := s.scenarios.ListBySignal(ctx, code)
	if err != nil {
		return domain.RiskSignalDetail{}, fmt.Errorf("сценарии с признаком %s: %w", code, err)
	}

	return domain.RiskSignalDetail{Signal: signal, RelatedScenarios: scenarios}, nil
}

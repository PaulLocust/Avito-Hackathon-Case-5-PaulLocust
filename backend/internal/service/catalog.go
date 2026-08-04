package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/domain"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/repository"
)

type catalogService struct {
	scenarios repository.ScenarioRepository
	progress  repository.ProgressRepository
	sessions  repository.SessionRepository
}

var _ CatalogService = (*catalogService)(nil)

// TODO(M2): активные сценарии для роли; для авторизованного — добрать
// статистику одним запросом; гостю карточки без прогресса.
func (s *catalogService) List(
	ctx context.Context,
	userID *uuid.UUID,
	role *domain.Role,
) ([]domain.ScenarioCard, error) {
	_, _, _ = ctx, userID, role
	return nil, domain.ErrNotImplemented
}

// TODO(M2): дополнительно проставить ActiveSessionID при незавершённой сессии.
func (s *catalogService) Get(
	ctx context.Context,
	userID *uuid.UUID,
	code string,
) (domain.ScenarioCard, error) {
	_, _, _ = ctx, userID, code
	return domain.ScenarioCard{}, domain.ErrNotImplemented
}

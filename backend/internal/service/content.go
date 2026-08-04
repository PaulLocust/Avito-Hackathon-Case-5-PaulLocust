package service

import (
	"context"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/repository"
)

type contentService struct {
	scenarios repository.ScenarioRepository
	signals   repository.RiskSignalRepository
}

var _ ContentService = (*contentService)(nil)

// TODO(M5): загрузить risk_signals.json как каталог для валидации, разобрать
// scenarios/*.json по scenario.schema.json, прогнать domain.ValidateScenario,
// посчитать хеш содержимого и вызвать Upsert. Загрузка идемпотентна: на
// неизменном контенте новых версий не создаётся.
//
// Пока не реализовано — отчитывается о пустой загрузке и завершается
// успешно, иначе контейнер бэкенда не стартует: в compose команда запуска
// выглядит как «загрузить контент и поднять API».
func (s *contentService) LoadFromDir(ctx context.Context, dir string) (LoadReport, error) {
	_, _ = ctx, dir
	return LoadReport{}, nil
}

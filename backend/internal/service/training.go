package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/domain"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/repository"
)

// trainingService отвечает за последовательность обращений к репозиториям и
// транзакционность; правила ветвления и оценки живут в пакете domain.
type trainingService struct {
	sessions   repository.SessionRepository
	scenarios  repository.ScenarioRepository
	signals    repository.RiskSignalRepository
	thresholds domain.Thresholds
}

var _ TrainingService = (*trainingService)(nil)

// TODO(M2): взять активную версию сценария; при незавершённой сессии —
// вернуть *domain.ActiveSessionError или прервать её при restart; создать
// сессию с фиксацией версии (FR32) и отдать стартовый шаг.
func (s *trainingService) Start(
	ctx context.Context,
	userID uuid.UUID,
	scenarioCode string,
	restart bool,
) (domain.SessionSnapshot, error) {
	_, _, _, _ = ctx, userID, scenarioCode, restart
	return domain.SessionSnapshot{}, domain.ErrNotImplemented
}

// TODO(M2): чужая сессия — domain.ErrNotFound, а не ошибка доступа (SEC2).
func (s *trainingService) Get(ctx context.Context, userID, sessionID uuid.UUID) (domain.SessionSnapshot, error) {
	_, _, _ = ctx, userID, sessionID
	return domain.SessionSnapshot{}, domain.ErrNotImplemented
}

// TODO(M2): проверить владельца и статус; уже отвеченный шаг — вернуть
// сохранённый результат (FR13); сверить stepCode с текущим; domain.ResolveNext;
// сохранить ответ с обновлением балла; терминальный шаг завершает сессию;
// собрать признаки риска шага и безопасную альтернативу.
func (s *trainingService) SubmitAnswer(
	ctx context.Context,
	userID, sessionID uuid.UUID,
	stepCode, optionCode string,
) (domain.AnswerOutcome, error) {
	_, _, _, _, _ = ctx, userID, sessionID, stepCode, optionCode
	return domain.AnswerOutcome{}, domain.ErrNotImplemented
}

// TODO(M2): завершённую сессию прерывать нельзя — domain.ErrSessionFinished.
func (s *trainingService) Abandon(ctx context.Context, userID, sessionID uuid.UUID) error {
	_, _, _ = ctx, userID, sessionID
	return domain.ErrNotImplemented
}

// TODO(M3): незавершённая сессия — domain.ErrSessionNotFinished; оценка через
// domain.Evaluate; разбор строить по версии сценария из сессии; карта
// признаков, сравнение с предыдущей попыткой, рекомендации, следующий шаг.
func (s *trainingService) Result(ctx context.Context, userID, sessionID uuid.UUID) (domain.Debrief, error) {
	_, _, _ = ctx, userID, sessionID
	return domain.Debrief{}, domain.ErrNotImplemented
}

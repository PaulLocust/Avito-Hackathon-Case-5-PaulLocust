package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/domain"
)

type sessionRepository struct {
	pool *pgxpool.Pool
}

var _ SessionRepository = (*sessionRepository)(nil)

// TODO(M2): нарушение sessions_single_active_idx — вернуть идентификатор
// активной сессии в domain.ActiveSessionError.
func (r *sessionRepository) Create(ctx context.Context, session domain.Session) (domain.Session, error) {
	_, _ = ctx, session
	return domain.Session{}, domain.ErrNotImplemented
}

// TODO(M2)
func (r *sessionRepository) Get(ctx context.Context, id uuid.UUID) (domain.Session, error) {
	_, _ = ctx, id
	return domain.Session{}, domain.ErrNotImplemented
}

// TODO(M2)
func (r *sessionRepository) GetActiveByUser(ctx context.Context, userID uuid.UUID) (domain.Session, error) {
	_, _ = ctx, userID
	return domain.Session{}, domain.ErrNotImplemented
}

// TODO(M2)
func (r *sessionRepository) GetActiveByUserScenario(
	ctx context.Context,
	userID uuid.UUID,
	scenarioCode string,
) (domain.Session, error) {
	_, _, _ = ctx, userID, scenarioCode
	return domain.Session{}, domain.ErrNotImplemented
}

// TODO(M2): одной транзакцией — INSERT в answers (конфликт по
// (session_id, step_code) означает уже отвеченный шаг), UPDATE score и
// current_step_code, при finished — status и finished_at.
func (r *sessionRepository) SaveAnswer(
	ctx context.Context,
	answer domain.Answer,
	nextStepCode string,
	finished bool,
) (domain.Session, error) {
	_, _, _, _ = ctx, answer, nextStepCode, finished
	return domain.Session{}, domain.ErrNotImplemented
}

// TODO(M2)
func (r *sessionRepository) GetAnswer(
	ctx context.Context,
	sessionID uuid.UUID,
	stepCode string,
) (domain.Answer, error) {
	_, _, _ = ctx, sessionID, stepCode
	return domain.Answer{}, domain.ErrNotImplemented
}

// TODO(M2): сортировка по position.
func (r *sessionRepository) ListAnswers(ctx context.Context, sessionID uuid.UUID) ([]domain.Answer, error) {
	_, _ = ctx, sessionID
	return nil, domain.ErrNotImplemented
}

// TODO(M2): прерывать только сессии в статусе in_progress.
func (r *sessionRepository) Abandon(ctx context.Context, id uuid.UUID) error {
	_, _ = ctx, id
	return domain.ErrNotImplemented
}

// TODO(M3): сортировка finished_at DESC.
func (r *sessionRepository) ListCompleted(
	ctx context.Context,
	userID uuid.UUID,
	scenarioCode string,
) ([]domain.Session, error) {
	_, _, _ = ctx, userID, scenarioCode
	return nil, domain.ErrNotImplemented
}

// TODO(M3): самая свежая завершённая с finished_at меньше, чем у before.
func (r *sessionRepository) PreviousCompleted(
	ctx context.Context,
	userID uuid.UUID,
	scenarioCode string,
	before uuid.UUID,
) (domain.Session, error) {
	_, _, _, _ = ctx, userID, scenarioCode, before
	return domain.Session{}, domain.ErrNotImplemented
}

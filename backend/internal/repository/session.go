package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/domain"
)

type sessionRepository struct {
	pool *pgxpool.Pool
}

var _ SessionRepository = (*sessionRepository)(nil)

// uniqueViolation — код SQLSTATE класса 23 (нарушение целостности).
const uniqueViolation = "23505"

// sessionColumns — колонки строки sessions для SELECT и RETURNING.
const sessionColumns = `id, user_id, scenario_id, scenario_code, scenario_version,
	status, current_step_code, score, started_at, finished_at`

// Create сохраняет сессию. Нарушение sessions_single_active_idx — у клиента
// уже есть незавершённая сессия по сценарию: возвращаем её идентификатор,
// чтобы клиент предложил продолжить (FR12).
func (r *sessionRepository) Create(ctx context.Context, session domain.Session) (domain.Session, error) {
	if session.ID == uuid.Nil {
		session.ID = uuid.New()
	}

	query := `
		INSERT INTO sessions (id, user_id, scenario_id, scenario_code, scenario_version,
		                      status, current_step_code, score)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING ` + sessionColumns

	created, err := scanSession(r.pool.QueryRow(ctx, query,
		session.ID, session.UserID, session.ScenarioID, session.ScenarioCode,
		session.ScenarioVersion, string(session.Status), session.CurrentStepCode, session.Score,
	))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
			active, getErr := r.GetActiveByUserScenario(ctx, session.UserID, session.ScenarioCode)
			if getErr != nil {
				return domain.Session{}, fmt.Errorf("чтение активной сессии после конфликта: %w", getErr)
			}

			return domain.Session{}, &domain.ActiveSessionError{SessionID: active.ID}
		}

		return domain.Session{}, fmt.Errorf("создание сессии: %w", err)
	}

	return created, nil
}

func (r *sessionRepository) Get(ctx context.Context, id uuid.UUID) (domain.Session, error) {
	session, err := scanSession(r.pool.QueryRow(ctx,
		"SELECT "+sessionColumns+" FROM sessions WHERE id = $1", id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Session{}, domain.ErrNotFound
		}

		return domain.Session{}, fmt.Errorf("чтение сессии %s: %w", id, err)
	}

	return session, nil
}

// TODO(M2): единственная незавершённая сессия пользователя для блока
// «продолжить тренировку» (FR12); не нужен training-эндпоинтам.
func (r *sessionRepository) GetActiveByUser(ctx context.Context, userID uuid.UUID) (domain.Session, error) {
	_, _ = ctx, userID
	return domain.Session{}, domain.ErrNotImplemented
}

func (r *sessionRepository) GetActiveByUserScenario(
	ctx context.Context,
	userID uuid.UUID,
	scenarioCode string,
) (domain.Session, error) {
	session, err := scanSession(r.pool.QueryRow(ctx,
		"SELECT "+sessionColumns+
			" FROM sessions WHERE user_id = $1 AND scenario_code = $2 AND status = 'in_progress'"+
			" ORDER BY started_at DESC LIMIT 1",
		userID, scenarioCode))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Session{}, domain.ErrNotFound
		}

		return domain.Session{}, fmt.Errorf("поиск активной сессии по сценарию %s: %w", scenarioCode, err)
	}

	return session, nil
}

// SaveAnswer одной транзакцией фиксирует ответ и начисляет баллы: инвариант
// «балл сессии = сумма весов ответов» держится схемой, а не бизнес-логикой.
// finished переводит сессию в completed и проставляет finished_at (FR14).
func (r *sessionRepository) SaveAnswer(
	ctx context.Context,
	answer domain.Answer,
	nextStepCode string,
	finished bool,
) (domain.Session, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Session{}, fmt.Errorf("начало транзакции фиксации ответа: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	insert := `
		INSERT INTO answers (session_id, step_code, option_code, outcome, score_delta,
		                     risk_signal_codes, position)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	if _, execErr := tx.Exec(ctx, insert,
		answer.SessionID, answer.StepCode, answer.OptionCode,
		string(answer.Outcome), answer.ScoreDelta, answer.RiskSignalCodes, answer.Position,
	); execErr != nil {
		return domain.Session{}, fmt.Errorf("фиксация ответа на шаг %s: %w", answer.StepCode, execErr)
	}

	if finished {
		if _, execErr := tx.Exec(ctx, `
			UPDATE sessions
			SET score = score + $1, current_step_code = NULL,
			    status = 'completed', finished_at = now()
			WHERE id = $2`,
			answer.ScoreDelta, answer.SessionID,
		); execErr != nil {
			return domain.Session{}, fmt.Errorf("завершение сессии %s: %w", answer.SessionID, execErr)
		}
	} else {
		if _, execErr := tx.Exec(ctx, `
			UPDATE sessions
			SET score = score + $1, current_step_code = $3
			WHERE id = $2`,
			answer.ScoreDelta, answer.SessionID, nextStepCode,
		); execErr != nil {
			return domain.Session{}, fmt.Errorf("перевод сессии %s на следующий шаг: %w", answer.SessionID, execErr)
		}
	}

	session, err := scanSession(tx.QueryRow(ctx,
		"SELECT "+sessionColumns+" FROM sessions WHERE id = $1", answer.SessionID))
	if err != nil {
		return domain.Session{}, fmt.Errorf("чтение сессии %s после ответа: %w", answer.SessionID, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Session{}, fmt.Errorf("завершение транзакции ответа: %w", err)
	}

	return session, nil
}

// GetAnswer читает ответ на шаг; для идемпотентности повторной отправки
// (FR13) отсутствие строки — domain.ErrNotFound.
func (r *sessionRepository) GetAnswer(
	ctx context.Context,
	sessionID uuid.UUID,
	stepCode string,
) (domain.Answer, error) {
	var (
		answer  domain.Answer
		outcome string
	)

	err := r.pool.QueryRow(ctx, `
		SELECT id, session_id, step_code, option_code, outcome, score_delta,
		       risk_signal_codes, position, answered_at
		FROM answers
		WHERE session_id = $1 AND step_code = $2`,
		sessionID, stepCode,
	).Scan(&answer.ID, &answer.SessionID, &answer.StepCode, &answer.OptionCode,
		&outcome, &answer.ScoreDelta, &answer.RiskSignalCodes, &answer.Position, &answer.AnsweredAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Answer{}, domain.ErrNotFound
		}

		return domain.Answer{}, fmt.Errorf("чтение ответа на шаг %s: %w", stepCode, err)
	}

	answer.Outcome = domain.Outcome(outcome)

	return answer, nil
}

// ListAnswers возвращает ответы в порядке прохождения (position).
func (r *sessionRepository) ListAnswers(ctx context.Context, sessionID uuid.UUID) ([]domain.Answer, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, session_id, step_code, option_code, outcome, score_delta,
		       risk_signal_codes, position, answered_at
		FROM answers
		WHERE session_id = $1
		ORDER BY position`,
		sessionID)
	if err != nil {
		return nil, fmt.Errorf("выбор ответов сессии %s: %w", sessionID, err)
	}
	defer rows.Close()

	answers := make([]domain.Answer, 0)
	for rows.Next() {
		var (
			answer  domain.Answer
			outcome string
		)

		if err := rows.Scan(&answer.ID, &answer.SessionID, &answer.StepCode, &answer.OptionCode,
			&outcome, &answer.ScoreDelta, &answer.RiskSignalCodes, &answer.Position, &answer.AnsweredAt); err != nil {
			return nil, fmt.Errorf("чтение ответа сессии %s: %w", sessionID, err)
		}

		answer.Outcome = domain.Outcome(outcome)
		answers = append(answers, answer)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("перебор ответов сессии %s: %w", sessionID, err)
	}

	return answers, nil
}

// Abandon прерывает только сессии в статусе in_progress: повторный вызов
// завершённой или уже прерванной сессии — domain.ErrNotFound.
func (r *sessionRepository) Abandon(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE sessions
		SET status = 'abandoned', finished_at = now()
		WHERE id = $1 AND status = 'in_progress'`, id)
	if err != nil {
		return fmt.Errorf("прерывание сессии %s: %w", id, err)
	}

	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}

// ListCompleted — завершённые попытки, свежие первыми.
func (r *sessionRepository) ListCompleted(
	ctx context.Context,
	userID uuid.UUID,
	scenarioCode string,
) ([]domain.Session, error) {
	rows, err := r.pool.Query(ctx,
		"SELECT "+sessionColumns+" FROM sessions"+
			" WHERE user_id = $1 AND scenario_code = $2 AND status = 'completed'"+
			" ORDER BY finished_at DESC",
		userID, scenarioCode)
	if err != nil {
		return nil, fmt.Errorf("выбор истории сценария %s: %w", scenarioCode, err)
	}
	defer rows.Close()

	sessions := make([]domain.Session, 0)
	for rows.Next() {
		session, err := scanSession(rows)
		if err != nil {
			return nil, fmt.Errorf("чтение завершённой сессии: %w", err)
		}

		sessions = append(sessions, session)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("перебор истории сценария %s: %w", scenarioCode, err)
	}

	return sessions, nil
}

// PreviousCompleted — самая свежая завершённая попытка, завершившаяся раньше,
// чем сессия before: для сравнения результата (FR23).
func (r *sessionRepository) PreviousCompleted(
	ctx context.Context,
	userID uuid.UUID,
	scenarioCode string,
	before uuid.UUID,
) (domain.Session, error) {
	session, err := scanSession(r.pool.QueryRow(ctx,
		"SELECT "+sessionColumns+" FROM sessions"+
			" WHERE user_id = $1 AND scenario_code = $2 AND status = 'completed'"+
			" AND finished_at < (SELECT finished_at FROM sessions WHERE id = $3)"+
			" ORDER BY finished_at DESC LIMIT 1",
		userID, scenarioCode, before))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Session{}, domain.ErrNotFound
		}

		return domain.Session{}, fmt.Errorf("поиск предыдущей попытки по сценарию %s: %w", scenarioCode, err)
	}

	return session, nil
}

// scanSession читает строку sessions в доменную модель; nullable-колонки
// (current_step_code, finished_at) разворачиваем в пустые значения.
func scanSession(row pgx.Row) (domain.Session, error) {
	var (
		session     domain.Session
		status      string
		currentStep *string
		finishedAt  *time.Time
	)

	err := row.Scan(
		&session.ID, &session.UserID, &session.ScenarioID, &session.ScenarioCode,
		&session.ScenarioVersion, &status, &currentStep, &session.Score,
		&session.StartedAt, &finishedAt,
	)
	if err != nil {
		return domain.Session{}, err
	}

	session.Status = domain.SessionStatus(status)
	if currentStep != nil {
		session.CurrentStepCode = *currentStep
	}
	session.FinishedAt = finishedAt

	return session, nil
}

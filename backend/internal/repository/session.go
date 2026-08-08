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

const uniqueViolation = "23505"

// sessionColumns — колонки строки sessions. Владелец хранится как две
// nullable-колонки: ровно одна заполнена (CHECK на уровне схемы).
const sessionColumns = `id, user_id, guest_session_id, scenario_id, scenario_code, scenario_version,
	status, current_step_code, score, started_at, finished_at`

// ownerColumns разворачивает Owner в пару значений для INSERT.
func ownerColumns(owner domain.Owner) (userID, guestSessionID *uuid.UUID) {
	id := owner.ID
	switch owner.Kind {
	case domain.OwnerUser:
		return &id, nil
	case domain.OwnerGuest:
		return nil, &id
	default:
		return nil, nil
	}
}

// ownerFromColumns — обратное преобразование при чтении строки.
func ownerFromColumns(userID, guestSessionID *uuid.UUID) domain.Owner {
	if userID != nil {
		return domain.UserOwner(*userID)
	}
	if guestSessionID != nil {
		return domain.GuestOwner(*guestSessionID)
	}
	return domain.Owner{}
}

// ownerWhere — условие WHERE по владельцу и позиционный аргумент начиная
// с индекса argPos. Переиспользуется и репозиторием прогресса (progress.go):
// аналитика собирается по тому же ключу (user_id либо guest_session_id).
func ownerWhere(owner domain.Owner, argPos int) (string, uuid.UUID) {
	if owner.Kind == domain.OwnerGuest {
		return fmt.Sprintf("guest_session_id = $%d", argPos), owner.ID
	}
	return fmt.Sprintf("user_id = $%d", argPos), owner.ID
}

func (r *sessionRepository) Create(ctx context.Context, session domain.Session) (domain.Session, error) {
	if session.ID == uuid.Nil {
		session.ID = uuid.New()
	}

	userID, guestSessionID := ownerColumns(session.Owner)

	query := `
		INSERT INTO sessions (id, user_id, guest_session_id, scenario_id, scenario_code, scenario_version,
		                      status, current_step_code, score)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING ` + sessionColumns

	created, err := scanSession(r.pool.QueryRow(ctx, query,
		session.ID, userID, guestSessionID, session.ScenarioID, session.ScenarioCode,
		session.ScenarioVersion, string(session.Status), session.CurrentStepCode, session.Score,
	))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
			active, getErr := r.GetActiveByOwnerScenario(ctx, session.Owner, session.ScenarioCode)
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

// GetActiveByOwner — единственная незавершённая сессия владельца (юзера или
// гостя), независимо от сценария. Используется блоком «продолжить
// тренировку» на главной (FR12).
func (r *sessionRepository) GetActiveByOwner(ctx context.Context, owner domain.Owner) (domain.Session, error) {
	condition, ownerID := ownerWhere(owner, 1)

	session, err := scanSession(r.pool.QueryRow(ctx,
		"SELECT "+sessionColumns+" FROM sessions WHERE "+condition+" AND status = 'in_progress'"+
			" ORDER BY started_at DESC LIMIT 1",
		ownerID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Session{}, domain.ErrNotFound
		}

		return domain.Session{}, fmt.Errorf("поиск активной сессии владельца: %w", err)
	}

	return session, nil
}

func (r *sessionRepository) GetActiveByOwnerScenario(
	ctx context.Context,
	owner domain.Owner,
	scenarioCode string,
) (domain.Session, error) {
	condition, ownerID := ownerWhere(owner, 2)

	session, err := scanSession(r.pool.QueryRow(ctx,
		"SELECT "+sessionColumns+
			" FROM sessions WHERE "+condition+" AND scenario_code = $1 AND status = 'in_progress'"+
			" ORDER BY started_at DESC LIMIT 1",
		scenarioCode, ownerID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Session{}, domain.ErrNotFound
		}

		return domain.Session{}, fmt.Errorf("поиск активной сессии по сценарию %s: %w", scenarioCode, err)
	}

	return session, nil
}

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

func (r *sessionRepository) ListCompleted(
	ctx context.Context,
	owner domain.Owner,
	scenarioCode string,
) ([]domain.Session, error) {
	condition, ownerID := ownerWhere(owner, 2)

	rows, err := r.pool.Query(ctx,
		"SELECT "+sessionColumns+" FROM sessions"+
			" WHERE "+condition+" AND scenario_code = $1 AND status = 'completed'"+
			" ORDER BY finished_at DESC",
		scenarioCode, ownerID)
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

func (r *sessionRepository) PreviousCompleted(
	ctx context.Context,
	owner domain.Owner,
	scenarioCode string,
	before uuid.UUID,
) (domain.Session, error) {
	condition, ownerID := ownerWhere(owner, 3)

	session, err := scanSession(r.pool.QueryRow(ctx,
		"SELECT "+sessionColumns+" FROM sessions"+
			" WHERE "+condition+" AND scenario_code = $1 AND status = 'completed'"+
			" AND finished_at < (SELECT finished_at FROM sessions WHERE id = $2)"+
			" ORDER BY finished_at DESC LIMIT 1",
		scenarioCode, before, ownerID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Session{}, domain.ErrNotFound
		}

		return domain.Session{}, fmt.Errorf("поиск предыдущей попытки по сценарию %s: %w", scenarioCode, err)
	}

	return session, nil
}

// ClaimByGuest переносит все сессии гостя на аккаунт после Register/Login.
// Это и есть момент, когда накопленная гостевая аналитика «материализуется»
// под учётной записью и становится доступна через /progress (FR12).
func (r *sessionRepository) ClaimByGuest(ctx context.Context, guestSessionID, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE sessions SET user_id = $2, guest_session_id = NULL WHERE guest_session_id = $1`,
		guestSessionID, userID)
	if err != nil {
		return fmt.Errorf("перенос сессий гостя %s: %w", guestSessionID, err)
	}

	return nil
}

func scanSession(row pgx.Row) (domain.Session, error) {
	var (
		session        domain.Session
		status         string
		userID         *uuid.UUID
		guestSessionID *uuid.UUID
		currentStep    *string
		finishedAt     *time.Time
	)

	err := row.Scan(
		&session.ID, &userID, &guestSessionID, &session.ScenarioID, &session.ScenarioCode,
		&session.ScenarioVersion, &status, &currentStep, &session.Score,
		&session.StartedAt, &finishedAt,
	)
	if err != nil {
		return domain.Session{}, err
	}

	session.Owner = ownerFromColumns(userID, guestSessionID)
	session.Status = domain.SessionStatus(status)
	if currentStep != nil {
		session.CurrentStepCode = *currentStep
	}
	session.FinishedAt = finishedAt

	return session, nil
}

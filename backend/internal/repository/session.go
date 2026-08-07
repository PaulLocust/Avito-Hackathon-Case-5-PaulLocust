package repository

import (
	"context"
	"errors"

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

func ownerColumns(owner domain.Owner) (userID, guestSessionID *uuid.UUID) {
	id := owner.ID
	if owner.IsUser() {
		return &id, nil
	}
	return nil, &id
}

func ownerWhere(owner domain.Owner) (column string, value uuid.UUID) {
	if owner.IsUser() {
		return "user_id", owner.ID
	}
	return "guest_session_id", owner.ID
}

const sessionColumns = `
	id, user_id, guest_session_id, scenario_id, scenario_code, scenario_version,
	status, current_step_code, score, started_at, finished_at
`

func scanSession(row pgx.Row) (domain.Session, error) {
	var (
		s       domain.Session
		userID  *uuid.UUID
		guestID *uuid.UUID
	)

	err := row.Scan(
		&s.ID, &userID, &guestID, &s.ScenarioID, &s.ScenarioCode, &s.ScenarioVersion,
		&s.Status, &s.CurrentStepCode, &s.Score, &s.StartedAt, &s.FinishedAt,
	)
	if err != nil {
		return domain.Session{}, err
	}

	switch {
	case userID != nil:
		s.Owner = domain.UserOwner(*userID)
	case guestID != nil:
		s.Owner = domain.GuestOwner(*guestID)
	}

	return s, nil
}

func (r *sessionRepository) Create(ctx context.Context, session domain.Session) (domain.Session, error) {
	userID, guestID := ownerColumns(session.Owner)

	row := r.pool.QueryRow(ctx, `
		INSERT INTO sessions (
			id, user_id, guest_session_id, scenario_id, scenario_code, scenario_version,
			status, current_step_code, score, started_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING `+sessionColumns,
		session.ID, userID, guestID, session.ScenarioID, session.ScenarioCode, session.ScenarioVersion,
		session.Status, session.CurrentStepCode, session.Score, session.StartedAt,
	)

	result, err := scanSession(row)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "sessions_single_active_idx" {
			active, getErr := r.GetActiveByOwnerScenario(ctx, session.Owner, session.ScenarioCode)
			if getErr != nil {
				return domain.Session{}, err
			}
			return domain.Session{}, &domain.ActiveSessionError{SessionID: active.ID}
		}
		return domain.Session{}, err
	}

	return result, nil
}

func (r *sessionRepository) Get(ctx context.Context, id uuid.UUID) (domain.Session, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+sessionColumns+` FROM sessions WHERE id = $1`, id)

	s, err := scanSession(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Session{}, domain.ErrNotFound
	}
	return s, err
}

func (r *sessionRepository) GetActiveByOwner(ctx context.Context, owner domain.Owner) (domain.Session, error) {
	col, val := ownerWhere(owner)

	row := r.pool.QueryRow(ctx,
		`SELECT `+sessionColumns+` FROM sessions
		 WHERE status = 'in_progress' AND `+col+` = $1
		 ORDER BY started_at DESC LIMIT 1`,
		val,
	)

	s, err := scanSession(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Session{}, domain.ErrNotFound
	}
	return s, err
}

func (r *sessionRepository) GetActiveByOwnerScenario(
	ctx context.Context,
	owner domain.Owner,
	scenarioCode string,
) (domain.Session, error) {
	col, val := ownerWhere(owner)

	row := r.pool.QueryRow(ctx,
		`SELECT `+sessionColumns+` FROM sessions
		 WHERE status = 'in_progress' AND `+col+` = $1 AND scenario_code = $2
		 LIMIT 1`,
		val, scenarioCode,
	)

	s, err := scanSession(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Session{}, domain.ErrNotFound
	}
	return s, err
}

func (r *sessionRepository) SaveAnswer(
	ctx context.Context,
	answer domain.Answer,
	nextStepCode string,
	finished bool,
) (domain.Session, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Session{}, err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		INSERT INTO answers (
			session_id, step_code, option_code, outcome, score_delta,
			risk_signal_codes, position, answered_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		answer.SessionID, answer.StepCode, answer.OptionCode, answer.Outcome,
		answer.ScoreDelta, answer.RiskSignalCodes, answer.Position, answer.AnsweredAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.Session{}, domain.ErrStepNotCurrent
		}
		return domain.Session{}, err
	}

	var row pgx.Row
	if finished {
		row = tx.QueryRow(ctx, `
			UPDATE sessions
			SET score = score + $2, current_step_code = $3,
			    status = 'completed', finished_at = now()
			WHERE id = $1
			RETURNING `+sessionColumns,
			answer.SessionID, answer.ScoreDelta, nextStepCode,
		)
	} else {
		row = tx.QueryRow(ctx, `
			UPDATE sessions
			SET score = score + $2, current_step_code = $3
			WHERE id = $1
			RETURNING `+sessionColumns,
			answer.SessionID, answer.ScoreDelta, nextStepCode,
		)
	}

	result, err := scanSession(row)
	if err != nil {
		return domain.Session{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Session{}, err
	}

	return result, nil
}

func (r *sessionRepository) GetAnswer(ctx context.Context, sessionID uuid.UUID, stepCode string) (domain.Answer, error) {
	var a domain.Answer

	err := r.pool.QueryRow(ctx, `
		SELECT id, session_id, step_code, option_code, outcome, score_delta,
		       risk_signal_codes, position, answered_at
		FROM answers WHERE session_id = $1 AND step_code = $2`,
		sessionID, stepCode,
	).Scan(&a.ID, &a.SessionID, &a.StepCode, &a.OptionCode, &a.Outcome,
		&a.ScoreDelta, &a.RiskSignalCodes, &a.Position, &a.AnsweredAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Answer{}, domain.ErrNotFound
	}
	return a, err
}

func (r *sessionRepository) ListAnswers(ctx context.Context, sessionID uuid.UUID) ([]domain.Answer, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, session_id, step_code, option_code, outcome, score_delta,
		       risk_signal_codes, position, answered_at
		FROM answers WHERE session_id = $1 ORDER BY position ASC`,
		sessionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []domain.Answer
	for rows.Next() {
		var a domain.Answer
		if err := rows.Scan(&a.ID, &a.SessionID, &a.StepCode, &a.OptionCode, &a.Outcome,
			&a.ScoreDelta, &a.RiskSignalCodes, &a.Position, &a.AnsweredAt); err != nil {
			return nil, err
		}
		result = append(result, a)
	}
	return result, rows.Err()
}

func (r *sessionRepository) Abandon(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE sessions SET status = 'abandoned', finished_at = now()
		WHERE id = $1 AND status = 'in_progress'`,
		id,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrSessionFinished
	}
	return nil
}

func (r *sessionRepository) ListCompleted(ctx context.Context, owner domain.Owner, scenarioCode string) ([]domain.Session, error) {
	col, val := ownerWhere(owner)

	rows, err := r.pool.Query(ctx,
		`SELECT `+sessionColumns+` FROM sessions
		 WHERE `+col+` = $1 AND scenario_code = $2 AND status = 'completed'
		 ORDER BY finished_at DESC`,
		val, scenarioCode,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []domain.Session
	for rows.Next() {
		s, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

func (r *sessionRepository) PreviousCompleted(
	ctx context.Context,
	owner domain.Owner,
	scenarioCode string,
	before uuid.UUID,
) (domain.Session, error) {
	col, val := ownerWhere(owner)

	row := r.pool.QueryRow(ctx, `
		SELECT `+sessionColumns+` FROM sessions
		WHERE `+col+` = $1 AND scenario_code = $2 AND status = 'completed'
		  AND finished_at < (SELECT finished_at FROM sessions WHERE id = $3)
		ORDER BY finished_at DESC LIMIT 1`,
		val, scenarioCode, before,
	)

	s, err := scanSession(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Session{}, domain.ErrNotFound
	}
	return s, err
}

func (r *sessionRepository) ClaimByGuest(ctx context.Context, guestSessionID, userID uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Если у аккаунта уже есть незавершённый сценарий, сохраняем его, а
	// конфликтующую гостевую сессию помечаем прерванной. Завершённые результаты
	// в любом случае переходят в аналитику пользователя.
	_, err = tx.Exec(ctx, `
		UPDATE sessions AS guest
		SET status = 'abandoned', finished_at = now()
		WHERE guest.guest_session_id = $1
		  AND guest.status = 'in_progress'
		  AND EXISTS (
			SELECT 1 FROM sessions AS user_session
			WHERE user_session.user_id = $2
			  AND user_session.scenario_code = guest.scenario_code
			  AND user_session.status = 'in_progress'
		  )`,
		guestSessionID, userID,
	)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
		UPDATE sessions SET user_id = $2, guest_session_id = NULL
		WHERE guest_session_id = $1`,
		guestSessionID, userID,
	)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

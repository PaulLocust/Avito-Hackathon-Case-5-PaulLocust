package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/domain"
)

type progressRepository struct {
	pool *pgxpool.Pool
}

var _ ProgressRepository = (*progressRepository)(nil)

// ScenarioStats собирает статистику по завершённым попыткам владельца
// (юзера или гостя — см. ownerWhere в session.go). Прерванные сессии не
// учитываются: они не являются результатом.
//
// Лучшая попытка выбирается по доле безопасных решений (score / число
// выборов), а не по сумме баллов: ветки сценария имеют разную длину, и
// сравнение сырых баллов сравнивало бы разное. Процент и уровень считает
// сервис — здесь возвращается только сырой агрегат.
func (r *progressRepository) ScenarioStats(
	ctx context.Context,
	owner domain.Owner,
) (map[string]domain.UserScenarioStats, error) {
	condition, ownerID := ownerWhere(owner, 1)

	query := `
		WITH attempts AS (
		    SELECT s.id, s.scenario_code, s.score, s.finished_at,
		           COUNT(a.id)::int AS answers_count
		    FROM sessions s
		    LEFT JOIN answers a ON a.session_id = s.id
		    WHERE ` + condition + ` AND s.status = 'completed'
		    GROUP BY s.id
		)
		SELECT scenario_code,
		       COUNT(*)::int AS attempts_count,
		       MAX(finished_at) AS last_attempt_at,
		       (array_agg(score ORDER BY score::numeric / GREATEST(answers_count, 1) DESC,
		                  finished_at DESC))[1] AS best_score,
		       (array_agg(answers_count ORDER BY score::numeric / GREATEST(answers_count, 1) DESC,
		                  finished_at DESC))[1] AS best_answers
		FROM attempts
		GROUP BY scenario_code`

	rows, err := r.pool.Query(ctx, query, ownerID)
	if err != nil {
		return nil, fmt.Errorf("статистика по сценариям: %w", err)
	}
	defer rows.Close()

	stats := make(map[string]domain.UserScenarioStats)

	for rows.Next() {
		var (
			code         string
			lastAttempt  time.Time
			scenarioStat domain.UserScenarioStats
		)

		if err := rows.Scan(&code, &scenarioStat.AttemptsCount, &lastAttempt,
			&scenarioStat.BestScore, &scenarioStat.BestAnswers); err != nil {
			return nil, fmt.Errorf("чтение статистики по сценарию: %w", err)
		}

		scenarioStat.Attempted = true
		scenarioStat.LastAttemptAt = &lastAttempt
		stats[code] = scenarioStat
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("перебор статистики по сценариям: %w", err)
	}

	return stats, nil
}

// ScoredAttempts возвращает завершённые попытки владельца по всем
// сценариям, отсортированные по finished_at по возрастанию. Только сырые
// баллы — процент и уровень считает сервис через domain.EvaluateTotals
// (FR22): формула — в одном месте.
func (r *progressRepository) ScoredAttempts(ctx context.Context, owner domain.Owner) ([]domain.ScoredAttempt, error) {
	condition, ownerID := ownerWhere(owner, 1)

	rows, err := r.pool.Query(ctx, `
		SELECT s.scenario_code, s.score, COUNT(a.id)::int AS answers_count, s.finished_at
		FROM sessions s
		LEFT JOIN answers a ON a.session_id = s.id
		WHERE `+condition+` AND s.status = 'completed'
		GROUP BY s.id
		ORDER BY s.finished_at`, ownerID)
	if err != nil {
		return nil, fmt.Errorf("завершённые попытки владельца: %w", err)
	}
	defer rows.Close()

	attempts := make([]domain.ScoredAttempt, 0)
	for rows.Next() {
		var attempt domain.ScoredAttempt

		if err := rows.Scan(&attempt.ScenarioCode, &attempt.Score,
			&attempt.AnswersCount, &attempt.FinishedAt); err != nil {
			return nil, fmt.Errorf("чтение попытки: %w", err)
		}

		attempts = append(attempts, attempt)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("перебор попыток владельца: %w", err)
	}

	return attempts, nil
}

// SignalStats — сколько раз признак встретился на шагах завершённых попыток
// владельца и сколько раз распознан (safe). Признаки, ни разу не
// встретившиеся, сюда не попадают — их достраивает сервис до статуса
// unknown, сверяя со справочником (FR26).
func (r *progressRepository) SignalStats(ctx context.Context, owner domain.Owner) ([]domain.SignalStat, error) {
	condition, ownerID := ownerWhere(owner, 1)

	rows, err := r.pool.Query(ctx, `
		SELECT rs.code, rs.side, rs.title, rs.summary, rs.description,
		       rs.how_to_recognize, rs.how_to_act,
		       COUNT(*)::int AS encountered,
		       COUNT(*) FILTER (WHERE a.outcome = 'safe')::int AS recognized
		FROM answers a
		JOIN sessions s ON s.id = a.session_id
		CROSS JOIN LATERAL unnest(a.risk_signal_codes) AS rsc(code)
		JOIN risk_signals rs ON rs.code = rsc.code
		WHERE `+condition+` AND s.status = 'completed'
		GROUP BY rs.code, rs.side, rs.title, rs.summary, rs.description,
		         rs.how_to_recognize, rs.how_to_act
		ORDER BY rs.side, rs.code`, ownerID)
	if err != nil {
		return nil, fmt.Errorf("статистика по признакам риска: %w", err)
	}
	defer rows.Close()

	stats := make([]domain.SignalStat, 0)
	for rows.Next() {
		var (
			stat domain.SignalStat
			side string
		)

		if err := rows.Scan(&stat.Signal.Code, &side, &stat.Signal.Title, &stat.Signal.Summary,
			&stat.Signal.Description, &stat.Signal.HowToRecognize, &stat.Signal.HowToAct,
			&stat.Encountered, &stat.Recognized); err != nil {
			return nil, fmt.Errorf("чтение статистики признака: %w", err)
		}

		stat.Signal.Side = domain.Side(side)
		stat.Missed = stat.Encountered - stat.Recognized
		stat.Status = domain.SignalWeak
		if stat.Recognized == stat.Encountered {
			stat.Status = domain.SignalMastered
		}

		stats = append(stats, stat)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("перебор статистики признаков: %w", err)
	}

	return stats, nil
}

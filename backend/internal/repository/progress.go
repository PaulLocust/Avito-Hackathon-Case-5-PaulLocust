package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/domain"
)

type progressRepository struct {
	pool *pgxpool.Pool
}

var _ ProgressRepository = (*progressRepository)(nil)

// ScenarioStats собирает статистику по завершённым попыткам пользователя.
// Прерванные сессии не учитываются: они не являются результатом.
//
// Лучшая попытка выбирается по доле безопасных решений (score / число
// выборов), а не по сумме баллов: ветки сценария имеют разную длину, и
// сравнение сырых баллов сравнивало бы разное. Процент и уровень считает
// сервис — здесь возвращается только сырой агрегат.
func (r *progressRepository) ScenarioStats(
	ctx context.Context,
	userID uuid.UUID,
) (map[string]domain.UserScenarioStats, error) {
	query := `
		WITH attempts AS (
		    SELECT s.id, s.scenario_code, s.score, s.finished_at,
		           COUNT(a.id)::int AS answers_count
		    FROM sessions s
		    LEFT JOIN answers a ON a.session_id = s.id
		    WHERE s.user_id = $1 AND s.status = 'completed'
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

	rows, err := r.pool.Query(ctx, query, userID)
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

// TODO(M3): дельту последней попытки удобно считать оконной функцией lag().
func (r *progressRepository) Summary(ctx context.Context, userID uuid.UUID) (domain.Progress, error) {
	_, _ = ctx, userID
	return domain.Progress{}, domain.ErrNotImplemented
}

// TODO(M3): unnest(answers.risk_signal_codes) с группировкой по коду:
// encountered — сколько раз встретился, recognized — сколько раз выбран safe.
func (r *progressRepository) SignalStats(ctx context.Context, userID uuid.UUID) ([]domain.SignalStat, error) {
	_, _ = ctx, userID
	return nil, domain.ErrNotImplemented
}

package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/domain"
)

type scenarioRepository struct {
	pool *pgxpool.Pool
}

var _ ScenarioRepository = (*scenarioRepository)(nil)

// scenarioSelect — карточка сценария и агрегат признаков риска со всех его
// шагов: признаков на самой таблице scenarios нет, они живут на шагах (FR16).
const scenarioSelect = `
	SELECT id, code, version, role, title, description, intro, difficulty,
	       steps_count, estimated_minutes, is_active,
	       COALESCE((
	           SELECT array_agg(DISTINCT sig) FILTER (WHERE sig IS NOT NULL)
	           FROM steps st, unnest(st.risk_signal_codes) AS t(sig)
	           WHERE st.scenario_id = scenarios.id
	       ), ARRAY[]::text[]) AS risk_signal_codes
	FROM scenarios`

// ListActive возвращает активные версии без шагов. Роль опциональна;
// порядок карточек стабильный, чтобы витрина не прыгала между запросами.
func (r *scenarioRepository) ListActive(ctx context.Context, role *domain.Role) ([]domain.Scenario, error) {
	query := scenarioSelect + " WHERE is_active ORDER BY id"
	var args []any

	if role != nil {
		query = scenarioSelect + " WHERE is_active AND role = $1 ORDER BY id"
		args = append(args, string(*role))
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("выбор активных сценариев: %w", err)
	}
	defer rows.Close()

	scenarios := make([]domain.Scenario, 0)
	for rows.Next() {
		scenario, err := scanScenario(rows)
		if err != nil {
			return nil, fmt.Errorf("чтение активного сценария: %w", err)
		}

		scenarios = append(scenarios, scenario)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("перебор активных сценариев: %w", err)
	}

	return scenarios, nil
}

func (r *scenarioRepository) GetActiveByCode(ctx context.Context, code string) (domain.Scenario, error) {
	return r.loadScenario(ctx, "is_active AND code = $1", code)
}

// GetByCodeVersion достаёт конкретную версию сценария: сессия проходится на
// своей версии, чтобы правка контента не меняла разбор (FR32).
func (r *scenarioRepository) GetByCodeVersion(ctx context.Context, code string, version int) (domain.Scenario, error) {
	return r.loadScenario(ctx, "code = $1 AND version = $2", code, version)
}

// ListBySignal возвращает активные сценарии, на шагах которых размечен
// признак риска. Шаги не читаются: карточке справочника достаточно
// метаданных сценария.
func (r *scenarioRepository) ListBySignal(ctx context.Context, signalCode string) ([]domain.Scenario, error) {
	query := scenarioSelect + `
		WHERE is_active AND EXISTS (
		    SELECT 1 FROM steps st
		    WHERE st.scenario_id = scenarios.id AND $1 = ANY(st.risk_signal_codes)
		)
		ORDER BY id`

	rows, err := r.pool.Query(ctx, query, signalCode)
	if err != nil {
		return nil, fmt.Errorf("выбор сценариев по признаку %s: %w", signalCode, err)
	}
	defer rows.Close()

	scenarios := make([]domain.Scenario, 0)

	for rows.Next() {
		scenario, err := scanScenario(rows)
		if err != nil {
			return nil, fmt.Errorf("чтение сценария по признаку %s: %w", signalCode, err)
		}

		scenarios = append(scenarios, scenario)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("перебор сценариев по признаку %s: %w", signalCode, err)
	}

	return scenarios, nil
}

// CountActive — знаменатель строки «пройдено X из Y» на главной (FR25).
func (r *scenarioRepository) CountActive(ctx context.Context) (int, error) {
	var count int

	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM scenarios WHERE is_active`).Scan(&count); err != nil {
		return 0, fmt.Errorf("подсчёт активных сценариев: %w", err)
	}

	return count, nil
}

// Upsert сохраняет сценарий из файла контента.
//
// Версии не переписываются: при изменении содержимого активная версия
// снимается с публикации, а новая добавляется рядом. Сессии ссылаются на
// конкретную версию, поэтому уже завершённые попытки продолжают разбираться
// на том тексте, который видел пользователь (FR32).
//
// Возвращает номер активной версии и признак того, была ли она создана
// сейчас. Совпадение content_hash означает, что контент не менялся, — тогда
// не делается ничего.
func (r *scenarioRepository) Upsert(
	ctx context.Context,
	scenario domain.Scenario,
	contentHash string,
) (int, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, false, fmt.Errorf("начало транзакции загрузки сценария %s: %w", scenario.Code, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var (
		activeVersion int
		activeHash    string
	)

	err = tx.QueryRow(ctx,
		`SELECT version, content_hash FROM scenarios WHERE code = $1 AND is_active`,
		scenario.Code).Scan(&activeVersion, &activeHash)

	switch {
	case err == nil && activeHash == contentHash:
		return activeVersion, false, nil
	case err != nil && !errors.Is(err, pgx.ErrNoRows):
		return 0, false, fmt.Errorf("чтение активной версии сценария %s: %w", scenario.Code, err)
	}

	var maxVersion int
	if err := tx.QueryRow(ctx,
		`SELECT COALESCE(MAX(version), 0) FROM scenarios WHERE code = $1`,
		scenario.Code).Scan(&maxVersion); err != nil {
		return 0, false, fmt.Errorf("чтение версий сценария %s: %w", scenario.Code, err)
	}

	if _, err := tx.Exec(ctx,
		`UPDATE scenarios SET is_active = FALSE WHERE code = $1 AND is_active`,
		scenario.Code); err != nil {
		return 0, false, fmt.Errorf("снятие с публикации сценария %s: %w", scenario.Code, err)
	}

	version := maxVersion + 1

	var scenarioID int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO scenarios (code, version, role, title, description, intro, difficulty,
		                       steps_count, estimated_minutes, is_active, content_hash)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, TRUE, $10)
		RETURNING id`,
		scenario.Code, version, string(scenario.Role), scenario.Title, scenario.Description,
		scenario.Intro, string(scenario.Difficulty), scenario.StepsCount,
		scenario.EstimatedMinutes, contentHash).Scan(&scenarioID); err != nil {
		return 0, false, fmt.Errorf("сохранение сценария %s: %w", scenario.Code, err)
	}

	if err := insertSteps(ctx, tx, scenarioID, scenario); err != nil {
		return 0, false, err
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, false, fmt.Errorf("фиксация загрузки сценария %s: %w", scenario.Code, err)
	}

	return version, true, nil
}

// insertSteps пишет шаги и варианты. Содержимое шага сохраняется как JSONB
// сериализацией доменной структуры — тем же форматом, каким его читает
// loadSteps.
func insertSteps(ctx context.Context, tx pgx.Tx, scenarioID int64, scenario domain.Scenario) error {
	for position, step := range scenario.Steps {
		content, err := json.Marshal(step.Content)
		if err != nil {
			return fmt.Errorf("сериализация содержимого шага %s: %w", step.Code, err)
		}

		var stepID int64
		if err := tx.QueryRow(ctx, `
			INSERT INTO steps (scenario_id, code, type, position, content, risk_signal_codes, is_start)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			RETURNING id`,
			scenarioID, step.Code, string(step.Type), position+1, content,
			step.RiskSignalCodes, step.IsStart).Scan(&stepID); err != nil {
			return fmt.Errorf("сохранение шага %s сценария %s: %w", step.Code, scenario.Code, err)
		}

		for optionPosition, option := range step.Options {
			if _, err := tx.Exec(ctx, `
				INSERT INTO options (step_id, code, text, outcome, score, feedback, next_step_code, position)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
				stepID, option.Code, option.Text, string(option.Outcome), option.Score,
				option.Feedback, nullableStr(option.NextStepCode), optionPosition+1); err != nil {
				return fmt.Errorf("сохранение варианта %s шага %s: %w", option.Code, step.Code, err)
			}
		}
	}

	return nil
}

func nullableStr(value string) any {
	if value == "" {
		return nil
	}

	return value
}

// loadScenario читает сценарий с шагами и вариантами; отсутствие строки —
// domain.ErrNotFound.
func (r *scenarioRepository) loadScenario(
	ctx context.Context,
	condition string,
	args ...any,
) (domain.Scenario, error) {
	scenario, err := scanScenario(r.pool.QueryRow(ctx, scenarioSelect+" WHERE "+condition, args...))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Scenario{}, domain.ErrNotFound
		}

		return domain.Scenario{}, fmt.Errorf("чтение сценария: %w", err)
	}

	if err := r.loadSteps(ctx, &scenario); err != nil {
		return domain.Scenario{}, err
	}

	return scenario, nil
}

// loadSteps читает шаги и варианты одним запросом с JOIN; у терминальных
// шагов вариантов нет, колонки варианта там NULL.
func (r *scenarioRepository) loadSteps(ctx context.Context, scenario *domain.Scenario) error {
	query := `
		SELECT s.id, s.code, s.type, s.position, s.content, s.risk_signal_codes, s.is_start,
		       o.id, o.code, o.text, o.outcome, o.score, o.feedback, o.next_step_code, o.position
		FROM steps s
		LEFT JOIN options o ON o.step_id = s.id
		WHERE s.scenario_id = $1
		ORDER BY s.position, o.position`

	rows, err := r.pool.Query(ctx, query, scenario.ID)
	if err != nil {
		return fmt.Errorf("выбор шагов сценария %s: %w", scenario.Code, err)
	}
	defer rows.Close()

	stepsByID := make(map[int64]*domain.Step)
	ordered := make([]int64, 0)

	for rows.Next() {
		var (
			step           domain.Step
			stepType       string
			content        []byte
			optionID       *int64
			optionCode     *string
			optionText     *string
			optionOutcome  *string
			optionScore    *int
			optionFeedback *string
			optionNext     *string
			optionPosition *int
		)

		if err := rows.Scan(
			&step.ID, &step.Code, &stepType, &step.Position, &content,
			&step.RiskSignalCodes, &step.IsStart,
			&optionID, &optionCode, &optionText, &optionOutcome, &optionScore,
			&optionFeedback, &optionNext, &optionPosition,
		); err != nil {
			return fmt.Errorf("чтение шага сценария %s: %w", scenario.Code, err)
		}

		step.Type = domain.StepType(stepType)

		if err := json.Unmarshal(content, &step.Content); err != nil {
			return fmt.Errorf("разбор контента шага %q: %w", step.Code, err)
		}

		if _, exists := stepsByID[step.ID]; !exists {
			ordered = append(ordered, step.ID)
			stepsByID[step.ID] = &step
		}

		if optionID != nil {
			stepsByID[step.ID].Options = append(stepsByID[step.ID].Options, domain.Option{
				ID:           *optionID,
				StepID:       step.ID,
				Code:         *optionCode,
				Text:         *optionText,
				Outcome:      domain.Outcome(*optionOutcome),
				Score:        *optionScore,
				Feedback:     *optionFeedback,
				NextStepCode: coalesceStr(optionNext),
				Position:     *optionPosition,
			})
		}
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("перебор шагов сценария %s: %w", scenario.Code, err)
	}

	scenario.Steps = make([]domain.Step, 0, len(ordered))
	for _, id := range ordered {
		scenario.Steps = append(scenario.Steps, *stepsByID[id])
	}

	return nil
}

// scanScenario читает строку сценария; перечисления из БД — строки, поэтому
// конвертируем их явно, а не полагаемся на маппинг pgx.
func scanScenario(row pgx.Row) (domain.Scenario, error) {
	var (
		scenario   domain.Scenario
		role       string
		difficulty string
	)

	err := row.Scan(
		&scenario.ID, &scenario.Code, &scenario.Version, &role, &scenario.Title,
		&scenario.Description, &scenario.Intro, &difficulty, &scenario.StepsCount,
		&scenario.EstimatedMinutes, &scenario.IsActive, &scenario.RiskSignalCodes,
	)
	if err != nil {
		return domain.Scenario{}, err
	}

	scenario.Role = domain.Role(role)
	scenario.Difficulty = domain.Difficulty(difficulty)

	return scenario, nil
}

func coalesceStr(value *string) string {
	if value == nil {
		return ""
	}

	return *value
}

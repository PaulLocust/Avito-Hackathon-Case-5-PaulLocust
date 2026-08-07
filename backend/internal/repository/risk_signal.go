package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/domain"
)

type riskSignalRepository struct {
	pool *pgxpool.Pool
}

var _ RiskSignalRepository = (*riskSignalRepository)(nil)

const riskSignalSelect = `
	SELECT code, side, title, summary, description, how_to_recognize, how_to_act
	FROM risk_signals`

// List возвращает справочник, при необходимости — только одну сторону сделки.
// Порядок фиксированный: справочник не должен перетасовываться между
// запросами.
func (r *riskSignalRepository) List(ctx context.Context, side *domain.Side) ([]domain.RiskSignal, error) {
	query := riskSignalSelect + " ORDER BY side, code"
	var args []any

	if side != nil {
		query = riskSignalSelect + " WHERE side = $1 ORDER BY code"
		args = append(args, string(*side))
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("выбор признаков риска: %w", err)
	}
	defer rows.Close()

	signals := make([]domain.RiskSignal, 0)

	for rows.Next() {
		signal, err := scanRiskSignal(rows)
		if err != nil {
			return nil, fmt.Errorf("чтение признака риска: %w", err)
		}

		signals = append(signals, signal)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("перебор признаков риска: %w", err)
	}

	return signals, nil
}

func (r *riskSignalRepository) Get(ctx context.Context, code string) (domain.RiskSignal, error) {
	signal, err := scanRiskSignal(r.pool.QueryRow(ctx, riskSignalSelect+" WHERE code = $1", code))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.RiskSignal{}, domain.ErrNotFound
		}

		return domain.RiskSignal{}, fmt.Errorf("чтение признака риска: %w", err)
	}

	return signal, nil
}

func scanRiskSignal(row pgx.Row) (domain.RiskSignal, error) {
	var (
		signal domain.RiskSignal
		side   string
	)

	err := row.Scan(&signal.Code, &side, &signal.Title, &signal.Summary,
		&signal.Description, &signal.HowToRecognize, &signal.HowToAct)
	if err != nil {
		return domain.RiskSignal{}, err
	}

	signal.Side = domain.Side(side)

	return signal, nil
}

// ListByCodes возвращает признаки риска в порядке запрошенных кодов: шаги
// перечисляют признаки в фиксированном порядке, и ответ не должен его
// менять (FR16).
func (r *riskSignalRepository) ListByCodes(ctx context.Context, codes []string) ([]domain.RiskSignal, error) {
	if len(codes) == 0 {
		return []domain.RiskSignal{}, nil
	}

	rows, err := r.pool.Query(ctx, `
		SELECT code, side, title, summary, description, how_to_recognize, how_to_act
		FROM risk_signals
		WHERE code = ANY($1)`, codes)
	if err != nil {
		return nil, fmt.Errorf("выбор признаков риска: %w", err)
	}
	defer rows.Close()

	byCode := make(map[string]domain.RiskSignal)
	for rows.Next() {
		var (
			signal domain.RiskSignal
			side   string
		)

		if err := rows.Scan(&signal.Code, &side, &signal.Title, &signal.Summary,
			&signal.Description, &signal.HowToRecognize, &signal.HowToAct); err != nil {
			return nil, fmt.Errorf("чтение признака риска: %w", err)
		}

		signal.Side = domain.Side(side)
		byCode[signal.Code] = signal
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("перебор признаков риска: %w", err)
	}

	ordered := make([]domain.RiskSignal, 0, len(codes))
	for _, code := range codes {
		if signal, ok := byCode[code]; ok {
			ordered = append(ordered, signal)
		}
	}

	return ordered, nil
}

// TODO(M5): INSERT ... ON CONFLICT (code) DO UPDATE.
func (r *riskSignalRepository) Upsert(ctx context.Context, signals []domain.RiskSignal) error {
	_, _ = ctx, signals
	return domain.ErrNotImplemented
}

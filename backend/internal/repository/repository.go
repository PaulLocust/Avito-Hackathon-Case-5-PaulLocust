// Package repository — слой доступа к данным. Знает о БД и не знает о HTTP.
// Все запросы параметризованные (SEC4).
package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/domain"
)

// UserRepository — учётные записи и отзыв токенов (M4).
type UserRepository interface {
	// Create возвращает domain.ErrNicknameTaken, если ник занят.
	Create(ctx context.Context, nickname, passwordHash string) (domain.User, error)
	GetByNickname(ctx context.Context, nickname string) (domain.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (domain.User, error)
	RevokeToken(ctx context.Context, jti string, expiresAt int64) error
	IsTokenRevoked(ctx context.Context, jti string) (bool, error)
}

// ScenarioRepository — контент сценариев (M2, M5).
type ScenarioRepository interface {
	// ListActive возвращает активные версии без шагов; role == nil — все роли.
	ListActive(ctx context.Context, role *domain.Role) ([]domain.Scenario, error)
	// GetActiveByCode возвращает сценарий вместе с шагами и вариантами.
	GetActiveByCode(ctx context.Context, code string) (domain.Scenario, error)
	// GetByCodeVersion нужен сессии: она проходится на своей версии (FR32).
	GetByCodeVersion(ctx context.Context, code string, version int) (domain.Scenario, error)
	ListBySignal(ctx context.Context, signalCode string) ([]domain.Scenario, error)
	CountActive(ctx context.Context) (int, error)
	// Upsert создаёт новую версию только при изменении содержимого.
	Upsert(ctx context.Context, scenario domain.Scenario, contentHash string) (version int, created bool, err error)
}

// RiskSignalRepository — справочник признаков риска (M5).
type RiskSignalRepository interface {
	List(ctx context.Context, side *domain.Side) ([]domain.RiskSignal, error)
	Get(ctx context.Context, code string) (domain.RiskSignal, error)
	ListByCodes(ctx context.Context, codes []string) ([]domain.RiskSignal, error)
	Upsert(ctx context.Context, signals []domain.RiskSignal) error
}

// SessionRepository — сессии прохождения и ответы (M2).
type SessionRepository interface {
	Create(ctx context.Context, session domain.Session) (domain.Session, error)
	Get(ctx context.Context, id uuid.UUID) (domain.Session, error)
	GetActiveByUser(ctx context.Context, userID uuid.UUID) (domain.Session, error)
	GetActiveByUserScenario(ctx context.Context, userID uuid.UUID, scenarioCode string) (domain.Session, error)
	// SaveAnswer одной транзакцией фиксирует ответ, начисляет баллы и
	// переводит сессию на следующий шаг; finished завершает её (FR14).
	SaveAnswer(ctx context.Context, answer domain.Answer, nextStepCode string, finished bool) (domain.Session, error)
	// GetAnswer нужен для идемпотентности повторной отправки (FR13).
	GetAnswer(ctx context.Context, sessionID uuid.UUID, stepCode string) (domain.Answer, error)
	ListAnswers(ctx context.Context, sessionID uuid.UUID) ([]domain.Answer, error)
	Abandon(ctx context.Context, id uuid.UUID) error
	ListCompleted(ctx context.Context, userID uuid.UUID, scenarioCode string) ([]domain.Session, error)
	// PreviousCompleted — предыдущая попытка для сравнения результатов (FR23).
	PreviousCompleted(ctx context.Context, userID uuid.UUID, scenarioCode string, before uuid.UUID) (domain.Session, error)
}

// ProgressRepository — агрегаты прогресса (M3), считаются в SQL.
type ProgressRepository interface {
	ScenarioStats(ctx context.Context, userID uuid.UUID) (map[string]domain.UserScenarioStats, error)
	Summary(ctx context.Context, userID uuid.UUID) (domain.Progress, error)
	SignalStats(ctx context.Context, userID uuid.UUID) ([]domain.SignalStat, error)
}

type Repositories struct {
	Users       UserRepository
	Scenarios   ScenarioRepository
	RiskSignals RiskSignalRepository
	Sessions    SessionRepository
	Progress    ProgressRepository
}

func New(pool *pgxpool.Pool) *Repositories {
	return &Repositories{
		Users:       &userRepository{pool: pool},
		Scenarios:   &scenarioRepository{pool: pool},
		RiskSignals: &riskSignalRepository{pool: pool},
		Sessions:    &sessionRepository{pool: pool},
		Progress:    &progressRepository{pool: pool},
	}
}

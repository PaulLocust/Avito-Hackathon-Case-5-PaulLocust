// Package repository — слой доступа к данным. Знает о БД и не знает о HTTP.
// Все запросы параметризованные (SEC4).
package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/domain"
)

//
// ────────────────────────────────────────────────────────────────────────────────
//   AUTH — USERS
// ────────────────────────────────────────────────────────────────────────────────
//

// UserRepository — учётные записи и отзыв токенов (M4).
type UserRepository interface {
	// Create возвращает domain.ErrNicknameTaken, если ник занят.
	Create(ctx context.Context, nickname, passwordHash string) (domain.User, error)
	GetByNickname(ctx context.Context, nickname string) (domain.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (domain.User, error)

	// JWT blacklist
	RevokeToken(ctx context.Context, jti string, expiresAt int64) error
	IsTokenRevoked(ctx context.Context, jti string) (bool, error)
}

//
// ────────────────────────────────────────────────────────────────────────────────
//   AUTH — REFRESH TOKENS
// ────────────────────────────────────────────────────────────────────────────────
//

type RefreshToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	SessionID uuid.UUID
	Hash      string
	ExpiresAt time.Time
	CreatedAt time.Time
}

type RefreshTokenRepository interface {
	Create(ctx context.Context, t RefreshToken) error
	GetByHash(ctx context.Context, hash string) (RefreshToken, error)
	DeleteByID(ctx context.Context, id uuid.UUID) error
	DeleteBySessionID(ctx context.Context, sessionID uuid.UUID) error
}

//
// ────────────────────────────────────────────────────────────────────────────────
//   AUTH — GUEST SESSIONS
// ────────────────────────────────────────────────────────────────────────────────
//

type GuestSession struct {
	ID        uuid.UUID
	TokenHash string
	ExpiresAt time.Time
	CreatedAt time.Time
}

type GuestRepository interface {
	Create(ctx context.Context, g GuestSession) error
	GetByHash(ctx context.Context, hash string) (GuestSession, error)
}

//
// ────────────────────────────────────────────────────────────────────────────────
//   SCENARIOS (M2, M5)
// ────────────────────────────────────────────────────────────────────────────────
//

type ScenarioRepository interface {
	ListActive(ctx context.Context, role *domain.Role) ([]domain.Scenario, error)
	GetActiveByCode(ctx context.Context, code string) (domain.Scenario, error)
	GetByCodeVersion(ctx context.Context, code string, version int) (domain.Scenario, error)
	ListBySignal(ctx context.Context, signalCode string) ([]domain.Scenario, error)
	CountActive(ctx context.Context) (int, error)
	Upsert(ctx context.Context, scenario domain.Scenario, contentHash string) (version int, created bool, err error)
}

//
// ────────────────────────────────────────────────────────────────────────────────
//   RISK SIGNALS (M5)
// ────────────────────────────────────────────────────────────────────────────────
//

type RiskSignalRepository interface {
	List(ctx context.Context, side *domain.Side) ([]domain.RiskSignal, error)
	Get(ctx context.Context, code string) (domain.RiskSignal, error)
	ListByCodes(ctx context.Context, codes []string) ([]domain.RiskSignal, error)
	Upsert(ctx context.Context, signals []domain.RiskSignal) error
}
type SessionRepository interface {
	Create(ctx context.Context, session domain.Session) (domain.Session, error)
	Get(ctx context.Context, id uuid.UUID) (domain.Session, error)
	GetActiveByOwner(ctx context.Context, owner domain.Owner) (domain.Session, error)
	GetActiveByOwnerScenario(ctx context.Context, owner domain.Owner, scenarioCode string) (domain.Session, error)
	SaveAnswer(ctx context.Context, answer domain.Answer, nextStepCode string, finished bool) (domain.Session, error)
	GetAnswer(ctx context.Context, sessionID uuid.UUID, stepCode string) (domain.Answer, error)
	ListAnswers(ctx context.Context, sessionID uuid.UUID) ([]domain.Answer, error)
	Abandon(ctx context.Context, id uuid.UUID) error
	ListCompleted(ctx context.Context, owner domain.Owner, scenarioCode string) ([]domain.Session, error)
	PreviousCompleted(ctx context.Context, owner domain.Owner, scenarioCode string, before uuid.UUID) (domain.Session, error)
	// ClaimByGuest переносит все сессии гостя на аккаунт после Register/Login:
	// UPDATE sessions SET user_id = $2, guest_session_id = NULL WHERE guest_session_id = $1
	ClaimByGuest(ctx context.Context, guestSessionID, userID uuid.UUID) error
}

//
// ────────────────────────────────────────────────────────────────────────────────
//   PROGRESS (M3)
// ────────────────────────────────────────────────────────────────────────────────
//

type ProgressRepository interface {
	ScenarioStats(ctx context.Context, userID uuid.UUID) (map[string]domain.UserScenarioStats, error)
	Summary(ctx context.Context, userID uuid.UUID) (domain.Progress, error)
	SignalStats(ctx context.Context, userID uuid.UUID) ([]domain.SignalStat, error)
}

//
// ────────────────────────────────────────────────────────────────────────────────
//   AGGREGATOR
// ────────────────────────────────────────────────────────────────────────────────
//

type Repositories struct {
	Users         UserRepository
	RefreshTokens RefreshTokenRepository
	Guests        GuestRepository

	Scenarios   ScenarioRepository
	RiskSignals RiskSignalRepository
	Sessions    SessionRepository
	Progress    ProgressRepository
}

func New(pool *pgxpool.Pool) *Repositories {
	return &Repositories{
		Users:         &userRepository{pool: pool},
		RefreshTokens: &refreshTokenRepository{pool: pool},
		Guests:        &guestRepository{pool: pool},

		Scenarios:   &scenarioRepository{pool: pool},
		RiskSignals: &riskSignalRepository{pool: pool},
		Sessions:    &sessionRepository{pool: pool},
		Progress:    &progressRepository{pool: pool},
	}
}

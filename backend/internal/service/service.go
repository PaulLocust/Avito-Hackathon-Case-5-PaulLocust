// Package service — слой бизнес-логики. Знает о репозиториях и домене,
// не знает о HTTP.
package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/config"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/domain"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/repository"
)

type Token struct {
	Value     string
	ExpiresAt time.Time
}

type GuestSessionToken struct {
	Value     string
	ExpiresAt time.Time
	OwnerID   uuid.UUID
}

// TokenPair — access (JWT, короткий TTL) + refresh (opaque, длинный TTL,
// хранится в БД только в виде хэша). Access передаётся клиентом в
// Authorization, refresh — в HttpOnly cookie refresh_token с Path=/api/v1/auth.
type TokenPair struct {
	Access  Token
	Refresh Token
}

// AuthService — регистрация, вход, рефреш и проверка токена (M4).
type AuthService interface {
	Register(ctx context.Context, nickname, password string) (domain.User, TokenPair, error)
	Login(ctx context.Context, nickname, password string) (domain.User, TokenPair, error)
	// Refresh ротирует refresh-токен: старый инвалидируется, выдаётся новая пара.
	Refresh(ctx context.Context, refreshToken string) (domain.User, TokenPair, error)
	// Logout отзывает access JWT и убивает refresh-токены всей сессии логина.
	Logout(ctx context.Context, accessToken string) error
	// Authenticate вызывается middleware транспорта, принимает access-токен.
	Authenticate(ctx context.Context, accessToken string) (domain.User, error)
	// ClaimGuest переносит прогресс гостя (по токену из guest_session куки)
	// на только что аутентифицированного пользователя. Вызывается хендлером
	// сразу после успешного Register/Login, если в запросе была гостевая кука.
	// Реализация зависит от SessionRepository (см. session.go, M2) —
	// пока не реализована, возвращает domain.ErrNotImplemented.
	ClaimGuest(ctx context.Context, userID uuid.UUID, guestToken string) error
}

// GuestService — анонимные сессии по куке guest_session (M4/M2).
type GuestService interface {
	// Start создаёт новую гостевую сессию и возвращает токен для куки.
	Start(ctx context.Context) (GuestSessionToken, error)
	// Validate проверяет токен из куки и возвращает id гостевой сессии
	// (repository.GuestSession.ID) — используйте его как владельца
	// domain.Session, пока нет юзера.
	Validate(ctx context.Context, guestToken string) (uuid.UUID, error)
}

// CatalogService — витрина сценариев (M2, M3).
type CatalogService interface {
	// List: userID == nil — запрос от гостя, карточки без прогресса.
	List(ctx context.Context, userID *uuid.UUID, role *domain.Role) ([]domain.ScenarioCard, error)
	Get(ctx context.Context, userID *uuid.UUID, code string) (domain.ScenarioCard, error)
}

// TrainingService — движок прохождения (M2). Владелец сессии — либо
// авторизованный юзер, либо гость (domain.Owner) — оба могут проходить
// сценарии и сохранять прогресс; аналитика (ProgressService) при этом
// доступна только реальным юзерам.
type TrainingService interface {
	// Start возвращает *domain.ActiveSessionError, если по сценарию есть
	// незавершённая сессия и restart == false.
	Start(ctx context.Context, owner domain.Owner, scenarioCode string, restart bool) (domain.SessionSnapshot, error)
	// Get: чужая сессия (owner не совпадает с владельцем) — domain.ErrNotFound (SEC2).
	Get(ctx context.Context, owner domain.Owner, sessionID uuid.UUID) (domain.SessionSnapshot, error)
	// SubmitAnswer на уже отвеченный шаг возвращает сохранённый результат
	// с AlreadyAnswered = true и не меняет состояние (FR13).
	SubmitAnswer(ctx context.Context, owner domain.Owner, sessionID uuid.UUID, stepCode, optionCode string) (domain.AnswerOutcome, error)
	Abandon(ctx context.Context, owner domain.Owner, sessionID uuid.UUID) error
	Result(ctx context.Context, owner domain.Owner, sessionID uuid.UUID) (domain.Debrief, error)
}

// ProgressService — прогресс и история (M3).
type ProgressService interface {
	Overview(ctx context.Context, userID uuid.UUID) (domain.Progress, error)
	Signals(ctx context.Context, userID uuid.UUID) ([]domain.SignalStat, error)
	Attempts(ctx context.Context, userID uuid.UUID, scenarioCode string) ([]domain.Attempt, error)
}

// ReferenceService — справочник признаков риска (M5).
type ReferenceService interface {
	ListSignals(ctx context.Context, side *domain.Side) ([]domain.RiskSignal, error)
	GetSignal(ctx context.Context, code string) (domain.RiskSignalDetail, error)
}

// ContentService — загрузка контента (M5), вызывается из cmd/seed.
type ContentService interface {
	LoadFromDir(ctx context.Context, dir string) (LoadReport, error)
}

// LoadReport — отчёт о загрузке. Непустой Issues означает, что сценарии не
// загружены, а ранее загруженные версии не изменились.
type LoadReport struct {
	SignalsLoaded    int
	ScenariosCreated []string
	ScenariosUpdated []string
	ScenariosSkipped []string
	Issues           map[string][]domain.Issue
}

func (r LoadReport) Failed() bool { return len(r.Issues) > 0 }

type Services struct {
	Auth      AuthService
	Guest     GuestService
	Catalog   CatalogService
	Training  TrainingService
	Progress  ProgressService
	Reference ReferenceService
	Content   ContentService
}

func New(repos *repository.Repositories, cfg config.Config) *Services {
	guest := &guestService{guests: repos.Guests, cfg: cfg.Auth}

	return &Services{
		Auth: &authService{
			users:    repos.Users,
			refresh:  repos.RefreshTokens,
			sessions: repos.Sessions,
			guests:   guest,
			cfg:      cfg.Auth,
		},
		Guest:     guest,
		Catalog:   &catalogService{scenarios: repos.Scenarios, progress: repos.Progress, sessions: repos.Sessions, thresholds: cfg.Scoring.Thresholds},
		Training:  &trainingService{sessions: repos.Sessions, scenarios: repos.Scenarios, signals: repos.RiskSignals, thresholds: cfg.Scoring.Thresholds},
		Progress:  &progressService{progress: repos.Progress, sessions: repos.Sessions, scenarios: repos.Scenarios, thresholds: cfg.Scoring.Thresholds},
		Reference: &referenceService{signals: repos.RiskSignals, scenarios: repos.Scenarios},
		Content:   &contentService{scenarios: repos.Scenarios, signals: repos.RiskSignals},
	}
}

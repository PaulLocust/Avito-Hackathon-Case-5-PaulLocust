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

// AuthService — регистрация, вход и проверка токена (M4).
type AuthService interface {
	Register(ctx context.Context, nickname, password string) (domain.User, Token, error)
	Login(ctx context.Context, nickname, password string) (domain.User, Token, error)
	Logout(ctx context.Context, token string) error
	// Authenticate вызывается middleware транспорта.
	Authenticate(ctx context.Context, token string) (domain.User, error)
}

// CatalogService — витрина сценариев (M2, M3).
type CatalogService interface {
	// List: userID == nil — запрос от гостя, карточки без прогресса.
	List(ctx context.Context, userID *uuid.UUID, role *domain.Role) ([]domain.ScenarioCard, error)
	Get(ctx context.Context, userID *uuid.UUID, code string) (domain.ScenarioCard, error)
}

// TrainingService — движок прохождения (M2).
type TrainingService interface {
	// Start возвращает *domain.ActiveSessionError, если по сценарию есть
	// незавершённая сессия и restart == false.
	Start(ctx context.Context, userID uuid.UUID, scenarioCode string, restart bool) (domain.SessionSnapshot, error)
	// Get: чужая сессия — domain.ErrNotFound (SEC2).
	Get(ctx context.Context, userID, sessionID uuid.UUID) (domain.SessionSnapshot, error)
	// SubmitAnswer на уже отвеченный шаг возвращает сохранённый результат
	// с AlreadyAnswered = true и не меняет состояние (FR13).
	SubmitAnswer(ctx context.Context, userID, sessionID uuid.UUID, stepCode, optionCode string) (domain.AnswerOutcome, error)
	Abandon(ctx context.Context, userID, sessionID uuid.UUID) error
	Result(ctx context.Context, userID, sessionID uuid.UUID) (domain.Debrief, error)
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
	Catalog   CatalogService
	Training  TrainingService
	Progress  ProgressService
	Reference ReferenceService
	Content   ContentService
}

func New(repos *repository.Repositories, cfg config.Config) *Services {
	return &Services{
		Auth:      &authService{users: repos.Users, cfg: cfg.Auth},
		Catalog:   &catalogService{scenarios: repos.Scenarios, progress: repos.Progress, sessions: repos.Sessions},
		Training:  &trainingService{sessions: repos.Sessions, scenarios: repos.Scenarios, signals: repos.RiskSignals, thresholds: cfg.Scoring.Thresholds},
		Progress:  &progressService{progress: repos.Progress, sessions: repos.Sessions, scenarios: repos.Scenarios, thresholds: cfg.Scoring.Thresholds},
		Reference: &referenceService{signals: repos.RiskSignals, scenarios: repos.Scenarios},
		Content:   &contentService{scenarios: repos.Scenarios, signals: repos.RiskSignals},
	}
}

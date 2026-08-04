package domain

import (
	"time"

	"github.com/google/uuid"
)

type SessionStatus string

const (
	StatusInProgress SessionStatus = "in_progress"
	StatusCompleted  SessionStatus = "completed"
	// StatusAbandoned — пользователь начал заново; в прогрессе не учитывается.
	StatusAbandoned SessionStatus = "abandoned"
)

type User struct {
	ID           uuid.UUID
	Nickname     string
	PasswordHash string // bcrypt (SEC1)
	CreatedAt    time.Time
}

// Session — попытка прохождения.
//
// Score равен сумме ScoreDelta ответов: фиксация ответа и обновление балла
// идут одной транзакцией. ScenarioVersion фиксируется при старте, чтобы
// правка контента не меняла завершённые сессии (FR32).
type Session struct {
	ID              uuid.UUID
	UserID          uuid.UUID
	ScenarioID      int64
	ScenarioCode    string
	ScenarioVersion int
	Status          SessionStatus
	CurrentStepCode string
	Score           int
	StartedAt       time.Time
	FinishedAt      *time.Time
}

func (s Session) Active() bool { return s.Status == StatusInProgress }

// Answer — зафиксированный выбор. Пара (SessionID, StepCode) уникальна:
// изменить сделанный выбор нельзя (FR13).
//
// Outcome и ScoreDelta хранятся в самом ответе, а не вычисляются из контента
// при чтении: разбор остаётся воспроизводимым после выхода новой версии
// сценария.
type Answer struct {
	ID              int64
	SessionID       uuid.UUID
	StepCode        string
	OptionCode      string
	Outcome         Outcome
	ScoreDelta      int
	RiskSignalCodes []string
	Position        int
	AnsweredAt      time.Time
}

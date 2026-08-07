package domain

import (
	"time"

	"github.com/google/uuid"
)

type SessionStatus string

const (
	StatusInProgress SessionStatus = "in_progress"
	StatusCompleted  SessionStatus = "completed"
	StatusAbandoned  SessionStatus = "abandoned"
)

type User struct {
	ID           uuid.UUID
	Email        string
	Nickname     string
	PasswordHash string // bcrypt (SEC1)
	CreatedAt    time.Time
	Role         string
}

// Session — попытка прохождения. Owner — либо авторизованный юзер, либо
// гость (см. owner.go); в БД это две nullable-колонки (user_id,
// guest_session_id) с CHECK "ровно одна заполнена" — репозиторий сам
// разворачивает Owner в эту пару и обратно (см. ownerColumns/ownerWhere
// в repository/session.go).
type Session struct {
	ID              uuid.UUID
	Owner           Owner
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

// Answer — зафиксированный выбор.
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

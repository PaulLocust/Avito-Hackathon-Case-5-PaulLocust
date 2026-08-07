package domain

import (
	"time"

	"github.com/google/uuid"
)

// Составные значения, которыми сервисы отвечают транспорту. Транспорт
// переводит их в DTO по контракту OpenAPI.

// UserScenarioStats — прогресс пользователя по одному сценарию.
//
// BestScore и BestAnswers заполняет репозиторий, а BestPercent и BestLevel
// считает сервис через domain.EvaluateTotals: пороги живут в конфигурации,
// а формула — в одном месте (FR22).
type UserScenarioStats struct {
	Attempted     bool
	AttemptsCount int
	BestScore     int
	BestAnswers   int
	BestPercent   *int
	BestLevel     *SecurityLevel
	LastAttemptAt *time.Time
}

type ScenarioCard struct {
	Scenario        Scenario
	Stats           *UserScenarioStats // nil для гостя
	ActiveSessionID *uuid.UUID
}

// SessionSnapshot — состояние сессии и текущий шаг для экрана прохождения.
type SessionSnapshot struct {
	Session      Session
	Scenario     Scenario // без Steps: только метаданные для шапки экрана
	CurrentStep  *Step    // nil, если сессия завершена или прервана
	AnswersCount int
	StepsTotal   int
}

// AnswerOutcome — результат фиксации выбора: обратная связь и новое состояние.
type AnswerOutcome struct {
	Answer          Answer
	Option          Option
	RiskSignals     []RiskSignal
	SafeAlternative *Option // nil, если выбран безопасный вариант
	// AlreadyAnswered — вернули сохранённый результат, состояние не менялось.
	AlreadyAnswered bool
	SessionFinished bool
	Snapshot        SessionSnapshot
}

type SecurityLevel string

const (
	LevelResilient  SecurityLevel = "resilient"
	LevelAttentive  SecurityLevel = "attentive"
	LevelVulnerable SecurityLevel = "vulnerable"
)

type Result struct {
	Score        int
	MinScore     int
	MaxScore     int
	Percent      int
	Level        SecurityLevel
	AnswersCount int
}

// Comparison — сравнение с предыдущей попыткой по тому же сценарию (FR23).
type Comparison struct {
	PreviousPercent    int
	PreviousScore      int
	DeltaPercent       int
	PreviousFinishedAt time.Time
}

type BreakdownItem struct {
	Order           int
	StepCode        string
	Situation       string
	Chosen          Option
	ScoreDelta      int
	SafeAlternative *Option
	RiskSignals     []RiskSignal
}

type SignalOutcome struct {
	Signal          RiskSignal
	Recognized      bool
	StepsTotal      int
	StepsRecognized int
}

type NextStepType string

const (
	NextStepNewScenario    NextStepType = "new_scenario"
	NextStepRetryScenario  NextStepType = "retry_scenario"
	NextStepExploreSignals NextStepType = "explore_signals"
	NextStepAllDone        NextStepType = "all_done"
)

// NextStep — предложение следующего шага обучения (FR27).
type NextStep struct {
	Type     NextStepType
	Scenario *Scenario
	Reason   string
}

// Debrief — экран результата целиком.
type Debrief struct {
	Session         Session
	Scenario        Scenario
	Result          Result
	Comparison      *Comparison // nil, если попытка первая
	Breakdown       []BreakdownItem
	Signals         []SignalOutcome
	Recommendations []string
	NextStep        NextStep
}

type Attempt struct {
	SessionID       uuid.UUID
	ScenarioCode    string
	ScenarioVersion int
	Score           int
	Percent         int
	Level           SecurityLevel
	FinishedAt      time.Time
}

// ActiveSession — блок «продолжить тренировку» на главной (FR12).
type ActiveSession struct {
	SessionID    uuid.UUID
	Scenario     Scenario
	AnswersCount int
	StepsTotal   int
	StartedAt    time.Time
}

type Progress struct {
	CompletedScenarios int
	TotalScenarios     int
	AttemptsCount      int
	BestPercent        *int
	BestLevel          *SecurityLevel
	AveragePercent     *int
	// LastDeltaPercent — динамика показывается сравнением с предыдущей
	// попыткой, а не временным рядом.
	LastDeltaPercent *int
	ActiveSession    *ActiveSession
	NextStep         NextStep
}

type SignalStatus string

const (
	SignalMastered SignalStatus = "mastered"
	SignalWeak     SignalStatus = "weak"
	SignalUnknown  SignalStatus = "unknown"
)

// SignalStat — статистика по признаку риска (FR26). Метрика независима от
// балла: можно набрать 75% и стабильно проваливать один и тот же признак.
type SignalStat struct {
	Signal      RiskSignal
	Encountered int
	Recognized  int
	Missed      int
	Status      SignalStatus
}

type RiskSignalDetail struct {
	Signal           RiskSignal
	RelatedScenarios []Scenario
}

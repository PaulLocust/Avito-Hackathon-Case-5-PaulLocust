package dto

import (
	"time"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/domain"
)

type Attempt struct {
	SessionID       string    `json:"session_id"`
	ScenarioCode    string    `json:"scenario_code"`
	ScenarioVersion int       `json:"scenario_version"`
	Score           int       `json:"score"`
	Percent         int       `json:"percent"`
	Level           string    `json:"level"`
	FinishedAt      time.Time `json:"finished_at"`
}

type AttemptListResponse struct {
	Items []Attempt `json:"items"`
}

// ActiveSession — блок «продолжить тренировку» на главной (FR12).
type ActiveSession struct {
	SessionID    string      `json:"session_id"`
	Scenario     ScenarioRef `json:"scenario"`
	AnswersCount int         `json:"answers_count"`
	StepsTotal   int         `json:"steps_total"`
	StartedAt    time.Time   `json:"started_at"`
}

type ProgressResponse struct {
	CompletedScenarios int                `json:"completed_scenarios"`
	TotalScenarios     int                `json:"total_scenarios"`
	AttemptsCount      int                `json:"attempts_count"`
	BestPercent        *int               `json:"best_percent"`
	BestLevel          *string            `json:"best_level"`
	AveragePercent     *int               `json:"average_percent"`
	LastDeltaPercent   *int               `json:"last_delta_percent"`
	ActiveSession      *ActiveSession     `json:"active_session"`
	NextStep           NextStepSuggestion `json:"next_step"`
}

type SignalProgressItem struct {
	Code        string `json:"code"`
	Title       string `json:"title"`
	Side        string `json:"side"`
	Encountered int    `json:"encountered"`
	Recognized  int    `json:"recognized"`
	Missed      int    `json:"missed"`
	Status      string `json:"status"`
}

type SignalProgressResponse struct {
	Items []SignalProgressItem `json:"items"`
}

func NewAttemptList(attempts []domain.Attempt) AttemptListResponse {
	items := make([]Attempt, 0, len(attempts))
	for _, attempt := range attempts {
		items = append(items, Attempt{
			SessionID:       attempt.SessionID.String(),
			ScenarioCode:    attempt.ScenarioCode,
			ScenarioVersion: attempt.ScenarioVersion,
			Score:           attempt.Score,
			Percent:         attempt.Percent,
			Level:           string(attempt.Level),
			FinishedAt:      attempt.FinishedAt,
		})
	}

	return AttemptListResponse{Items: items}
}

func NewProgress(progress domain.Progress) ProgressResponse {
	response := ProgressResponse{
		CompletedScenarios: progress.CompletedScenarios,
		TotalScenarios:     progress.TotalScenarios,
		AttemptsCount:      progress.AttemptsCount,
		BestPercent:        progress.BestPercent,
		AveragePercent:     progress.AveragePercent,
		LastDeltaPercent:   progress.LastDeltaPercent,
		NextStep:           NewNextStep(progress.NextStep),
	}

	if progress.BestLevel != nil {
		level := string(*progress.BestLevel)
		response.BestLevel = &level
	}

	if progress.ActiveSession != nil {
		response.ActiveSession = &ActiveSession{
			SessionID:    progress.ActiveSession.SessionID.String(),
			Scenario:     NewScenarioRef(progress.ActiveSession.Scenario),
			AnswersCount: progress.ActiveSession.AnswersCount,
			StepsTotal:   progress.ActiveSession.StepsTotal,
			StartedAt:    progress.ActiveSession.StartedAt,
		}
	}

	return response
}

func NewSignalProgress(stats []domain.SignalStat) SignalProgressResponse {
	items := make([]SignalProgressItem, 0, len(stats))
	for _, stat := range stats {
		items = append(items, SignalProgressItem{
			Code:        stat.Signal.Code,
			Title:       stat.Signal.Title,
			Side:        string(stat.Signal.Side),
			Encountered: stat.Encountered,
			Recognized:  stat.Recognized,
			Missed:      stat.Missed,
			Status:      string(stat.Status),
		})
	}

	return SignalProgressResponse{Items: items}
}

package dto

import (
	"time"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/domain"
)

type ScenarioRef struct {
	Code       string `json:"code"`
	Title      string `json:"title"`
	Role       string `json:"role"`
	Difficulty string `json:"difficulty"`
	Version    int    `json:"version"`
}

// ScenarioCard — поля прогресса заполняются только для авторизованного
// пользователя (FR4).
type ScenarioCard struct {
	ScenarioRef
	Description     string     `json:"description"`
	StepsCount      int        `json:"steps_count"`
	RiskSignalCodes []string   `json:"risk_signal_codes,omitempty"`
	Attempted       *bool      `json:"attempted,omitempty"`
	BestPercent     *int       `json:"best_percent,omitempty"`
	BestLevel       *string    `json:"best_level,omitempty"`
	AttemptsCount   *int       `json:"attempts_count,omitempty"`
	LastAttemptAt   *time.Time `json:"last_attempt_at,omitempty"`
}

type ScenarioDetail struct {
	ScenarioCard
	Intro            string  `json:"intro,omitempty"`
	EstimatedMinutes int     `json:"estimated_minutes,omitempty"`
	ActiveSessionID  *string `json:"active_session_id,omitempty"`
}

type ScenarioListResponse struct {
	Items []ScenarioCard `json:"items"`
}

func NewScenarioRef(scenario domain.Scenario) ScenarioRef {
	return ScenarioRef{
		Code:       scenario.Code,
		Title:      scenario.Title,
		Role:       string(scenario.Role),
		Difficulty: string(scenario.Difficulty),
		Version:    scenario.Version,
	}
}

func NewScenarioCard(card domain.ScenarioCard) ScenarioCard {
	result := ScenarioCard{
		ScenarioRef:     NewScenarioRef(card.Scenario),
		Description:     card.Scenario.Description,
		StepsCount:      card.Scenario.StepsCount,
		RiskSignalCodes: card.Scenario.RiskSignalCodes,
	}

	if card.Stats != nil {
		attempted := card.Stats.Attempted
		attempts := card.Stats.AttemptsCount

		result.Attempted = &attempted
		result.AttemptsCount = &attempts
		result.BestPercent = card.Stats.BestPercent
		result.LastAttemptAt = card.Stats.LastAttemptAt

		if card.Stats.BestLevel != nil {
			level := string(*card.Stats.BestLevel)
			result.BestLevel = &level
		}
	}

	return result
}

func NewScenarioDetail(card domain.ScenarioCard) ScenarioDetail {
	result := ScenarioDetail{
		ScenarioCard:     NewScenarioCard(card),
		Intro:            card.Scenario.Intro,
		EstimatedMinutes: card.Scenario.EstimatedMinutes,
	}

	if card.ActiveSessionID != nil {
		id := card.ActiveSessionID.String()
		result.ActiveSessionID = &id
	}

	return result
}

func NewScenarioList(cards []domain.ScenarioCard) ScenarioListResponse {
	items := make([]ScenarioCard, 0, len(cards))
	for _, card := range cards {
		items = append(items, NewScenarioCard(card))
	}

	return ScenarioListResponse{Items: items}
}

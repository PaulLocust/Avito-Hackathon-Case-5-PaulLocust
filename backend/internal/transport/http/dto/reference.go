package dto

import (
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/domain"
)

type RiskSignal struct {
	Code           string   `json:"code"`
	Side           string   `json:"side"`
	Title          string   `json:"title"`
	Summary        string   `json:"summary"`
	Description    string   `json:"description"`
	HowToRecognize []string `json:"how_to_recognize,omitempty"`
	HowToAct       string   `json:"how_to_act"`
}

type RiskSignalDetail struct {
	RiskSignal
	RelatedScenarios []ScenarioRef `json:"related_scenarios,omitempty"`
}

type RiskSignalListResponse struct {
	Items []RiskSignal `json:"items"`
}

func NewRiskSignal(signal domain.RiskSignal) RiskSignal {
	return RiskSignal{
		Code:           signal.Code,
		Side:           string(signal.Side),
		Title:          signal.Title,
		Summary:        signal.Summary,
		Description:    signal.Description,
		HowToRecognize: signal.HowToRecognize,
		HowToAct:       signal.HowToAct,
	}
}

func NewRiskSignalList(signals []domain.RiskSignal) RiskSignalListResponse {
	items := make([]RiskSignal, 0, len(signals))
	for _, signal := range signals {
		items = append(items, NewRiskSignal(signal))
	}

	return RiskSignalListResponse{Items: items}
}

func NewRiskSignalDetail(detail domain.RiskSignalDetail) RiskSignalDetail {
	scenarios := make([]ScenarioRef, 0, len(detail.RelatedScenarios))
	for _, scenario := range detail.RelatedScenarios {
		scenarios = append(scenarios, NewScenarioRef(scenario))
	}

	return RiskSignalDetail{
		RiskSignal:       NewRiskSignal(detail.Signal),
		RelatedScenarios: scenarios,
	}
}

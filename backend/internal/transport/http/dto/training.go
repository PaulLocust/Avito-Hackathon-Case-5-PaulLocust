package dto

import (
	"time"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/domain"
)

type StartSessionRequest struct {
	ScenarioCode string `json:"scenario_code"`
	Restart      bool   `json:"restart"`
}

func (r StartSessionRequest) Validate() error {
	if r.ScenarioCode == "" {
		return domain.NewValidationError("scenario_code", "Укажите сценарий")
	}

	return nil
}

type SubmitAnswerRequest struct {
	StepCode   string `json:"step_code"`
	OptionCode string `json:"option_code"`
}

func (r SubmitAnswerRequest) Validate() error {
	validationErr := &domain.ValidationError{}

	if r.StepCode == "" {
		validationErr.Add("step_code", "Укажите шаг")
	}

	if r.OptionCode == "" {
		validationErr.Add("option_code", "Укажите вариант ответа")
	}

	if validationErr.Empty() {
		return nil
	}

	return validationErr
}

type Attachment struct {
	Kind    string `json:"kind"`
	Caption string `json:"caption"`
}

type StepContent struct {
	Message    string      `json:"message"`
	Sender     string      `json:"sender,omitempty"`
	Context    string      `json:"context,omitempty"`
	Attachment *Attachment `json:"attachment,omitempty"`
}

// OptionView — вариант до выбора. Ни outcome, ни вес, ни feedback здесь не
// передаются: иначе безопасный вариант вычисляется из ответа API.
type OptionView struct {
	Code string `json:"code"`
	Text string `json:"text"`
}

// StepView — признаки риска шага раскрываются только после выбора (FR16).
type StepView struct {
	Code     string       `json:"code"`
	Type     string       `json:"type"`
	Position int          `json:"position"`
	Content  StepContent  `json:"content"`
	Options  []OptionView `json:"options"`
}

type SessionState struct {
	ID           string      `json:"id"`
	Scenario     ScenarioRef `json:"scenario"`
	Status       string      `json:"status"`
	CurrentStep  *StepView   `json:"current_step"`
	Score        int         `json:"score"`
	AnswersCount int         `json:"answers_count"`
	StepsTotal   int         `json:"steps_total"`
	StartedAt    time.Time   `json:"started_at"`
	FinishedAt   *time.Time  `json:"finished_at"`
}

// RiskSignalRef — ссылка на карточку справочника (FR29).
type RiskSignalRef struct {
	Code  string `json:"code"`
	Title string `json:"title"`
	Side  string `json:"side"`
}

// SafeAlternative — как следовало поступить; nil при безопасном выборе.
type SafeAlternative struct {
	OptionCode string `json:"option_code"`
	Text       string `json:"text"`
	Feedback   string `json:"feedback"`
}

// AnswerResult — обратная связь и следующий шаг одним ответом.
type AnswerResult struct {
	StepCode        string           `json:"step_code"`
	OptionCode      string           `json:"option_code"`
	Outcome         string           `json:"outcome"`
	ScoreDelta      int              `json:"score_delta"`
	Feedback        string           `json:"feedback"`
	RiskSignals     []RiskSignalRef  `json:"risk_signals"`
	SafeAlternative *SafeAlternative `json:"safe_alternative"`
	AlreadyAnswered bool             `json:"already_answered"`
	SessionFinished bool             `json:"session_finished"`
	Session         SessionState     `json:"session"`
}

type ScoreSummary struct {
	Score        int    `json:"score"`
	MinScore     int    `json:"min_score"`
	MaxScore     int    `json:"max_score"`
	Percent      int    `json:"percent"`
	Level        string `json:"level"`
	AnswersCount int    `json:"answers_count"`
}

// AttemptComparison — сравнение с предыдущей попыткой (FR23).
type AttemptComparison struct {
	PreviousPercent    int       `json:"previous_percent"`
	PreviousScore      int       `json:"previous_score"`
	DeltaPercent       int       `json:"delta_percent"`
	PreviousFinishedAt time.Time `json:"previous_finished_at"`
}

type ChosenOption struct {
	OptionCode string `json:"option_code"`
	Text       string `json:"text"`
	Outcome    string `json:"outcome"`
	ScoreDelta int    `json:"score_delta"`
	Feedback   string `json:"feedback"`
}

// BreakdownItem — строка пошагового разбора (FR18).
type BreakdownItem struct {
	Order           int              `json:"order"`
	StepCode        string           `json:"step_code"`
	Situation       string           `json:"situation"`
	Chosen          ChosenOption     `json:"chosen"`
	SafeAlternative *SafeAlternative `json:"safe_alternative"`
	RiskSignals     []RiskSignalRef  `json:"risk_signals"`
}

type SessionSignalOutcome struct {
	Code            string `json:"code"`
	Title           string `json:"title"`
	Side            string `json:"side"`
	Recognized      bool   `json:"recognized"`
	StepsTotal      int    `json:"steps_total"`
	StepsRecognized int    `json:"steps_recognized"`
}

type NextStepSuggestion struct {
	Type     string       `json:"type"`
	Scenario *ScenarioRef `json:"scenario"`
	Reason   string       `json:"reason"`
}

type SessionResult struct {
	SessionID       string                 `json:"session_id"`
	Scenario        ScenarioRef            `json:"scenario"`
	Score           ScoreSummary           `json:"score"`
	Comparison      *AttemptComparison     `json:"comparison"`
	Breakdown       []BreakdownItem        `json:"breakdown"`
	Signals         []SessionSignalOutcome `json:"signals"`
	Recommendations []string               `json:"recommendations"`
	NextStep        NextStepSuggestion     `json:"next_step"`
	StartedAt       time.Time              `json:"started_at"`
	FinishedAt      time.Time              `json:"finished_at"`
	DurationSeconds int                    `json:"duration_seconds"`
}

func NewStepView(step domain.Step, position int) StepView {
	options := make([]OptionView, 0, len(step.Options))
	for _, option := range step.Options {
		options = append(options, OptionView{Code: option.Code, Text: option.Text})
	}

	content := StepContent{
		Message: step.Content.Message,
		Sender:  string(step.Content.Sender),
		Context: step.Content.Context,
	}

	if step.Content.Attachment != nil {
		content.Attachment = &Attachment{
			Kind:    step.Content.Attachment.Kind,
			Caption: step.Content.Attachment.Caption,
		}
	}

	return StepView{
		Code:     step.Code,
		Type:     string(step.Type),
		Position: position,
		Content:  content,
		Options:  options,
	}
}

func NewSessionState(snapshot domain.SessionSnapshot) SessionState {
	state := SessionState{
		ID:           snapshot.Session.ID.String(),
		Scenario:     NewScenarioRef(snapshot.Scenario),
		Status:       string(snapshot.Session.Status),
		Score:        snapshot.Session.Score,
		AnswersCount: snapshot.AnswersCount,
		StepsTotal:   snapshot.StepsTotal,
		StartedAt:    snapshot.Session.StartedAt,
		FinishedAt:   snapshot.Session.FinishedAt,
	}

	if snapshot.CurrentStep != nil {
		step := NewStepView(*snapshot.CurrentStep, domain.PathPosition(snapshot.AnswersCount))
		state.CurrentStep = &step
	}

	return state
}

func NewRiskSignalRefs(signals []domain.RiskSignal) []RiskSignalRef {
	refs := make([]RiskSignalRef, 0, len(signals))
	for _, signal := range signals {
		refs = append(refs, RiskSignalRef{
			Code:  signal.Code,
			Title: signal.Title,
			Side:  string(signal.Side),
		})
	}

	return refs
}

func newSafeAlternative(option *domain.Option) *SafeAlternative {
	if option == nil {
		return nil
	}

	return &SafeAlternative{
		OptionCode: option.Code,
		Text:       option.Text,
		Feedback:   option.Feedback,
	}
}

func NewAnswerResult(outcome domain.AnswerOutcome) AnswerResult {
	return AnswerResult{
		StepCode:        outcome.Answer.StepCode,
		OptionCode:      outcome.Answer.OptionCode,
		Outcome:         string(outcome.Answer.Outcome),
		ScoreDelta:      outcome.Answer.ScoreDelta,
		Feedback:        outcome.Option.Feedback,
		RiskSignals:     NewRiskSignalRefs(outcome.RiskSignals),
		SafeAlternative: newSafeAlternative(outcome.SafeAlternative),
		AlreadyAnswered: outcome.AlreadyAnswered,
		SessionFinished: outcome.SessionFinished,
		Session:         NewSessionState(outcome.Snapshot),
	}
}

func NewSessionResult(debrief domain.Debrief) SessionResult {
	breakdown := make([]BreakdownItem, 0, len(debrief.Breakdown))
	for _, item := range debrief.Breakdown {
		breakdown = append(breakdown, BreakdownItem{
			Order:     item.Order,
			StepCode:  item.StepCode,
			Situation: item.Situation,
			Chosen: ChosenOption{
				OptionCode: item.Chosen.Code,
				Text:       item.Chosen.Text,
				Outcome:    string(item.Chosen.Outcome),
				ScoreDelta: item.ScoreDelta,
				Feedback:   item.Chosen.Feedback,
			},
			SafeAlternative: newSafeAlternative(item.SafeAlternative),
			RiskSignals:     NewRiskSignalRefs(item.RiskSignals),
		})
	}

	signals := make([]SessionSignalOutcome, 0, len(debrief.Signals))
	for _, signal := range debrief.Signals {
		signals = append(signals, SessionSignalOutcome{
			Code:            signal.Signal.Code,
			Title:           signal.Signal.Title,
			Side:            string(signal.Signal.Side),
			Recognized:      signal.Recognized,
			StepsTotal:      signal.StepsTotal,
			StepsRecognized: signal.StepsRecognized,
		})
	}

	recommendations := debrief.Recommendations
	if recommendations == nil {
		recommendations = []string{}
	}

	result := SessionResult{
		SessionID: debrief.Session.ID.String(),
		Scenario:  NewScenarioRef(debrief.Scenario),
		Score: ScoreSummary{
			Score:        debrief.Result.Score,
			MinScore:     debrief.Result.MinScore,
			MaxScore:     debrief.Result.MaxScore,
			Percent:      debrief.Result.Percent,
			Level:        string(debrief.Result.Level),
			AnswersCount: debrief.Result.AnswersCount,
		},
		Breakdown:       breakdown,
		Signals:         signals,
		Recommendations: recommendations,
		NextStep:        NewNextStep(debrief.NextStep),
		StartedAt:       debrief.Session.StartedAt,
	}

	if debrief.Comparison != nil {
		result.Comparison = &AttemptComparison{
			PreviousPercent:    debrief.Comparison.PreviousPercent,
			PreviousScore:      debrief.Comparison.PreviousScore,
			DeltaPercent:       debrief.Comparison.DeltaPercent,
			PreviousFinishedAt: debrief.Comparison.PreviousFinishedAt,
		}
	}

	if debrief.Session.FinishedAt != nil {
		result.FinishedAt = *debrief.Session.FinishedAt
		result.DurationSeconds = int(result.FinishedAt.Sub(result.StartedAt).Seconds())
	}

	return result
}

func NewNextStep(next domain.NextStep) NextStepSuggestion {
	suggestion := NextStepSuggestion{
		Type:   string(next.Type),
		Reason: next.Reason,
	}

	if next.Scenario != nil {
		ref := NewScenarioRef(*next.Scenario)
		suggestion.Scenario = &ref
	}

	return suggestion
}

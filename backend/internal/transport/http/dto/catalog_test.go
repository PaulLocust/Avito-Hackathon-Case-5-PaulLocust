package dto_test

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/domain"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/transport/http/dto"
)

func scenario() domain.Scenario {
	return domain.Scenario{
		Code:             "too-good-price",
		Version:          1,
		Role:             domain.RoleBuyer,
		Title:            "Слишком выгодная цена",
		Description:      "Смартфон продают вдвое дешевле рынка.",
		Intro:            "Вы покупатель.",
		Difficulty:       domain.DifficultyDemo,
		StepsCount:       3,
		EstimatedMinutes: 2,
		RiskSignalCodes:  []string{"RISK_TOO_GOOD_PRICE"},
	}
}

func marshal(t *testing.T, payload any) map[string]any {
	t.Helper()

	raw, err := json.Marshal(payload)
	require.NoError(t, err)

	// UseNumber: числа сравниваются точно, а не через float64.
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()

	var decoded map[string]any
	require.NoError(t, decoder.Decode(&decoded))

	return decoded
}

func number(t *testing.T, payload map[string]any, field string) int {
	t.Helper()

	raw, ok := payload[field].(json.Number)
	require.True(t, ok, "поле %s должно быть числом", field)

	value, err := raw.Int64()
	require.NoError(t, err)

	return int(value)
}

// Гостю блок прогресса не отдаётся вовсе: полей нет в ответе, а не
// присутствуют с нулевыми значениями (FR4).
func TestScenarioCardForGuestOmitsProgress(t *testing.T) {
	card := marshal(t, dto.NewScenarioCard(domain.ScenarioCard{Scenario: scenario()}))

	require.Equal(t, "too-good-price", card["code"])
	require.Equal(t, 3, number(t, card, "steps_count"))

	for _, field := range []string{"attempted", "best_percent", "best_level", "attempts_count", "last_attempt_at"} {
		require.NotContains(t, card, field, "гостю поле %s не полагается", field)
	}
}

func TestScenarioCardForUserCarriesProgress(t *testing.T) {
	lastAttempt := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)
	percent := 83
	level := domain.LevelResilient

	card := marshal(t, dto.NewScenarioCard(domain.ScenarioCard{
		Scenario: scenario(),
		Stats: &domain.UserScenarioStats{
			Attempted:     true,
			AttemptsCount: 2,
			BestScore:     20,
			BestAnswers:   3,
			BestPercent:   &percent,
			BestLevel:     &level,
			LastAttemptAt: &lastAttempt,
		},
	}))

	require.Equal(t, true, card["attempted"])
	require.Equal(t, 2, number(t, card, "attempts_count"))
	require.Equal(t, 83, number(t, card, "best_percent"))
	require.Equal(t, "resilient", card["best_level"])
	require.Equal(t, "2026-08-03T12:00:00Z", card["last_attempt_at"])

	// Сырой агрегат — внутренняя кухня оценки, наружу он не уходит.
	require.NotContains(t, card, "best_score")
	require.NotContains(t, card, "best_answers")
}

// Пользователь без попыток получает блок прогресса с отметкой, что сценарий
// ещё не проходился, а не молчание.
func TestScenarioCardForUserWithoutAttempts(t *testing.T) {
	card := marshal(t, dto.NewScenarioCard(domain.ScenarioCard{
		Scenario: scenario(),
		Stats:    &domain.UserScenarioStats{},
	}))

	require.Equal(t, false, card["attempted"])
	require.Equal(t, 0, number(t, card, "attempts_count"))
	require.NotContains(t, card, "best_percent")
	require.NotContains(t, card, "best_level")
}

func TestScenarioDetailCarriesActiveSession(t *testing.T) {
	sessionID := uuid.New()

	detail := marshal(t, dto.NewScenarioDetail(domain.ScenarioCard{
		Scenario:        scenario(),
		ActiveSessionID: &sessionID,
	}))

	require.Equal(t, "Вы покупатель.", detail["intro"])
	require.Equal(t, 2, number(t, detail, "estimated_minutes"))
	require.Equal(t, sessionID.String(), detail["active_session_id"])
}

func TestScenarioDetailWithoutActiveSession(t *testing.T) {
	detail := marshal(t, dto.NewScenarioDetail(domain.ScenarioCard{Scenario: scenario()}))

	require.NotContains(t, detail, "active_session_id")
}

// Пустая витрина должна приезжать как [], иначе фронтенд получает null
// вместо списка и падает на переборе.
func TestScenarioListEmptyIsArray(t *testing.T) {
	raw, err := json.Marshal(dto.NewScenarioList(nil))

	require.NoError(t, err)
	require.JSONEq(t, `{"items":[]}`, string(raw))
}

func TestRiskSignalDetailCarriesRelatedScenarios(t *testing.T) {
	detail := marshal(t, dto.NewRiskSignalDetail(domain.RiskSignalDetail{
		Signal: domain.RiskSignal{
			Code:     "RISK_TOO_GOOD_PRICE",
			Side:     domain.SideBuyer,
			Title:    "Аномально низкая цена",
			Summary:  "Цена ниже рынка как приманка.",
			HowToAct: "Сравните цену с другими объявлениями.",
		},
		RelatedScenarios: []domain.Scenario{scenario()},
	}))

	require.Equal(t, "RISK_TOO_GOOD_PRICE", detail["code"])
	require.Equal(t, "buyer", detail["side"])

	related, ok := detail["related_scenarios"].([]any)
	require.True(t, ok)
	require.Len(t, related, 1)

	first, ok := related[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "too-good-price", first["code"])
}

func TestRiskSignalListEmptyIsArray(t *testing.T) {
	raw, err := json.Marshal(dto.NewRiskSignalList(nil))

	require.NoError(t, err)
	require.JSONEq(t, `{"items":[]}`, string(raw))
}

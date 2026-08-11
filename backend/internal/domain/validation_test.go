package domain_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/domain"
)

// validScenario — минимальный корректный сценарий: три шага диалога,
// сходящихся в терминальный. Тесты мутируют копию и проверяют, что валидатор
// ловит нарушение.
func validScenario() domain.Scenario {
	dialog := func(code, next string, start bool) domain.Step {
		return domain.Step{
			Code:            code,
			Type:            domain.StepTypeDialog,
			IsStart:         start,
			RiskSignalCodes: []string{"RISK_TOO_GOOD_PRICE"},
			Content:         domain.StepContent{Message: "Реплика контрагента"},
			Options: []domain.Option{
				{Code: "a", Text: "Опасно", Outcome: domain.OutcomeCritical, Score: -10, Feedback: "…", NextStepCode: next},
				{Code: "b", Text: "Спорно", Outcome: domain.OutcomeRisky, Score: 0, Feedback: "…", NextStepCode: next},
				{Code: "c", Text: "Безопасно", Outcome: domain.OutcomeSafe, Score: 10, Feedback: "…", NextStepCode: next},
			},
		}
	}

	return domain.Scenario{
		Code:       "demo",
		Version:    1,
		Role:       domain.RoleBuyer,
		Difficulty: domain.DifficultyDemo,
		Title:      "Демонстрационный сценарий",
		Steps: []domain.Step{
			dialog("s1", "s2", true),
			dialog("s2", "s3", false),
			dialog("s3", "end", false),
			{
				Code:            "end",
				Type:            domain.StepTypeTerminal,
				RiskSignalCodes: []string{"RISK_TOO_GOOD_PRICE"},
				Content:         domain.StepContent{Message: "Разговор окончен"},
			},
		},
	}
}

func knownSignals() map[string]domain.RiskSignal {
	return map[string]domain.RiskSignal{
		"RISK_TOO_GOOD_PRICE": {Code: "RISK_TOO_GOOD_PRICE", Side: domain.SideBuyer},
	}
}

func TestValidateScenario(t *testing.T) {

	tests := []struct {
		name    string
		mutate  func(*domain.Scenario)
		wantErr bool
	}{
		{
			name:    "корректный сценарий",
			mutate:  func(*domain.Scenario) {},
			wantErr: false,
		},
		{
			name:    "меньше трёх шагов диалога",
			mutate:  func(s *domain.Scenario) { s.Steps = s.Steps[2:] },
			wantErr: true,
		},
		{
			name:    "нет терминального шага",
			mutate:  func(s *domain.Scenario) { s.Steps = s.Steps[:3] },
			wantErr: true,
		},
		{
			name:    "нет стартового шага",
			mutate:  func(s *domain.Scenario) { s.Steps[0].IsStart = false },
			wantErr: true,
		},
		{
			name:    "вариант ведёт на несуществующий шаг",
			mutate:  func(s *domain.Scenario) { s.Steps[0].Options[0].NextStepCode = "s99" },
			wantErr: true,
		},
		{
			name:    "недостижимый шаг",
			mutate:  func(s *domain.Scenario) { s.Steps[1].Code = "orphan" },
			wantErr: true,
		},
		{
			name:    "два варианта вместо трёх",
			mutate:  func(s *domain.Scenario) { s.Steps[0].Options = s.Steps[0].Options[:2] },
			wantErr: true,
		},
		{
			name: "нет безопасного варианта",
			mutate: func(s *domain.Scenario) {
				s.Steps[0].Options[2].Outcome = domain.OutcomeRisky
				s.Steps[0].Options[2].Score = 0
			},
			wantErr: true,
		},
		{
			name:    "вес не соответствует outcome",
			mutate:  func(s *domain.Scenario) { s.Steps[0].Options[2].Score = 5 },
			wantErr: true,
		},
		{
			name:    "шаг не размечен признаком риска",
			mutate:  func(s *domain.Scenario) { s.Steps[1].RiskSignalCodes = nil },
			wantErr: true,
		},
		{
			name:    "ссылка на признак риска вне каталога",
			mutate:  func(s *domain.Scenario) { s.Steps[1].RiskSignalCodes = []string{"RISK_UNKNOWN"} },
			wantErr: true,
		},
		{
			name:    "пустая обратная связь",
			mutate:  func(s *domain.Scenario) { s.Steps[0].Options[0].Feedback = "" },
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			scenario := validScenario()
			test.mutate(&scenario)

			issues := domain.ValidateScenario(scenario, knownSignals())

			if test.wantErr {
				require.NotEmpty(t, issues, "валидатор должен был найти нарушение")
				require.NotEmpty(t, issues[0].Path, "в отчёте должно быть указано место ошибки")
			} else {
				require.Empty(t, issues)
			}
		})
	}
}

func TestResolveNext(t *testing.T) {
	step := validScenario().Steps[0]

	t.Run("вариант найден", func(t *testing.T) {
		option, next, err := domain.ResolveNext(step, "c")

		require.NoError(t, err)
		require.Equal(t, domain.OutcomeSafe, option.Outcome)
		require.Equal(t, "s2", next)
	})

	t.Run("варианта нет на шаге", func(t *testing.T) {
		_, _, err := domain.ResolveNext(step, "z")

		require.ErrorIs(t, err, domain.ErrOptionNotFound)
	})
}

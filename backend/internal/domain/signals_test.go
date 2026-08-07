package domain_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/domain"
)

func TestLevelFor(t *testing.T) {
	thresholds := domain.Thresholds{Resilient: 80, Attentive: 60}

	tests := []struct {
		name    string
		percent int
		want    domain.SecurityLevel
	}{
		{name: "сто процентов", percent: 100, want: domain.LevelResilient},
		{name: "ровно нижняя граница устойчивого", percent: 80, want: domain.LevelResilient},
		{name: "чуть ниже устойчивого", percent: 79, want: domain.LevelAttentive},
		{name: "ровно нижняя граница внимательного", percent: 60, want: domain.LevelAttentive},
		{name: "чуть ниже внимательного", percent: 59, want: domain.LevelVulnerable},
		{name: "ноль", percent: 0, want: domain.LevelVulnerable},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.want, domain.LevelFor(test.percent, thresholds))
		})
	}
}

func TestRecognizedSignals(t *testing.T) {
	tooGood := domain.RiskSignal{Code: "RISK_TOO_GOOD_PRICE", Side: domain.SideBuyer}
	prepay := domain.RiskSignal{Code: "RISK_PREPAY_OUTSIDE", Side: domain.SideBuyer}

	answered := func(step string, outcome domain.Outcome, codes ...string) domain.Answer {
		return domain.Answer{
			StepCode:        step,
			Outcome:         outcome,
			ScoreDelta:      outcome.Score(),
			RiskSignalCodes: codes,
			Position:        1,
		}
	}

	t.Run("признак распознан на всех шагах", func(t *testing.T) {
		answers := []domain.Answer{
			answered("s1", domain.OutcomeSafe, "RISK_TOO_GOOD_PRICE"),
			answered("s2", domain.OutcomeSafe, "RISK_TOO_GOOD_PRICE", "RISK_PREPAY_OUTSIDE"),
		}

		outcomes := domain.RecognizedSignals(answers, []domain.RiskSignal{tooGood, prepay})

		require.Len(t, outcomes, 2)
		require.True(t, outcomes[0].Recognized)
		require.Equal(t, 2, outcomes[0].StepsTotal)
		require.Equal(t, 2, outcomes[0].StepsRecognized)
	})

	t.Run("признак пропущен на одном из шагов", func(t *testing.T) {
		answers := []domain.Answer{
			answered("s1", domain.OutcomeCritical, "RISK_TOO_GOOD_PRICE"),
			answered("s2", domain.OutcomeSafe, "RISK_TOO_GOOD_PRICE"),
		}

		outcomes := domain.RecognizedSignals(answers, []domain.RiskSignal{tooGood})

		require.Len(t, outcomes, 1)
		require.False(t, outcomes[0].Recognized)
		require.Equal(t, 1, outcomes[0].StepsRecognized)
	})

	t.Run("не встретившийся в ответах признак не попадает в результат", func(t *testing.T) {
		answers := []domain.Answer{answered("s1", domain.OutcomeSafe, "RISK_TOO_GOOD_PRICE")}

		outcomes := domain.RecognizedSignals(answers, []domain.RiskSignal{tooGood, prepay})

		require.Len(t, outcomes, 1)
		require.Equal(t, "RISK_TOO_GOOD_PRICE", outcomes[0].Signal.Code)
	})

	t.Run("без ответов и сигналов", func(t *testing.T) {
		require.Empty(t, domain.RecognizedSignals(nil, nil))
	})
}

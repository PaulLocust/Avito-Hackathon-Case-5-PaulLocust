package domain_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/domain"
)

// Обязательные тест-кейсы оценки зафиксированы в разделе 9 анализа.
// Таблица заполнена заранее и служит спецификацией для реализации:
// после того как domain.Evaluate написан, снимите t.Skip.

func answers(outcomes ...domain.Outcome) []domain.Answer {
	result := make([]domain.Answer, 0, len(outcomes))
	for i, outcome := range outcomes {
		result = append(result, domain.Answer{
			StepCode:   string(rune('a' + i)),
			Outcome:    outcome,
			ScoreDelta: outcome.Score(),
			Position:   i + 1,
		})
	}
	return result
}

func TestEvaluate(t *testing.T) {
	thresholds := domain.Thresholds{Resilient: 80, Attentive: 60}

	const (
		safe     = domain.OutcomeSafe
		risky    = domain.OutcomeRisky
		critical = domain.OutcomeCritical
	)

	tests := []struct {
		name        string
		answers     []domain.Answer
		wantScore   int
		wantPercent int
		wantLevel   domain.SecurityLevel
	}{
		{
			name:        "все выборы безопасны",
			answers:     answers(safe, safe, safe, safe, safe, safe),
			wantScore:   60,
			wantPercent: 100,
			wantLevel:   domain.LevelResilient,
		},
		{
			name:        "все выборы спорные: уклонение от решения не является безопасным поведением",
			answers:     answers(risky, risky, risky, risky, risky, risky),
			wantScore:   0,
			wantPercent: 50,
			wantLevel:   domain.LevelVulnerable,
		},
		{
			name:        "все выборы опасны",
			answers:     answers(critical, critical, critical, critical, critical, critical),
			wantScore:   -60,
			wantPercent: 0,
			wantLevel:   domain.LevelVulnerable,
		},
		{
			name:        "пример первой попытки из раздела 9 анализа",
			answers:     answers(risky, safe, critical, safe, critical, safe),
			wantScore:   10,
			wantPercent: 58,
			wantLevel:   domain.LevelVulnerable,
		},
		{
			// 110/120 = 91,67 → 92 при округлении. Ровно этим значением
			// раздел 9 анализа считает дельту второй попытки (+34 п.п.).
			name:        "пример второй попытки из раздела 9 анализа",
			answers:     answers(safe, safe, safe, safe, safe, risky),
			wantScore:   50,
			wantPercent: 92,
			wantLevel:   domain.LevelResilient,
		},
		{
			name:        "нижняя граница уровня «устойчив»: ровно 80%",
			answers:     answers(safe, safe, safe, safe, safe, safe, risky, risky, risky, risky),
			wantScore:   60,
			wantPercent: 80,
			wantLevel:   domain.LevelResilient,
		},
		{
			name:        "нижняя граница уровня «внимателен»: ровно 60%",
			answers:     answers(safe, safe, safe, critical, risky, risky, risky, risky, risky, risky),
			wantScore:   20,
			wantPercent: 60,
			wantLevel:   domain.LevelAttentive,
		},
		{
			name:        "незавершённая сессия: нормировка от числа сделанных выборов",
			answers:     answers(safe, risky),
			wantScore:   10,
			wantPercent: 75,
			wantLevel:   domain.LevelAttentive,
		},
		{
			name:        "демонстрационный сценарий: одна спорная реплика из трёх",
			answers:     answers(safe, safe, risky),
			wantScore:   20,
			wantPercent: 83,
			wantLevel:   domain.LevelResilient,
		},
		{
			// 40/60 = 66,67 → 67 при округлении. Целочисленное деление дало бы
			// 66; уровень тот же, но на витрине и в дельте цифры разошлись бы
			// с примерами из анализа. Округление — часть контракта оценки.
			name:        "демонстрационный сценарий: один опасный выбор из трёх",
			answers:     answers(safe, safe, critical),
			wantScore:   10,
			wantPercent: 67,
			wantLevel:   domain.LevelAttentive,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := domain.Evaluate(test.answers, thresholds)

			require.Equal(t, test.wantScore, got.Score)
			require.Equal(t, test.wantPercent, got.Percent)
			require.Equal(t, test.wantLevel, got.Level)
			require.Equal(t, len(test.answers), got.AnswersCount)
		})
	}
}

// Защита от деления на ноль: сессия без ответов не должна ронять сервис.
func TestEvaluateEmptyAnswers(t *testing.T) {
	got := domain.Evaluate(nil, domain.Thresholds{Resilient: 80, Attentive: 60})

	require.Equal(t, 0, got.AnswersCount)
	require.Equal(t, 0, got.Percent)
}

func TestWeightsScale(t *testing.T) {
	// Шкала весов — часть контракта контента: её изменение делает
	// несопоставимыми результаты, полученные до и после правки.
	require.Equal(t, 10, domain.OutcomeSafe.Score())
	require.Equal(t, 0, domain.OutcomeRisky.Score())
	require.Equal(t, -10, domain.OutcomeCritical.Score())
	require.Len(t, domain.Weights, 3)
}

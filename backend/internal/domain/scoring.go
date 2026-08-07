package domain

// Расчёт оценки (модуль M3). Чистые функции: ни БД, ни HTTP, ни структуры
// сценария — только список ответов. На них набирается покрытие (MNT5).

// Thresholds — границы уровней безопасности, приходят из конфигурации (FR21).
//
// Нижний порог 60, а не 50, намеренно: при 50 пользователь, который только
// затягивал диалог, попадал бы во «внимателен».
type Thresholds struct {
	Resilient int // по умолчанию 80
	Attentive int // по умолчанию 60
}

// Evaluate считает итог прохождения:
//
//	n       = количество выборов
//	score   = Σ весов
//	percent = round((score + 10n) × 100 / 20n)
//
// Нормировка от числа сделанных выборов, а не от «лучшего пути»: при
// ветвлении ветки разной длины, и лучший путь был бы неоднозначен.
// Округление арифметическое: 110/120 даёт 92%, как в примерах анализа.
// Пустой список ответов возвращает нулевой Result.
func Evaluate(answers []Answer, thresholds Thresholds) Result {
	score := 0
	for _, answer := range answers {
		score += answer.ScoreDelta
	}

	return EvaluateTotals(score, len(answers), thresholds)
}

// EvaluateTotals — та же оценка по готовому агрегату: сумме баллов и числу
// выборов. Нужна там, где попытки уже свёрнуты запросом (витрина, прогресс),
// чтобы формула оставалась в одном месте (FR22).
func EvaluateTotals(score, answersCount int, thresholds Thresholds) Result {
	if answersCount <= 0 {
		return Result{}
	}

	minScore, maxScore := -10*answersCount, 10*answersCount
	percent := roundPercent((score-minScore)*100, maxScore-minScore)

	return Result{
		Score:        score,
		MinScore:     minScore,
		MaxScore:     maxScore,
		Percent:      percent,
		Level:        LevelFor(percent, thresholds),
		AnswersCount: answersCount,
	}
}

// roundPercent округляет num/denom арифметически (половина — вверх).
// 4000/60 = 66,67 → 67, 11000/120 = 91,67 → 92, как в разделе 9 анализа.
func roundPercent(num, denom int) int {
	return (2*num + denom) / (2 * denom)
}

// LevelFor: >= Resilient — «устойчив», >= Attentive — «внимателен», иначе
// «уязвим».
func LevelFor(percent int, thresholds Thresholds) SecurityLevel {
	if percent >= thresholds.Resilient {
		return LevelResilient
	}

	if percent >= thresholds.Attentive {
		return LevelAttentive
	}

	return LevelVulnerable
}

// RecognizedSignals раскладывает признаки риска на распознанные и
// пропущенные. Признак распознан, если на всех шагах с ним выбран safe.
// Признаки, не встретившиеся в ответах, в результат не попадают.
func RecognizedSignals(answers []Answer, signals []RiskSignal) []SignalOutcome {
	outcomes := make([]SignalOutcome, 0, len(signals))

	for _, signal := range signals {
		total, recognized := 0, 0

		for _, answer := range answers {
			if !contains(answer.RiskSignalCodes, signal.Code) {
				continue
			}

			total++

			if answer.Outcome == OutcomeSafe {
				recognized++
			}
		}

		if total == 0 {
			continue
		}

		outcomes = append(outcomes, SignalOutcome{
			Signal:          signal,
			Recognized:      recognized == total,
			StepsTotal:      total,
			StepsRecognized: recognized,
		})
	}

	return outcomes
}

func contains(codes []string, code string) bool {
	for _, candidate := range codes {
		if candidate == code {
			return true
		}
	}

	return false
}

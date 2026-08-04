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
//
// TODO(M3): реализовать, тесты — в scoring_test.go.
func Evaluate(answers []Answer, thresholds Thresholds) Result {
	_ = answers
	_ = thresholds

	return Result{}
}

// LevelFor: >= Resilient — «устойчив», >= Attentive — «внимателен», иначе
// «уязвим».
//
// TODO(M3): реализовать.
func LevelFor(percent int, thresholds Thresholds) SecurityLevel {
	_ = percent
	_ = thresholds

	return LevelVulnerable
}

// RecognizedSignals раскладывает признаки риска на распознанные и
// пропущенные. Признак распознан, если на всех шагах с ним выбран safe.
//
// TODO(M3): реализовать, используется в разборе (FR18) и статистике (FR26).
func RecognizedSignals(answers []Answer, signals []RiskSignal) []SignalOutcome {
	_ = answers
	_ = signals

	return nil
}

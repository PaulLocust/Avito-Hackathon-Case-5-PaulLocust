package domain

// Движок ветвления (модуль M2). Следующий шаг определяется только
// содержимым сценария и кодом выбранного варианта.

// ResolveNext возвращает выбранный вариант и код следующего шага.
// Если варианта нет на шаге — ErrOptionNotFound.
//
// TODO(M2): реализовать.
func ResolveNext(step Step, optionCode string) (Option, string, error) {
	_ = step
	_ = optionCode

	return Option{}, "", ErrNotImplemented
}

func IsTerminal(step Step) bool { return step.Type == StepTypeTerminal }

// PathPosition — номер текущего шага для индикатора «шаг X из Y».
func PathPosition(answersCount int) int { return answersCount + 1 }

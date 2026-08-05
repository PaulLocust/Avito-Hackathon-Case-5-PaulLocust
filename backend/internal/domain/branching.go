package domain

// Движок ветвления (модуль M2). Следующий шаг определяется только
// содержимым сценария и кодом выбранного варианта.

// ResolveNext возвращает выбранный вариант и код следующего шага.
// Если варианта нет на шаге — ErrOptionNotFound.
func ResolveNext(step Step, optionCode string) (Option, string, error) {
	option, ok := step.FindOption(optionCode)
	if !ok {
		return Option{}, "", ErrOptionNotFound
	}

	return option, option.NextStepCode, nil
}

func IsTerminal(step Step) bool { return step.Type == StepTypeTerminal }

// PathPosition — номер текущего шага для индикатора «шаг X из Y».
func PathPosition(answersCount int) int { return answersCount + 1 }

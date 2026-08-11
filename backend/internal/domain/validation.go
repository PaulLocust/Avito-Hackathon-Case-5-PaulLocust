package domain

import "fmt"

// Валидатор сценариев (модуль M5). Проверяет структуру; содержание
// (правдоподобность вариантов, тон обратной связи) проверяется на ревью.

// Issue — нарушение правила. Path указывает место в файле сценария.
type Issue struct {
	Path    string // например: steps[2].options[0].next_step
	Message string
}

// Границы структуры сценария из правил написания контента.
const (
	minDialogSteps = 3
	maxDialogSteps = 8
)

// ValidateScenario возвращает все найденные нарушения; пустой срез означает,
// что сценарий пригоден к загрузке. knownSignals — каталог признаков риска.
//
// Проверяются только структурные правила: они одинаковы для любого контента и
// ловят ошибки, из-за которых прохождение сломалось бы уже на пользователе —
// висячие ссылки на шаги, недостижимые ветки, неверные веса.
func ValidateScenario(scenario Scenario, knownSignals map[string]RiskSignal) []Issue {
	issues := validateScenarioMeta(scenario)
	issues = append(issues, validateSteps(scenario, knownSignals)...)
	issues = append(issues, validateGraph(scenario)...)

	return issues
}

func validateScenarioMeta(scenario Scenario) []Issue {
	var issues []Issue

	if scenario.Code == "" {
		issues = append(issues, Issue{Path: "code", Message: "код сценария обязателен"})
	}

	if scenario.Title == "" {
		issues = append(issues, Issue{Path: "title", Message: "название обязательно"})
	}

	if !scenario.Role.Valid() {
		issues = append(issues, Issue{
			Path:    "role",
			Message: fmt.Sprintf("недопустимая роль %q, ожидается buyer или seller", scenario.Role),
		})
	}

	if !scenario.Difficulty.Valid() {
		issues = append(issues, Issue{
			Path:    "difficulty",
			Message: fmt.Sprintf("недопустимая сложность %q", scenario.Difficulty),
		})
	}

	dialogs, terminals, starts := 0, 0, 0
	for _, step := range scenario.Steps {
		switch step.Type {
		case StepTypeDialog:
			dialogs++
		case StepTypeTerminal:
			terminals++
		}

		if step.IsStart {
			starts++
		}
	}

	if dialogs < minDialogSteps || dialogs > maxDialogSteps {
		issues = append(issues, Issue{
			Path:    "steps",
			Message: fmt.Sprintf("шагов диалога %d, ожидается от %d до %d", dialogs, minDialogSteps, maxDialogSteps),
		})
	}

	if terminals == 0 {
		issues = append(issues, Issue{Path: "steps", Message: "нет ни одного терминального шага"})
	}

	if starts != 1 {
		issues = append(issues, Issue{
			Path:    "start_step",
			Message: fmt.Sprintf("стартовых шагов %d, должен быть ровно один", starts),
		})
	}

	return issues
}

func validateSteps(scenario Scenario, knownSignals map[string]RiskSignal) []Issue {
	var issues []Issue

	seen := make(map[string]struct{}, len(scenario.Steps))

	for index, step := range scenario.Steps {
		path := fmt.Sprintf("steps[%d]", index)

		if step.Code == "" {
			issues = append(issues, Issue{Path: path + ".code", Message: "код шага обязателен"})
		}

		if _, duplicate := seen[step.Code]; duplicate {
			issues = append(issues, Issue{
				Path:    path + ".code",
				Message: fmt.Sprintf("код шага %q встречается несколько раз", step.Code),
			})
		}
		seen[step.Code] = struct{}{}

		if !step.Type.Valid() {
			issues = append(issues, Issue{
				Path:    path + ".type",
				Message: fmt.Sprintf("недопустимый тип шага %q", step.Type),
			})
		}

		if step.Content.Message == "" {
			issues = append(issues, Issue{Path: path + ".content.message", Message: "текст шага обязателен"})
		}

		issues = append(issues, validateStepSignals(step, path, knownSignals)...)

		if step.Type == StepTypeDialog {
			issues = append(issues, validateOptions(step, path)...)
		}
	}

	return issues
}

func validateStepSignals(step Step, path string, knownSignals map[string]RiskSignal) []Issue {
	var issues []Issue

	if len(step.RiskSignalCodes) == 0 {
		issues = append(issues, Issue{
			Path:    path + ".risk_signals",
			Message: "шаг должен быть размечен хотя бы одним признаком риска",
		})
	}

	for signalIndex, code := range step.RiskSignalCodes {
		if _, ok := knownSignals[code]; !ok {
			issues = append(issues, Issue{
				Path:    fmt.Sprintf("%s.risk_signals[%d]", path, signalIndex),
				Message: fmt.Sprintf("признак %q отсутствует в каталоге", code),
			})
		}
	}

	return issues
}

// validateOptions проверяет сетку вариантов: ровно три, по одному на каждое
// последствие, вес соответствует последствию, обратная связь не пуста.
func validateOptions(step Step, path string) []Issue {
	var issues []Issue

	if len(step.Options) != len(Weights) {
		issues = append(issues, Issue{
			Path:    path + ".options",
			Message: fmt.Sprintf("вариантов %d, ожидается ровно %d", len(step.Options), len(Weights)),
		})
	}

	byOutcome := make(map[Outcome]int, len(Weights))

	for optionIndex, option := range step.Options {
		optionPath := fmt.Sprintf("%s.options[%d]", path, optionIndex)

		if !option.Outcome.Valid() {
			issues = append(issues, Issue{
				Path:    optionPath + ".outcome",
				Message: fmt.Sprintf("недопустимое последствие %q", option.Outcome),
			})

			continue
		}

		byOutcome[option.Outcome]++

		if option.Score != option.Outcome.Score() {
			issues = append(issues, Issue{
				Path: optionPath + ".score",
				Message: fmt.Sprintf("вес %d не соответствует последствию %q, ожидается %d",
					option.Score, option.Outcome, option.Outcome.Score()),
			})
		}

		if option.Text == "" {
			issues = append(issues, Issue{Path: optionPath + ".text", Message: "текст варианта обязателен"})
		}

		if option.Feedback == "" {
			issues = append(issues, Issue{Path: optionPath + ".feedback", Message: "обратная связь обязательна"})
		}
	}

	for outcome := range Weights {
		if byOutcome[outcome] != 1 {
			issues = append(issues, Issue{
				Path: path + ".options",
				Message: fmt.Sprintf("вариантов с последствием %q — %d, должен быть ровно один",
					outcome, byOutcome[outcome]),
			})
		}
	}

	return issues
}

// validateGraph проверяет связность: ссылки ведут на существующие шаги, все
// шаги достижимы от стартового и из каждого достижим терминальный.
func validateGraph(scenario Scenario) []Issue {
	var issues []Issue

	steps := make(map[string]Step, len(scenario.Steps))
	for _, step := range scenario.Steps {
		steps[step.Code] = step
	}

	for index, step := range scenario.Steps {
		for optionIndex, option := range step.Options {
			if option.NextStepCode == "" {
				issues = append(issues, Issue{
					Path:    fmt.Sprintf("steps[%d].options[%d].next_step", index, optionIndex),
					Message: "не указан следующий шаг",
				})

				continue
			}

			if _, ok := steps[option.NextStepCode]; !ok {
				issues = append(issues, Issue{
					Path:    fmt.Sprintf("steps[%d].options[%d].next_step", index, optionIndex),
					Message: fmt.Sprintf("шаг %q не найден в сценарии", option.NextStepCode),
				})
			}
		}
	}

	start, ok := scenario.StartStep()
	if !ok {
		return issues
	}

	reachable := reachableFrom(start.Code, steps)

	for index, step := range scenario.Steps {
		if _, ok := reachable[step.Code]; !ok {
			issues = append(issues, Issue{
				Path:    fmt.Sprintf("steps[%d]", index),
				Message: fmt.Sprintf("шаг %q недостижим от стартового", step.Code),
			})

			continue
		}

		if !leadsToTerminal(step.Code, steps) {
			issues = append(issues, Issue{
				Path:    fmt.Sprintf("steps[%d]", index),
				Message: fmt.Sprintf("из шага %q не достижим терминальный шаг", step.Code),
			})
		}
	}

	return issues
}

func reachableFrom(start string, steps map[string]Step) map[string]struct{} {
	reachable := make(map[string]struct{}, len(steps))
	queue := []string{start}

	for len(queue) > 0 {
		code := queue[0]
		queue = queue[1:]

		if _, visited := reachable[code]; visited {
			continue
		}

		reachable[code] = struct{}{}

		for _, option := range steps[code].Options {
			if _, ok := steps[option.NextStepCode]; ok {
				queue = append(queue, option.NextStepCode)
			}
		}
	}

	return reachable
}

func leadsToTerminal(from string, steps map[string]Step) bool {
	visited := make(map[string]struct{}, len(steps))
	queue := []string{from}

	for len(queue) > 0 {
		code := queue[0]
		queue = queue[1:]

		if _, seen := visited[code]; seen {
			continue
		}

		visited[code] = struct{}{}

		step, ok := steps[code]
		if !ok {
			continue
		}

		if step.Type == StepTypeTerminal {
			return true
		}

		for _, option := range step.Options {
			queue = append(queue, option.NextStepCode)
		}
	}

	return false
}

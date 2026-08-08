package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/domain"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/metrics"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/repository"
)

// trainingService отвечает за последовательность обращений к репозиториям и
// транзакционность; правила ветвления и оценки живут в пакете domain.
type trainingService struct {
	sessions   repository.SessionRepository
	scenarios  repository.ScenarioRepository
	signals    repository.RiskSignalRepository
	thresholds domain.Thresholds
}

var _ TrainingService = (*trainingService)(nil)

// Start создаёт новую попытку. Версия сценария фиксируется в сессии (FR32),
// чтобы правка контента не меняла завершённые попытки. Незавершённая сессия
// по сценарию — *domain.ActiveSessionError, если restart не требует прервать
// её и начать заново.
func (s *trainingService) Start(
	ctx context.Context,
	owner domain.Owner,
	scenarioCode string,
	restart bool,
) (domain.SessionSnapshot, error) {
	scenario, err := s.scenarios.GetActiveByCode(ctx, scenarioCode)
	if err != nil {
		return domain.SessionSnapshot{}, err
	}

	if ensureErr := s.ensureNoActiveSession(ctx, owner, scenarioCode, restart); ensureErr != nil {
		return domain.SessionSnapshot{}, ensureErr
	}

	startStep, ok := scenario.StartStep()
	if !ok {
		return domain.SessionSnapshot{}, fmt.Errorf("сценарий %s без стартового шага", scenario.Code)
	}

	session := domain.Session{
		Owner:           owner,
		ScenarioID:      scenario.ID,
		ScenarioCode:    scenario.Code,
		ScenarioVersion: scenario.Version,
		Status:          domain.StatusInProgress,
		CurrentStepCode: startStep.Code,
	}

	created, err := s.sessions.Create(ctx, session)
	if err != nil {
		return domain.SessionSnapshot{}, err
	}

	metrics.SessionsStartedTotal.WithLabelValues(scenario.Code).Inc()

	return s.snapshot(ctx, created, scenario)
}

// ensureNoActiveSession проверяет незавершённую сессию по сценарию: без
// restart она — ошибка, с restart прерывается.
func (s *trainingService) ensureNoActiveSession(ctx context.Context, owner domain.Owner, scenarioCode string, restart bool) error {
	active, err := s.sessions.GetActiveByOwnerScenario(ctx, owner, scenarioCode)
	if errors.Is(err, domain.ErrNotFound) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("поиск активной сессии по сценарию %s: %w", scenarioCode, err)
	}

	if !restart {
		return &domain.ActiveSessionError{SessionID: active.ID}
	}

	abandonErr := s.sessions.Abandon(ctx, active.ID)
	if abandonErr != nil && !errors.Is(abandonErr, domain.ErrNotFound) {
		return fmt.Errorf("прерывание предыдущей попытки: %w", abandonErr)
	}

	return nil
}

// Get отдаёт состояние сессии владельцу; чужая сессия — domain.ErrNotFound,
// чтобы не раскрывать факт её существования (SEC2).
func (s *trainingService) Get(ctx context.Context, owner domain.Owner, sessionID uuid.UUID) (domain.SessionSnapshot, error) {
	session, err := s.sessions.Get(ctx, sessionID)
	if err != nil {
		return domain.SessionSnapshot{}, err
	}

	if session.Owner != owner {
		return domain.SessionSnapshot{}, domain.ErrNotFound
	}

	scenario, err := s.loadScenarioForSession(ctx, session)
	if err != nil {
		return domain.SessionSnapshot{}, err
	}

	return s.snapshot(ctx, session, scenario)
}

// SubmitAnswer фиксирует выбор: сверка с текущим шагом, доменное ветвление,
// одна транзакция «ответ + баллы + следующий шаг», терминальный шаг завершает
// сессию (FR14). Повторная отправка уже отвеченного шага возвращает
// сохранённый результат без изменения состояния (FR13).
func (s *trainingService) SubmitAnswer(
	ctx context.Context,
	owner domain.Owner,
	sessionID uuid.UUID,
	stepCode, optionCode string,
) (domain.AnswerOutcome, error) {
	session, err := s.sessions.Get(ctx, sessionID)
	if err != nil {
		return domain.AnswerOutcome{}, err
	}

	if session.Owner != owner {
		return domain.AnswerOutcome{}, domain.ErrNotFound
	}

	scenario, err := s.loadScenarioForSession(ctx, session)
	if err != nil {
		return domain.AnswerOutcome{}, err
	}

	// Повторная отправка проверяется до сверки с текущим шагом: после
	// сетевого таймаута клиент не должен получать ошибку за уже принятый
	// ответ.
	if answer, getErr := s.sessions.GetAnswer(ctx, session.ID, stepCode); getErr == nil {
		return s.replayAnswer(ctx, session, scenario, answer)
	} else if !errors.Is(getErr, domain.ErrNotFound) {
		return domain.AnswerOutcome{}, fmt.Errorf("поиск ответа сессии %s: %w", session.ID, getErr)
	}

	if session.Status != domain.StatusInProgress {
		return domain.AnswerOutcome{}, domain.ErrSessionFinished
	}

	if session.CurrentStepCode != stepCode {
		return domain.AnswerOutcome{}, domain.ErrStepNotCurrent
	}

	step, ok := scenario.FindStep(stepCode)
	if !ok {
		return domain.AnswerOutcome{}, domain.ErrStepNotCurrent
	}

	option, nextStepCode, err := domain.ResolveNext(step, optionCode)
	if err != nil {
		return domain.AnswerOutcome{}, err
	}

	next, ok := scenario.FindStep(nextStepCode)
	if !ok {
		return domain.AnswerOutcome{}, fmt.Errorf("шаг %q после варианта %q не найден в сценарии %s",
			nextStepCode, optionCode, scenario.Code)
	}

	answers, err := s.sessions.ListAnswers(ctx, session.ID)
	if err != nil {
		return domain.AnswerOutcome{}, fmt.Errorf("список ответов сессии %s: %w", session.ID, err)
	}

	answer := domain.Answer{
		SessionID:       session.ID,
		StepCode:        stepCode,
		OptionCode:      option.Code,
		Outcome:         option.Outcome,
		ScoreDelta:      option.Score,
		RiskSignalCodes: step.RiskSignalCodes,
		Position:        len(answers) + 1,
	}

	finished := next.Type == domain.StepTypeTerminal

	saved, err := s.sessions.SaveAnswer(ctx, answer, nextStepCode, finished)
	if err != nil {
		// Гонка с параллельной отправкой: ответ уже зафиксирован другим
		// запросом. Вернуть сохранённый результат вместо ошибки.
		if existing, getErr := s.sessions.GetAnswer(ctx, session.ID, stepCode); getErr == nil {
			if current, currentErr := s.sessions.Get(ctx, session.ID); currentErr == nil {
				return s.replayAnswer(ctx, current, scenario, existing)
			}
		}

		return domain.AnswerOutcome{}, fmt.Errorf("сохранение ответа: %w", err)
	}

	if finished {
		result := domain.Evaluate(append(answers, answer), s.thresholds)
		metrics.SessionsCompletedTotal.WithLabelValues(scenario.Code, string(result.Level)).Inc()
		metrics.SessionScorePercent.WithLabelValues(scenario.Code).Observe(float64(result.Percent))
	}

	signals, err := s.loadSignals(ctx, step.RiskSignalCodes)
	if err != nil {
		return domain.AnswerOutcome{}, err
	}

	snapshot, err := s.snapshot(ctx, saved, scenario)
	if err != nil {
		return domain.AnswerOutcome{}, err
	}

	return domain.AnswerOutcome{
		Answer:          answer,
		Option:          option,
		RiskSignals:     signals,
		SafeAlternative: s.safeAlternative(step, answer),
		SessionFinished: finished,
		Snapshot:        snapshot,
	}, nil
}

// Abandon прерывает незавершённую сессию. Завершённую прерывать нельзя —
// domain.ErrSessionFinished; повторный вызов прерванной безвреден.
func (s *trainingService) Abandon(ctx context.Context, owner domain.Owner, sessionID uuid.UUID) error {
	session, err := s.sessions.Get(ctx, sessionID)
	if err != nil {
		return err
	}

	if session.Owner != owner {
		return domain.ErrNotFound
	}

	switch session.Status {
	case domain.StatusCompleted:
		return domain.ErrSessionFinished
	case domain.StatusAbandoned:
		return nil
	case domain.StatusInProgress:
		return s.sessions.Abandon(ctx, sessionID)
	}

	return domain.ErrSessionFinished
}

// Result собирает экран разбора: оценка, пошаговый разбор, карта признаков,
// сравнение с предыдущей попыткой, рекомендации и следующий шаг (FR18–FR27).
func (s *trainingService) Result(ctx context.Context, owner domain.Owner, sessionID uuid.UUID) (domain.Debrief, error) {
	session, err := s.sessions.Get(ctx, sessionID)
	if err != nil {
		return domain.Debrief{}, err
	}

	if session.Owner != owner {
		return domain.Debrief{}, domain.ErrNotFound
	}

	if session.Status != domain.StatusCompleted {
		return domain.Debrief{}, domain.ErrSessionNotFinished
	}

	scenario, err := s.loadScenarioForSession(ctx, session)
	if err != nil {
		return domain.Debrief{}, err
	}

	answers, err := s.sessions.ListAnswers(ctx, session.ID)
	if err != nil {
		return domain.Debrief{}, fmt.Errorf("список ответов сессии %s: %w", session.ID, err)
	}

	signals, err := s.loadSignalMap(ctx, answers)
	if err != nil {
		return domain.Debrief{}, err
	}

	debrief := domain.Debrief{
		Session:  session,
		Scenario: scenario,
		Result:   domain.Evaluate(answers, s.thresholds),
	}

	debrief.Breakdown = s.buildBreakdown(scenario, answers, signals)
	debrief.Signals = domain.RecognizedSignals(answers, orderedSignals(signals, answers))
	debrief.Recommendations = s.buildRecommendations(scenario, answers, debrief.Signals)

	if buildErr := s.buildComparison(ctx, &debrief); buildErr != nil {
		return domain.Debrief{}, buildErr
	}

	next, err := s.suggestNextStep(ctx, owner, scenario, debrief.Result)
	if err != nil {
		return domain.Debrief{}, err
	}
	debrief.NextStep = next

	return debrief, nil
}

// snapshot собирает состояние сессии и текущий шаг. Текущий шаг отсутствует
// у завершённых и прерванных сессий.
func (s *trainingService) snapshot(
	ctx context.Context,
	session domain.Session,
	scenario domain.Scenario,
) (domain.SessionSnapshot, error) {
	answers, err := s.sessions.ListAnswers(ctx, session.ID)
	if err != nil {
		return domain.SessionSnapshot{}, fmt.Errorf("список ответов сессии %s: %w", session.ID, err)
	}

	snapshot := domain.SessionSnapshot{
		Session:      session,
		Scenario:     scenario,
		AnswersCount: len(answers),
		StepsTotal:   scenario.StepsCount,
	}

	if session.Active() {
		step, ok := scenario.FindStep(session.CurrentStepCode)
		if !ok {
			return domain.SessionSnapshot{}, fmt.Errorf("текущий шаг %q не найден в сценарии %s",
				session.CurrentStepCode, scenario.Code)
		}

		snapshot.CurrentStep = &step
	}

	return snapshot, nil
}

// loadScenarioForSession достаёт сценарий той версии, на которой проходилась
// сессия (FR32): разбор не зависит от выхода новых версий контента.
func (s *trainingService) loadScenarioForSession(ctx context.Context, session domain.Session) (domain.Scenario, error) {
	return s.scenarios.GetByCodeVersion(ctx, session.ScenarioCode, session.ScenarioVersion)
}

// replayAnswer возвращает ранее зафиксированный результат (FR13): состояние
// сессии и баллы не меняются.
func (s *trainingService) replayAnswer(
	ctx context.Context,
	session domain.Session,
	scenario domain.Scenario,
	answer domain.Answer,
) (domain.AnswerOutcome, error) {
	step, ok := scenario.FindStep(answer.StepCode)
	if !ok {
		return domain.AnswerOutcome{}, fmt.Errorf("шаг %q не найден в сценарии %s", answer.StepCode, scenario.Code)
	}

	option, ok := step.FindOption(answer.OptionCode)
	if !ok {
		return domain.AnswerOutcome{}, fmt.Errorf("вариант %q не найден на шаге %q", answer.OptionCode, answer.StepCode)
	}

	signals, err := s.loadSignals(ctx, step.RiskSignalCodes)
	if err != nil {
		return domain.AnswerOutcome{}, err
	}

	snapshot, err := s.snapshot(ctx, session, scenario)
	if err != nil {
		return domain.AnswerOutcome{}, err
	}

	return domain.AnswerOutcome{
		Answer:          answer,
		Option:          option,
		RiskSignals:     signals,
		SafeAlternative: s.safeAlternative(step, answer),
		AlreadyAnswered: true,
		SessionFinished: session.Status == domain.StatusCompleted,
		Snapshot:        snapshot,
	}, nil
}

// safeAlternative — «как следовало поступить»; nil, если выбор был безопасным.
func (s *trainingService) safeAlternative(step domain.Step, answer domain.Answer) *domain.Option {
	if answer.Outcome == domain.OutcomeSafe {
		return nil
	}

	safe, ok := step.SafeOption()
	if !ok {
		return nil
	}

	return &safe
}

// loadSignals достаёт признаки риска шага в порядке их перечисления.
func (s *trainingService) loadSignals(ctx context.Context, codes []string) ([]domain.RiskSignal, error) {
	if len(codes) == 0 {
		return []domain.RiskSignal{}, nil
	}

	signals, err := s.signals.ListByCodes(ctx, codes)
	if err != nil {
		return nil, fmt.Errorf("список признаков риска: %w", err)
	}

	return signals, nil
}

// loadSignalMap — признаки всех отвеченных шагов в виде карты code → сигнал.
func (s *trainingService) loadSignalMap(ctx context.Context, answers []domain.Answer) (map[string]domain.RiskSignal, error) {
	seen := make(map[string]struct{})
	codes := make([]string, 0)

	for _, answer := range answers {
		for _, code := range answer.RiskSignalCodes {
			if _, ok := seen[code]; ok {
				continue
			}

			seen[code] = struct{}{}
			codes = append(codes, code)
		}
	}

	signals, err := s.loadSignals(ctx, codes)
	if err != nil {
		return nil, err
	}

	byCode := make(map[string]domain.RiskSignal, len(signals))
	for _, signal := range signals {
		byCode[signal.Code] = signal
	}

	return byCode, nil
}

// orderedSignals возвращает признаки в порядке первого появления в ответах.
func orderedSignals(byCode map[string]domain.RiskSignal, answers []domain.Answer) []domain.RiskSignal {
	seen := make(map[string]struct{})
	ordered := make([]domain.RiskSignal, 0, len(byCode))

	for _, answer := range answers {
		for _, code := range answer.RiskSignalCodes {
			if _, ok := seen[code]; ok {
				continue
			}

			if signal, ok := byCode[code]; ok {
				seen[code] = struct{}{}
				ordered = append(ordered, signal)
			}
		}
	}

	return ordered
}

// buildBreakdown — пошаговый разбор (FR18): ситуация, выбранный вариант,
// последствие, безопасная альтернатива и признаки риска шага.
func (s *trainingService) buildBreakdown(
	scenario domain.Scenario,
	answers []domain.Answer,
	signals map[string]domain.RiskSignal,
) []domain.BreakdownItem {
	items := make([]domain.BreakdownItem, 0, len(answers))

	for order, answer := range answers {
		step, ok := scenario.FindStep(answer.StepCode)
		if !ok {
			continue
		}

		option, ok := step.FindOption(answer.OptionCode)
		if !ok {
			continue
		}

		item := domain.BreakdownItem{
			Order:      order + 1,
			StepCode:   step.Code,
			Situation:  step.Content.Message,
			Chosen:     option,
			ScoreDelta: answer.ScoreDelta,
		}

		if answer.Outcome != domain.OutcomeSafe {
			if safe, ok := step.SafeOption(); ok {
				safeCopy := safe
				item.SafeAlternative = &safeCopy
			}
		}

		for _, code := range step.RiskSignalCodes {
			if signal, ok := signals[code]; ok {
				item.RiskSignals = append(item.RiskSignals, signal)
			}
		}

		items = append(items, item)
	}

	return items
}

// buildRecommendations — практические рекомендации (FR19): как следовало
// поступить на шагах с небезопасным выбором и как вести себя на пропущенных
// признаках. Не больше трёх, дубликаты убираются.
func (s *trainingService) buildRecommendations(
	scenario domain.Scenario,
	answers []domain.Answer,
	signals []domain.SignalOutcome,
) []string {
	answered := make(map[string]domain.Answer, len(answers))
	for _, answer := range answers {
		answered[answer.StepCode] = answer
	}

	seen := make(map[string]struct{})
	recommendations := make([]string, 0)

	add := func(text string) {
		if text == "" {
			return
		}

		if _, ok := seen[text]; ok {
			return
		}

		seen[text] = struct{}{}
		recommendations = append(recommendations, text)
	}

	for _, step := range scenario.Steps {
		if len(recommendations) >= 3 {
			break
		}

		if step.Type != domain.StepTypeDialog {
			continue
		}

		answer, ok := answered[step.Code]
		if !ok {
			continue
		}

		if answer.Outcome == domain.OutcomeSafe {
			continue
		}

		if safe, ok := step.SafeOption(); ok {
			add(safe.Feedback)
		}
	}

	for _, outcome := range signals {
		if len(recommendations) >= 3 {
			break
		}

		if outcome.Recognized {
			continue
		}

		add(outcome.Signal.HowToAct)
	}

	// Идеальное прохождение: рекомендации по распознанным признакам, чтобы
	// экран результата всё равно давал полезное.
	if len(recommendations) == 0 {
		for _, outcome := range signals {
			if len(recommendations) >= 3 {
				break
			}

			add(outcome.Signal.HowToAct)
		}
	}

	if recommendations == nil {
		recommendations = []string{}
	}

	return recommendations
}

// buildComparison — сравнение с предыдущей завершённой попыткой (FR23).
// Первой попытке сравнение не нужно.
func (s *trainingService) buildComparison(ctx context.Context, debrief *domain.Debrief) error {
	previous, err := s.sessions.PreviousCompleted(ctx, debrief.Session.Owner, debrief.Scenario.Code, debrief.Session.ID)
	if errors.Is(err, domain.ErrNotFound) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("поиск предыдущей попытки: %w", err)
	}

	// Процент не хранится в сессии — считаем его из ответов той попытки.
	previousAnswers, err := s.sessions.ListAnswers(ctx, previous.ID)
	if err != nil {
		return fmt.Errorf("список ответов предыдущей попытки: %w", err)
	}

	previousResult := domain.Evaluate(previousAnswers, s.thresholds)

	debrief.Comparison = &domain.Comparison{
		PreviousPercent:    previousResult.Percent,
		PreviousScore:      previousResult.Score,
		DeltaPercent:       debrief.Result.Percent - previousResult.Percent,
		PreviousFinishedAt: *previous.FinishedAt,
	}

	return nil
}

// suggestNextStep — предложение следующего шага (FR27): непройденный сценарий
// или повторное прохождение при слабом результате.
func (s *trainingService) suggestNextStep(
	ctx context.Context,
	owner domain.Owner,
	scenario domain.Scenario,
	result domain.Result,
) (domain.NextStep, error) {
	active, err := s.scenarios.ListActive(ctx, nil)
	if err != nil {
		return domain.NextStep{}, fmt.Errorf("список активных сценариев: %w", err)
	}

	for _, candidate := range active {
		if candidate.Code == scenario.Code {
			continue
		}

		completed, err := s.sessions.ListCompleted(ctx, owner, candidate.Code)
		if err != nil {
			return domain.NextStep{}, fmt.Errorf("история сценария %s: %w", candidate.Code, err)
		}

		if len(completed) == 0 {
			return domain.NextStep{
				Type:     domain.NextStepNewScenario,
				Scenario: &candidate,
				Reason:   fmt.Sprintf("Ещё не пройден сценарий «%s»", candidate.Title),
			}, nil
		}
	}

	if result.Percent < s.thresholds.Attentive {
		return domain.NextStep{
			Type:     domain.NextStepRetryScenario,
			Scenario: &scenario,
			Reason:   "Пройдите сценарий ещё раз, чтобы отработать пропущенные признаки",
		}, nil
	}

	return domain.NextStep{
		Type: domain.NextStepAllDone,
		// FR28: когда все сценарии пройдены, остаётся справочник признаков.
		Reason: "Все сценарии пройдены. Загляните в справочник признаков риска",
	}, nil
}

package service

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/domain"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/repository"
)

// fakeScenarioRepo хранит сценарии в памяти и повторяет семантику активной
// версии (FR32): список без шагов, чтение по коду и по (код, версия).
type fakeScenarioRepo struct {
	active   map[string]domain.Scenario
	versions map[string]map[int]domain.Scenario
}

func newFakeScenarioRepo(scenarios ...domain.Scenario) *fakeScenarioRepo {
	repo := &fakeScenarioRepo{
		active:   make(map[string]domain.Scenario),
		versions: make(map[string]map[int]domain.Scenario),
	}

	for _, scenario := range scenarios {
		repo.active[scenario.Code] = scenario

		if repo.versions[scenario.Code] == nil {
			repo.versions[scenario.Code] = make(map[int]domain.Scenario)
		}

		repo.versions[scenario.Code][scenario.Version] = scenario
	}

	return repo
}

func (f *fakeScenarioRepo) ListActive(ctx context.Context, role *domain.Role) ([]domain.Scenario, error) {
	_ = ctx

	codes := make([]string, 0, len(f.active))
	for code, scenario := range f.active {
		if role != nil && scenario.Role != *role {
			continue
		}

		codes = append(codes, code)
	}

	sort.Strings(codes)

	scenarios := make([]domain.Scenario, 0, len(codes))
	for _, code := range codes {
		scenarios = append(scenarios, f.active[code])
	}

	return scenarios, nil
}

func (f *fakeScenarioRepo) GetActiveByCode(ctx context.Context, code string) (domain.Scenario, error) {
	_ = ctx

	scenario, ok := f.active[code]
	if !ok {
		return domain.Scenario{}, domain.ErrNotFound
	}

	return scenario, nil
}

func (f *fakeScenarioRepo) GetByCodeVersion(ctx context.Context, code string, version int) (domain.Scenario, error) {
	_ = ctx

	versions, ok := f.versions[code]
	if !ok {
		return domain.Scenario{}, domain.ErrNotFound
	}

	scenario, ok := versions[version]
	if !ok {
		return domain.Scenario{}, domain.ErrNotFound
	}

	return scenario, nil
}

func (f *fakeScenarioRepo) ListBySignal(context.Context, string) ([]domain.Scenario, error) {
	return nil, domain.ErrNotImplemented
}

func (f *fakeScenarioRepo) CountActive(context.Context) (int, error) {
	return 0, domain.ErrNotImplemented
}

func (f *fakeScenarioRepo) Upsert(context.Context, domain.Scenario, string) (int, bool, error) {
	return 0, false, domain.ErrNotImplemented
}

// fakeSessionRepo повторяет поведение таблиц sessions/answers: частичный
// уникальный индекс на активную сессию, уникальность (session_id, step_code),
// балл как сумма весов ответов.
type fakeSessionRepo struct {
	sessions   map[uuid.UUID]domain.Session
	answers    map[uuid.UUID][]domain.Answer
	nextAnswer int64
}

func newFakeSessionRepo() *fakeSessionRepo {
	return &fakeSessionRepo{
		sessions: make(map[uuid.UUID]domain.Session),
		answers:  make(map[uuid.UUID][]domain.Answer),
	}
}

func (f *fakeSessionRepo) Create(ctx context.Context, session domain.Session) (domain.Session, error) {
	_ = ctx

	if session.ID == uuid.Nil {
		session.ID = uuid.New()
	}

	if session.StartedAt.IsZero() {
		session.StartedAt = time.Now()
	}

	for _, existing := range f.sessions {
		if existing.Owner == session.Owner &&
			existing.ScenarioCode == session.ScenarioCode &&
			existing.Status == domain.StatusInProgress {
			return domain.Session{}, &domain.ActiveSessionError{SessionID: existing.ID}
		}
	}

	f.sessions[session.ID] = session

	return session, nil
}

func (f *fakeSessionRepo) Get(ctx context.Context, id uuid.UUID) (domain.Session, error) {
	_ = ctx

	session, ok := f.sessions[id]
	if !ok {
		return domain.Session{}, domain.ErrNotFound
	}

	return session, nil
}

func (f *fakeSessionRepo) GetActiveByOwner(
	ctx context.Context,
	owner domain.Owner,
) (domain.Session, error) {
	_ = ctx

	var active *domain.Session

	for _, session := range f.sessions {
		if session.Owner != owner ||
			session.Status != domain.StatusInProgress {
			continue
		}

		if active == nil || session.StartedAt.After(active.StartedAt) {
			sessionCopy := session
			active = &sessionCopy
		}
	}

	if active == nil {
		return domain.Session{}, domain.ErrNotFound
	}

	return *active, nil
}

func (f *fakeSessionRepo) ClaimByGuest(
	ctx context.Context,
	guestSessionID uuid.UUID,
	userID uuid.UUID,
) error {
	_ = ctx

	guestOwner := domain.GuestOwner(guestSessionID)
	userOwner := domain.UserOwner(userID)

	for id, session := range f.sessions {
		if session.Owner != guestOwner {
			continue
		}

		session.Owner = userOwner
		f.sessions[id] = session
	}

	return nil
}

func (f *fakeSessionRepo) GetActiveByOwnerScenario(
	ctx context.Context,
	owner domain.Owner,
	scenarioCode string,
) (domain.Session, error) {
	_ = ctx

	var active *domain.Session

	for _, session := range f.sessions {
		if session.Owner != owner ||
			session.ScenarioCode != scenarioCode ||
			session.Status != domain.StatusInProgress {
			continue
		}

		if active == nil || session.StartedAt.After(active.StartedAt) {
			sessionCopy := session
			active = &sessionCopy
		}
	}

	if active == nil {
		return domain.Session{}, domain.ErrNotFound
	}

	return *active, nil
}

func (f *fakeSessionRepo) SaveAnswer(
	ctx context.Context,
	answer domain.Answer,
	nextStepCode string,
	finished bool,
) (domain.Session, error) {
	_ = ctx

	session, ok := f.sessions[answer.SessionID]
	if !ok {
		return domain.Session{}, domain.ErrNotFound
	}

	for _, existing := range f.answers[session.ID] {
		if existing.StepCode == answer.StepCode {
			return domain.Session{}, errors.New("duplicate answer")
		}
	}

	f.nextAnswer++
	answer.ID = f.nextAnswer
	answer.AnsweredAt = time.Now()
	f.answers[session.ID] = append(f.answers[session.ID], answer)

	session.Score += answer.ScoreDelta
	if finished {
		now := time.Now()
		session.Status = domain.StatusCompleted
		session.CurrentStepCode = ""
		session.FinishedAt = &now
	} else {
		session.CurrentStepCode = nextStepCode
	}

	f.sessions[session.ID] = session

	return session, nil
}

func (f *fakeSessionRepo) GetAnswer(ctx context.Context, sessionID uuid.UUID, stepCode string) (domain.Answer, error) {
	_ = ctx

	for _, answer := range f.answers[sessionID] {
		if answer.StepCode == stepCode {
			return answer, nil
		}
	}

	return domain.Answer{}, domain.ErrNotFound
}

func (f *fakeSessionRepo) ListAnswers(ctx context.Context, sessionID uuid.UUID) ([]domain.Answer, error) {
	_ = ctx

	answers := make([]domain.Answer, len(f.answers[sessionID]))
	copy(answers, f.answers[sessionID])
	sort.Slice(answers, func(i, j int) bool { return answers[i].Position < answers[j].Position })

	return answers, nil
}

func (f *fakeSessionRepo) Abandon(ctx context.Context, id uuid.UUID) error {
	_ = ctx

	session, ok := f.sessions[id]
	if !ok || session.Status != domain.StatusInProgress {
		return domain.ErrNotFound
	}

	now := time.Now()
	session.Status = domain.StatusAbandoned
	session.FinishedAt = &now
	f.sessions[id] = session

	return nil
}

func (f *fakeSessionRepo) ListCompleted(
	ctx context.Context,
	owner domain.Owner,
	scenarioCode string,
) ([]domain.Session, error) {
	_ = ctx

	var completed []domain.Session
	for _, session := range f.sessions {
		if session.Owner == owner && session.ScenarioCode == scenarioCode &&
			session.Status == domain.StatusCompleted {
			completed = append(completed, session)
		}
	}

	sort.Slice(completed, func(i, j int) bool {
		return completed[i].FinishedAt.After(*completed[j].FinishedAt)
	})

	if completed == nil {
		completed = []domain.Session{}
	}

	return completed, nil
}

func (f *fakeSessionRepo) PreviousCompleted(
	ctx context.Context,
	owner domain.Owner,
	scenarioCode string,
	excludeSessionID uuid.UUID,
) (domain.Session, error) {
	_ = ctx

	var previous domain.Session
	found := false

	for _, session := range f.sessions {
		if session.ID == excludeSessionID ||
			session.Owner != owner ||
			session.ScenarioCode != scenarioCode ||
			session.Status != domain.StatusCompleted {
			continue
		}

		previous = session
		found = true
	}

	if !found {
		return domain.Session{}, domain.ErrNotFound
	}

	return previous, nil
}

// fakeSignalRepo повторяет порядок результата, совпадающий с порядком кодов.
type fakeSignalRepo struct {
	signals map[string]domain.RiskSignal
}

func (f *fakeSignalRepo) List(context.Context, *domain.Side) ([]domain.RiskSignal, error) {
	return nil, domain.ErrNotImplemented
}

func (f *fakeSignalRepo) Get(context.Context, string) (domain.RiskSignal, error) {
	return domain.RiskSignal{}, domain.ErrNotImplemented
}

func (f *fakeSignalRepo) ListByCodes(ctx context.Context, codes []string) ([]domain.RiskSignal, error) {
	_ = ctx

	ordered := make([]domain.RiskSignal, 0, len(codes))
	for _, code := range codes {
		if signal, ok := f.signals[code]; ok {
			ordered = append(ordered, signal)
		}
	}

	return ordered, nil
}

func (f *fakeSignalRepo) Upsert(context.Context, []domain.RiskSignal) error {
	return domain.ErrNotImplemented
}

var _ repository.SessionRepository = (*fakeSessionRepo)(nil)
var _ repository.ScenarioRepository = (*fakeScenarioRepo)(nil)
var _ repository.RiskSignalRepository = (*fakeSignalRepo)(nil)

// demoScenario — три диалоговых шага, сходящихся в терминальный; варианты
// критический/спорный/безопасный, как в реальном контенте.
func demoScenario() domain.Scenario {
	option := func(code, text string, outcome domain.Outcome, next string) domain.Option {
		return domain.Option{
			Code:         code,
			Text:         text,
			Outcome:      outcome,
			Score:        outcome.Score(),
			Feedback:     "объяснение для " + code,
			NextStepCode: next,
		}
	}

	step := func(code, next string, start bool, signals ...string) domain.Step {
		return domain.Step{
			Code:            code,
			Type:            domain.StepTypeDialog,
			IsStart:         start,
			RiskSignalCodes: signals,
			Content:         domain.StepContent{Message: "Реплика: " + code, Sender: domain.SenderCounterparty},
			Options: []domain.Option{
				option("a", "опасно", domain.OutcomeCritical, next),
				option("b", "спорно", domain.OutcomeRisky, next),
				option("c", "безопасно", domain.OutcomeSafe, next),
			},
		}
	}

	return domain.Scenario{
		ID:          1,
		Code:        "too-good-price",
		Version:     1,
		Role:        domain.RoleBuyer,
		Title:       "Слишком выгодная цена",
		Description: "Демо-сценарий для тестов",
		Difficulty:  domain.DifficultyDemo,
		StepsCount:  3,
		IsActive:    true,
		Steps: []domain.Step{
			step("s1", "s2", true, "RISK_TOO_GOOD_PRICE", "RISK_URGENCY_PRESSURE"),
			step("s2", "s3", false, "RISK_PREPAY_OUTSIDE"),
			step("s3", "end", false, "RISK_PHISHING_LINK"),
			{
				Code:            "end",
				Type:            domain.StepTypeTerminal,
				RiskSignalCodes: []string{"RISK_TOO_GOOD_PRICE"},
				Content:         domain.StepContent{Message: "Разговор окончен"},
			},
		},
	}
}

func demoSignals() map[string]domain.RiskSignal {
	return map[string]domain.RiskSignal{
		"RISK_TOO_GOOD_PRICE":   {Code: "RISK_TOO_GOOD_PRICE", Side: domain.SideBuyer, Title: "Слишком выгодно", HowToAct: "проверяйте способ оплаты"},
		"RISK_URGENCY_PRESSURE": {Code: "RISK_URGENCY_PRESSURE", Side: domain.SideBuyer, Title: "Давление срочностью", HowToAct: "не спешите с решением"},
		"RISK_PREPAY_OUTSIDE":   {Code: "RISK_PREPAY_OUTSIDE", Side: domain.SideBuyer, Title: "Предоплата мимо площадки", HowToAct: "платите только внутри платформы"},
		"RISK_PHISHING_LINK":    {Code: "RISK_PHISHING_LINK", Side: domain.SideBuyer, Title: "Фишинговая ссылка", HowToAct: "оформляйте доставку кнопкой в объявлении"},
	}
}

// newTestService собирает trainingService на фейках; настройки уровней как в
// конфигурации по умолчанию.
func newTestService(t *testing.T) (*trainingService, *fakeSessionRepo) {
	t.Helper()

	scenarioRepo := newFakeScenarioRepo(demoScenario())
	sessionRepo := newFakeSessionRepo()
	signalRepo := &fakeSignalRepo{signals: demoSignals()}

	service := &trainingService{
		sessions:   sessionRepo,
		scenarios:  scenarioRepo,
		signals:    signalRepo,
		thresholds: domain.Thresholds{Resilient: 80, Attentive: 60},
	}

	return service, sessionRepo
}

// complete — полностью проходит сценарий выборами из choices и возвращает
// завершённую сессию.
func complete(t *testing.T, service *trainingService, owner domain.Owner, choices []string) domain.Session {
	t.Helper()

	ctx := context.Background()
	snapshot, err := service.Start(ctx, owner, "too-good-price", false)
	require.NoError(t, err)

	for _, choice := range choices {
		outcome, err := service.SubmitAnswer(
			ctx, owner, snapshot.Session.ID, snapshot.Session.CurrentStepCode, choice,
		)
		require.NoError(t, err)

		if outcome.SessionFinished {
			return outcome.Snapshot.Session
		}

		snapshot = outcome.Snapshot
	}

	require.FailNow(t, "сценарий не завершился за заданные ответы")

	return domain.Session{}
}

func TestStart(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	owner := domain.UserOwner(userID)

	t.Run("создаёт сессию со стартовым шагом и фиксирует версию", func(t *testing.T) {
		service, _ := newTestService(t)

		snapshot, err := service.Start(ctx, owner, "too-good-price", false)

		require.NoError(t, err)
		require.Equal(t, domain.StatusInProgress, snapshot.Session.Status)
		require.Equal(t, 1, snapshot.Session.ScenarioVersion, "версия фиксируется при старте (FR32)")
		require.Equal(t, "s1", snapshot.Session.CurrentStepCode)
		require.NotNil(t, snapshot.CurrentStep)
		require.Equal(t, "s1", snapshot.CurrentStep.Code)
		require.Equal(t, 0, snapshot.AnswersCount)
		require.Equal(t, 3, snapshot.StepsTotal)
	})

	t.Run("незавершённая сессия — ошибка с её идентификатором", func(t *testing.T) {
		service, _ := newTestService(t)

		first, err := service.Start(ctx, owner, "too-good-price", false)
		require.NoError(t, err)

		_, err = service.Start(ctx, owner, "too-good-price", false)

		var activeErr *domain.ActiveSessionError
		require.ErrorAs(t, err, &activeErr)
		require.Equal(t, first.Session.ID, activeErr.SessionID)
	})

	t.Run("restart прерывает предыдущую и создаёт новую", func(t *testing.T) {
		service, _ := newTestService(t)

		first, err := service.Start(ctx, owner, "too-good-price", false)
		require.NoError(t, err)

		second, err := service.Start(ctx, owner, "too-good-price", true)

		require.NoError(t, err)
		require.NotEqual(t, first.Session.ID, second.Session.ID)

		abandoned, err := service.Get(ctx, owner, first.Session.ID)
		require.NoError(t, err)
		require.Equal(t, domain.StatusAbandoned, abandoned.Session.Status)
		require.Nil(t, abandoned.CurrentStep)
	})

	t.Run("неизвестный сценарий — не найдено", func(t *testing.T) {
		service, _ := newTestService(t)

		_, err := service.Start(ctx, owner, "unknown-scenario", false)

		require.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestGet(t *testing.T) {
	ctx := context.Background()
	service, _ := newTestService(t)
	userID := uuid.New()
	owner := domain.UserOwner(userID)

	snapshot, err := service.Start(ctx, owner, "too-good-price", false)
	require.NoError(t, err)

	t.Run("владелец видит сессию", func(t *testing.T) {
		got, err := service.Get(ctx, owner, snapshot.Session.ID)

		require.NoError(t, err)
		require.Equal(t, snapshot.Session.ID, got.Session.ID)
		require.NotNil(t, got.CurrentStep)
	})

	t.Run("чужой пользователь получает 404", func(t *testing.T) {
		otherOwner := domain.UserOwner(uuid.New())

		_, err := service.Get(ctx, otherOwner, snapshot.Session.ID)

		require.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestSubmitAnswer(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	owner := domain.UserOwner(userID)

	t.Run("успешный ответ: баллы, следующий шаг, признаки и альтернатива", func(t *testing.T) {
		service, _ := newTestService(t)

		snapshot, err := service.Start(ctx, owner, "too-good-price", false)
		require.NoError(t, err)

		outcome, err := service.SubmitAnswer(ctx, owner, snapshot.Session.ID, "s1", "b")
		require.NoError(t, err)

		require.Equal(t, domain.OutcomeRisky, outcome.Answer.Outcome)
		require.Equal(t, 0, outcome.Answer.ScoreDelta)
		require.Equal(t, 0, outcome.Snapshot.Session.Score)
		require.Equal(t, "s2", outcome.Snapshot.Session.CurrentStepCode)
		require.False(t, outcome.AlreadyAnswered)
		require.False(t, outcome.SessionFinished)
		require.Len(t, outcome.RiskSignals, 2)
		require.Equal(t, "RISK_TOO_GOOD_PRICE", outcome.RiskSignals[0].Code)
		require.NotNil(t, outcome.SafeAlternative, "спорному выбору полагается безопасная альтернатива")
		require.Equal(t, "c", outcome.SafeAlternative.Code)

		outcome, err = service.SubmitAnswer(ctx, owner, snapshot.Session.ID, "s2", "c")
		require.NoError(t, err)
		require.Equal(t, 10, outcome.Snapshot.Session.Score)
		require.Nil(t, outcome.SafeAlternative, "безопасному выбору альтернатива не нужна")
	})

	t.Run("терминальный шаг завершает сессию", func(t *testing.T) {
		service, _ := newTestService(t)

		session := complete(t, service, owner, []string{"c", "c", "a"})

		require.Equal(t, domain.StatusCompleted, session.Status)
		require.Equal(t, 10, session.Score) // 10 + 10 - 10
		require.NotNil(t, session.FinishedAt)
	})

	t.Run("повторная отправка возвращает сохранённый результат без начисления", func(t *testing.T) {
		service, _ := newTestService(t)

		snapshot, err := service.Start(ctx, owner, "too-good-price", false)
		require.NoError(t, err)

		first, err := service.SubmitAnswer(ctx, owner, snapshot.Session.ID, "s1", "b")
		require.NoError(t, err)

		// Другой вариант игнорируется: засчитан первый выбор (FR13).
		replayed, err := service.SubmitAnswer(ctx, owner, snapshot.Session.ID, "s1", "c")
		require.NoError(t, err)

		require.True(t, replayed.AlreadyAnswered)
		require.Equal(t, first.Answer.OptionCode, replayed.Answer.OptionCode)
		require.Equal(t, 0, replayed.Snapshot.Session.Score)

		next, err := service.SubmitAnswer(ctx, owner, snapshot.Session.ID, "s2", "c")
		require.NoError(t, err)
		require.Equal(t, 10, next.Snapshot.Session.Score, "баллы не начисляются повторно")
	})

	t.Run("ответ вне очереди — шаг не текущий", func(t *testing.T) {
		service, _ := newTestService(t)

		snapshot, err := service.Start(ctx, owner, "too-good-price", false)
		require.NoError(t, err)

		_, err = service.SubmitAnswer(ctx, owner, snapshot.Session.ID, "s2", "c")

		require.ErrorIs(t, err, domain.ErrStepNotCurrent)
	})

	t.Run("варианта нет на шаге — ошибка", func(t *testing.T) {
		service, _ := newTestService(t)

		snapshot, err := service.Start(ctx, owner, "too-good-price", false)
		require.NoError(t, err)

		_, err = service.SubmitAnswer(ctx, owner, snapshot.Session.ID, "s1", "z")

		require.ErrorIs(t, err, domain.ErrOptionNotFound)
	})

	t.Run("завершённую сессию отвечать нельзя", func(t *testing.T) {
		service, _ := newTestService(t)

		session := complete(t, service, owner, []string{"c", "c", "c"})

		// Уже отвеченный шаг завершённой сессии — идемпотентный повтор (FR13).
		replayed, err := service.SubmitAnswer(ctx, owner, session.ID, "s1", "c")
		require.NoError(t, err)
		require.True(t, replayed.AlreadyAnswered)
		require.True(t, replayed.SessionFinished)

		// Новый шаг завершённой сессии отклоняется.
		_, err = service.SubmitAnswer(ctx, owner, session.ID, "end", "c")
		require.ErrorIs(t, err, domain.ErrSessionFinished)
	})
}

func TestAbandon(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	service, _ := newTestService(t)
	owner := domain.UserOwner(userID)

	snapshot, err := service.Start(ctx, owner, "too-good-price", false)
	require.NoError(t, err)

	t.Run("прерывает незавершённую", func(t *testing.T) {
		err := service.Abandon(ctx, owner, snapshot.Session.ID)

		require.NoError(t, err)

		got, err := service.Get(ctx, owner, snapshot.Session.ID)
		require.NoError(t, err)
		require.Equal(t, domain.StatusAbandoned, got.Session.Status)
		require.Nil(t, got.CurrentStep)
	})

	t.Run("повторное прерывание безвредно", func(t *testing.T) {
		err := service.Abandon(ctx, owner, snapshot.Session.ID)

		require.NoError(t, err)
	})

	t.Run("чужую сессию прервать нельзя", func(t *testing.T) {
		otherOwner := domain.UserOwner(uuid.New())

		other, err := service.Start(ctx, otherOwner, "too-good-price", false)
		require.NoError(t, err)

		err = service.Abandon(ctx, owner, other.Session.ID)
		require.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestResult(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	owner := domain.UserOwner(userID)

	t.Run("незавершённая сессия — ошибка", func(t *testing.T) {
		service, _ := newTestService(t)

		snapshot, err := service.Start(ctx, owner, "too-good-price", false)
		require.NoError(t, err)

		_, err = service.Result(ctx, owner, snapshot.Session.ID)

		require.ErrorIs(t, err, domain.ErrSessionNotFinished)
	})

	t.Run("разбор, карта признаков, рекомендации, следующий шаг", func(t *testing.T) {
		service, _ := newTestService(t)

		session := complete(t, service, owner, []string{"b", "c", "a"})

		debrief, err := service.Result(ctx, owner, session.ID)
		require.NoError(t, err)

		require.Equal(t, 3, debrief.Result.AnswersCount)
		require.Equal(t, 0, debrief.Result.Score)
		require.Equal(t, 50, debrief.Result.Percent)
		require.Equal(t, domain.LevelVulnerable, debrief.Result.Level)

		require.Len(t, debrief.Breakdown, 3)
		require.Equal(t, "s1", debrief.Breakdown[0].StepCode)
		require.Equal(t, "b", debrief.Breakdown[0].Chosen.Code)
		require.NotNil(t, debrief.Breakdown[0].SafeAlternative)
		require.Equal(t, "c", debrief.Breakdown[0].SafeAlternative.Code)
		require.Len(t, debrief.Breakdown[0].RiskSignals, 2)

		byCode := make(map[string]domain.SignalOutcome, len(debrief.Signals))
		for _, outcome := range debrief.Signals {
			byCode[outcome.Signal.Code] = outcome
		}

		require.False(t, byCode["RISK_TOO_GOOD_PRICE"].Recognized, "s1 — спорный выбор")
		require.False(t, byCode["RISK_URGENCY_PRESSURE"].Recognized, "s1 — спорный выбор")
		require.True(t, byCode["RISK_PREPAY_OUTSIDE"].Recognized, "s2 — безопасный выбор")
		require.False(t, byCode["RISK_PHISHING_LINK"].Recognized, "s3 — опасный выбор")

		require.NotEmpty(t, debrief.Recommendations)
		require.LessOrEqual(t, len(debrief.Recommendations), 3)

		require.Nil(t, debrief.Comparison, "первой попытке сравнение не полагается")

		require.Equal(t, domain.NextStepRetryScenario, debrief.NextStep.Type, "50% ниже порога — повторное прохождение")
	})

	t.Run("сравнение со второй попыткой", func(t *testing.T) {
		service, _ := newTestService(t)

		complete(t, service, owner, []string{"b", "c", "a"})

		second := complete(t, service, owner, []string{"c", "c", "c"})

		debrief, err := service.Result(ctx, owner, second.ID)
		require.NoError(t, err)

		require.NotNil(t, debrief.Comparison)
		require.Equal(t, 50, debrief.Comparison.PreviousPercent)
		require.Equal(t, 0, debrief.Comparison.PreviousScore)
		require.Equal(t, 50, debrief.Comparison.DeltaPercent, "100% − 50%")

		require.Equal(t, domain.NextStepAllDone, debrief.NextStep.Type, "идеальное прохождение — все сценарии пройдены")
	})

	t.Run("чужой результат — 404", func(t *testing.T) {
		service, _ := newTestService(t)

		otherOwner := domain.UserOwner(uuid.New())

		session := complete(t, service, owner, []string{"c", "c", "c"})

		_, err := service.Result(ctx, otherOwner, session.ID)

		require.ErrorIs(t, err, domain.ErrNotFound)
	})
}

// Согласованность балла и суммы ответов — инвариант, который держит схема:
// сервис всегда пишет ответ и балл одной транзакцией.
func TestScoreMatchesAnswers(t *testing.T) {
	ctx := context.Background()
	service, sessionRepo := newTestService(t)
	userID := uuid.New()
	owner := domain.UserOwner(userID)

	session := complete(t, service, owner, []string{"a", "b", "c"})

	answers, err := sessionRepo.ListAnswers(ctx, session.ID)
	require.NoError(t, err)

	sum := 0
	for _, answer := range answers {
		sum += answer.ScoreDelta
	}

	require.Equal(t, sum, session.Score)
}

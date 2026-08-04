package domain

type Role string

const (
	RoleBuyer  Role = "buyer"
	RoleSeller Role = "seller"
)

func (r Role) Valid() bool { return r == RoleBuyer || r == RoleSeller }

type Side string

const (
	SideBuyer  Side = "buyer"
	SideSeller Side = "seller"
)

func (s Side) Valid() bool { return s == SideBuyer || s == SideSeller }

type Difficulty string

const (
	DifficultyBasic    Difficulty = "basic"
	DifficultyAdvanced Difficulty = "advanced"
	DifficultyDemo     Difficulty = "demo"
)

func (d Difficulty) Valid() bool {
	return d == DifficultyBasic || d == DifficultyAdvanced || d == DifficultyDemo
}

// Outcome — последствие выбранного варианта: риск распознан, диалог продолжен
// без защиты, действие стоило бы денег или аккаунта.
type Outcome string

const (
	OutcomeSafe     Outcome = "safe"
	OutcomeRisky    Outcome = "risky"
	OutcomeCritical Outcome = "critical"
)

// Weights — шкала весов (FR20). Три значения делают оценки сопоставимыми
// между сценариями: при свободных весах одинаковый процент означал бы разное
// качество решений.
var Weights = map[Outcome]int{
	OutcomeSafe:     10,
	OutcomeRisky:    0,
	OutcomeCritical: -10,
}

func (o Outcome) Valid() bool {
	_, ok := Weights[o]
	return ok
}

func (o Outcome) Score() int { return Weights[o] }

type StepType string

const (
	StepTypeDialog StepType = "dialog"
	// StepTypeTerminal завершает сессию (FR14).
	StepTypeTerminal StepType = "terminal"
)

func (t StepType) Valid() bool { return t == StepTypeDialog || t == StepTypeTerminal }

type MessageSender string

const (
	SenderCounterparty MessageSender = "counterparty"
	SenderPlatform     MessageSender = "platform"
	SenderNarrator     MessageSender = "narrator"
)

type Scenario struct {
	ID               int64
	Code             string
	Version          int
	Role             Role
	Title            string
	Description      string
	Intro            string
	Difficulty       Difficulty
	StepsCount       int // число шагов dialog, то есть число решений
	EstimatedMinutes int
	IsActive         bool
	RiskSignalCodes  []string
	Steps            []Step // только при загрузке сценария целиком
}

type Step struct {
	ID              int64
	ScenarioID      int64
	Code            string
	Type            StepType
	Position        int
	Content         StepContent
	RiskSignalCodes []string
	IsStart         bool
	Options         []Option
}

type StepContent struct {
	Message    string
	Sender     MessageSender
	Context    string
	Attachment *Attachment
}

// Attachment — имитация вложения в переписке (SEC8: ссылки нерабочие).
type Attachment struct {
	Kind    string // link | screenshot | document
	Caption string
}

type Option struct {
	ID           int64
	StepID       int64
	Code         string
	Text         string
	Outcome      Outcome
	Score        int
	Feedback     string
	NextStepCode string
	Position     int
}

// RiskSignal — признак риска из каталога seed/risk_signals.json.
type RiskSignal struct {
	Code           string
	Side           Side
	Title          string
	Summary        string
	Description    string
	HowToRecognize []string
	HowToAct       string
}

func (s Scenario) FindStep(code string) (Step, bool) {
	for _, step := range s.Steps {
		if step.Code == code {
			return step, true
		}
	}

	return Step{}, false
}

func (s Scenario) StartStep() (Step, bool) {
	for _, step := range s.Steps {
		if step.IsStart {
			return step, true
		}
	}

	return Step{}, false
}

func (s Step) FindOption(code string) (Option, bool) {
	for _, option := range s.Options {
		if option.Code == code {
			return option, true
		}
	}

	return Option{}, false
}

// SafeOption — «как следовало поступить» для обратной связи и разбора.
func (s Step) SafeOption() (Option, bool) {
	for _, option := range s.Options {
		if option.Outcome == OutcomeSafe {
			return option, true
		}
	}

	return Option{}, false
}

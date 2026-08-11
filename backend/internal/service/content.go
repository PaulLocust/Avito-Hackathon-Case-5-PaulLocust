package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/domain"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/repository"
)

// Файл схемы лежит рядом со сценариями и сам сценарием не является.
const scenarioSchemaFile = "scenario.schema.json"

type contentService struct {
	scenarios repository.ScenarioRepository
	signals   repository.RiskSignalRepository
}

var _ ContentService = (*contentService)(nil)

// LoadFromDir заливает контент из каталога: `risk_signals.json` и
// `scenarios/*.json`.
//
// Загрузка идемпотентна: версия сценария поднимается только при изменении
// содержимого файла, поэтому повторный запуск на неизменном контенте ничего
// не создаёт и не влияет на уже начатые сессии.
//
// Сценарий, не прошедший валидацию, не загружается, а его нарушения попадают
// в отчёт; ранее загруженные версии при этом не изменяются. Остальные
// сценарии загружаются как обычно — одна ошибка в контенте не блокирует весь
// каталог.
func (s *contentService) LoadFromDir(ctx context.Context, dir string) (LoadReport, error) {
	report := LoadReport{Issues: make(map[string][]domain.Issue)}

	signals, err := readRiskSignals(filepath.Join(dir, "risk_signals.json"))
	if err != nil {
		return report, err
	}

	if upsertErr := s.signals.Upsert(ctx, signals); upsertErr != nil {
		return report, fmt.Errorf("загрузка справочника признаков риска: %w", upsertErr)
	}

	report.SignalsLoaded = len(signals)

	// Каталог нужен валидатору: ссылка на несуществующий признак — ошибка.
	known := make(map[string]domain.RiskSignal, len(signals))
	for _, signal := range signals {
		known[signal.Code] = signal
	}

	files, err := scenarioFiles(filepath.Join(dir, "scenarios"))
	if err != nil {
		return report, err
	}

	for _, path := range files {
		if err := s.loadScenario(ctx, path, known, &report); err != nil {
			return report, err
		}
	}

	return report, nil
}

func (s *contentService) loadScenario(
	ctx context.Context,
	path string,
	known map[string]domain.RiskSignal,
	report *LoadReport,
) error {
	//nolint:gosec // G304: путь берётся из каталога контента в конфигурации, не из пользовательского ввода.
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("чтение сценария %s: %w", path, err)
	}

	name := filepath.Base(path)

	var file scenarioFile
	if parseErr := json.Unmarshal(raw, &file); parseErr != nil {
		report.Issues[name] = []domain.Issue{{Path: "", Message: "файл не разбирается как JSON: " + parseErr.Error()}}
		return nil
	}

	scenario := file.toDomain()

	if issues := domain.ValidateScenario(scenario, known); len(issues) > 0 {
		report.Issues[name] = issues
		return nil
	}

	// Хеш считается по содержимому файла: любая правка текста даёт новую
	// версию, переформатирование — тоже, и это честнее, чем угадывать
	// значимость изменения.
	sum := sha256.Sum256(raw)

	version, created, err := s.scenarios.Upsert(ctx, scenario, hex.EncodeToString(sum[:]))
	if err != nil {
		return fmt.Errorf("сохранение сценария %s: %w", scenario.Code, err)
	}

	switch {
	case !created:
		report.ScenariosSkipped = append(report.ScenariosSkipped, scenario.Code)
	case version == 1:
		report.ScenariosCreated = append(report.ScenariosCreated, scenario.Code)
	default:
		report.ScenariosUpdated = append(report.ScenariosUpdated, scenario.Code)
	}

	return nil
}

// scenarioFiles возвращает файлы сценариев в стабильном порядке: витрина
// сортируется по идентификатору, и порядок загрузки не должен зависеть от
// файловой системы.
func scenarioFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("чтение каталога сценариев %s: %w", dir, err)
	}

	files := make([]string, 0, len(entries))

	for _, entry := range entries {
		name := entry.Name()

		if entry.IsDir() || filepath.Ext(name) != ".json" || name == scenarioSchemaFile {
			continue
		}

		files = append(files, filepath.Join(dir, name))
	}

	sort.Strings(files)

	return files, nil
}

func readRiskSignals(path string) ([]domain.RiskSignal, error) {
	//nolint:gosec // G304: путь берётся из каталога контента в конфигурации, не из пользовательского ввода.
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("чтение каталога признаков риска %s: %w", path, err)
	}

	var files []riskSignalFile
	if err := json.Unmarshal(raw, &files); err != nil {
		return nil, fmt.Errorf("разбор каталога признаков риска: %w", err)
	}

	signals := make([]domain.RiskSignal, 0, len(files))
	for _, file := range files {
		signals = append(signals, domain.RiskSignal{
			Code:           file.Code,
			Side:           domain.Side(file.Side),
			Title:          file.Title,
			Summary:        file.Summary,
			Description:    file.Description,
			HowToRecognize: file.HowToRecognize,
			HowToAct:       file.HowToAct,
		})
	}

	return signals, nil
}

// Формат файлов контента. Отдельный слой от доменных структур: файл может
// содержать поля, которых нет в домене (например, ссылку на схему), а домен —
// поля, которые в файле не задаются (версия, флаг публикации).

type riskSignalFile struct {
	Code           string   `json:"code"`
	Side           string   `json:"side"`
	Title          string   `json:"title"`
	Summary        string   `json:"summary"`
	Description    string   `json:"description"`
	HowToRecognize []string `json:"how_to_recognize"`
	HowToAct       string   `json:"how_to_act"`
}

type scenarioFile struct {
	Code             string     `json:"code"`
	Role             string     `json:"role"`
	Title            string     `json:"title"`
	Description      string     `json:"description"`
	Intro            string     `json:"intro"`
	Difficulty       string     `json:"difficulty"`
	EstimatedMinutes int        `json:"estimated_minutes"`
	StartStep        string     `json:"start_step"`
	Steps            []stepFile `json:"steps"`
}

type stepFile struct {
	Code        string       `json:"code"`
	Type        string       `json:"type"`
	Content     contentFile  `json:"content"`
	RiskSignals []string     `json:"risk_signals"`
	Options     []optionFile `json:"options"`
}

type contentFile struct {
	Message    string          `json:"message"`
	Sender     string          `json:"sender"`
	Context    string          `json:"context"`
	Attachment *attachmentFile `json:"attachment"`
}

type attachmentFile struct {
	Kind    string `json:"kind"`
	Caption string `json:"caption"`
}

type optionFile struct {
	Code     string `json:"code"`
	Text     string `json:"text"`
	Outcome  string `json:"outcome"`
	Score    int    `json:"score"`
	Feedback string `json:"feedback"`
	NextStep string `json:"next_step"`
}

func (f scenarioFile) toDomain() domain.Scenario {
	steps := make([]domain.Step, 0, len(f.Steps))
	dialogSteps := 0

	for position, step := range f.Steps {
		if step.Type == string(domain.StepTypeDialog) {
			dialogSteps++
		}

		steps = append(steps, step.toDomain(position+1, step.Code == f.StartStep))
	}

	estimated := f.EstimatedMinutes
	if estimated <= 0 {
		estimated = 3
	}

	return domain.Scenario{
		Code:        f.Code,
		Role:        domain.Role(f.Role),
		Title:       f.Title,
		Description: f.Description,
		Intro:       f.Intro,
		Difficulty:  domain.Difficulty(f.Difficulty),
		// Витрина показывает число решений, поэтому терминальные шаги в
		// счётчик не входят.
		StepsCount:       dialogSteps,
		EstimatedMinutes: estimated,
		IsActive:         true,
		Steps:            steps,
	}
}

func (f stepFile) toDomain(position int, isStart bool) domain.Step {
	sender := domain.MessageSender(f.Content.Sender)
	if sender == "" {
		sender = domain.SenderCounterparty
	}

	content := domain.StepContent{
		Message: f.Content.Message,
		Sender:  sender,
		Context: f.Content.Context,
	}

	if f.Content.Attachment != nil {
		content.Attachment = &domain.Attachment{
			Kind:    f.Content.Attachment.Kind,
			Caption: f.Content.Attachment.Caption,
		}
	}

	options := make([]domain.Option, 0, len(f.Options))
	for optionPosition, option := range f.Options {
		options = append(options, domain.Option{
			Code:         option.Code,
			Text:         option.Text,
			Outcome:      domain.Outcome(option.Outcome),
			Score:        option.Score,
			Feedback:     option.Feedback,
			NextStepCode: option.NextStep,
			Position:     optionPosition + 1,
		})
	}

	return domain.Step{
		Code:            f.Code,
		Type:            domain.StepType(f.Type),
		Position:        position,
		Content:         content,
		RiskSignalCodes: f.RiskSignals,
		IsStart:         isStart,
		Options:         options,
	}
}

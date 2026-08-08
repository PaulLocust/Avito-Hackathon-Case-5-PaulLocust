package service_test

import (
	"context"

	"github.com/google/uuid"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/config"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/domain"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/repository"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/service"
)

// Заглушки репозиториев встраивают интерфейс: методы, которых нет в тесте,
// вызовут панику, и станет видно, что сервис ходит туда, куда не должен.

type fakeScenarios struct {
	repository.ScenarioRepository

	active   []domain.Scenario
	byCode   map[string]domain.Scenario
	bySignal map[string][]domain.Scenario
	err      error

	gotRole   *domain.Role
	gotSignal string
}

func (f *fakeScenarios) ListActive(_ context.Context, role *domain.Role) ([]domain.Scenario, error) {
	f.gotRole = role

	if f.err != nil {
		return nil, f.err
	}

	return f.active, nil
}

func (f *fakeScenarios) GetActiveByCode(_ context.Context, code string) (domain.Scenario, error) {
	if f.err != nil {
		return domain.Scenario{}, f.err
	}

	scenario, ok := f.byCode[code]
	if !ok {
		return domain.Scenario{}, domain.ErrNotFound
	}

	return scenario, nil
}

func (f *fakeScenarios) ListBySignal(_ context.Context, signalCode string) ([]domain.Scenario, error) {
	f.gotSignal = signalCode

	if f.err != nil {
		return nil, f.err
	}

	return f.bySignal[signalCode], nil
}

type fakeProgress struct {
	repository.ProgressRepository

	stats  map[string]domain.UserScenarioStats
	err    error
	called bool
}

func (f *fakeProgress) ScenarioStats(
	_ context.Context,
	_ domain.Owner,
) (map[string]domain.UserScenarioStats, error) {
	f.called = true

	if f.err != nil {
		return nil, f.err
	}

	return f.stats, nil
}

type fakeSessions struct {
	repository.SessionRepository

	active map[string]domain.Session
}

func (f *fakeSessions) ClaimByGuest(
	_ context.Context,
	_ uuid.UUID,
	_ uuid.UUID,
) error {
	return nil
}

func (f *fakeSessions) GetActiveByOwnerScenario(
	_ context.Context,
	owner domain.Owner,
	scenarioCode string,
) (domain.Session, error) {
	session, ok := f.active[scenarioCode]
	if !ok {
		return domain.Session{}, domain.ErrNotFound
	}

	return session, nil
}

func (f *fakeSessions) GetActiveByUserScenario(
	_ context.Context,
	_ uuid.UUID,
	scenarioCode string,
) (domain.Session, error) {
	session, ok := f.active[scenarioCode]
	if !ok {
		return domain.Session{}, domain.ErrNotFound
	}

	return session, nil
}

type fakeSignals struct {
	repository.RiskSignalRepository

	list    []domain.RiskSignal
	byCode  map[string]domain.RiskSignal
	err     error
	gotSide *domain.Side
}

func (f *fakeSignals) List(_ context.Context, side *domain.Side) ([]domain.RiskSignal, error) {
	f.gotSide = side

	if f.err != nil {
		return nil, f.err
	}

	return f.list, nil
}

func (f *fakeSignals) Get(_ context.Context, code string) (domain.RiskSignal, error) {
	if f.err != nil {
		return domain.RiskSignal{}, f.err
	}

	signal, ok := f.byCode[code]
	if !ok {
		return domain.RiskSignal{}, domain.ErrNotFound
	}

	return signal, nil
}

func newServices(repos *repository.Repositories) *service.Services {
	cfg := config.Config{
		Scoring: config.ScoringConfig{
			Thresholds: domain.Thresholds{Resilient: 80, Attentive: 60},
		},
	}

	return service.New(repos, cfg)
}

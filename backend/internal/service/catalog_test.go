package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/domain"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/repository"
)

func buyerScenario() domain.Scenario {
	return domain.Scenario{
		ID:         1,
		Code:       "too-good-price",
		Version:    1,
		Role:       domain.RoleBuyer,
		Title:      "Слишком выгодная цена",
		Difficulty: domain.DifficultyDemo,
		StepsCount: 3,
		IsActive:   true,
		Steps: []domain.Step{
			{Code: "s1", Type: domain.StepTypeDialog},
		},
	}
}

func sellerScenario() domain.Scenario {
	return domain.Scenario{
		ID:         2,
		Code:       "payment-screenshot",
		Version:    1,
		Role:       domain.RoleSeller,
		Title:      "Скриншот перевода",
		Difficulty: domain.DifficultyBasic,
		StepsCount: 6,
		IsActive:   true,
	}
}

// Гость видит витрину, но без блока прогресса (FR4): к статистике сервис
// не обращается вовсе.
func TestCatalogListForGuest(t *testing.T) {
	scenarios := &fakeScenarios{active: []domain.Scenario{buyerScenario(), sellerScenario()}}
	progress := &fakeProgress{}

	services := newServices(&repository.Repositories{Scenarios: scenarios, Progress: progress})

	cards, err := services.Catalog.List(context.Background(), nil, nil)

	require.NoError(t, err)
	require.Len(t, cards, 2)
	require.Nil(t, cards[0].Stats)
	require.Nil(t, cards[1].Stats)
	require.False(t, progress.called, "гостю статистика не запрашивается")
}

func TestCatalogListFiltersByRole(t *testing.T) {
	scenarios := &fakeScenarios{active: []domain.Scenario{sellerScenario()}}
	services := newServices(&repository.Repositories{Scenarios: scenarios, Progress: &fakeProgress{}})

	role := domain.RoleSeller
	cards, err := services.Catalog.List(context.Background(), nil, &role)

	require.NoError(t, err)
	require.Len(t, cards, 1)
	require.NotNil(t, scenarios.gotRole)
	require.Equal(t, domain.RoleSeller, *scenarios.gotRole)
}

// Процент и уровень лучшей попытки считаются сервисом из сырого агрегата:
// 20 баллов за 3 выбора — это (20 + 30) / 60 = 83%, то есть «устойчив».
func TestCatalogListAddsProgressForUser(t *testing.T) {
	lastAttempt := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)

	scenarios := &fakeScenarios{active: []domain.Scenario{buyerScenario(), sellerScenario()}}
	progress := &fakeProgress{stats: map[string]domain.UserScenarioStats{
		"too-good-price": {
			Attempted:     true,
			AttemptsCount: 2,
			BestScore:     20,
			BestAnswers:   3,
			LastAttemptAt: &lastAttempt,
		},
	}}

	services := newServices(&repository.Repositories{Scenarios: scenarios, Progress: progress})

	userID := uuid.New()
	cards, err := services.Catalog.List(context.Background(), &userID, nil)

	require.NoError(t, err)
	require.Len(t, cards, 2)

	attempted := cards[0].Stats
	require.NotNil(t, attempted)
	require.True(t, attempted.Attempted)
	require.Equal(t, 2, attempted.AttemptsCount)
	require.NotNil(t, attempted.BestPercent)
	require.Equal(t, 83, *attempted.BestPercent)
	require.NotNil(t, attempted.BestLevel)
	require.Equal(t, domain.LevelResilient, *attempted.BestLevel)
	require.Equal(t, &lastAttempt, attempted.LastAttemptAt)

	// Сценарий без попыток тоже получает блок прогресса — с отметкой,
	// что тренировка ещё не проходилась (FR6).
	untouched := cards[1].Stats
	require.NotNil(t, untouched)
	require.False(t, untouched.Attempted)
	require.Nil(t, untouched.BestPercent)
	require.Nil(t, untouched.BestLevel)
}

func TestCatalogListLevelBelowThreshold(t *testing.T) {
	scenarios := &fakeScenarios{active: []domain.Scenario{buyerScenario()}}
	progress := &fakeProgress{stats: map[string]domain.UserScenarioStats{
		// 10 баллов за 6 выборов: (10 + 60) / 120 = 58% — «уязвим».
		"too-good-price": {Attempted: true, AttemptsCount: 1, BestScore: 10, BestAnswers: 6},
	}}

	services := newServices(&repository.Repositories{Scenarios: scenarios, Progress: progress})

	userID := uuid.New()
	cards, err := services.Catalog.List(context.Background(), &userID, nil)

	require.NoError(t, err)
	require.Equal(t, 58, *cards[0].Stats.BestPercent)
	require.Equal(t, domain.LevelVulnerable, *cards[0].Stats.BestLevel)
}

// Карточка не должна раскрывать содержание диалога до старта тренировки.
func TestCatalogGetHidesSteps(t *testing.T) {
	scenarios := &fakeScenarios{byCode: map[string]domain.Scenario{"too-good-price": buyerScenario()}}
	services := newServices(&repository.Repositories{
		Scenarios: scenarios,
		Progress:  &fakeProgress{},
		Sessions:  &fakeSessions{},
	})

	card, err := services.Catalog.Get(context.Background(), nil, "too-good-price")

	require.NoError(t, err)
	require.Equal(t, "too-good-price", card.Scenario.Code)
	require.Empty(t, card.Scenario.Steps)
	require.Nil(t, card.ActiveSessionID)
}

func TestCatalogGetUnknownScenario(t *testing.T) {
	services := newServices(&repository.Repositories{
		Scenarios: &fakeScenarios{byCode: map[string]domain.Scenario{}},
		Progress:  &fakeProgress{},
		Sessions:  &fakeSessions{},
	})

	_, err := services.Catalog.Get(context.Background(), nil, "unknown")

	require.ErrorIs(t, err, domain.ErrNotFound)
}

// Незавершённая сессия попадает в карточку, чтобы экран предложил
// продолжить тренировку, а не начать её заново (FR12).
func TestCatalogGetReportsActiveSession(t *testing.T) {
	sessionID := uuid.New()

	services := newServices(&repository.Repositories{
		Scenarios: &fakeScenarios{byCode: map[string]domain.Scenario{"too-good-price": buyerScenario()}},
		Progress:  &fakeProgress{},
		Sessions: &fakeSessions{active: map[string]domain.Session{
			"too-good-price": {ID: sessionID, Status: domain.StatusInProgress},
		}},
	})

	userID := uuid.New()
	card, err := services.Catalog.Get(context.Background(), &userID, "too-good-price")

	require.NoError(t, err)
	require.NotNil(t, card.ActiveSessionID)
	require.Equal(t, sessionID, *card.ActiveSessionID)
}

func TestCatalogGetWithoutActiveSession(t *testing.T) {
	services := newServices(&repository.Repositories{
		Scenarios: &fakeScenarios{byCode: map[string]domain.Scenario{"too-good-price": buyerScenario()}},
		Progress:  &fakeProgress{},
		Sessions:  &fakeSessions{active: map[string]domain.Session{}},
	})

	userID := uuid.New()
	card, err := services.Catalog.Get(context.Background(), &userID, "too-good-price")

	require.NoError(t, err)
	require.Nil(t, card.ActiveSessionID)
}

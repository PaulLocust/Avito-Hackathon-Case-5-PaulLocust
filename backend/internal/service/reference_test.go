package service_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/domain"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/repository"
)

func phishingSignal() domain.RiskSignal {
	return domain.RiskSignal{
		Code:           "RISK_PHISHING_LINK",
		Side:           domain.SideBuyer,
		Title:          "Фишинговая ссылка",
		Summary:        "Ссылка на поддельную страницу оплаты.",
		Description:    "Домен только похож на официальный.",
		HowToRecognize: []string{"Домен отличается приставкой или дефисом."},
		HowToAct:       "Не переходите по ссылкам из переписки.",
	}
}

func TestReferenceListSignals(t *testing.T) {
	signals := &fakeSignals{list: []domain.RiskSignal{phishingSignal()}}
	services := newServices(&repository.Repositories{RiskSignals: signals})

	got, err := services.Reference.ListSignals(context.Background(), nil)

	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "RISK_PHISHING_LINK", got[0].Code)
	require.Nil(t, signals.gotSide, "без фильтра сторона в репозиторий не передаётся")
}

func TestReferenceListSignalsBySide(t *testing.T) {
	signals := &fakeSignals{list: []domain.RiskSignal{}}
	services := newServices(&repository.Repositories{RiskSignals: signals})

	side := domain.SideSeller
	got, err := services.Reference.ListSignals(context.Background(), &side)

	require.NoError(t, err)
	require.Empty(t, got)
	require.NotNil(t, signals.gotSide)
	require.Equal(t, domain.SideSeller, *signals.gotSide)
}

// Из карточки признака должен быть путь в тренировку: справочник
// перечисляет сценарии, где этот признак отрабатывается (FR29).
func TestReferenceGetSignalListsRelatedScenarios(t *testing.T) {
	scenarios := &fakeScenarios{bySignal: map[string][]domain.Scenario{
		"RISK_PHISHING_LINK": {buyerScenario()},
	}}
	signals := &fakeSignals{byCode: map[string]domain.RiskSignal{
		"RISK_PHISHING_LINK": phishingSignal(),
	}}

	services := newServices(&repository.Repositories{RiskSignals: signals, Scenarios: scenarios})

	detail, err := services.Reference.GetSignal(context.Background(), "RISK_PHISHING_LINK")

	require.NoError(t, err)
	require.Equal(t, "RISK_PHISHING_LINK", detail.Signal.Code)
	require.Equal(t, domain.SideBuyer, detail.Signal.Side)
	require.Len(t, detail.RelatedScenarios, 1)
	require.Equal(t, "too-good-price", detail.RelatedScenarios[0].Code)
	require.Equal(t, "RISK_PHISHING_LINK", scenarios.gotSignal)
}

func TestReferenceGetSignalNotFound(t *testing.T) {
	services := newServices(&repository.Repositories{
		RiskSignals: &fakeSignals{byCode: map[string]domain.RiskSignal{}},
		Scenarios:   &fakeScenarios{},
	})

	_, err := services.Reference.GetSignal(context.Background(), "RISK_UNKNOWN")

	require.ErrorIs(t, err, domain.ErrNotFound)
}

// Признак без сценариев — нормальная ситуация: RISK_OVERPAYMENT в наборе
// к защите не задействован и остаётся в каталоге как резерв.
func TestReferenceGetSignalWithoutScenarios(t *testing.T) {
	services := newServices(&repository.Repositories{
		RiskSignals: &fakeSignals{byCode: map[string]domain.RiskSignal{
			"RISK_OVERPAYMENT": {Code: "RISK_OVERPAYMENT", Side: domain.SideSeller},
		}},
		Scenarios: &fakeScenarios{bySignal: map[string][]domain.Scenario{}},
	})

	detail, err := services.Reference.GetSignal(context.Background(), "RISK_OVERPAYMENT")

	require.NoError(t, err)
	require.Empty(t, detail.RelatedScenarios)
}

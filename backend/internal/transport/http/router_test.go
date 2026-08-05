package http_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/config"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/domain"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/service"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/transport/http/dto"

	httptransport "github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/transport/http"
)

// Тесты транспорта проверяют контракт, а не бизнес-логику: коды ответов,
// форму ошибок и разграничение публичных и защищённых маршрутов.
// Сервисы подменяются заглушками — БД для этого не нужна.

const validToken = "valid-token"

// errStub изображает недоступную зависимость в пробе готовности.
var errStub = errors.New("зависимость недоступна")

type stubAuth struct {
	user domain.User
}

func (s *stubAuth) Register(context.Context, string, string) (domain.User, service.Token, error) {
	return s.user, service.Token{Value: validToken}, nil
}

func (s *stubAuth) Login(context.Context, string, string) (domain.User, service.Token, error) {
	return s.user, service.Token{Value: validToken}, nil
}

func (s *stubAuth) Logout(context.Context, string) error { return nil }

func (s *stubAuth) Authenticate(_ context.Context, token string) (domain.User, error) {
	if token != validToken {
		return domain.User{}, domain.ErrUnauthorized
	}

	return s.user, nil
}

type stubCatalog struct {
	cards []domain.ScenarioCard
}

func (s *stubCatalog) List(context.Context, *uuid.UUID, *domain.Role) ([]domain.ScenarioCard, error) {
	return s.cards, nil
}

func (s *stubCatalog) Get(context.Context, *uuid.UUID, string) (domain.ScenarioCard, error) {
	return domain.ScenarioCard{}, domain.ErrNotFound
}

type stubProgress struct{}

func (s *stubProgress) Overview(context.Context, uuid.UUID) (domain.Progress, error) {
	return domain.Progress{TotalScenarios: 5}, nil
}

func (s *stubProgress) Signals(context.Context, uuid.UUID) ([]domain.SignalStat, error) {
	return nil, nil
}

func (s *stubProgress) Attempts(context.Context, uuid.UUID, string) ([]domain.Attempt, error) {
	return nil, nil
}

// stubTraining с настраиваемыми ответами: контрактные тесты проверяют только
// то, как транспорт превращает результат сервиса в код и форму ответа.
type stubTraining struct {
	startSnapshot domain.SessionSnapshot
	startErr      error
	getSnapshot   domain.SessionSnapshot
	getErr        error
	submitOutcome domain.AnswerOutcome
	submitErr     error
	abandonErr    error
	result        domain.Debrief
	resultErr     error
}

func (s *stubTraining) Start(context.Context, uuid.UUID, string, bool) (domain.SessionSnapshot, error) {
	return s.startSnapshot, s.startErr
}

func (s *stubTraining) Get(context.Context, uuid.UUID, uuid.UUID) (domain.SessionSnapshot, error) {
	return s.getSnapshot, s.getErr
}

func (s *stubTraining) SubmitAnswer(context.Context, uuid.UUID, uuid.UUID, string, string) (domain.AnswerOutcome, error) {
	return s.submitOutcome, s.submitErr
}

func (s *stubTraining) Abandon(context.Context, uuid.UUID, uuid.UUID) error {
	return s.abandonErr
}

func (s *stubTraining) Result(context.Context, uuid.UUID, uuid.UUID) (domain.Debrief, error) {
	return s.result, s.resultErr
}

type stubPinger struct{ err error }

func (s stubPinger) Ping(context.Context) error { return s.err }

func newTestServer(t *testing.T) http.Handler {
	t.Helper()

	return newTestServerWithTraining(t, &stubTraining{})
}

func newTestServerWithTraining(t *testing.T, training service.TrainingService) http.Handler {
	t.Helper()

	services := &service.Services{
		Auth:     &stubAuth{user: domain.User{ID: uuid.New(), Nickname: "tester"}},
		Catalog:  &stubCatalog{},
		Progress: &stubProgress{},
		Training: training,
	}

	cfg := config.Config{HTTP: config.HTTPConfig{AllowedOrigins: []string{"*"}}}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := httptransport.NewHandler(services, cfg, log, stubPinger{}, "test")

	return httptransport.NewRouter(handler, cfg, log)
}

// sampleSnapshot — состояние сессии, которое контрактный тест ожидает на
// выходе из сервиса.
func sampleSnapshot() domain.SessionSnapshot {
	return domain.SessionSnapshot{
		Session: domain.Session{
			ID:              uuid.New(),
			ScenarioID:      1,
			ScenarioCode:    "too-good-price",
			ScenarioVersion: 1,
			Status:          domain.StatusInProgress,
			CurrentStepCode: "s1",
			StartedAt:       time.Now(),
		},
		Scenario: domain.Scenario{
			Code: "too-good-price", Title: "Слишком выгодная цена",
			Role: domain.RoleBuyer, Difficulty: domain.DifficultyDemo, Version: 1,
		},
		CurrentStep: &domain.Step{
			Code: "s1", Type: domain.StepTypeDialog, Position: 1,
			Content: domain.StepContent{Message: "Реплика", Sender: domain.SenderCounterparty},
			Options: []domain.Option{{Code: "a", Text: "Опасно"}},
		},
		StepsTotal: 3,
	}
}

func decodeError(t *testing.T, recorder *httptest.ResponseRecorder) dto.ErrorResponse {
	t.Helper()

	var response dto.ErrorResponse
	require.NoError(t, json.NewDecoder(recorder.Body).Decode(&response))

	return response
}

func TestHealthEndpoints(t *testing.T) {
	server := newTestServer(t)

	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/healthz", http.NoBody))

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Header().Get("Content-Type"), "application/json")
}

func TestProtectedEndpointRequiresToken(t *testing.T) {
	server := newTestServer(t)

	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/progress", http.NoBody))

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.Equal(t, dto.CodeUnauthorized, decodeError(t, recorder).Error.Code)
}

func TestProtectedEndpointWithToken(t *testing.T) {
	server := newTestServer(t)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/progress", http.NoBody)
	request.Header.Set("Authorization", "Bearer "+validToken)

	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
}

// Витрина доступна гостю: без токена запрос не должен отклоняться (FR4).
func TestCatalogAvailableToGuest(t *testing.T) {
	server := newTestServer(t)

	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/scenarios", http.NoBody))

	require.Equal(t, http.StatusOK, recorder.Code)

	var response dto.ScenarioListResponse
	require.NoError(t, json.NewDecoder(recorder.Body).Decode(&response))
	require.NotNil(t, response.Items, "пустой список должен сериализоваться как [], а не null")
}

func TestInvalidRoleRejected(t *testing.T) {
	server := newTestServer(t)

	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/scenarios?role=admin", http.NoBody))

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Equal(t, dto.CodeValidationError, decodeError(t, recorder).Error.Code)
}

func TestRegisterValidation(t *testing.T) {
	server := newTestServer(t)

	tests := []struct {
		name string
		body string
	}{
		{name: "короткий ник", body: `{"nickname":"ab","password":"correct-horse-42"}`},
		{name: "ник с недопустимыми символами", body: `{"nickname":"ник!","password":"correct-horse-42"}`},
		{name: "короткий пароль", body: `{"nickname":"tester","password":"123"}`},
		{name: "лишнее поле", body: `{"nickname":"tester","password":"correct-horse-42","role":"admin"}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/json")

			recorder := httptest.NewRecorder()
			server.ServeHTTP(recorder, request)

			require.Equal(t, http.StatusBadRequest, recorder.Code)

			response := decodeError(t, recorder)
			require.Equal(t, dto.CodeValidationError, response.Error.Code)
			require.NotEmpty(t, response.Error.Details)
		})
	}
}

// Неизвестный маршрут должен отдавать тот же конверт ошибки, что и остальное
// API: клиенту не приходится разбирать HTML.
func TestUnknownRouteReturnsJSONError(t *testing.T) {
	server := newTestServer(t)

	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/unknown", http.NoBody))

	require.Equal(t, http.StatusNotFound, recorder.Code)
	require.Equal(t, dto.CodeNotFound, decodeError(t, recorder).Error.Code)
}

func TestReadinessReportsDatabaseFailure(t *testing.T) {
	services := &service.Services{Auth: &stubAuth{}}
	cfg := config.Config{}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := httptransport.NewHandler(services, cfg, log, stubPinger{err: errStub}, "test")
	server := httptransport.NewRouter(handler, cfg, log)

	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/readyz", http.NoBody))

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
}

func bearer(request *http.Request) {
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+validToken)
}

// Тренировка — защищённый маршрут: без токена сессию не создать.
func TestTrainingRequiresToken(t *testing.T) {
	server := newTestServer(t)

	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/sessions",
		strings.NewReader(`{"scenario_code":"too-good-price"}`)))

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.Equal(t, dto.CodeUnauthorized, decodeError(t, recorder).Error.Code)
}

func TestStartSession(t *testing.T) {
	server := newTestServerWithTraining(t, &stubTraining{startSnapshot: sampleSnapshot()})

	request := httptest.NewRequest(http.MethodPost, "/api/v1/sessions",
		strings.NewReader(`{"scenario_code":"too-good-price"}`))
	bearer(request)

	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusCreated, recorder.Code)

	var response dto.SessionState
	require.NoError(t, json.NewDecoder(recorder.Body).Decode(&response))
	require.Equal(t, "in_progress", response.Status)
	require.Equal(t, "too-good-price", response.Scenario.Code)
	require.Equal(t, 1, response.Scenario.Version)
	require.NotNil(t, response.CurrentStep)
	require.Equal(t, 1, response.CurrentStep.Position, "индикатор «шаг 1 из N»")
	require.Equal(t, 0, response.AnswersCount)
	require.Equal(t, 3, response.StepsTotal)
}

// Несуществующий вариант ответа — ошибка валидации, а не 500.
func TestSubmitAnswerEmptyStepRejected(t *testing.T) {
	server := newTestServerWithTraining(t, &stubTraining{})

	request := httptest.NewRequest(http.MethodPost,
		"/api/v1/sessions/"+uuid.New().String()+"/answers",
		strings.NewReader(`{"step_code":"","option_code":"a"}`))
	bearer(request)

	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Equal(t, dto.CodeValidationError, decodeError(t, recorder).Error.Code)
}

// Уже отвеченный шаг возвращает сохранённый результат (FR13) — контрактная
// форма ответа с already_answered.
func TestSubmitAnswerAlreadyAnswered(t *testing.T) {
	snapshot := sampleSnapshot()
	snapshot.Session.Status = domain.StatusCompleted
	snapshot.CurrentStep = nil

	server := newTestServerWithTraining(t, &stubTraining{
		submitOutcome: domain.AnswerOutcome{
			Answer:          domain.Answer{StepCode: "s1", OptionCode: "a", Outcome: domain.OutcomeSafe, ScoreDelta: 10},
			Option:          domain.Option{Code: "a", Feedback: "объяснение"},
			AlreadyAnswered: true,
			Snapshot:        snapshot,
		},
	})

	request := httptest.NewRequest(http.MethodPost,
		"/api/v1/sessions/"+snapshot.Session.ID.String()+"/answers",
		strings.NewReader(`{"step_code":"s1","option_code":"a"}`))
	bearer(request)

	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)

	var response dto.AnswerResult
	require.NoError(t, json.NewDecoder(recorder.Body).Decode(&response))
	require.True(t, response.AlreadyAnswered)
	require.Equal(t, "safe", response.Outcome)
	require.Equal(t, 10, response.ScoreDelta)
	require.Equal(t, "completed", response.Session.Status)
	require.Nil(t, response.Session.CurrentStep)
}

// Незавершённая сессия по сценарию — 409 с идентификатором активной.
func TestStartSessionAlreadyActive(t *testing.T) {
	activeID := uuid.New()
	server := newTestServerWithTraining(t, &stubTraining{
		startErr: &domain.ActiveSessionError{SessionID: activeID},
	})

	request := httptest.NewRequest(http.MethodPost, "/api/v1/sessions",
		strings.NewReader(`{"scenario_code":"too-good-price"}`))
	bearer(request)

	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusConflict, recorder.Code)

	response := decodeError(t, recorder)
	require.Equal(t, dto.CodeSessionAlreadyActive, response.Error.Code)
	require.Equal(t, activeID.String(), response.Error.Details["session_id"])
}

// Чужая сессия — 404, а не 403: факт существования не раскрывается (SEC2).
func TestForeignSessionNotFound(t *testing.T) {
	server := newTestServerWithTraining(t, &stubTraining{getErr: domain.ErrNotFound})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/"+uuid.New().String(), http.NoBody)
	bearer(request)

	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNotFound, recorder.Code)
	require.Equal(t, dto.CodeNotFound, decodeError(t, recorder).Error.Code)
}

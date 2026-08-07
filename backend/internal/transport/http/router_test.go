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

func (s *stubAuth) Register(context.Context, string, string) (domain.User, service.TokenPair, error) {
	return s.user, service.TokenPair{Access: service.Token{Value: validToken}}, nil
}

func (s *stubAuth) Login(context.Context, string, string) (domain.User, service.TokenPair, error) {
	return s.user, service.TokenPair{Access: service.Token{Value: validToken}}, nil
}

func (s *stubAuth) Refresh(context.Context, string) (domain.User, service.TokenPair, error) {
	return s.user, service.TokenPair{Access: service.Token{Value: validToken}}, nil
}

func (s *stubAuth) Logout(context.Context, string) error { return nil }

func (s *stubAuth) ClaimGuest(context.Context, uuid.UUID, string) error { return nil }

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

type stubGuest struct{}

func (stubGuest) Start(context.Context) (service.GuestSessionToken, error) {
	return service.GuestSessionToken{Value: "guest-token", OwnerID: uuid.New()}, nil
}

func (stubGuest) Validate(context.Context, string) (uuid.UUID, error) {
	return uuid.New(), nil
}

type stubPinger struct{ err error }

func (s stubPinger) Ping(context.Context) error { return s.err }

func newTestServer(t *testing.T) http.Handler {
	t.Helper()

	services := &service.Services{
		Auth:     &stubAuth{user: domain.User{ID: uuid.New(), Nickname: "tester"}},
		Guest:    stubGuest{},
		Catalog:  &stubCatalog{},
		Progress: &stubProgress{},
	}

	cfg := config.Config{HTTP: config.HTTPConfig{AllowedOrigins: []string{"*"}}}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := httptransport.NewHandler(services, cfg, log, stubPinger{}, "test")

	return httptransport.NewRouter(handler, cfg, log)
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

func TestGuestCookieCannotAccessAnalytics(t *testing.T) {
	server := newTestServer(t)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/progress", http.NoBody)
	request.AddCookie(&http.Cookie{Name: "guest_session", Value: "guest-token"})
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)

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

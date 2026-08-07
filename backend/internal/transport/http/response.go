// Package http — транспортный слой: маршруты, middleware, обработчики.
// Обработчик разбирает запрос, вызывает сервис и оформляет ответ по
// контракту api/openapi.yaml; бизнес-правил здесь нет.
package http

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/domain"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/logger"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/transport/http/dto"
)

const maxRequestBody = 64 << 10 // 64 КиБ

func writeJSON(w http.ResponseWriter, log *slog.Logger, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	if payload == nil {
		return
	}

	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Error("не удалось записать ответ", slog.String("error", err.Error()))
	}
}

// decodeJSON разбирает и валидирует тело запроса (SEC3).
func decodeJSON[T interface{ Validate() error }](w http.ResponseWriter, r *http.Request) (T, bool) {
	var payload T

	reader := http.MaxBytesReader(w, r.Body, maxRequestBody)
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&payload); err != nil {
		message := "Некорректное тело запроса"
		if errors.Is(err, io.EOF) {
			message = "Пустое тело запроса"
		}

		writeError(w, r, domain.NewValidationError("body", message))

		return payload, false
	}

	if err := payload.Validate(); err != nil {
		writeError(w, r, err)
		return payload, false
	}

	return payload, true
}

type errorMapping struct {
	status  int
	code    string
	message string
}

// mappings — ожидаемые ошибки. Всё остальное отдаётся как internal_error
// без подробностей (SEC6).
var mappings = []struct {
	target  error
	mapping errorMapping
}{
	{domain.ErrNotFound, errorMapping{http.StatusNotFound, dto.CodeNotFound, "Ресурс не найден"}},
	{domain.ErrUnauthorized, errorMapping{http.StatusUnauthorized, dto.CodeUnauthorized, "Требуется вход в систему"}},
	{domain.ErrInvalidCredentials, errorMapping{http.StatusUnauthorized, dto.CodeInvalidCredentials, "Неверный ник или пароль"}},
	{domain.ErrNicknameTaken, errorMapping{http.StatusConflict, dto.CodeNicknameTaken, "Пользователь с таким ником уже существует"}},
	{domain.ErrSessionFinished, errorMapping{http.StatusConflict, dto.CodeSessionFinished, "Тренировка уже завершена"}},
	{domain.ErrSessionNotFinished, errorMapping{http.StatusConflict, dto.CodeSessionNotFinished, "Тренировка ещё не завершена"}},
	{domain.ErrStepNotCurrent, errorMapping{http.StatusBadRequest, dto.CodeStepNotCurrent, "Шаг не является текущим"}},
	{domain.ErrOptionNotFound, errorMapping{http.StatusBadRequest, dto.CodeOptionNotFound, "Указанного варианта нет на этом шаге"}},
	{domain.ErrRefreshTokenInvalid, errorMapping{http.StatusUnauthorized, dto.CodeInvalidRefreshToken, "Refresh-токен недействителен или истёк"}},
	{domain.ErrRefreshTokenMissing, errorMapping{http.StatusBadRequest, dto.CodeInvalidRefreshToken, "Refresh-токен не передан"}},
	{domain.ErrGuestSessionExpired, errorMapping{http.StatusUnauthorized, dto.CodeGuestSessionExpired, "Гостевая сессия истекла"}},
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	ctx := r.Context()
	log := logger.FromContext(ctx, slog.Default())

	response := dto.ErrorResponse{RequestID: logger.RequestID(ctx)}
	status := http.StatusInternalServerError

	var validationErr *domain.ValidationError
	var activeSessionErr *domain.ActiveSessionError

	switch {
	case errors.As(err, &validationErr):
		status = http.StatusBadRequest
		response.Error = dto.ErrorBody{
			Code:    dto.CodeValidationError,
			Message: "Проверьте правильность заполнения полей",
			Details: map[string]any{"fields": validationErr.Fields},
		}

	case errors.As(err, &activeSessionErr):
		status = http.StatusConflict
		response.Error = dto.ErrorBody{
			Code:    dto.CodeSessionAlreadyActive,
			Message: "По этому сценарию есть незавершённая тренировка",
			Details: map[string]any{"session_id": activeSessionErr.SessionID.String()},
		}

	// Состояние каркаса: эндпоинт есть в контракте, модуль ещё не написан.
	case errors.Is(err, domain.ErrNotImplemented):
		status = http.StatusNotImplemented
		response.Error = dto.ErrorBody{
			Code:    dto.CodeInternalError,
			Message: "Эндпоинт ещё не реализован",
		}

	default:
		if mapping, ok := lookupMapping(err); ok {
			status = mapping.status
			response.Error = dto.ErrorBody{Code: mapping.code, Message: mapping.message}
		} else {
			response.Error = dto.ErrorBody{
				Code:    dto.CodeInternalError,
				Message: "Внутренняя ошибка сервиса",
			}
			log.Error("необработанная ошибка",
				slog.String("error", err.Error()),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
			)
		}
	}

	writeJSON(w, log, status, response)
}

func lookupMapping(err error) (errorMapping, bool) {
	for _, candidate := range mappings {
		if errors.Is(err, candidate.target) {
			return candidate.mapping, true
		}
	}

	return errorMapping{}, false
}

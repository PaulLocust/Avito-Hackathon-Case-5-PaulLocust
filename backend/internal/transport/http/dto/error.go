// Package dto — структуры запросов и ответов HTTP API.
//
// Доменные структуры в ответ не сериализуются: DTO защищают от утечки полей
// и позволяют менять домен, не ломая фронтенд. Источник истины —
// api/openapi.yaml, правки едут в одном pull request со спецификацией.
package dto

// Коды ошибок из перечисления ErrorCode в OpenAPI: клиент принимает решения
// по коду, а не по тексту сообщения.
const (
	CodeValidationError = "validation_error"
	CodeUnauthorized    = "unauthorized"
	//nolint:gosec // G101: это машинный код ошибки для клиента, а не секрет.
	CodeInvalidCredentials   = "invalid_credentials"
	CodeForbidden            = "forbidden"
	CodeNotFound             = "not_found"
	CodeNicknameTaken        = "nickname_taken"
	CodeSessionAlreadyActive = "session_already_active"
	CodeSessionFinished      = "session_finished"
	CodeSessionNotFinished   = "session_not_finished"
	CodeStepNotCurrent       = "step_not_current"
	CodeOptionNotFound       = "option_not_found"
	CodeInternalError        = "internal_error"

	CodeInvalidRefreshToken = "invalid_refresh_token"
	CodeGuestSessionExpired = "guest_session_expired"
)

type ErrorResponse struct {
	Error     ErrorBody `json:"error"`
	RequestID string    `json:"request_id,omitempty"`
}

// ErrorBody: Details не должен раскрывать детали реализации (SEC6).
type ErrorBody struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

type HealthResponse struct {
	Status  string            `json:"status"`
	Version string            `json:"version,omitempty"`
	Checks  map[string]string `json:"checks,omitempty"`
}

package domain

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// Доменные ошибки. Транспорт переводит их в коды ответа в response.go.
var (
	// ErrNotFound возвращается и на чужой ресурс: факт его существования не
	// раскрывается (SEC2).
	ErrNotFound           = errors.New("ресурс не найден")
	ErrNicknameTaken      = errors.New("ник уже занят")
	ErrInvalidCredentials = errors.New("неверный ник или пароль")
	ErrUnauthorized       = errors.New("требуется авторизация")
	ErrSessionFinished    = errors.New("тренировка уже завершена")
	ErrSessionNotFinished = errors.New("тренировка ещё не завершена")
	ErrStepNotCurrent     = errors.New("шаг не является текущим")
	ErrOptionNotFound     = errors.New("вариант не найден на этом шаге")

	// ErrNotImplemented — заглушка каркаса, транспорт отдаёт на неё 501.
	ErrNotImplemented = errors.New("функциональность ещё не реализована")
)

// ActiveSessionError — по сценарию уже есть незавершённая сессия.
// Идентификатор нужен клиенту, чтобы предложить её продолжить.
type ActiveSessionError struct {
	SessionID uuid.UUID
}

func (e *ActiveSessionError) Error() string {
	return fmt.Sprintf("по сценарию есть незавершённая сессия %s", e.SessionID)
}

// ValidationError — ошибка проверки входных данных: поле → сообщение.
type ValidationError struct {
	Fields map[string]string
}

func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{Fields: map[string]string{field: message}}
}

func (e *ValidationError) Add(field, message string) {
	if e.Fields == nil {
		e.Fields = make(map[string]string)
	}

	e.Fields[field] = message
}

func (e *ValidationError) Empty() bool { return len(e.Fields) == 0 }

func (e *ValidationError) Error() string {
	parts := make([]string, 0, len(e.Fields))
	for field, message := range e.Fields {
		parts = append(parts, field+": "+message)
	}

	return "ошибка валидации: " + strings.Join(parts, "; ")
}

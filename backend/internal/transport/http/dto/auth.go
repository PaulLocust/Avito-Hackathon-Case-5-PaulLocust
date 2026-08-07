package dto

import (
	"regexp"
	"time"
	"unicode/utf8"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/domain"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/service"
)

// internal/transport/http/dto/auth.go

type CredentialsRequest struct {
	Nickname string `json:"nickname"`
	Password string `json:"password"`
}

// Ограничения повторяют схемы из OpenAPI. Верхняя граница пароля продиктована
// bcrypt: байты сверх 72 алгоритм отбрасывает молча.
const (
	nicknameMinLength = 3
	nicknameMaxLength = 32
	passwordMinLength = 8
	passwordMaxLength = 72
)

var nicknamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

type RegisterRequest struct {
	Nickname string `json:"nickname"`
	Password string `json:"password"`
}

func (r CredentialsRequest) Validate() error {
	ve := &domain.ValidationError{}

	if len(r.Nickname) < 3 || len(r.Nickname) > 32 {
		ve.Add("nickname", "должен быть от 3 до 32 символов")
	}
	if len(r.Password) < 8 {
		ve.Add("password", "должен быть не короче 8 символов")
	}

	if ve.Empty() {
		return nil
	}
	return ve
}

func (r RegisterRequest) Validate() error {
	validationErr := &domain.ValidationError{}

	if length := utf8.RuneCountInString(r.Nickname); length < nicknameMinLength || length > nicknameMaxLength {
		validationErr.Add("nickname", "Ник должен быть длиной от 3 до 32 символов")
	} else if !nicknamePattern.MatchString(r.Nickname) {
		validationErr.Add("nickname", "Допустимы латинские буквы, цифры, дефис и подчёркивание")
	}

	if length := len(r.Password); length < passwordMinLength || length > passwordMaxLength {
		validationErr.Add("password", "Пароль должен быть длиной от 8 до 72 символов")
	}

	if validationErr.Empty() {
		return nil
	}

	return validationErr
}

type LoginRequest struct {
	Nickname string `json:"nickname"`
	Password string `json:"password"`
}

// Validate проверяет только заполненность: несовпадение формата всегда даёт
// invalid_credentials, чтобы не подсказывать правила подбора.
func (r LoginRequest) Validate() error {
	validationErr := &domain.ValidationError{}

	if r.Nickname == "" {
		validationErr.Add("nickname", "Укажите ник")
	}

	if r.Password == "" {
		validationErr.Add("password", "Укажите пароль")
	}

	if validationErr.Empty() {
		return nil
	}

	return validationErr
}

// User — хеша пароля в DTO нет и не должно появиться (SEC1).
type User struct {
	ID        string    `json:"id"`
	Nickname  string    `json:"nickname"`
	CreatedAt time.Time `json:"created_at"`
}

type AuthResponse struct {
	Token     string    `json:"token"`
	TokenType string    `json:"token_type"`
	ExpiresAt time.Time `json:"expires_at"`
	User      User      `json:"user"`
}

func NewUser(user domain.User) User {
	return User{
		ID:        user.ID.String(),
		Nickname:  user.Nickname,
		CreatedAt: user.CreatedAt,
	}
}

func NewAuthResponse(user domain.User, token service.Token) AuthResponse {
	return AuthResponse{
		Token:     token.Value,
		TokenType: "Bearer",
		ExpiresAt: token.ExpiresAt,
		User:      NewUser(user),
	}
}

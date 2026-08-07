package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/config"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/domain"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/security"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/service"
)

var errInternal = errors.New("внутренняя ошибка")

// Pinger — зависимость, готовность которой проверяет /readyz.
type Pinger interface {
	Ping(ctx context.Context) error
}

type Handler struct {
	services  *service.Services
	cfg       config.Config
	log       *slog.Logger
	db        Pinger
	version   string
	cookieCfg security.CookieConfig
}

func NewHandler(
	services *service.Services,
	cfg config.Config,
	log *slog.Logger,
	db Pinger,
	version string,
) *Handler {
	return &Handler{
		services: services,
		cfg:      cfg,
		log:      log,
		db:       db,
		version:  version,
		cookieCfg: security.CookieConfig{
			Secure:   cfg.Production(),
			SameSite: http.SameSiteLaxMode,
		},
	}
}

func currentUser(r *http.Request) (domain.User, bool) {
	return userFromContext(r.Context())
}

// currentOwner — владелец сессии прохождения (юзер или гость). Используйте
// в StartSession/GetSession/SubmitAnswer/AbandonSession/GetSessionResult —
// эти роуты висят за optionalAuth и должны работать для обоих.
func currentOwner(r *http.Request) (domain.Owner, bool) {
	return ownerFromContext(r.Context())
}

// optionalUserID: nil означает гостя. Оставлено там, где нужен именно
// nullable userID (например, каталог: гостю карточки без прогресса).
func optionalUserID(r *http.Request) *uuid.UUID {
	user, ok := userFromContext(r.Context())
	if !ok {
		return nil
	}

	id := user.ID
	return &id
}

// parseUUID: некорректный идентификатор — это несуществующий ресурс, а не
// ошибка валидации; отвечаем так же, как на чужую сессию (SEC2).
func parseUUID(value string) (uuid.UUID, error) {
	parsed, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, domain.ErrNotFound
	}

	return parsed, nil
}

func parseRole(value string) (*domain.Role, error) {
	if value == "" {
		return nil, nil
	}

	role := domain.Role(value)
	if !role.Valid() {
		return nil, domain.NewValidationError("role", "Допустимые значения: buyer, seller")
	}

	return &role, nil
}

func parseSide(value string) (*domain.Side, error) {
	if value == "" {
		return nil, nil
	}

	side := domain.Side(value)
	if !side.Valid() {
		return nil, domain.NewValidationError("side", "Допустимые значения: buyer, seller")
	}

	return &side, nil
}

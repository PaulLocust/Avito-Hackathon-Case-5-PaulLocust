package service

import (
	"context"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/config"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/domain"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/repository"
)

type authService struct {
	users repository.UserRepository
	cfg   config.AuthConfig
}

var _ AuthService = (*authService)(nil)

// TODO(M4): проверить данные, захешировать пароль bcrypt с cfg.BcryptCost,
// создать пользователя (занятый ник — domain.ErrNicknameTaken), выпустить
// токен. Пароль не логировать (SEC7).
func (s *authService) Register(ctx context.Context, nickname, password string) (domain.User, Token, error) {
	_, _, _ = ctx, nickname, password
	return domain.User{}, Token{}, domain.ErrNotImplemented
}

// TODO(M4): при отсутствии пользователя всё равно сравнивать хеши — иначе
// разница во времени ответа выдаёт существующие ники.
func (s *authService) Login(ctx context.Context, nickname, password string) (domain.User, Token, error) {
	_, _, _ = ctx, nickname, password
	return domain.User{}, Token{}, domain.ErrNotImplemented
}

// TODO(M4): записать jti токена в revoked_tokens до истечения его срока.
func (s *authService) Logout(ctx context.Context, token string) error {
	_, _ = ctx, token
	return domain.ErrNotImplemented
}

// TODO(M4): проверить подпись и отзыв, вернуть владельца. Любая ошибка
// разбора — domain.ErrUnauthorized без подробностей.
func (s *authService) Authenticate(ctx context.Context, token string) (domain.User, error) {
	_, _ = ctx, token
	return domain.User{}, domain.ErrNotImplemented
}

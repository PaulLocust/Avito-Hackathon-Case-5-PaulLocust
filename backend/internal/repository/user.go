package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/domain"
)

type userRepository struct {
	pool *pgxpool.Pool
}

var _ UserRepository = (*userRepository)(nil)

// TODO(M4): нарушение уникальности nickname → domain.ErrNicknameTaken,
// отсутствие строки → domain.ErrNotFound.
func (r *userRepository) Create(ctx context.Context, nickname, passwordHash string) (domain.User, error) {
	_, _, _ = ctx, nickname, passwordHash
	return domain.User{}, domain.ErrNotImplemented
}

// TODO(M4)
func (r *userRepository) GetByNickname(ctx context.Context, nickname string) (domain.User, error) {
	_, _ = ctx, nickname
	return domain.User{}, domain.ErrNotImplemented
}

// TODO(M4)
func (r *userRepository) GetByID(ctx context.Context, id uuid.UUID) (domain.User, error) {
	_, _ = ctx, id
	return domain.User{}, domain.ErrNotImplemented
}

// TODO(M4)
func (r *userRepository) RevokeToken(ctx context.Context, jti string, expiresAt int64) error {
	_, _, _ = ctx, jti, expiresAt
	return domain.ErrNotImplemented
}

// TODO(M4)
func (r *userRepository) IsTokenRevoked(ctx context.Context, jti string) (bool, error) {
	_, _ = ctx, jti
	return false, domain.ErrNotImplemented
}

package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/domain"
)

type userRepository struct {
	pool *pgxpool.Pool
}

func (r *userRepository) Create(ctx context.Context, nickname, passwordHash string) (domain.User, error) {
	var u domain.User

	err := r.pool.QueryRow(ctx,
		`INSERT INTO users (nickname, password_hash)
         VALUES ($1, $2)
         RETURNING id, nickname, password_hash, role, created_at`,
		nickname, passwordHash,
	).Scan(&u.ID, &u.Nickname, &u.PasswordHash, &u.Role, &u.CreatedAt)

	return u, err // unique_violation ловится в service.authService.Register через pgconn.PgError
}

func (r *userRepository) GetByNickname(ctx context.Context, nickname string) (domain.User, error) {
	var u domain.User

	err := r.pool.QueryRow(ctx,
		`SELECT id, nickname, password_hash, role, created_at
         FROM users WHERE nickname = $1`,
		nickname,
	).Scan(&u.ID, &u.Nickname, &u.PasswordHash, &u.Role, &u.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, domain.ErrNotFound
	}
	return u, err
}

func (r *userRepository) GetByID(ctx context.Context, id uuid.UUID) (domain.User, error) {
	var u domain.User

	err := r.pool.QueryRow(ctx,
		`SELECT id, nickname, password_hash, role, created_at
         FROM users WHERE id = $1`,
		id,
	).Scan(&u.ID, &u.Nickname, &u.PasswordHash, &u.Role, &u.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, domain.ErrNotFound
	}
	return u, err
}

func (r *userRepository) RevokeToken(ctx context.Context, jti string, expiresAt int64) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO revoked_tokens (jti, expires_at)
         VALUES ($1, to_timestamp($2))
         ON CONFLICT (jti) DO NOTHING`,
		jti, expiresAt,
	)
	return err
}

func (r *userRepository) IsTokenRevoked(ctx context.Context, jti string) (bool, error) {
	var exists bool

	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS (
            SELECT 1 FROM revoked_tokens
            WHERE jti = $1 AND expires_at > now()
        )`,
		jti,
	).Scan(&exists)

	return exists, err
}

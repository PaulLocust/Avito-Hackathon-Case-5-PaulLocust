package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/domain"
)

type refreshTokenRepository struct {
	pool *pgxpool.Pool
}

func (r *refreshTokenRepository) Create(ctx context.Context, t RefreshToken) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO refresh_tokens (id, user_id, session_id, hash, expires_at, created_at)
         VALUES ($1, $2, $3, $4, $5, $6)`,
		t.ID, t.UserID, t.SessionID, t.Hash, t.ExpiresAt, t.CreatedAt,
	)
	return err
}

func (r *refreshTokenRepository) GetByHash(ctx context.Context, hash string) (RefreshToken, error) {
	var rt RefreshToken

	err := r.pool.QueryRow(ctx,
		`SELECT id, user_id, session_id, hash, expires_at, created_at
         FROM refresh_tokens WHERE hash = $1`,
		hash,
	).Scan(&rt.ID, &rt.UserID, &rt.SessionID, &rt.Hash, &rt.ExpiresAt, &rt.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return RefreshToken{}, domain.ErrNotFound
	}
	return rt, err
}

func (r *refreshTokenRepository) DeleteByID(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM refresh_tokens WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *refreshTokenRepository) DeleteBySessionID(ctx context.Context, sessionID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM refresh_tokens WHERE session_id = $1`, sessionID)
	return err
}

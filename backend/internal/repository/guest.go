package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/domain"
)

type guestRepository struct {
	pool *pgxpool.Pool
}

func (r *guestRepository) Create(ctx context.Context, g GuestSession) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO guest_sessions (id, token_hash, expires_at, created_at)
         VALUES ($1, $2, $3, $4)`,
		g.ID, g.TokenHash, g.ExpiresAt, g.CreatedAt,
	)
	return err
}

func (r *guestRepository) GetByHash(ctx context.Context, hash string) (GuestSession, error) {
	var gs GuestSession

	err := r.pool.QueryRow(ctx,
		`SELECT id, token_hash, expires_at, created_at
         FROM guest_sessions WHERE token_hash = $1`,
		hash,
	).Scan(&gs.ID, &gs.TokenHash, &gs.ExpiresAt, &gs.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return GuestSession{}, domain.ErrNotFound
	}
	return gs, err
}

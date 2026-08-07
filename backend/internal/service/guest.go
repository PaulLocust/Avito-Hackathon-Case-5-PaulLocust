package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/config"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/domain"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/repository"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/security"
)

const guestTokenSize = 32

type guestService struct {
	guests repository.GuestRepository
	cfg    config.AuthConfig
}

var _ GuestService = (*guestService)(nil)

func (s *guestService) Start(ctx context.Context) (GuestSessionToken, error) {
	raw, err := security.RandomToken(guestTokenSize)
	if err != nil {
		return GuestSessionToken{}, err
	}

	id := uuid.New()
	now := time.Now()
	exp := now.Add(s.cfg.GuestTTL)

	err = s.guests.Create(ctx, repository.GuestSession{
		ID:        id,
		TokenHash: security.HashToken(raw),
		ExpiresAt: exp,
		CreatedAt: now,
	})
	if err != nil {
		return GuestSessionToken{}, err
	}

	return GuestSessionToken{Value: raw, ExpiresAt: exp, OwnerID: id}, nil
}

func (s *guestService) Validate(ctx context.Context, guestToken string) (uuid.UUID, error) {
	gs, err := s.guests.GetByHash(ctx, security.HashToken(guestToken))
	if err != nil {
		return uuid.Nil, err // репозиторий уже отдаёт domain.ErrNotFound (см. ниже)
	}

	if time.Now().After(gs.ExpiresAt) {
		return uuid.Nil, domain.ErrGuestSessionExpired
	}

	return gs.ID, nil
}

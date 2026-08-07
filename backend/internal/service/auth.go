package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/config"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/domain"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/repository"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/security"
)

// authService реализует AuthService (см. service.go).
type authService struct {
	users    repository.UserRepository
	refresh  repository.RefreshTokenRepository
	sessions repository.SessionRepository
	guests   GuestService

	cfg config.AuthConfig
}

var _ AuthService = (*authService)(nil)

func (s *authService) tokenManager() security.TokenManager {
	return security.NewJWTManager(s.cfg.JWTSecret, s.cfg.Issuer, s.cfg.AccessTTL)
}

func (s *authService) hasher() security.PasswordHasher {
	return security.NewBCryptHasher(s.cfg.BcryptCost)
}

func (s *authService) Register(ctx context.Context, nickname, password string) (domain.User, TokenPair, error) {
	hash, err := s.hasher().Hash(password)
	if err != nil {
		return domain.User{}, TokenPair{}, err
	}

	user, err := s.users.Create(ctx, nickname, hash)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.User{}, TokenPair{}, domain.ErrNicknameTaken
		}
		return domain.User{}, TokenPair{}, err
	}

	pair, err := s.issuePair(ctx, user, uuid.New())
	if err != nil {
		return domain.User{}, TokenPair{}, err
	}

	return user, pair, nil
}

func (s *authService) Login(ctx context.Context, nickname, password string) (domain.User, TokenPair, error) {
	user, err := s.users.GetByNickname(ctx, nickname)
	if err != nil {
		return domain.User{}, TokenPair{}, domain.ErrInvalidCredentials
	}

	if compareErr := s.hasher().Compare(user.PasswordHash, password); compareErr != nil {
		return domain.User{}, TokenPair{}, domain.ErrInvalidCredentials
	}

	pair, err := s.issuePair(ctx, user, uuid.New())
	if err != nil {
		return domain.User{}, TokenPair{}, err
	}

	return user, pair, nil
}

func (s *authService) Refresh(ctx context.Context, rawToken string) (domain.User, TokenPair, error) {
	hash := security.HashToken(rawToken)

	stored, err := s.refresh.GetByHash(ctx, hash)
	if err != nil {
		return domain.User{}, TokenPair{}, domain.ErrRefreshTokenInvalid
	}

	if time.Now().After(stored.ExpiresAt) {
		_ = s.refresh.DeleteByID(ctx, stored.ID)
		return domain.User{}, TokenPair{}, domain.ErrRefreshTokenInvalid
	}

	user, err := s.users.GetByID(ctx, stored.UserID)
	if err != nil {
		return domain.User{}, TokenPair{}, domain.ErrRefreshTokenInvalid
	}

	if deleteErr := s.refresh.DeleteByID(ctx, stored.ID); deleteErr != nil {
		if errors.Is(deleteErr, domain.ErrNotFound) {
			return domain.User{}, TokenPair{}, domain.ErrRefreshTokenInvalid
		}
		return domain.User{}, TokenPair{}, deleteErr
	}

	pair, err := s.issuePair(ctx, user, stored.SessionID)
	if err != nil {
		return domain.User{}, TokenPair{}, err
	}

	return user, pair, nil
}

// Logout принимает refresh-токен и удаляет все refresh-токены этой
// "сессии логина" (SessionID), а не только предъявленный.
func (s *authService) Logout(ctx context.Context, accessToken string) error {
	claims, err := s.tokenManager().ParseAccess(accessToken)
	if err != nil {
		return domain.ErrUnauthorized
	}

	if claims.ExpiresAt == nil {
		return domain.ErrUnauthorized
	}

	if err := s.users.RevokeToken(ctx, claims.ID, claims.ExpiresAt.Unix()); err != nil {
		return err
	}

	return s.refresh.DeleteBySessionID(ctx, claims.SessionID)
}

func (s *authService) Authenticate(ctx context.Context, accessToken string) (domain.User, error) {
	claims, err := s.tokenManager().ParseAccess(accessToken)
	if err != nil {
		return domain.User{}, domain.ErrUnauthorized
	}

	revoked, err := s.users.IsTokenRevoked(ctx, claims.ID)
	if err != nil {
		return domain.User{}, err
	}

	if revoked {
		return domain.User{}, domain.ErrUnauthorized
	}

	user, err := s.users.GetByID(ctx, claims.UserID)
	if err != nil {
		return domain.User{}, domain.ErrUnauthorized
	}

	return user, nil
}

// ClaimGuest переносит сессии прохождения гостя на userID после успешного
// Register/Login.
func (s *authService) ClaimGuest(ctx context.Context, userID uuid.UUID, guestToken string) error {
	guestSessionID, err := s.guests.Validate(ctx, guestToken)
	if err != nil {
		// протухший/невалидный гостевой токен — не ошибка логина,
		// просто нечего переносить.
		return nil
	}

	return s.sessions.ClaimByGuest(ctx, guestSessionID, userID)
}

func (s *authService) issuePair(ctx context.Context, user domain.User, sessionID uuid.UUID) (TokenPair, error) {
	access, accessExp, err := s.tokenManager().GenerateAccess(user.ID, sessionID, user.Role)
	if err != nil {
		return TokenPair{}, err
	}

	rawRefresh, err := security.RandomToken(32)
	if err != nil {
		return TokenPair{}, err
	}

	now := time.Now()
	refreshExp := now.Add(s.cfg.RefreshTTL)

	err = s.refresh.Create(ctx, repository.RefreshToken{
		ID:        uuid.New(),
		UserID:    user.ID,
		SessionID: sessionID,
		Hash:      security.HashToken(rawRefresh),
		ExpiresAt: refreshExp,
		CreatedAt: now,
	})
	if err != nil {
		return TokenPair{}, err
	}

	return TokenPair{
		Access:  Token{Value: access, ExpiresAt: accessExp},
		Refresh: Token{Value: rawRefresh, ExpiresAt: refreshExp},
	}, nil
}

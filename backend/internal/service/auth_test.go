package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/config"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/domain"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/repository"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/security"
)

type authUserRepositoryStub struct {
	user       domain.User
	revokedJTI string
	revokedExp int64
}

func (s *authUserRepositoryStub) Create(context.Context, string, string) (domain.User, error) {
	return domain.User{}, errors.New("метод не используется в этом тесте")
}

func (s *authUserRepositoryStub) GetByNickname(context.Context, string) (domain.User, error) {
	return domain.User{}, errors.New("метод не используется в этом тесте")
}

func (s *authUserRepositoryStub) GetByID(_ context.Context, id uuid.UUID) (domain.User, error) {
	if id != s.user.ID {
		return domain.User{}, domain.ErrNotFound
	}
	return s.user, nil
}

func (s *authUserRepositoryStub) RevokeToken(_ context.Context, jti string, expiresAt int64) error {
	s.revokedJTI = jti
	s.revokedExp = expiresAt
	return nil
}

func (s *authUserRepositoryStub) IsTokenRevoked(context.Context, string) (bool, error) {
	return false, nil
}

type refreshRepositoryStub struct {
	byHash  map[string]repository.RefreshToken
	created repository.RefreshToken
}

func (s *refreshRepositoryStub) Create(_ context.Context, token repository.RefreshToken) error {
	s.created = token
	s.byHash[token.Hash] = token
	return nil
}

func (s *refreshRepositoryStub) GetByHash(_ context.Context, hash string) (repository.RefreshToken, error) {
	token, ok := s.byHash[hash]
	if !ok {
		return repository.RefreshToken{}, domain.ErrNotFound
	}
	return token, nil
}

func (s *refreshRepositoryStub) DeleteByID(_ context.Context, id uuid.UUID) error {
	for hash, token := range s.byHash {
		if token.ID == id {
			delete(s.byHash, hash)
			return nil
		}
	}
	return domain.ErrNotFound
}

func (s *refreshRepositoryStub) DeleteBySessionID(_ context.Context, sessionID uuid.UUID) error {
	for hash, token := range s.byHash {
		if token.SessionID == sessionID {
			delete(s.byHash, hash)
		}
	}
	return nil
}

func TestRefreshRotatesToken(t *testing.T) {
	user := domain.User{ID: uuid.New(), Nickname: "tester", Role: "user"}
	sessionID := uuid.New()
	rawToken := "old-refresh-token"
	repo := &refreshRepositoryStub{byHash: map[string]repository.RefreshToken{
		security.HashToken(rawToken): {
			ID:        uuid.New(),
			UserID:    user.ID,
			SessionID: sessionID,
			Hash:      security.HashToken(rawToken),
			ExpiresAt: time.Now().Add(time.Hour),
		},
	}}
	auth := &authService{
		users:   &authUserRepositoryStub{user: user},
		refresh: repo,
		cfg: config.AuthConfig{
			JWTSecret:  "test-secret",
			Issuer:     "test",
			AccessTTL:  time.Minute,
			RefreshTTL: time.Hour,
		},
	}

	gotUser, pair, err := auth.Refresh(context.Background(), rawToken)
	require.NoError(t, err)
	require.Equal(t, user.ID, gotUser.ID)
	require.NotEmpty(t, pair.Access.Value)
	require.NotEmpty(t, pair.Refresh.Value)
	require.NotEqual(t, rawToken, pair.Refresh.Value)
	require.NotEmpty(t, repo.created.Hash)

	claims, err := security.NewJWTManager("test-secret", "test", time.Minute).ParseAccess(pair.Access.Value)
	require.NoError(t, err)
	require.Equal(t, user.ID, claims.UserID)
	require.Equal(t, sessionID, claims.SessionID)

	_, _, err = auth.Refresh(context.Background(), rawToken)
	require.ErrorIs(t, err, domain.ErrRefreshTokenInvalid)
}

func TestLogoutRevokesAccessAndRefreshSession(t *testing.T) {
	user := domain.User{ID: uuid.New(), Nickname: "tester", Role: "user"}
	sessionID := uuid.New()
	users := &authUserRepositoryStub{user: user}
	refresh := &refreshRepositoryStub{byHash: map[string]repository.RefreshToken{
		"refresh": {ID: uuid.New(), UserID: user.ID, SessionID: sessionID, Hash: "refresh", ExpiresAt: time.Now().Add(time.Hour)},
	}}
	auth := &authService{
		users:   users,
		refresh: refresh,
		cfg: config.AuthConfig{
			JWTSecret: "test-secret",
			Issuer:    "test",
			AccessTTL: time.Minute,
		},
	}
	access, _, err := auth.tokenManager().GenerateAccess(user.ID, sessionID, user.Role)
	require.NoError(t, err)

	require.NoError(t, auth.Logout(context.Background(), access))
	require.NotEmpty(t, users.revokedJTI)
	require.Positive(t, users.revokedExp)
	require.Empty(t, refresh.byHash)
}

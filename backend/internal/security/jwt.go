package security

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type JWTManager struct {
	Secret []byte

	issuer string

	accessTTL time.Duration
}

func NewJWTManager(
	secret string,
	issuer string,
	accessTTL time.Duration,
) *JWTManager {

	return &JWTManager{
		Secret: []byte(secret),

		issuer: issuer,

		accessTTL: accessTTL,
	}
}

func (m *JWTManager) GenerateAccess(
	userID uuid.UUID,
	sessionID uuid.UUID,
	role string,
) (string, time.Time, error) {

	now := time.Now()

	exp := now.Add(m.accessTTL)

	claims := Claims{
		UserID: userID,

		SessionID: sessionID,

		Role: role,

		RegisteredClaims: jwt.RegisteredClaims{
			ID: uuid.NewString(),

			Subject: userID.String(),

			Issuer: m.issuer,

			IssuedAt: jwt.NewNumericDate(now),

			NotBefore: jwt.NewNumericDate(now),

			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}

	token := jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		claims,
	)

	signed, err := token.SignedString(m.Secret)
	if err != nil {
		return "", time.Time{}, err
	}

	return signed, exp, nil
}

func (m *JWTManager) ParseAccess(tokenString string) (*Claims, error) {

	token, err := jwt.ParseWithClaims(
		tokenString,
		&Claims{},
		func(token *jwt.Token) (any, error) {

			method, ok := token.Method.(*jwt.SigningMethodHMAC)

			if !ok || method != jwt.SigningMethodHS256 {
				return nil, ErrInvalidSignature
			}

			return m.Secret, nil
		},
		jwt.WithIssuer(m.issuer),
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
	)

	if err != nil {

		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}

		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)

	if !ok {
		return nil, ErrInvalidClaims
	}

	if claims.Subject == "" || claims.UserID == uuid.Nil || claims.SessionID == uuid.Nil {
		return nil, ErrMissingSubject
	}

	if claims.Subject != claims.UserID.String() {
		return nil, ErrInvalidClaims
	}

	return claims, nil
}

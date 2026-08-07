package security

import (
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Claims struct {
	UserID uuid.UUID `json:"uid"`

	SessionID uuid.UUID `json:"sid"`

	Role string `json:"role"`

	jwt.RegisteredClaims
}

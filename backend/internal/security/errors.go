package security

import "errors"

var (
	ErrInvalidToken     = errors.New("invalid token")
	ErrExpiredToken     = errors.New("expired token")
	ErrInvalidClaims    = errors.New("invalid claims")
	ErrInvalidSignature = errors.New("invalid signature")
	ErrMissingSubject   = errors.New("missing subject")

	ErrMissingAuthorizationHeader = errors.New("missing authorization header")
	ErrInvalidAuthorizationHeader = errors.New("invalid authorization header")
	ErrMissingToken               = errors.New("missing bearer token")
)

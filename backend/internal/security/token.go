package security

import (
	"time"

	"github.com/google/uuid"
)

type TokenManager interface {
	GenerateAccess(
		userID uuid.UUID,
		sessionID uuid.UUID,
		role string,
	) (string, time.Time, error)

	ParseAccess(token string) (*Claims, error)
}

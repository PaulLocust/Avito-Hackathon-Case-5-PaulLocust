package security

// Raw secrets.

type Password string

type AccessToken string

type RefreshToken string

type BearerToken string

type VerificationCode string

type ResetToken string

// Stored secrets.

type PasswordHash string

type RefreshTokenHash string

type VerificationCodeHash string

type ResetTokenHash string

type Role string

const (
	RoleGuest Role = "guest"
	RoleUser  Role = "user"
	RoleAdmin Role = "admin"
)

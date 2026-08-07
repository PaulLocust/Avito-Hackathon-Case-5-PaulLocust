package security

func (t Password) String() string {
	return string(t)
}

func (t PasswordHash) String() string {
	return string(t)
}

func (t AccessToken) String() string {
	return string(t)
}

func (t RefreshToken) String() string {
	return string(t)
}

func (t BearerToken) String() string {
	return string(t)
}

func (t RefreshTokenHash) String() string {
	return string(t)
}

func (t VerificationCode) String() string {
	return string(t)
}

func (t VerificationCodeHash) String() string {
	return string(t)
}

func (t ResetToken) String() string {
	return string(t)
}

func (t ResetTokenHash) String() string {
	return string(t)
}

func (r Role) String() string {
	return string(r)
}

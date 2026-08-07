package security

import "golang.org/x/crypto/bcrypt"

type BCryptHasher struct {
	Cost int
}

func NewBCryptHasher(cost int) *BCryptHasher {
	return &BCryptHasher{
		Cost: cost,
	}
}

func (b *BCryptHasher) Hash(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword(
		[]byte(password),
		b.Cost,
	)

	if err != nil {
		return "", err
	}

	return string(hash), nil
}

func (b *BCryptHasher) Compare(hash, password string) error {
	return bcrypt.CompareHashAndPassword(
		[]byte(hash),
		[]byte(password),
	)
}

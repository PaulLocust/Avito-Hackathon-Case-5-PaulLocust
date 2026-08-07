package security

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"math/big"
)

const verificationAlphabet = "0123456789"

func RandomBytes(size int) ([]byte, error) {
	b := make([]byte, size)

	if _, err := rand.Read(b); err != nil {
		return nil, err
	}

	return b, nil
}

// Рефреш
func RandomToken(size int) (string, error) {
	bytes, err := RandomBytes(size)
	if err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func RandomCode(length int) (string, error) {
	if length <= 0 {
		return "", fmt.Errorf("invalid code length")
	}

	out := make([]byte, length)

	for i := range out {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(verificationAlphabet))))
		if err != nil {
			return "", err
		}

		out[i] = verificationAlphabet[n.Int64()]
	}

	return string(out), nil
}

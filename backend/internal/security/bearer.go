package security

import "strings"

const bearerPrefix = "Bearer "

func ParseBearer(header string) (BearerToken, error) {
	header = strings.TrimSpace(header)

	if header == "" {
		return "", ErrMissingAuthorizationHeader
	}

	if !strings.HasPrefix(header, bearerPrefix) {
		return "", ErrInvalidAuthorizationHeader
	}

	token := strings.TrimSpace(strings.TrimPrefix(header, bearerPrefix))

	if token == "" {
		return "", ErrMissingToken
	}

	return BearerToken(token), nil
}

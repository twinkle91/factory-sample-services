package links

import (
	"crypto/rand"
	"errors"
	"math/big"
	"net/url"
	"strconv"
	"strings"
)

const (
	codeLength   = 7
	codeAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	maxURLLength = 2048

	// DefaultLimit is how many links GET /links returns without ?limit=.
	DefaultLimit = 20
	// MaxLimit is the most links GET /links returns, whatever ?limit= asks for.
	MaxLimit = 100
)

// NewCode returns a random short code such as "aZ3kQ9x".
func NewCode() (string, error) {
	var b strings.Builder
	max := big.NewInt(int64(len(codeAlphabet)))
	for range codeLength {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		b.WriteByte(codeAlphabet[n.Int64()])
	}
	return b.String(), nil
}

// validateURL checks that raw is an absolute http or https URL and returns it trimmed.
func validateURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("url is required")
	}
	if len(raw) > maxURLLength {
		return "", errors.New("url must be at most 2048 characters")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", errors.New("url is not a valid URL")
	}
	if s := strings.ToLower(u.Scheme); s != "http" && s != "https" {
		return "", errors.New("url must start with http:// or https://")
	}
	if u.Host == "" {
		return "", errors.New("url must include a host")
	}
	return raw, nil
}

// parseLimit reads ?limit= for GET /links.
func parseLimit(raw string) (int, error) {
	if raw == "" {
		return DefaultLimit, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return 0, errors.New("limit must be a positive integer")
	}
	return n, nil
}

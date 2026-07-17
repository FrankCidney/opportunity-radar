package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

const tokenBytes = 32

type TokenGenerator interface {
	Generate() (string, []byte, error)
	Hash(rawToken string) []byte
}

type SecureTokenGenerator struct{}

func (SecureTokenGenerator) Generate() (string, []byte, error) {
	random := make([]byte, tokenBytes)
	if _, err := rand.Read(random); err != nil {
		return "", nil, fmt.Errorf("generate secure token: %w", err)
	}

	raw := base64.RawURLEncoding.EncodeToString(random)
	return raw, hashToken(raw), nil
}

func (SecureTokenGenerator) Hash(rawToken string) []byte {
	return hashToken(rawToken)
}

func hashToken(rawToken string) []byte {
	sum := sha256.Sum256([]byte(rawToken))
	hash := make([]byte, len(sum))
	copy(hash, sum[:])
	return hash
}

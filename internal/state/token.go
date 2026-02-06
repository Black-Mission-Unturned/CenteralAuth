package state

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/BlackMission/centralauth/internal/domain"
)

const (
	defaultExpiry = 5 * time.Minute
	nonceBytes    = 16
)

// Service generates and validates HMAC-signed state tokens.
type Service struct {
	key    []byte
	expiry time.Duration
	now    func() time.Time
}

// NewService creates a state token service with the given HMAC signing key.
func NewService(key []byte) *Service {
	return &Service{
		key:    key,
		expiry: defaultExpiry,
		now:    time.Now,
	}
}

// Generate creates an HMAC-signed state token containing the given payload.
func (s *Service) Generate(payload domain.StatePayload) (string, error) {
	nonce := make([]byte, nonceBytes)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generating nonce: %w", err)
	}
	payload.Nonce = hex.EncodeToString(nonce)
	payload.ExpiresAt = s.now().Add(s.expiry)

	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshaling state payload: %w", err)
	}

	encoded := base64.RawURLEncoding.EncodeToString(data)
	sig := s.sign(encoded)

	return encoded + "." + sig, nil
}

// Validate verifies the HMAC signature and expiry of a state token.
func (s *Service) Validate(token string) (*domain.StatePayload, error) {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return nil, domain.ErrMalformedState
	}

	encoded, sig := parts[0], parts[1]

	expectedSig := s.sign(encoded)
	if !hmac.Equal([]byte(sig), []byte(expectedSig)) {
		return nil, domain.ErrInvalidState
	}

	data, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, domain.ErrMalformedState
	}

	var payload domain.StatePayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, domain.ErrMalformedState
	}

	if s.now().After(payload.ExpiresAt) {
		return nil, domain.ErrExpiredState
	}

	return &payload, nil
}

// SetNow overrides the time function (for testing).
func (s *Service) SetNow(fn func() time.Time) {
	s.now = fn
}

func (s *Service) sign(data string) string {
	mac := hmac.New(sha256.New, s.key)
	mac.Write([]byte(data))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

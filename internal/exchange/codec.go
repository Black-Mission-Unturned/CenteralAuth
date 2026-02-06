package exchange

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/BlackMission/centralauth/internal/domain"
)

const defaultCodeExpiry = 30 * time.Second

// Codec encrypts and decrypts exchange codes using AES-256-GCM.
type Codec struct {
	aead   cipher.AEAD
	expiry time.Duration
	now    func() time.Time
}

// NewCodec creates an exchange codec with the given 32-byte AES key.
func NewCodec(key []byte) (*Codec, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("creating AES cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creating GCM: %w", err)
	}
	return &Codec{
		aead:   aead,
		expiry: defaultCodeExpiry,
		now:    time.Now,
	}, nil
}

// SetNow overrides the time function (for testing).
func (c *Codec) SetNow(fn func() time.Time) {
	c.now = fn
}

// Encode encrypts an ExchangePayload into a base64url-encoded exchange code.
func (c *Codec) Encode(payload domain.ExchangePayload) (string, error) {
	payload.ExpiresAt = c.now().Add(c.expiry)

	plaintext, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshaling exchange payload: %w", err)
	}

	nonce := make([]byte, c.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generating nonce: %w", err)
	}

	// nonce || ciphertext+tag
	ciphertext := c.aead.Seal(nonce, nonce, plaintext, nil)

	return base64.RawURLEncoding.EncodeToString(ciphertext), nil
}

// Decode decrypts a base64url-encoded exchange code back into an ExchangePayload.
func (c *Codec) Decode(code string) (*domain.ExchangePayload, error) {
	raw, err := base64.RawURLEncoding.DecodeString(code)
	if err != nil {
		return nil, domain.ErrInvalidExchangeCode
	}

	if len(raw) < c.aead.NonceSize() {
		return nil, domain.ErrInvalidExchangeCode
	}

	nonce := raw[:c.aead.NonceSize()]
	ciphertext := raw[c.aead.NonceSize():]

	plaintext, err := c.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, domain.ErrInvalidExchangeCode
	}

	var payload domain.ExchangePayload
	if err := json.Unmarshal(plaintext, &payload); err != nil {
		return nil, domain.ErrInvalidExchangeCode
	}

	if c.now().After(payload.ExpiresAt) {
		return nil, domain.ErrExpiredExchangeCode
	}

	return &payload, nil
}

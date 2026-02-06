package state

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/BlackMission/centralauth/internal/domain"
)

var testKey = []byte("test-signing-key-1234567890abcdef")

func newTestService() *Service {
	return NewService(testKey)
}

func TestRoundTrip(t *testing.T) {
	svc := newTestService()

	payload := domain.StatePayload{
		ClientID:    "website",
		Provider:    "discord",
		RedirectURI: "https://example.com/callback",
	}

	token, err := svc.Generate(payload)
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	got, err := svc.Validate(token)
	if err != nil {
		t.Fatalf("Validate error: %v", err)
	}

	if got.ClientID != "website" {
		t.Errorf("expected ClientID 'website', got %q", got.ClientID)
	}
	if got.Provider != "discord" {
		t.Errorf("expected Provider 'discord', got %q", got.Provider)
	}
	if got.RedirectURI != "https://example.com/callback" {
		t.Errorf("expected RedirectURI, got %q", got.RedirectURI)
	}
	if got.Nonce == "" {
		t.Error("expected non-empty nonce")
	}
}

func TestTamperedPayload(t *testing.T) {
	svc := newTestService()

	token, err := svc.Generate(domain.StatePayload{
		ClientID: "website",
		Provider: "discord",
	})
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	parts := strings.SplitN(token, ".", 2)
	// Decode, modify, re-encode the payload
	data, _ := base64.RawURLEncoding.DecodeString(parts[0])
	modified := strings.Replace(string(data), "website", "hacked!", 1)
	parts[0] = base64.RawURLEncoding.EncodeToString([]byte(modified))

	_, err = svc.Validate(parts[0] + "." + parts[1])
	if !errors.Is(err, domain.ErrInvalidState) {
		t.Errorf("expected ErrInvalidState, got %v", err)
	}
}

func TestTamperedSignature(t *testing.T) {
	svc := newTestService()

	token, err := svc.Generate(domain.StatePayload{
		ClientID: "website",
		Provider: "discord",
	})
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	parts := strings.SplitN(token, ".", 2)
	// Flip a character in the signature
	tampered := []byte(parts[1])
	tampered[0] ^= 0xFF
	parts[1] = string(tampered)

	_, err = svc.Validate(parts[0] + "." + parts[1])
	if !errors.Is(err, domain.ErrInvalidState) {
		t.Errorf("expected ErrInvalidState, got %v", err)
	}
}

func TestExpiredToken(t *testing.T) {
	svc := newTestService()
	now := time.Now()
	svc.now = func() time.Time { return now }

	token, err := svc.Generate(domain.StatePayload{
		ClientID: "website",
		Provider: "discord",
	})
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	// Advance time past expiry
	svc.now = func() time.Time { return now.Add(6 * time.Minute) }

	_, err = svc.Validate(token)
	if !errors.Is(err, domain.ErrExpiredState) {
		t.Errorf("expected ErrExpiredState, got %v", err)
	}
}

func TestWrongKey(t *testing.T) {
	svc1 := NewService([]byte("key-one-1234567890abcdef12345678"))
	svc2 := NewService([]byte("key-two-1234567890abcdef12345678"))

	token, err := svc1.Generate(domain.StatePayload{
		ClientID: "website",
		Provider: "discord",
	})
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	_, err = svc2.Validate(token)
	if !errors.Is(err, domain.ErrInvalidState) {
		t.Errorf("expected ErrInvalidState, got %v", err)
	}
}

func TestMalformedInput(t *testing.T) {
	svc := newTestService()

	tests := []struct {
		name  string
		token string
	}{
		{"empty", ""},
		{"no dot", "nodothere"},
		{"just dots", "..."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Validate(tt.token)
			if err == nil {
				t.Error("expected error")
			}
		})
	}
}

func TestNonceUniqueness(t *testing.T) {
	svc := newTestService()

	payload := domain.StatePayload{
		ClientID: "website",
		Provider: "discord",
	}

	token1, _ := svc.Generate(payload)
	token2, _ := svc.Generate(payload)

	if token1 == token2 {
		t.Error("tokens should have unique nonces")
	}

	p1, _ := svc.Validate(token1)
	p2, _ := svc.Validate(token2)
	if p1.Nonce == p2.Nonce {
		t.Error("nonces should be different")
	}
}

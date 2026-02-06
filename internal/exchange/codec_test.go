package exchange

import (
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/BlackMission/centralauth/internal/domain"
)

var testKey = []byte("01234567890123456789012345678901") // 32 bytes

func newTestCodec(t *testing.T) *Codec {
	t.Helper()
	c, err := NewCodec(testKey)
	if err != nil {
		t.Fatalf("NewCodec error: %v", err)
	}
	return c
}

func TestRoundTrip(t *testing.T) {
	c := newTestCodec(t)

	payload := domain.ExchangePayload{
		ClientID: "website",
		User: domain.UserInfo{
			ProviderName: "discord",
			ProviderID:   "123456789",
			Username:     "testuser",
			DisplayName:  "Test User",
			AvatarURL:    "https://cdn.example.com/avatar.png",
			Email:        "test@example.com",
		},
	}

	code, err := c.Encode(payload)
	if err != nil {
		t.Fatalf("Encode error: %v", err)
	}

	got, err := c.Decode(code)
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}

	if got.ClientID != "website" {
		t.Errorf("expected ClientID 'website', got %q", got.ClientID)
	}
	if got.User.ProviderName != "discord" {
		t.Errorf("expected provider 'discord', got %q", got.User.ProviderName)
	}
	if got.User.ProviderID != "123456789" {
		t.Errorf("expected provider_id '123456789', got %q", got.User.ProviderID)
	}
	if got.User.Username != "testuser" {
		t.Errorf("expected username 'testuser', got %q", got.User.Username)
	}
	if got.User.Email != "test@example.com" {
		t.Errorf("expected email, got %q", got.User.Email)
	}
}

func TestExpiredCode(t *testing.T) {
	c := newTestCodec(t)
	now := time.Now()
	c.now = func() time.Time { return now }

	code, err := c.Encode(domain.ExchangePayload{
		ClientID: "website",
		User:     domain.UserInfo{ProviderName: "discord", ProviderID: "123"},
	})
	if err != nil {
		t.Fatalf("Encode error: %v", err)
	}

	// Advance time past 30-second expiry
	c.now = func() time.Time { return now.Add(31 * time.Second) }

	_, err = c.Decode(code)
	if !errors.Is(err, domain.ErrExpiredExchangeCode) {
		t.Errorf("expected ErrExpiredExchangeCode, got %v", err)
	}
}

func TestWrongKey(t *testing.T) {
	c1 := newTestCodec(t)
	c2, err := NewCodec([]byte("different-key-567890123456789012"))
	if err != nil {
		t.Fatalf("NewCodec error: %v", err)
	}

	code, err := c1.Encode(domain.ExchangePayload{
		ClientID: "website",
		User:     domain.UserInfo{ProviderName: "discord", ProviderID: "123"},
	})
	if err != nil {
		t.Fatalf("Encode error: %v", err)
	}

	_, err = c2.Decode(code)
	if !errors.Is(err, domain.ErrInvalidExchangeCode) {
		t.Errorf("expected ErrInvalidExchangeCode, got %v", err)
	}
}

func TestTamperedCiphertext(t *testing.T) {
	c := newTestCodec(t)

	code, err := c.Encode(domain.ExchangePayload{
		ClientID: "website",
		User:     domain.UserInfo{ProviderName: "discord", ProviderID: "123"},
	})
	if err != nil {
		t.Fatalf("Encode error: %v", err)
	}

	raw, _ := base64.RawURLEncoding.DecodeString(code)
	raw[len(raw)-1] ^= 0xFF // flip last byte
	tampered := base64.RawURLEncoding.EncodeToString(raw)

	_, err = c.Decode(tampered)
	if !errors.Is(err, domain.ErrInvalidExchangeCode) {
		t.Errorf("expected ErrInvalidExchangeCode, got %v", err)
	}
}

func TestMalformedInput(t *testing.T) {
	c := newTestCodec(t)

	tests := []struct {
		name string
		code string
	}{
		{"empty", ""},
		{"not base64", "!!!not-valid!!!"},
		{"too short", base64.RawURLEncoding.EncodeToString([]byte("short"))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := c.Decode(tt.code)
			if !errors.Is(err, domain.ErrInvalidExchangeCode) {
				t.Errorf("expected ErrInvalidExchangeCode, got %v", err)
			}
		})
	}
}

func TestClientIDPreserved(t *testing.T) {
	c := newTestCodec(t)

	for _, cid := range []string{"website", "admin-panel", "discord-bot"} {
		code, err := c.Encode(domain.ExchangePayload{
			ClientID: cid,
			User:     domain.UserInfo{ProviderName: "discord", ProviderID: "123"},
		})
		if err != nil {
			t.Fatalf("Encode error: %v", err)
		}
		got, err := c.Decode(code)
		if err != nil {
			t.Fatalf("Decode error: %v", err)
		}
		if got.ClientID != cid {
			t.Errorf("expected ClientID %q, got %q", cid, got.ClientID)
		}
	}
}

func TestInvalidKeySize(t *testing.T) {
	_, err := NewCodec([]byte("too-short"))
	if err == nil {
		t.Error("expected error for invalid key size")
	}
}

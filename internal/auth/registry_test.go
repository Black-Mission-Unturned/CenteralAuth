package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/BlackMission/centralauth/internal/domain"
)

type mockProvider struct {
	name string
}

func (m *mockProvider) Name() string { return m.name }
func (m *mockProvider) AuthURL(stateToken string) (string, error) {
	return "https://example.com/auth", nil
}
func (m *mockProvider) Exchange(ctx context.Context, params map[string]string) (*domain.AuthResult, error) {
	return nil, nil
}

func TestRegisterAndGet(t *testing.T) {
	r := NewRegistry()

	p := &mockProvider{name: "discord"}
	if err := r.Register(p); err != nil {
		t.Fatalf("Register error: %v", err)
	}

	got, err := r.Get("discord")
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if got.Name() != "discord" {
		t.Errorf("expected 'discord', got %q", got.Name())
	}
}

func TestUnknownProvider(t *testing.T) {
	r := NewRegistry()

	_, err := r.Get("unknown")
	if !errors.Is(err, domain.ErrProviderNotFound) {
		t.Errorf("expected ErrProviderNotFound, got %v", err)
	}
}

func TestDuplicateProvider(t *testing.T) {
	r := NewRegistry()

	if err := r.Register(&mockProvider{name: "discord"}); err != nil {
		t.Fatalf("first Register error: %v", err)
	}

	err := r.Register(&mockProvider{name: "discord"})
	if !errors.Is(err, domain.ErrDuplicateProvider) {
		t.Errorf("expected ErrDuplicateProvider, got %v", err)
	}
}

func TestNames(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockProvider{name: "discord"})
	r.Register(&mockProvider{name: "steam"})

	names := r.Names()
	if len(names) != 2 {
		t.Fatalf("expected 2 names, got %d", len(names))
	}

	nameSet := make(map[string]bool)
	for _, n := range names {
		nameSet[n] = true
	}
	if !nameSet["discord"] || !nameSet["steam"] {
		t.Errorf("expected discord and steam, got %v", names)
	}
}

package client

import (
	"errors"
	"testing"

	"github.com/BlackMission/centralauth/internal/domain"
)

func testClients() []domain.ClientApp {
	return []domain.ClientApp{
		{
			ID:               "website",
			Name:             "Website",
			APIKey:           "web-api-key-secret",
			AllowedCallbacks: []string{"https://example.com/callback", "http://localhost:3000/callback"},
			AllowedProviders: []string{"discord", "steam"},
		},
		{
			ID:               "admin",
			Name:             "Admin Panel",
			APIKey:           "admin-api-key-secret",
			AllowedCallbacks: []string{"https://admin.example.com/callback"},
			AllowedProviders: []string{"discord"},
		},
	}
}

func TestGetByID(t *testing.T) {
	r, err := NewRegistry(testClients())
	if err != nil {
		t.Fatalf("NewRegistry error: %v", err)
	}

	c, err := r.Get("website")
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if c.Name != "Website" {
		t.Errorf("expected name 'Website', got %q", c.Name)
	}
}

func TestGetByAPIKey(t *testing.T) {
	r, err := NewRegistry(testClients())
	if err != nil {
		t.Fatalf("NewRegistry error: %v", err)
	}

	c, err := r.GetByAPIKey("admin-api-key-secret")
	if err != nil {
		t.Fatalf("GetByAPIKey error: %v", err)
	}
	if c.ID != "admin" {
		t.Errorf("expected ID 'admin', got %q", c.ID)
	}
}

func TestUnknownID(t *testing.T) {
	r, err := NewRegistry(testClients())
	if err != nil {
		t.Fatalf("NewRegistry error: %v", err)
	}

	_, err = r.Get("nonexistent")
	if !errors.Is(err, domain.ErrClientNotFound) {
		t.Errorf("expected ErrClientNotFound, got %v", err)
	}
}

func TestUnknownAPIKey(t *testing.T) {
	r, err := NewRegistry(testClients())
	if err != nil {
		t.Fatalf("NewRegistry error: %v", err)
	}

	_, err = r.GetByAPIKey("wrong-key")
	if !errors.Is(err, domain.ErrInvalidAPIKey) {
		t.Errorf("expected ErrInvalidAPIKey, got %v", err)
	}
}

func TestValidateCallback_Allowed(t *testing.T) {
	r, err := NewRegistry(testClients())
	if err != nil {
		t.Fatalf("NewRegistry error: %v", err)
	}

	if err := r.ValidateCallback("website", "https://example.com/callback"); err != nil {
		t.Errorf("expected allowed callback, got error: %v", err)
	}
}

func TestValidateCallback_Disallowed(t *testing.T) {
	r, err := NewRegistry(testClients())
	if err != nil {
		t.Fatalf("NewRegistry error: %v", err)
	}

	err = r.ValidateCallback("website", "https://evil.com/callback")
	if !errors.Is(err, domain.ErrCallbackNotAllowed) {
		t.Errorf("expected ErrCallbackNotAllowed, got %v", err)
	}
}

func TestValidateProvider_Allowed(t *testing.T) {
	r, err := NewRegistry(testClients())
	if err != nil {
		t.Fatalf("NewRegistry error: %v", err)
	}

	if err := r.ValidateProvider("website", "discord"); err != nil {
		t.Errorf("expected allowed provider, got error: %v", err)
	}
}

func TestValidateProvider_Disallowed(t *testing.T) {
	r, err := NewRegistry(testClients())
	if err != nil {
		t.Fatalf("NewRegistry error: %v", err)
	}

	err = r.ValidateProvider("admin", "steam")
	if !errors.Is(err, domain.ErrProviderNotAllowed) {
		t.Errorf("expected ErrProviderNotAllowed, got %v", err)
	}
}

func TestDuplicateIDs(t *testing.T) {
	clients := []domain.ClientApp{
		{ID: "dup", Name: "One", APIKey: "key1"},
		{ID: "dup", Name: "Two", APIKey: "key2"},
	}
	_, err := NewRegistry(clients)
	if !errors.Is(err, domain.ErrDuplicateClientID) {
		t.Errorf("expected ErrDuplicateClientID, got %v", err)
	}
}

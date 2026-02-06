package config

import (
	"errors"
	"testing"

	"github.com/BlackMission/centralauth/internal/domain"
)

// setRequiredEnv sets the minimum required env vars for a valid config.
func setRequiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("STATE_SIGNING_KEY", "test-signing-key-1234567890123456")
	t.Setenv("EXCHANGE_ENCRYPTION_KEY", "test-encrypt-key-1234567890123456")
	t.Setenv("CLIENT_WEBSITE_API_KEY", "test-api-key")
}

func TestLoadFromEnv_FullConfig(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("HOST", "127.0.0.1")
	t.Setenv("BASE_URL", "https://auth.example.com")
	t.Setenv("STATE_SIGNING_KEY", "test-signing-key-1234567890123456")
	t.Setenv("EXCHANGE_ENCRYPTION_KEY", "test-encrypt-key-1234567890123456")
	t.Setenv("DISCORD_CLIENT_ID", "discord-id")
	t.Setenv("DISCORD_CLIENT_SECRET", "discord-secret")
	t.Setenv("DISCORD_SCOPES", "identify,email,guilds")
	t.Setenv("STEAM_API_KEY", "steam-key")
	t.Setenv("STEAM_REALM", "https://auth.example.com")
	t.Setenv("CLIENT_WEBSITE_API_KEY", "test-api-key")
	t.Setenv("CLIENT_WEBSITE_NAME", "Test Website")
	t.Setenv("CLIENT_WEBSITE_ALLOWED_CALLBACKS", "https://example.com/callback")
	t.Setenv("CLIENT_WEBSITE_ALLOWED_PROVIDERS", "discord")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Server.Port != 9090 {
		t.Errorf("expected port 9090, got %d", cfg.Server.Port)
	}
	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("expected host 127.0.0.1, got %s", cfg.Server.Host)
	}
	if cfg.Server.BaseURL != "https://auth.example.com" {
		t.Errorf("expected base_url https://auth.example.com, got %s", cfg.Server.BaseURL)
	}
	if cfg.Secrets.StateSigningKey != "test-signing-key-1234567890123456" {
		t.Errorf("unexpected state signing key: %s", cfg.Secrets.StateSigningKey)
	}

	// Discord provider
	dc, ok := cfg.Providers["discord"]
	if !ok {
		t.Fatal("expected discord provider to be registered")
	}
	if dc.ClientID != "discord-id" {
		t.Errorf("expected discord client_id 'discord-id', got %s", dc.ClientID)
	}
	if dc.ClientSecret != "discord-secret" {
		t.Errorf("expected discord client_secret 'discord-secret', got %s", dc.ClientSecret)
	}
	if len(dc.Scopes) != 3 || dc.Scopes[2] != "guilds" {
		t.Errorf("expected scopes [identify email guilds], got %v", dc.Scopes)
	}

	// Steam provider
	sc, ok := cfg.Providers["steam"]
	if !ok {
		t.Fatal("expected steam provider to be registered")
	}
	if sc.APIKey != "steam-key" {
		t.Errorf("expected steam api_key 'steam-key', got %s", sc.APIKey)
	}
	if sc.Realm != "https://auth.example.com" {
		t.Errorf("expected steam realm 'https://auth.example.com', got %s", sc.Realm)
	}

	// Client
	if len(cfg.Clients) != 1 {
		t.Fatalf("expected 1 client, got %d", len(cfg.Clients))
	}
	c := cfg.Clients[0]
	if c.ID != "website" {
		t.Errorf("expected client ID 'website', got %s", c.ID)
	}
	if c.Name != "Test Website" {
		t.Errorf("expected client name 'Test Website', got %s", c.Name)
	}
	if len(c.AllowedCallbacks) != 1 || c.AllowedCallbacks[0] != "https://example.com/callback" {
		t.Errorf("unexpected callbacks: %v", c.AllowedCallbacks)
	}
	if len(c.AllowedProviders) != 1 || c.AllowedProviders[0] != "discord" {
		t.Errorf("unexpected providers: %v", c.AllowedProviders)
	}
}

func TestLoadFromEnv_DefaultValues(t *testing.T) {
	setRequiredEnv(t)

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("expected default port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("expected default host 0.0.0.0, got %s", cfg.Server.Host)
	}
}

func TestLoadFromEnv_DiscordDefaultScopes(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("DISCORD_CLIENT_ID", "discord-id")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dc, ok := cfg.Providers["discord"]
	if !ok {
		t.Fatal("expected discord provider")
	}
	if len(dc.Scopes) != 2 || dc.Scopes[0] != "identify" || dc.Scopes[1] != "email" {
		t.Errorf("expected default scopes [identify email], got %v", dc.Scopes)
	}
}

func TestLoadFromEnv_SteamRealmDefaultsToBaseURL(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("BASE_URL", "https://auth.example.com")
	t.Setenv("STEAM_API_KEY", "steam-key")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sc := cfg.Providers["steam"]
	if sc.Realm != "https://auth.example.com" {
		t.Errorf("expected steam realm to default to BASE_URL, got %s", sc.Realm)
	}
}

func TestLoadFromEnv_MissingSigningKey(t *testing.T) {
	t.Setenv("EXCHANGE_ENCRYPTION_KEY", "some-key")
	t.Setenv("CLIENT_WEBSITE_API_KEY", "key")

	_, err := LoadFromEnv()
	if err == nil {
		t.Fatal("expected error for missing signing key")
	}
	if !errors.Is(err, domain.ErrMissingConfig) {
		t.Errorf("expected ErrMissingConfig, got %v", err)
	}
}

func TestLoadFromEnv_MissingEncryptionKey(t *testing.T) {
	t.Setenv("STATE_SIGNING_KEY", "some-key")
	t.Setenv("CLIENT_WEBSITE_API_KEY", "key")

	_, err := LoadFromEnv()
	if err == nil {
		t.Fatal("expected error for missing encryption key")
	}
	if !errors.Is(err, domain.ErrMissingConfig) {
		t.Errorf("expected ErrMissingConfig, got %v", err)
	}
}

func TestLoadFromEnv_MissingClients(t *testing.T) {
	t.Setenv("STATE_SIGNING_KEY", "key1")
	t.Setenv("EXCHANGE_ENCRYPTION_KEY", "key2")

	_, err := LoadFromEnv()
	if err == nil {
		t.Fatal("expected error for missing clients")
	}
	if !errors.Is(err, domain.ErrMissingConfig) {
		t.Errorf("expected ErrMissingConfig, got %v", err)
	}
}

func TestLoadFromEnv_InvalidPort(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("PORT", "not-a-number")

	_, err := LoadFromEnv()
	if err == nil {
		t.Fatal("expected error for invalid port")
	}
	if !errors.Is(err, domain.ErrInvalidConfig) {
		t.Errorf("expected ErrInvalidConfig, got %v", err)
	}
}

func TestLoadFromEnv_ClientAutoDiscovery(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("CLIENT_WEBSITE_NAME", "My Website")
	t.Setenv("CLIENT_WEBSITE_ALLOWED_CALLBACKS", "https://example.com/cb")
	t.Setenv("CLIENT_WEBSITE_ALLOWED_PROVIDERS", "discord,steam")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Clients) != 1 {
		t.Fatalf("expected 1 client, got %d", len(cfg.Clients))
	}
	c := cfg.Clients[0]
	if c.ID != "website" {
		t.Errorf("expected id 'website', got %s", c.ID)
	}
	if c.Name != "My Website" {
		t.Errorf("expected name 'My Website', got %s", c.Name)
	}
	if len(c.AllowedProviders) != 2 {
		t.Errorf("expected 2 providers, got %d", len(c.AllowedProviders))
	}
}

func TestLoadFromEnv_ClientNameDefaultsToID(t *testing.T) {
	setRequiredEnv(t)

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Clients[0].Name != "website" {
		t.Errorf("expected name to default to id 'website', got %s", cfg.Clients[0].Name)
	}
}

func TestLoadFromEnv_MultipleClients(t *testing.T) {
	t.Setenv("STATE_SIGNING_KEY", "key1")
	t.Setenv("EXCHANGE_ENCRYPTION_KEY", "key2")
	t.Setenv("CLIENT_WEBSITE_API_KEY", "key-a")
	t.Setenv("CLIENT_ADMIN_PANEL_API_KEY", "key-b")
	t.Setenv("CLIENT_ADMIN_PANEL_NAME", "Admin Panel")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Clients) != 2 {
		t.Fatalf("expected 2 clients, got %d", len(cfg.Clients))
	}

	// Sorted by ID: admin-panel < website
	if cfg.Clients[0].ID != "admin-panel" {
		t.Errorf("expected first client 'admin-panel', got %s", cfg.Clients[0].ID)
	}
	if cfg.Clients[0].Name != "Admin Panel" {
		t.Errorf("expected name 'Admin Panel', got %s", cfg.Clients[0].Name)
	}
	if cfg.Clients[1].ID != "website" {
		t.Errorf("expected second client 'website', got %s", cfg.Clients[1].ID)
	}
}

func TestLoadFromEnv_MultiWordClientID(t *testing.T) {
	t.Setenv("STATE_SIGNING_KEY", "key1")
	t.Setenv("EXCHANGE_ENCRYPTION_KEY", "key2")
	t.Setenv("CLIENT_ADMIN_PANEL_API_KEY", "key-a")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Clients[0].ID != "admin-panel" {
		t.Errorf("expected id 'admin-panel', got %s", cfg.Clients[0].ID)
	}
}

func TestLoadFromEnv_CommaSeparatedCallbacks(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("CLIENT_WEBSITE_ALLOWED_CALLBACKS", "https://example.com/cb, http://localhost:3000/cb")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cbs := cfg.Clients[0].AllowedCallbacks
	if len(cbs) != 2 {
		t.Fatalf("expected 2 callbacks, got %d", len(cbs))
	}
	if cbs[0] != "https://example.com/cb" {
		t.Errorf("expected first callback trimmed, got %q", cbs[0])
	}
	if cbs[1] != "http://localhost:3000/cb" {
		t.Errorf("expected second callback trimmed, got %q", cbs[1])
	}
}

func TestLoadFromEnv_NoProvidersByDefault(t *testing.T) {
	setRequiredEnv(t)

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Providers) != 0 {
		t.Errorf("expected no providers by default, got %d", len(cfg.Providers))
	}
}

func TestLoadFromEnv_ProviderEnabledByPresence(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("DISCORD_CLIENT_ID", "some-id")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := cfg.Providers["discord"]; !ok {
		t.Error("expected discord provider to be enabled")
	}
	if _, ok := cfg.Providers["steam"]; ok {
		t.Error("expected steam provider to be disabled")
	}
}

func TestSplitComma(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"", nil},
		{"a", []string{"a"}},
		{"a,b,c", []string{"a", "b", "c"}},
		{"a, b , c", []string{"a", "b", "c"}},
		{" a , ", []string{"a"}},
	}
	for _, tt := range tests {
		got := splitComma(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("splitComma(%q) = %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("splitComma(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}


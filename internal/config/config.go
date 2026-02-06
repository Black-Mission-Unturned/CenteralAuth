package config

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/BlackMission/centralauth/internal/domain"
)

// Config is the top-level application configuration.
type Config struct {
	Server    ServerConfig
	Secrets   SecretsConfig
	Providers map[string]ProviderConfig
	Clients   []ClientConfig
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Port    int
	Host    string
	BaseURL string
}

// SecretsConfig holds cryptographic key references.
type SecretsConfig struct {
	StateSigningKey       string
	ExchangeEncryptionKey string
}

// ProviderConfig holds provider-specific settings.
type ProviderConfig struct {
	ClientID     string
	ClientSecret string
	Scopes       []string
	APIKey       string
	Realm        string
}

// ClientConfig holds a registered client app's settings.
type ClientConfig struct {
	ID               string
	Name             string
	APIKey           string
	AllowedCallbacks []string
	AllowedProviders []string
}

// LoadFromEnv reads configuration purely from environment variables.
func LoadFromEnv() (*Config, error) {
	port := 8080
	if v := os.Getenv("PORT"); v != "" {
		p, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("%w: PORT must be a number: %v", domain.ErrInvalidConfig, err)
		}
		port = p
	}

	cfg := &Config{
		Server: ServerConfig{
			Port:    port,
			Host:    getenvDefault("HOST", "0.0.0.0"),
			BaseURL: os.Getenv("BASE_URL"),
		},
		Secrets: SecretsConfig{
			StateSigningKey:       os.Getenv("STATE_SIGNING_KEY"),
			ExchangeEncryptionKey: os.Getenv("EXCHANGE_ENCRYPTION_KEY"),
		},
		Providers: make(map[string]ProviderConfig),
	}

	// Discord provider — enabled by presence of DISCORD_CLIENT_ID
	if id := os.Getenv("DISCORD_CLIENT_ID"); id != "" {
		cfg.Providers["discord"] = ProviderConfig{
			ClientID:     id,
			ClientSecret: os.Getenv("DISCORD_CLIENT_SECRET"),
			Scopes:       splitComma(getenvDefault("DISCORD_SCOPES", "identify,email")),
		}
	}

	// Steam provider — enabled by presence of STEAM_API_KEY
	if key := os.Getenv("STEAM_API_KEY"); key != "" {
		cfg.Providers["steam"] = ProviderConfig{
			APIKey: key,
			Realm:  getenvDefault("STEAM_REALM", cfg.Server.BaseURL),
		}
	}

	// Client discovery — scan env for CLIENT_<ID>_API_KEY
	cfg.Clients = discoverClients()

	if err := validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// discoverClients scans environment variables for CLIENT_<ID>_API_KEY patterns
// and builds client configs from related env vars.
func discoverClients() []ClientConfig {
	// Collect client IDs from CLIENT_*_API_KEY vars
	type clientEntry struct {
		envPrefix string // e.g. "CLIENT_WEBSITE"
		id        string // e.g. "website"
	}

	var entries []clientEntry
	seen := make(map[string]bool)

	for _, env := range os.Environ() {
		key, _, ok := strings.Cut(env, "=")
		if !ok {
			continue
		}
		if !strings.HasPrefix(key, "CLIENT_") || !strings.HasSuffix(key, "_API_KEY") {
			continue
		}

		// Extract prefix: CLIENT_WEBSITE_API_KEY → CLIENT_WEBSITE
		prefix := strings.TrimSuffix(key, "_API_KEY")
		if prefix == "CLIENT" {
			continue // no ID segment
		}

		idPart := strings.TrimPrefix(prefix, "CLIENT_")
		id := strings.ToLower(strings.ReplaceAll(idPart, "_", "-"))

		if seen[id] {
			continue
		}
		seen[id] = true
		entries = append(entries, clientEntry{envPrefix: prefix, id: id})
	}

	// Sort by ID for deterministic ordering
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].id < entries[j].id
	})

	clients := make([]ClientConfig, 0, len(entries))
	for _, e := range entries {
		apiKey := os.Getenv(e.envPrefix + "_API_KEY")
		if apiKey == "" {
			continue
		}

		name := os.Getenv(e.envPrefix + "_NAME")
		if name == "" {
			name = e.id
		}

		var callbacks []string
		if v := os.Getenv(e.envPrefix + "_ALLOWED_CALLBACKS"); v != "" {
			callbacks = splitComma(v)
		}

		var providers []string
		if v := os.Getenv(e.envPrefix + "_ALLOWED_PROVIDERS"); v != "" {
			providers = splitComma(v)
		}

		clients = append(clients, ClientConfig{
			ID:               e.id,
			Name:             name,
			APIKey:           apiKey,
			AllowedCallbacks: callbacks,
			AllowedProviders: providers,
		})
	}

	return clients
}

func validate(cfg *Config) error {
	if cfg.Secrets.StateSigningKey == "" {
		return fmt.Errorf("%w: STATE_SIGNING_KEY is required", domain.ErrMissingConfig)
	}
	if cfg.Secrets.ExchangeEncryptionKey == "" {
		return fmt.Errorf("%w: EXCHANGE_ENCRYPTION_KEY is required", domain.ErrMissingConfig)
	}
	if len(cfg.Clients) == 0 {
		return fmt.Errorf("%w: at least one client must be configured (CLIENT_<ID>_API_KEY)", domain.ErrMissingConfig)
	}
	for _, c := range cfg.Clients {
		if c.APIKey == "" {
			return fmt.Errorf("%w: client %q API_KEY is required", domain.ErrMissingConfig, c.ID)
		}
	}
	return nil
}

func getenvDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func splitComma(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

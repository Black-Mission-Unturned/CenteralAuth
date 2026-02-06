package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/BlackMission/centralauth/internal/auth"
	"github.com/BlackMission/centralauth/internal/client"
	"github.com/BlackMission/centralauth/internal/domain"
	"github.com/BlackMission/centralauth/internal/exchange"
	"github.com/BlackMission/centralauth/internal/state"
)

// fakeProvider allows full control over Exchange results for integration tests.
type fakeProvider struct {
	name    string
	authURL string
	result  *domain.AuthResult
	err     error
}

func (f *fakeProvider) Name() string { return f.name }
func (f *fakeProvider) AuthURL(stateToken string) (string, error) {
	return f.authURL + "?state=" + stateToken, nil
}
func (f *fakeProvider) Exchange(ctx context.Context, params map[string]string) (*domain.AuthResult, error) {
	return f.result, f.err
}

func setupTestServer() (*httptest.Server, *exchange.Codec, *state.Service) {
	clients, _ := client.NewRegistry([]domain.ClientApp{
		{
			ID:               "website",
			Name:             "Test Website",
			APIKey:           "test-api-key",
			AllowedCallbacks: []string{"https://example.com/auth/callback"},
			AllowedProviders: []string{"discord", "steam"},
		},
	})

	providers := auth.NewRegistry()

	stateKey := []byte("test-state-key-1234567890abcdef")
	encKey := []byte("01234567890123456789012345678901") // 32 bytes
	stateSvc := state.NewService(stateKey)
	codec, _ := exchange.NewCodec(encKey)

	srv := New(Config{Host: "127.0.0.1", Port: 0}, Deps{
		Clients:   clients,
		Providers: providers,
		State:     stateSvc,
		Exchange:  codec,
	})

	return httptest.NewServer(srv.Handler()), codec, stateSvc
}

func TestIntegration_HealthEndpoint(t *testing.T) {
	ts, _, _ := setupTestServer()
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("request error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Errorf("expected ok, got %q", body["status"])
	}
}

func TestIntegration_ProvidersEndpoint(t *testing.T) {
	clients, _ := client.NewRegistry([]domain.ClientApp{
		{ID: "t", Name: "T", APIKey: "k", AllowedProviders: []string{"discord"}},
	})
	providers := auth.NewRegistry()
	providers.Register(&fakeProvider{name: "discord"})
	providers.Register(&fakeProvider{name: "steam"})

	stateSvc := state.NewService([]byte("key-1234567890abcdef12345678"))
	codec, _ := exchange.NewCodec([]byte("01234567890123456789012345678901"))

	srv := New(Config{Host: "127.0.0.1", Port: 0}, Deps{
		Clients: clients, Providers: providers, State: stateSvc, Exchange: codec,
	})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, _ := http.Get(ts.URL + "/providers")
	defer resp.Body.Close()

	var names []string
	json.NewDecoder(resp.Body).Decode(&names)
	if len(names) != 2 {
		t.Fatalf("expected 2, got %d", len(names))
	}
}

func TestIntegration_FullDiscordFlow(t *testing.T) {
	clients, _ := client.NewRegistry([]domain.ClientApp{
		{
			ID:               "website",
			Name:             "Test Website",
			APIKey:           "test-api-key",
			AllowedCallbacks: []string{"https://example.com/auth/callback"},
			AllowedProviders: []string{"discord"},
		},
	})

	discordProvider := &fakeProvider{
		name: "discord",
		result: &domain.AuthResult{
			User: domain.UserInfo{
				ProviderName: "discord",
				ProviderID:   "987654321",
				Username:     "tactical",
				DisplayName:  "Tactical Commander",
				AvatarURL:    "https://cdn.discordapp.com/avatars/987654321/abc.png",
				Email:        "tactical@example.com",
			},
		},
	}

	providers := auth.NewRegistry()
	providers.Register(discordProvider)

	stateSvc := state.NewService([]byte("test-state-key-1234567890abcdef"))
	codec, _ := exchange.NewCodec([]byte("01234567890123456789012345678901"))

	srv := New(Config{Host: "127.0.0.1", Port: 0}, Deps{
		Clients: clients, Providers: providers, State: stateSvc, Exchange: codec,
	})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// The fake provider's authURL will include a callback URL we need to simulate
	// For this integration test, we'll generate the callback URL from the auth redirect
	discordProvider.authURL = ts.URL + "/callback/discord"

	// Step 1: Initiate auth flow
	httpClient := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse // Don't follow redirects
	}}

	resp, err := httpClient.Get(ts.URL + "/auth/discord?client_id=website&redirect_uri=" +
		url.QueryEscape("https://example.com/auth/callback"))
	if err != nil {
		t.Fatalf("auth request error: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusFound {
		t.Fatalf("expected 302 from /auth, got %d", resp.StatusCode)
	}

	// Step 2: Follow redirect to "provider" (which is actually our callback)
	providerURL := resp.Header.Get("Location")
	resp, err = httpClient.Get(providerURL)
	if err != nil {
		t.Fatalf("callback request error: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusFound {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 302 from /callback, got %d: %s", resp.StatusCode, body)
	}

	// Step 3: Extract exchange code from redirect
	callbackRedirect := resp.Header.Get("Location")
	callbackURL, err := url.Parse(callbackRedirect)
	if err != nil {
		t.Fatalf("parse callback redirect: %v", err)
	}

	exchangeCode := callbackURL.Query().Get("code")
	if exchangeCode == "" {
		t.Fatal("no exchange code in callback redirect")
	}

	// Step 4: Exchange code for user info
	req, _ := http.NewRequest(http.MethodGet,
		fmt.Sprintf("%s/exchange?code=%s", ts.URL, url.QueryEscape(exchangeCode)), nil)
	req.Header.Set("Authorization", "Bearer test-api-key")

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("exchange request error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200 from /exchange, got %d: %s", resp.StatusCode, body)
	}

	var result domain.AuthResult
	json.NewDecoder(resp.Body).Decode(&result)

	if result.User.ProviderName != "discord" {
		t.Errorf("expected provider 'discord', got %q", result.User.ProviderName)
	}
	if result.User.ProviderID != "987654321" {
		t.Errorf("expected provider_id '987654321', got %q", result.User.ProviderID)
	}
	if result.User.Username != "tactical" {
		t.Errorf("expected username 'tactical', got %q", result.User.Username)
	}
	if result.User.Email != "tactical@example.com" {
		t.Errorf("expected email, got %q", result.User.Email)
	}
}

func TestIntegration_InvalidClientRejected(t *testing.T) {
	ts, _, _ := setupTestServer()
	defer ts.Close()

	httpClient := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}

	resp, err := httpClient.Get(ts.URL + "/auth/discord?client_id=unknown&redirect_uri=https://example.com/callback")
	if err != nil {
		t.Fatalf("request error: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestIntegration_GracefulShutdown(t *testing.T) {
	clients, _ := client.NewRegistry([]domain.ClientApp{
		{ID: "t", Name: "T", APIKey: "k"},
	})
	providers := auth.NewRegistry()
	stateSvc := state.NewService([]byte("key-1234567890abcdef12345678"))
	codec, _ := exchange.NewCodec([]byte("01234567890123456789012345678901"))

	srv := New(Config{Host: "127.0.0.1", Port: 0}, Deps{
		Clients: clients, Providers: providers, State: stateSvc, Exchange: codec,
	})

	// Start server in background
	go srv.Start()

	// Shutdown should work without error
	err := srv.Shutdown(context.Background())
	if err != nil {
		t.Errorf("shutdown error: %v", err)
	}
}

package discord

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/BlackMission/centralauth/internal/domain"
)

func setupTestProvider(tokenHandler, userHandler http.HandlerFunc) *Provider {
	mux := http.NewServeMux()
	if tokenHandler != nil {
		mux.HandleFunc("/oauth2/token", tokenHandler)
	}
	if userHandler != nil {
		mux.HandleFunc("/users/@me", userHandler)
	}
	server := httptest.NewServer(mux)

	p := New(Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		Scopes:       []string{"identify", "email"},
		CallbackURL:  "https://auth.example.com/callback/discord",
	})
	p.httpClient = server.Client()
	p.tokenURL = server.URL + "/oauth2/token"
	p.userURL = server.URL + "/users/@me"
	return p
}

func TestAuthURL_ContainsCorrectParams(t *testing.T) {
	p := New(Config{
		ClientID:    "test-client-id",
		Scopes:      []string{"identify", "email"},
		CallbackURL: "https://auth.example.com/callback/discord",
	})

	authURL, err := p.AuthURL("test-state-token")
	if err != nil {
		t.Fatalf("AuthURL error: %v", err)
	}

	u, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("parse URL error: %v", err)
	}

	if got := u.Query().Get("client_id"); got != "test-client-id" {
		t.Errorf("expected client_id 'test-client-id', got %q", got)
	}
	if got := u.Query().Get("response_type"); got != "code" {
		t.Errorf("expected response_type 'code', got %q", got)
	}
	if got := u.Query().Get("scope"); got != "identify email" {
		t.Errorf("expected scope 'identify email', got %q", got)
	}
	if got := u.Query().Get("state"); got != "test-state-token" {
		t.Errorf("expected state 'test-state-token', got %q", got)
	}
	if got := u.Query().Get("redirect_uri"); got != "https://auth.example.com/callback/discord" {
		t.Errorf("expected redirect_uri, got %q", got)
	}
}

func TestExchange_Success(t *testing.T) {
	p := setupTestProvider(
		func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(tokenResponse{
				AccessToken: "test-access-token",
				TokenType:   "Bearer",
			})
		},
		func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer test-access-token" {
				t.Error("expected Bearer token in Authorization header")
			}
			json.NewEncoder(w).Encode(discordUser{
				ID:         "123456789",
				Username:   "testuser",
				GlobalName: "Test User",
				Avatar:     "abc123",
				Email:      "test@example.com",
			})
		},
	)

	result, err := p.Exchange(context.Background(), map[string]string{"code": "auth-code"})
	if err != nil {
		t.Fatalf("Exchange error: %v", err)
	}

	if result.User.ProviderName != "discord" {
		t.Errorf("expected provider 'discord', got %q", result.User.ProviderName)
	}
	if result.User.ProviderID != "123456789" {
		t.Errorf("expected provider_id '123456789', got %q", result.User.ProviderID)
	}
	if result.User.Username != "testuser" {
		t.Errorf("expected username 'testuser', got %q", result.User.Username)
	}
	if result.User.DisplayName != "Test User" {
		t.Errorf("expected display_name 'Test User', got %q", result.User.DisplayName)
	}
	if result.User.Email != "test@example.com" {
		t.Errorf("expected email 'test@example.com', got %q", result.User.Email)
	}
	if result.User.AvatarURL != "https://cdn.discordapp.com/avatars/123456789/abc123.png" {
		t.Errorf("unexpected avatar URL: %q", result.User.AvatarURL)
	}
}

func TestExchange_TokenFailure(t *testing.T) {
	p := setupTestProvider(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":"invalid_grant"}`))
		},
		nil,
	)

	_, err := p.Exchange(context.Background(), map[string]string{"code": "bad-code"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, domain.ErrProviderExchange) {
		t.Errorf("expected ErrProviderExchange, got %v", err)
	}
}

func TestExchange_UserFetchFailure(t *testing.T) {
	p := setupTestProvider(
		func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(tokenResponse{
				AccessToken: "test-access-token",
				TokenType:   "Bearer",
			})
		},
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"message":"401: Unauthorized"}`))
		},
	)

	_, err := p.Exchange(context.Background(), map[string]string{"code": "auth-code"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, domain.ErrProviderUserFetch) {
		t.Errorf("expected ErrProviderUserFetch, got %v", err)
	}
}

func TestExchange_MissingCode(t *testing.T) {
	p := New(Config{})

	_, err := p.Exchange(context.Background(), map[string]string{})
	if !errors.Is(err, domain.ErrMissingProviderParams) {
		t.Errorf("expected ErrMissingProviderParams, got %v", err)
	}
}

func TestExchange_MalformedJSON(t *testing.T) {
	p := setupTestProvider(
		func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`not json`))
		},
		nil,
	)

	_, err := p.Exchange(context.Background(), map[string]string{"code": "auth-code"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, domain.ErrProviderExchange) {
		t.Errorf("expected ErrProviderExchange, got %v", err)
	}
}

func TestExchange_FallbackDisplayName(t *testing.T) {
	p := setupTestProvider(
		func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(tokenResponse{
				AccessToken: "token",
				TokenType:   "Bearer",
			})
		},
		func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(discordUser{
				ID:       "123",
				Username: "user",
				// GlobalName intentionally empty
			})
		},
	)

	result, err := p.Exchange(context.Background(), map[string]string{"code": "auth-code"})
	if err != nil {
		t.Fatalf("Exchange error: %v", err)
	}
	if result.User.DisplayName != "user" {
		t.Errorf("expected display_name to fallback to username, got %q", result.User.DisplayName)
	}
}

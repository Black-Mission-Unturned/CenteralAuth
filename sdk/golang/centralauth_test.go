package centralauth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthorizeURL(t *testing.T) {
	client := New(Config{
		BaseURL:  "https://auth.example.com",
		ClientID: "my-app",
		APIKey:   "secret",
	})

	url := client.AuthorizeURL("discord", "https://myapp.com/callback")
	expected := "https://auth.example.com/auth/discord?client_id=my-app&redirect_uri=https%3A%2F%2Fmyapp.com%2Fcallback"
	if url != expected {
		t.Errorf("got %q, want %q", url, expected)
	}
}

func TestAuthorizeURL_TrailingSlash(t *testing.T) {
	client := New(Config{
		BaseURL:  "https://auth.example.com///",
		ClientID: "my-app",
		APIKey:   "secret",
	})

	url := client.AuthorizeURL("steam", "https://myapp.com/cb")
	if got := "https://auth.example.com/auth/steam?client_id=my-app&redirect_uri=https%3A%2F%2Fmyapp.com%2Fcb"; url != got {
		t.Errorf("trailing slash not stripped: got %q", url)
	}
}

func TestExchange_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/exchange" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("code") != "test-code" {
			t.Errorf("unexpected code: %s", r.URL.Query().Get("code"))
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("unexpected auth header: %s", r.Header.Get("Authorization"))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(exchangeResponse{
			User: UserInfo{
				Provider:    "discord",
				ProviderID:  "12345",
				Username:    "testuser",
				DisplayName: "Test User",
				AvatarURL:   "https://example.com/avatar.png",
				Email:       "test@example.com",
			},
		})
	}))
	defer srv.Close()

	client := New(Config{BaseURL: srv.URL, ClientID: "app", APIKey: "test-key"})
	user, err := client.Exchange(context.Background(), "test-code")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.Provider != "discord" {
		t.Errorf("provider = %q, want %q", user.Provider, "discord")
	}
	if user.ProviderID != "12345" {
		t.Errorf("provider_id = %q, want %q", user.ProviderID, "12345")
	}
	if user.Username != "testuser" {
		t.Errorf("username = %q, want %q", user.Username, "testuser")
	}
	if user.DisplayName != "Test User" {
		t.Errorf("display_name = %q, want %q", user.DisplayName, "Test User")
	}
	if user.Email != "test@example.com" {
		t.Errorf("email = %q, want %q", user.Email, "test@example.com")
	}
}

func TestExchange_Expired(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "exchange code expired"})
	}))
	defer srv.Close()

	client := New(Config{BaseURL: srv.URL, ClientID: "app", APIKey: "key"})
	_, err := client.Exchange(context.Background(), "old-code")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var expiredErr *ExchangeExpiredError
	if !errors.As(err, &expiredErr) {
		t.Fatalf("expected ExchangeExpiredError, got %T: %v", err, err)
	}
}

func TestExchange_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid API key"})
	}))
	defer srv.Close()

	client := New(Config{BaseURL: srv.URL, ClientID: "app", APIKey: "bad-key"})
	_, err := client.Exchange(context.Background(), "code")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var unauthErr *UnauthorizedError
	if !errors.As(err, &unauthErr) {
		t.Fatalf("expected UnauthorizedError, got %T: %v", err, err)
	}
}

func TestExchange_Forbidden(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": "API key does not match the client that initiated the auth flow"})
	}))
	defer srv.Close()

	client := New(Config{BaseURL: srv.URL, ClientID: "app", APIKey: "wrong-client-key"})
	_, err := client.Exchange(context.Background(), "code")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var forbiddenErr *ForbiddenError
	if !errors.As(err, &forbiddenErr) {
		t.Fatalf("expected ForbiddenError, got %T: %v", err, err)
	}
}

func TestExchange_NetworkError(t *testing.T) {
	// Point to a closed server to get a network error.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close()

	client := New(Config{BaseURL: srv.URL, ClientID: "app", APIKey: "key"})
	_, err := client.Exchange(context.Background(), "code")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var sdkErr *Error
	if !errors.As(err, &sdkErr) {
		t.Fatalf("expected *Error, got %T: %v", err, err)
	}
}

func TestExchange_ProviderError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(w).Encode(map[string]string{"error": "provider exchange failed"})
	}))
	defer srv.Close()

	client := New(Config{BaseURL: srv.URL, ClientID: "app", APIKey: "key"})
	_, err := client.Exchange(context.Background(), "code")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var provErr *ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("expected ProviderError, got %T: %v", err, err)
	}
}

func TestProviders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/providers" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]string{"discord", "steam"})
	}))
	defer srv.Close()

	client := New(Config{BaseURL: srv.URL, ClientID: "app", APIKey: "key"})
	providers, err := client.Providers(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(providers) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(providers))
	}
	if providers[0] != "discord" || providers[1] != "steam" {
		t.Errorf("providers = %v, want [discord steam]", providers)
	}
}

func TestProviders_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := New(Config{BaseURL: srv.URL, ClientID: "app", APIKey: "key"})
	_, err := client.Providers(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestHealthCheck_Healthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(healthResponse{Status: "ok"})
	}))
	defer srv.Close()

	client := New(Config{BaseURL: srv.URL, ClientID: "app", APIKey: "key"})
	if !client.HealthCheck(context.Background()) {
		t.Error("expected healthy, got unhealthy")
	}
}

func TestHealthCheck_Unhealthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	client := New(Config{BaseURL: srv.URL, ClientID: "app", APIKey: "key"})
	if client.HealthCheck(context.Background()) {
		t.Error("expected unhealthy, got healthy")
	}
}

func TestHealthCheck_NetworkError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close()

	client := New(Config{BaseURL: srv.URL, ClientID: "app", APIKey: "key"})
	if client.HealthCheck(context.Background()) {
		t.Error("expected unhealthy on network error, got healthy")
	}
}

func TestCallbackHandler_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(exchangeResponse{
			User: UserInfo{
				Provider:    "discord",
				ProviderID:  "99",
				Username:    "cbuser",
				DisplayName: "CB User",
				AvatarURL:   "https://example.com/a.png",
			},
		})
	}))
	defer srv.Close()

	client := New(Config{BaseURL: srv.URL, ClientID: "app", APIKey: "key"})

	var gotUser *UserInfo
	handler := CallbackHandler(client,
		func(user *UserInfo, w http.ResponseWriter, r *http.Request) {
			gotUser = user
			w.WriteHeader(http.StatusOK)
		},
		func(err error, w http.ResponseWriter, r *http.Request) {
			t.Fatalf("unexpected error callback: %v", err)
		},
	)

	req := httptest.NewRequest(http.MethodGet, "/callback?code=abc123", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if gotUser == nil {
		t.Fatal("onSuccess was not called")
	}
	if gotUser.Username != "cbuser" {
		t.Errorf("username = %q, want %q", gotUser.Username, "cbuser")
	}
}

func TestCallbackHandler_MissingCode(t *testing.T) {
	client := New(Config{BaseURL: "http://localhost", ClientID: "app", APIKey: "key"})

	var gotErr error
	handler := CallbackHandler(client,
		func(user *UserInfo, w http.ResponseWriter, r *http.Request) {
			t.Fatal("onSuccess should not be called")
		},
		func(err error, w http.ResponseWriter, r *http.Request) {
			gotErr = err
			w.WriteHeader(http.StatusBadRequest)
		},
	)

	req := httptest.NewRequest(http.MethodGet, "/callback", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if gotErr == nil {
		t.Fatal("onError was not called")
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCallbackHandler_ExchangeFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid API key"})
	}))
	defer srv.Close()

	client := New(Config{BaseURL: srv.URL, ClientID: "app", APIKey: "bad"})

	var gotErr error
	handler := CallbackHandler(client,
		func(user *UserInfo, w http.ResponseWriter, r *http.Request) {
			t.Fatal("onSuccess should not be called")
		},
		func(err error, w http.ResponseWriter, r *http.Request) {
			gotErr = err
			w.WriteHeader(http.StatusUnauthorized)
		},
	)

	req := httptest.NewRequest(http.MethodGet, "/callback?code=some-code", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if gotErr == nil {
		t.Fatal("onError was not called")
	}
	var unauthErr *UnauthorizedError
	if !errors.As(gotErr, &unauthErr) {
		t.Fatalf("expected UnauthorizedError, got %T: %v", gotErr, gotErr)
	}
}

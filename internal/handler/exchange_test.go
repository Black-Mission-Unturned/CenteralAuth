package handler

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/BlackMission/centralauth/internal/client"
	"github.com/BlackMission/centralauth/internal/domain"
	"github.com/BlackMission/centralauth/internal/exchange"
	"github.com/BlackMission/centralauth/pkg/testutil"
)

func setupExchange() (http.Handler, *exchange.Codec) {
	clients, _ := client.NewRegistry([]domain.ClientApp{
		{
			ID:     "website",
			Name:   "Website",
			APIKey: "web-api-key-secret",
		},
		{
			ID:     "admin",
			Name:   "Admin",
			APIKey: "admin-api-key-secret",
		},
	})

	codec, _ := exchange.NewCodec([]byte("01234567890123456789012345678901"))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /exchange", Exchange(clients, codec))
	return mux, codec
}

func TestExchange_ValidCodeAndKey(t *testing.T) {
	handler, codec := setupExchange()

	code, _ := codec.Encode(domain.ExchangePayload{
		ClientID: "website",
		User: domain.UserInfo{
			ProviderName: "discord",
			ProviderID:   "123",
			Username:     "testuser",
			DisplayName:  "Test User",
			AvatarURL:    "https://cdn.example.com/avatar.png",
			Email:        "test@example.com",
		},
	})

	rr := testutil.DoRequest(t, handler, http.MethodGet,
		"/exchange?code="+url.QueryEscape(code),
		map[string]string{"Authorization": "Bearer web-api-key-secret"})

	testutil.AssertStatus(t, rr, http.StatusOK)

	var result domain.AuthResult
	testutil.ParseJSON(t, rr, &result)

	if result.User.ProviderName != "discord" {
		t.Errorf("expected provider 'discord', got %q", result.User.ProviderName)
	}
	if result.User.Username != "testuser" {
		t.Errorf("expected username 'testuser', got %q", result.User.Username)
	}
}

func TestExchange_MissingCode(t *testing.T) {
	handler, _ := setupExchange()
	rr := testutil.DoRequest(t, handler, http.MethodGet, "/exchange",
		map[string]string{"Authorization": "Bearer web-api-key-secret"})
	testutil.AssertStatus(t, rr, http.StatusBadRequest)
}

func TestExchange_ExpiredCode(t *testing.T) {
	clients, _ := client.NewRegistry([]domain.ClientApp{
		{ID: "website", Name: "Website", APIKey: "web-api-key-secret"},
	})
	codec, _ := exchange.NewCodec([]byte("01234567890123456789012345678901"))

	// Encode at current time
	now := time.Now()
	codec.SetNow(func() time.Time { return now })

	code, _ := codec.Encode(domain.ExchangePayload{
		ClientID: "website",
		User:     domain.UserInfo{ProviderName: "discord", ProviderID: "123"},
	})

	// Advance time past 30s expiry
	codec.SetNow(func() time.Time { return now.Add(31 * time.Second) })

	mux := http.NewServeMux()
	mux.HandleFunc("GET /exchange", Exchange(clients, codec))

	rr := testutil.DoRequest(t, mux, http.MethodGet,
		"/exchange?code="+url.QueryEscape(code),
		map[string]string{"Authorization": "Bearer web-api-key-secret"})
	testutil.AssertStatus(t, rr, http.StatusBadRequest)
}

func TestExchange_InvalidAPIKey(t *testing.T) {
	handler, codec := setupExchange()

	code, _ := codec.Encode(domain.ExchangePayload{
		ClientID: "website",
		User:     domain.UserInfo{ProviderName: "discord", ProviderID: "123"},
	})

	rr := testutil.DoRequest(t, handler, http.MethodGet,
		"/exchange?code="+url.QueryEscape(code),
		map[string]string{"Authorization": "Bearer wrong-api-key"})
	testutil.AssertStatus(t, rr, http.StatusUnauthorized)
}

func TestExchange_ClientMismatch(t *testing.T) {
	handler, codec := setupExchange()

	// Code was created for "website" client
	code, _ := codec.Encode(domain.ExchangePayload{
		ClientID: "website",
		User:     domain.UserInfo{ProviderName: "discord", ProviderID: "123"},
	})

	// But exchanged with admin's API key
	rr := testutil.DoRequest(t, handler, http.MethodGet,
		"/exchange?code="+url.QueryEscape(code),
		map[string]string{"Authorization": "Bearer admin-api-key-secret"})
	testutil.AssertStatus(t, rr, http.StatusForbidden)
}

func TestExchange_MissingAuthHeader(t *testing.T) {
	handler, codec := setupExchange()

	code, _ := codec.Encode(domain.ExchangePayload{
		ClientID: "website",
		User:     domain.UserInfo{ProviderName: "discord", ProviderID: "123"},
	})

	rr := testutil.DoRequest(t, handler, http.MethodGet,
		"/exchange?code="+url.QueryEscape(code), nil)
	testutil.AssertStatus(t, rr, http.StatusUnauthorized)
}

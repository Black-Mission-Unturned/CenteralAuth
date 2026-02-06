package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/BlackMission/centralauth/internal/auth"
	"github.com/BlackMission/centralauth/internal/client"
	"github.com/BlackMission/centralauth/internal/domain"
	"github.com/BlackMission/centralauth/internal/state"
	"github.com/BlackMission/centralauth/pkg/testutil"
)

type stubProvider struct {
	name    string
	authURL string
}

func (s *stubProvider) Name() string { return s.name }
func (s *stubProvider) AuthURL(stateToken string) (string, error) {
	return s.authURL + "?state=" + stateToken, nil
}
func (s *stubProvider) Exchange(ctx context.Context, params map[string]string) (*domain.AuthResult, error) {
	return nil, nil
}

func setupAuthorize() (http.Handler, *client.Registry, *auth.Registry, *state.Service) {
	clients, _ := client.NewRegistry([]domain.ClientApp{
		{
			ID:               "website",
			Name:             "Website",
			APIKey:           "web-key",
			AllowedCallbacks: []string{"https://example.com/callback"},
			AllowedProviders: []string{"discord"},
		},
	})
	providers := auth.NewRegistry()
	providers.Register(&stubProvider{name: "discord", authURL: "https://discord.com/oauth2/authorize"})

	stateSvc := state.NewService([]byte("test-key-1234567890abcdef"))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /auth/{provider}", Authorize(clients, providers, stateSvc))
	return mux, clients, providers, stateSvc
}

func TestAuthorize_ValidRequest(t *testing.T) {
	handler, _, _, _ := setupAuthorize()
	rr := testutil.DoRequest(t, handler, http.MethodGet,
		"/auth/discord?client_id=website&redirect_uri=https://example.com/callback", nil)

	testutil.AssertStatus(t, rr, http.StatusFound)
	loc := rr.Header().Get("Location")
	if loc == "" {
		t.Fatal("expected Location header")
	}
}

func TestAuthorize_MissingClientID(t *testing.T) {
	handler, _, _, _ := setupAuthorize()
	rr := testutil.DoRequest(t, handler, http.MethodGet,
		"/auth/discord?redirect_uri=https://example.com/callback", nil)
	testutil.AssertStatus(t, rr, http.StatusBadRequest)
}

func TestAuthorize_UnknownClient(t *testing.T) {
	handler, _, _, _ := setupAuthorize()
	rr := testutil.DoRequest(t, handler, http.MethodGet,
		"/auth/discord?client_id=unknown&redirect_uri=https://example.com/callback", nil)
	testutil.AssertStatus(t, rr, http.StatusBadRequest)
}

func TestAuthorize_DisallowedRedirectURI(t *testing.T) {
	handler, _, _, _ := setupAuthorize()
	rr := testutil.DoRequest(t, handler, http.MethodGet,
		"/auth/discord?client_id=website&redirect_uri=https://evil.com/callback", nil)
	testutil.AssertStatus(t, rr, http.StatusBadRequest)
}

func TestAuthorize_UnknownProvider(t *testing.T) {
	handler, _, _, _ := setupAuthorize()
	rr := testutil.DoRequest(t, handler, http.MethodGet,
		"/auth/github?client_id=website&redirect_uri=https://example.com/callback", nil)
	testutil.AssertStatus(t, rr, http.StatusBadRequest)
}

func TestAuthorize_ProviderNotAllowedForClient(t *testing.T) {
	clients, _ := client.NewRegistry([]domain.ClientApp{
		{
			ID:               "website",
			Name:             "Website",
			APIKey:           "web-key",
			AllowedCallbacks: []string{"https://example.com/callback"},
			AllowedProviders: []string{"steam"}, // only steam allowed
		},
	})
	providers := auth.NewRegistry()
	providers.Register(&stubProvider{name: "discord", authURL: "https://discord.com/oauth2/authorize"})
	providers.Register(&stubProvider{name: "steam", authURL: "https://steamcommunity.com/openid/login"})
	stateSvc := state.NewService([]byte("test-key-1234567890abcdef"))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /auth/{provider}", Authorize(clients, providers, stateSvc))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet,
		"/auth/discord?client_id=website&redirect_uri=https://example.com/callback", nil)
	mux.ServeHTTP(rr, req)

	testutil.AssertStatus(t, rr, http.StatusForbidden)
}

package handler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/BlackMission/centralauth/internal/auth"
	"github.com/BlackMission/centralauth/internal/domain"
	"github.com/BlackMission/centralauth/internal/exchange"
	"github.com/BlackMission/centralauth/internal/state"
	"github.com/BlackMission/centralauth/pkg/testutil"
)

type callbackStubProvider struct {
	name   string
	result *domain.AuthResult
	err    error
}

func (s *callbackStubProvider) Name() string { return s.name }
func (s *callbackStubProvider) AuthURL(stateToken string) (string, error) {
	return "", nil
}
func (s *callbackStubProvider) Exchange(ctx context.Context, params map[string]string) (*domain.AuthResult, error) {
	return s.result, s.err
}

func setupCallback(provider *callbackStubProvider) (http.Handler, *state.Service, *exchange.Codec) {
	providers := auth.NewRegistry()
	providers.Register(provider)

	stateSvc := state.NewService([]byte("test-key-1234567890abcdef"))
	codec, _ := exchange.NewCodec([]byte("01234567890123456789012345678901"))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /callback/{provider}", Callback(providers, stateSvc, codec))
	return mux, stateSvc, codec
}

func TestCallback_ValidFlow(t *testing.T) {
	provider := &callbackStubProvider{
		name: "discord",
		result: &domain.AuthResult{
			User: domain.UserInfo{
				ProviderName: "discord",
				ProviderID:   "123",
				Username:     "testuser",
				DisplayName:  "Test User",
			},
		},
	}

	handler, stateSvc, codec := setupCallback(provider)

	// Generate a valid state token
	stateToken, err := stateSvc.Generate(domain.StatePayload{
		ClientID:    "website",
		Provider:    "discord",
		RedirectURI: "https://example.com/callback",
	})
	if err != nil {
		t.Fatalf("Generate state error: %v", err)
	}

	rr := testutil.DoRequest(t, handler, http.MethodGet,
		fmt.Sprintf("/callback/discord?code=auth-code&state=%s", url.QueryEscape(stateToken)), nil)

	testutil.AssertStatus(t, rr, http.StatusFound)
	loc := rr.Header().Get("Location")
	if loc == "" {
		t.Fatal("expected Location header")
	}

	locURL, err := url.Parse(loc)
	if err != nil {
		t.Fatalf("parse Location: %v", err)
	}

	exchangeCode := locURL.Query().Get("code")
	if exchangeCode == "" {
		t.Fatal("expected code in redirect URL")
	}

	// Verify the exchange code is valid
	payload, err := codec.Decode(exchangeCode)
	if err != nil {
		t.Fatalf("Decode exchange code error: %v", err)
	}
	if payload.ClientID != "website" {
		t.Errorf("expected ClientID 'website', got %q", payload.ClientID)
	}
	if payload.User.Username != "testuser" {
		t.Errorf("expected username 'testuser', got %q", payload.User.Username)
	}
}

func TestCallback_InvalidState(t *testing.T) {
	provider := &callbackStubProvider{name: "discord"}
	handler, _, _ := setupCallback(provider)

	rr := testutil.DoRequest(t, handler, http.MethodGet,
		"/callback/discord?code=auth-code&state=invalid-state", nil)
	testutil.AssertStatus(t, rr, http.StatusBadRequest)
}

func TestCallback_ExpiredState(t *testing.T) {
	provider := &callbackStubProvider{
		name:   "discord",
		result: &domain.AuthResult{},
	}
	providers := auth.NewRegistry()
	providers.Register(provider)

	stateSvc := state.NewService([]byte("test-key-1234567890abcdef"))
	codec, _ := exchange.NewCodec([]byte("01234567890123456789012345678901"))

	// Generate state at "current" time
	now := time.Now()
	stateSvc.SetNow(func() time.Time { return now })
	stateToken, _ := stateSvc.Generate(domain.StatePayload{
		ClientID:    "website",
		Provider:    "discord",
		RedirectURI: "https://example.com/callback",
	})

	// Advance time past expiry for validation
	stateSvc.SetNow(func() time.Time { return now.Add(6 * time.Minute) })

	mux := http.NewServeMux()
	mux.HandleFunc("GET /callback/{provider}", Callback(providers, stateSvc, codec))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet,
		fmt.Sprintf("/callback/discord?code=auth-code&state=%s", url.QueryEscape(stateToken)), nil)
	mux.ServeHTTP(rr, req)

	testutil.AssertStatus(t, rr, http.StatusBadRequest)
}

func TestCallback_ProviderExchangeFailure(t *testing.T) {
	provider := &callbackStubProvider{
		name: "discord",
		err:  domain.ErrProviderExchange,
	}
	handler, stateSvc, _ := setupCallback(provider)

	stateToken, _ := stateSvc.Generate(domain.StatePayload{
		ClientID:    "website",
		Provider:    "discord",
		RedirectURI: "https://example.com/callback",
	})

	rr := testutil.DoRequest(t, handler, http.MethodGet,
		fmt.Sprintf("/callback/discord?code=auth-code&state=%s", url.QueryEscape(stateToken)), nil)
	testutil.AssertStatus(t, rr, http.StatusBadGateway)
}

func TestCallback_MissingState(t *testing.T) {
	provider := &callbackStubProvider{name: "discord"}
	handler, _, _ := setupCallback(provider)

	rr := testutil.DoRequest(t, handler, http.MethodGet,
		"/callback/discord?code=auth-code", nil)
	testutil.AssertStatus(t, rr, http.StatusBadRequest)
}

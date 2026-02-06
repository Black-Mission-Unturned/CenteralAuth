package steam

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/BlackMission/centralauth/internal/domain"
)

func setupTestProvider(openidHandler, summaryHandler http.HandlerFunc) *Provider {
	mux := http.NewServeMux()
	if openidHandler != nil {
		mux.HandleFunc("/openid/login", openidHandler)
	}
	if summaryHandler != nil {
		mux.HandleFunc("/ISteamUser/GetPlayerSummaries/v2/", summaryHandler)
	}
	server := httptest.NewServer(mux)

	p := New(Config{
		APIKey:      "test-steam-api-key",
		Realm:       "https://auth.example.com",
		CallbackURL: "https://auth.example.com/callback/steam",
	})
	p.httpClient = server.Client()
	p.openIDEndpoint = server.URL + "/openid/login"
	p.playerSummaryURL = server.URL + "/ISteamUser/GetPlayerSummaries/v2/"
	return p
}

func TestAuthURL_ContainsOpenIDParams(t *testing.T) {
	p := New(Config{
		Realm:       "https://auth.example.com",
		CallbackURL: "https://auth.example.com/callback/steam",
	})

	authURL, err := p.AuthURL("test-state-token")
	if err != nil {
		t.Fatalf("AuthURL error: %v", err)
	}

	u, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("parse URL error: %v", err)
	}

	if got := u.Query().Get("openid.ns"); got != "http://specs.openid.net/auth/2.0" {
		t.Errorf("unexpected openid.ns: %q", got)
	}
	if got := u.Query().Get("openid.mode"); got != "checkid_setup" {
		t.Errorf("unexpected openid.mode: %q", got)
	}
	if got := u.Query().Get("openid.realm"); got != "https://auth.example.com" {
		t.Errorf("unexpected openid.realm: %q", got)
	}
}

func TestAuthURL_StateInReturnTo(t *testing.T) {
	p := New(Config{
		Realm:       "https://auth.example.com",
		CallbackURL: "https://auth.example.com/callback/steam",
	})

	authURL, err := p.AuthURL("my-state-token")
	if err != nil {
		t.Fatalf("AuthURL error: %v", err)
	}

	u, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("parse URL error: %v", err)
	}

	returnTo := u.Query().Get("openid.return_to")
	returnToURL, err := url.Parse(returnTo)
	if err != nil {
		t.Fatalf("parse return_to error: %v", err)
	}

	if got := returnToURL.Query().Get("state"); got != "my-state-token" {
		t.Errorf("expected state in return_to, got %q", got)
	}
}

func TestExchange_Success(t *testing.T) {
	p := setupTestProvider(
		func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("ns:http://specs.openid.net/auth/2.0\nis_valid:true\n"))
		},
		func(w http.ResponseWriter, r *http.Request) {
			resp := playerSummaryResponse{}
			resp.Response.Players = []struct {
				SteamID     string `json:"steamid"`
				PersonaName string `json:"personaname"`
				AvatarFull  string `json:"avatarfull"`
				ProfileURL  string `json:"profileurl"`
				RealName    string `json:"realname"`
			}{
				{
					SteamID:     "76561198012345678",
					PersonaName: "GamerTag",
					AvatarFull:  "https://avatars.example.com/full.jpg",
				},
			}
			json.NewEncoder(w).Encode(resp)
		},
	)

	params := map[string]string{
		"openid.claimed_id": "https://steamcommunity.com/openid/id/76561198012345678",
		"openid.sig":        "test-sig",
		"openid.signed":     "test-signed",
	}

	result, err := p.Exchange(context.Background(), params)
	if err != nil {
		t.Fatalf("Exchange error: %v", err)
	}

	if result.User.ProviderName != "steam" {
		t.Errorf("expected provider 'steam', got %q", result.User.ProviderName)
	}
	if result.User.ProviderID != "76561198012345678" {
		t.Errorf("expected provider_id '76561198012345678', got %q", result.User.ProviderID)
	}
	if result.User.Username != "GamerTag" {
		t.Errorf("expected username 'GamerTag', got %q", result.User.Username)
	}
}

func TestExchange_AssertionFailure(t *testing.T) {
	p := setupTestProvider(
		func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("ns:http://specs.openid.net/auth/2.0\nis_valid:false\n"))
		},
		nil,
	)

	params := map[string]string{
		"openid.claimed_id": "https://steamcommunity.com/openid/id/76561198012345678",
	}

	_, err := p.Exchange(context.Background(), params)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, domain.ErrProviderExchange) {
		t.Errorf("expected ErrProviderExchange, got %v", err)
	}
}

func TestExtractSteamID(t *testing.T) {
	tests := []struct {
		claimedID string
		expected  string
		wantErr   bool
	}{
		{"https://steamcommunity.com/openid/id/76561198012345678", "76561198012345678", false},
		{"http://steamcommunity.com/openid/id/12345", "12345", false},
		{"https://evil.com/openid/id/12345", "", true},
		{"not-a-url", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.claimedID, func(t *testing.T) {
			got, err := extractSteamID(tt.claimedID)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestExchange_PlayerSummaryFailure(t *testing.T) {
	p := setupTestProvider(
		func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("ns:http://specs.openid.net/auth/2.0\nis_valid:true\n"))
		},
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("internal error"))
		},
	)

	params := map[string]string{
		"openid.claimed_id": "https://steamcommunity.com/openid/id/76561198012345678",
	}

	_, err := p.Exchange(context.Background(), params)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, domain.ErrProviderUserFetch) {
		t.Errorf("expected ErrProviderUserFetch, got %v", err)
	}
}

func TestExchange_MissingOpenIDParams(t *testing.T) {
	p := New(Config{})

	_, err := p.Exchange(context.Background(), map[string]string{})
	if !errors.Is(err, domain.ErrMissingProviderParams) {
		t.Errorf("expected ErrMissingProviderParams, got %v", err)
	}
}

func TestValidateAssertion_SendsCheckAuthentication(t *testing.T) {
	var receivedMode string
	p := setupTestProvider(
		func(w http.ResponseWriter, r *http.Request) {
			r.ParseForm()
			receivedMode = r.FormValue("openid.mode")
			w.Write([]byte("is_valid:true\n"))
		},
		func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(playerSummaryResponse{})
		},
	)

	// We need a valid player summary or it will fail after assertion
	p2 := setupTestProvider(
		func(w http.ResponseWriter, r *http.Request) {
			r.ParseForm()
			receivedMode = r.FormValue("openid.mode")
			w.Write([]byte("is_valid:true\n"))
		},
		func(w http.ResponseWriter, r *http.Request) {
			resp := playerSummaryResponse{}
			resp.Response.Players = []struct {
				SteamID     string `json:"steamid"`
				PersonaName string `json:"personaname"`
				AvatarFull  string `json:"avatarfull"`
				ProfileURL  string `json:"profileurl"`
				RealName    string `json:"realname"`
			}{{SteamID: "123", PersonaName: "test"}}
			json.NewEncoder(w).Encode(resp)
		},
	)
	_ = p // not used, p2 is the one with valid summary

	params := map[string]string{
		"openid.claimed_id": "https://steamcommunity.com/openid/id/123",
		"openid.mode":       "id_res",
	}

	p2.Exchange(context.Background(), params)

	if !strings.Contains(receivedMode, "check_authentication") {
		t.Errorf("expected check_authentication mode, got %q", receivedMode)
	}
}

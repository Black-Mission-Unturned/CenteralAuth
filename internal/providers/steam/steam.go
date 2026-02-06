package steam

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/BlackMission/centralauth/internal/domain"
)

const (
	providerName            = "steam"
	defaultOpenIDEndpoint   = "https://steamcommunity.com/openid/login"
	defaultPlayerSummaryURL = "https://api.steampowered.com/ISteamUser/GetPlayerSummaries/v2/"
)

var steamIDRegex = regexp.MustCompile(`https?://steamcommunity\.com/openid/id/(\d+)`)

// Config holds Steam OpenID settings.
type Config struct {
	APIKey      string
	Realm       string // e.g. https://auth.blackmission.com
	CallbackURL string // {base_url}/callback/steam
}

// Provider implements OpenID 2.0 for Steam.
type Provider struct {
	cfg              Config
	httpClient       *http.Client
	openIDEndpoint   string
	playerSummaryURL string
}

// New creates a Steam provider.
func New(cfg Config) *Provider {
	return &Provider{
		cfg:              cfg,
		httpClient:       http.DefaultClient,
		openIDEndpoint:   defaultOpenIDEndpoint,
		playerSummaryURL: defaultPlayerSummaryURL,
	}
}

func (p *Provider) Name() string { return providerName }

func (p *Provider) AuthURL(stateToken string) (string, error) {
	// Embed state token in return_to as a query param
	returnTo, err := url.Parse(p.cfg.CallbackURL)
	if err != nil {
		return "", fmt.Errorf("parsing callback URL: %w", err)
	}
	q := returnTo.Query()
	q.Set("state", stateToken)
	returnTo.RawQuery = q.Encode()

	params := url.Values{
		"openid.ns":         {"http://specs.openid.net/auth/2.0"},
		"openid.mode":       {"checkid_setup"},
		"openid.return_to":  {returnTo.String()},
		"openid.realm":      {p.cfg.Realm},
		"openid.identity":   {"http://specs.openid.net/auth/2.0/identifier_select"},
		"openid.claimed_id": {"http://specs.openid.net/auth/2.0/identifier_select"},
	}
	return p.openIDEndpoint + "?" + params.Encode(), nil
}

func (p *Provider) Exchange(ctx context.Context, params map[string]string) (*domain.AuthResult, error) {
	claimedID, ok := params["openid.claimed_id"]
	if !ok || claimedID == "" {
		return nil, domain.ErrMissingProviderParams
	}

	if err := p.validateAssertion(ctx, params); err != nil {
		return nil, err
	}

	steamID, err := extractSteamID(claimedID)
	if err != nil {
		return nil, err
	}

	user, err := p.fetchPlayerSummary(ctx, steamID)
	if err != nil {
		return nil, err
	}

	return &domain.AuthResult{User: *user}, nil
}

func (p *Provider) validateAssertion(ctx context.Context, params map[string]string) error {
	// Build verification request with check_authentication mode
	verifyParams := url.Values{}
	for k, v := range params {
		if strings.HasPrefix(k, "openid.") {
			verifyParams.Set(k, v)
		}
	}
	verifyParams.Set("openid.mode", "check_authentication")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.openIDEndpoint,
		strings.NewReader(verifyParams.Encode()))
	if err != nil {
		return fmt.Errorf("creating verify request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", domain.ErrProviderExchange, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("%w: reading response: %v", domain.ErrProviderExchange, err)
	}

	if !strings.Contains(string(body), "is_valid:true") {
		return fmt.Errorf("%w: assertion not valid", domain.ErrProviderExchange)
	}

	return nil
}

func extractSteamID(claimedID string) (string, error) {
	matches := steamIDRegex.FindStringSubmatch(claimedID)
	if len(matches) < 2 {
		return "", fmt.Errorf("%w: cannot extract Steam ID from %q", domain.ErrProviderExchange, claimedID)
	}
	return matches[1], nil
}

type playerSummaryResponse struct {
	Response struct {
		Players []struct {
			SteamID      string `json:"steamid"`
			PersonaName  string `json:"personaname"`
			AvatarFull   string `json:"avatarfull"`
			ProfileURL   string `json:"profileurl"`
			RealName     string `json:"realname"`
		} `json:"players"`
	} `json:"response"`
}

func (p *Provider) fetchPlayerSummary(ctx context.Context, steamID string) (*domain.UserInfo, error) {
	params := url.Values{
		"key":      {p.cfg.APIKey},
		"steamids": {steamID},
	}

	reqURL := p.playerSummaryURL + "?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating player summary request: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrProviderUserFetch, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: reading response: %v", domain.ErrProviderUserFetch, err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status %d: %s", domain.ErrProviderUserFetch, resp.StatusCode, body)
	}

	var summaryResp playerSummaryResponse
	if err := json.Unmarshal(body, &summaryResp); err != nil {
		return nil, fmt.Errorf("%w: invalid JSON: %v", domain.ErrProviderUserFetch, err)
	}

	if len(summaryResp.Response.Players) == 0 {
		return nil, fmt.Errorf("%w: no player data returned", domain.ErrProviderUserFetch)
	}

	player := summaryResp.Response.Players[0]
	return &domain.UserInfo{
		ProviderName: providerName,
		ProviderID:   player.SteamID,
		Username:     player.PersonaName,
		DisplayName:  player.PersonaName,
		AvatarURL:    player.AvatarFull,
	}, nil
}

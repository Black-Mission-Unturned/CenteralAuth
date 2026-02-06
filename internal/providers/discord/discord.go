package discord

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/BlackMission/centralauth/internal/domain"
)

const (
	providerName        = "discord"
	defaultAuthEndpoint = "https://discord.com/api/oauth2/authorize"
	defaultTokenURL     = "https://discord.com/api/oauth2/token"
	defaultUserURL      = "https://discord.com/api/users/@me"
)

// Config holds Discord OAuth2 settings.
type Config struct {
	ClientID     string
	ClientSecret string
	Scopes       []string
	CallbackURL  string // The CentralAuth callback URL: {base_url}/callback/discord
}

// Provider implements OAuth2 for Discord.
type Provider struct {
	cfg          Config
	httpClient   *http.Client
	authEndpoint string
	tokenURL     string
	userURL      string
}

// New creates a Discord provider.
func New(cfg Config) *Provider {
	return &Provider{
		cfg:          cfg,
		httpClient:   http.DefaultClient,
		authEndpoint: defaultAuthEndpoint,
		tokenURL:     defaultTokenURL,
		userURL:      defaultUserURL,
	}
}

func (p *Provider) Name() string { return providerName }

func (p *Provider) AuthURL(stateToken string) (string, error) {
	params := url.Values{
		"client_id":     {p.cfg.ClientID},
		"redirect_uri":  {p.cfg.CallbackURL},
		"response_type": {"code"},
		"scope":         {strings.Join(p.cfg.Scopes, " ")},
		"state":         {stateToken},
	}
	return p.authEndpoint + "?" + params.Encode(), nil
}

func (p *Provider) Exchange(ctx context.Context, params map[string]string) (*domain.AuthResult, error) {
	code, ok := params["code"]
	if !ok || code == "" {
		return nil, domain.ErrMissingProviderParams
	}

	token, err := p.exchangeCode(ctx, code)
	if err != nil {
		return nil, err
	}

	user, err := p.fetchUser(ctx, token)
	if err != nil {
		return nil, err
	}

	return &domain.AuthResult{User: *user}, nil
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}

func (p *Provider) exchangeCode(ctx context.Context, code string) (string, error) {
	data := url.Values{
		"client_id":     {p.cfg.ClientID},
		"client_secret": {p.cfg.ClientSecret},
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {p.cfg.CallbackURL},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("creating token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: %v", domain.ErrProviderExchange, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("%w: reading response: %v", domain.ErrProviderExchange, err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%w: status %d: %s", domain.ErrProviderExchange, resp.StatusCode, body)
	}

	var tokenResp tokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("%w: invalid JSON: %v", domain.ErrProviderExchange, err)
	}

	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("%w: empty access token", domain.ErrProviderExchange)
	}

	return tokenResp.AccessToken, nil
}

type discordUser struct {
	ID            string `json:"id"`
	Username      string `json:"username"`
	GlobalName    string `json:"global_name"`
	Avatar        string `json:"avatar"`
	Email         string `json:"email"`
	Discriminator string `json:"discriminator"`
}

func (p *Provider) fetchUser(ctx context.Context, accessToken string) (*domain.UserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.userURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating user request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

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

	var du discordUser
	if err := json.Unmarshal(body, &du); err != nil {
		return nil, fmt.Errorf("%w: invalid JSON: %v", domain.ErrProviderUserFetch, err)
	}

	avatarURL := ""
	if du.Avatar != "" {
		avatarURL = fmt.Sprintf("https://cdn.discordapp.com/avatars/%s/%s.png", du.ID, du.Avatar)
	}

	displayName := du.GlobalName
	if displayName == "" {
		displayName = du.Username
	}

	return &domain.UserInfo{
		ProviderName: providerName,
		ProviderID:   du.ID,
		Username:     du.Username,
		DisplayName:  displayName,
		AvatarURL:    avatarURL,
		Email:        du.Email,
	}, nil
}

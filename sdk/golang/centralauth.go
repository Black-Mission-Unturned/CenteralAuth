package centralauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultTimeout = 5 * time.Second

// Config holds the configuration for a CentralAuth client.
type Config struct {
	BaseURL  string
	ClientID string
	APIKey   string
	Timeout  time.Duration
}

// UserInfo represents the authenticated user returned by CentralAuth.
type UserInfo struct {
	Provider    string `json:"provider"`
	ProviderID  string `json:"provider_id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url"`
	Email       string `json:"email,omitempty"`
}

type exchangeResponse struct {
	User UserInfo `json:"user"`
}

type healthResponse struct {
	Status string `json:"status"`
}

type errorResponse struct {
	ErrorMsg string `json:"error"`
}

// Client is the CentralAuth SDK client.
type Client struct {
	baseURL  string
	clientID string
	apiKey   string
	http     *http.Client
}

// New creates a new CentralAuth client.
func New(cfg Config) *Client {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}
	return &Client{
		baseURL:  strings.TrimRight(cfg.BaseURL, "/"),
		clientID: cfg.ClientID,
		apiKey:   cfg.APIKey,
		http:     &http.Client{Timeout: timeout},
	}
}

// AuthorizeURL builds the authorization URL for redirecting a user to CentralAuth.
func (c *Client) AuthorizeURL(provider, redirectURI string) string {
	params := url.Values{}
	params.Set("client_id", c.clientID)
	params.Set("redirect_uri", redirectURI)
	return fmt.Sprintf("%s/auth/%s?%s", c.baseURL, url.PathEscape(provider), params.Encode())
}

// Exchange trades an authorization code for user info (server-to-server).
func (c *Client) Exchange(ctx context.Context, code string) (*UserInfo, error) {
	params := url.Values{}
	params.Set("code", code)
	reqURL := fmt.Sprintf("%s/exchange?%s", c.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, &Error{Message: fmt.Sprintf("failed to create request: %s", err.Error())}
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, &Error{Message: fmt.Sprintf("network error: %s", err.Error())}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleErrorResponse(resp)
	}

	var data exchangeResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, &Error{Message: fmt.Sprintf("failed to decode response: %s", err.Error())}
	}
	return &data.User, nil
}

// Providers returns the list of available authentication provider names.
func (c *Client) Providers(ctx context.Context) ([]string, error) {
	reqURL := fmt.Sprintf("%s/providers", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, &Error{Message: fmt.Sprintf("failed to create request: %s", err.Error())}
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, &Error{Message: fmt.Sprintf("network error: %s", err.Error())}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &Error{
			Message:    fmt.Sprintf("failed to fetch providers: %d", resp.StatusCode),
			StatusCode: resp.StatusCode,
		}
	}

	var providers []string
	if err := json.NewDecoder(resp.Body).Decode(&providers); err != nil {
		return nil, &Error{Message: fmt.Sprintf("failed to decode response: %s", err.Error())}
	}
	return providers, nil
}

// HealthCheck returns true if the CentralAuth server is healthy.
func (c *Client) HealthCheck(ctx context.Context) bool {
	reqURL := fmt.Sprintf("%s/health", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return false
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false
	}

	var data healthResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return false
	}
	return data.Status == "ok"
}

func (c *Client) handleErrorResponse(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)

	var errResp errorResponse
	message := http.StatusText(resp.StatusCode)
	if json.Unmarshal(body, &errResp) == nil && errResp.ErrorMsg != "" {
		message = errResp.ErrorMsg
	}

	switch resp.StatusCode {
	case http.StatusBadRequest:
		if strings.Contains(strings.ToLower(message), "expired") {
			return newExchangeExpiredError(message)
		}
		return &Error{Message: message, StatusCode: 400}
	case http.StatusUnauthorized:
		return newUnauthorizedError(message)
	case http.StatusForbidden:
		return newForbiddenError(message)
	case http.StatusBadGateway:
		return newProviderError(message)
	default:
		return &Error{Message: message, StatusCode: resp.StatusCode}
	}
}

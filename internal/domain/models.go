package domain

import "time"

// UserInfo represents the normalized user profile returned by any provider.
type UserInfo struct {
	ProviderName string `json:"provider"`
	ProviderID   string `json:"provider_id"`
	Username     string `json:"username"`
	DisplayName  string `json:"display_name"`
	AvatarURL    string `json:"avatar_url"`
	Email        string `json:"email,omitempty"`
}

// AuthResult is the result of a successful provider authentication.
type AuthResult struct {
	User UserInfo `json:"user"`
}

// StatePayload is the data embedded in the HMAC-signed OAuth state token.
type StatePayload struct {
	ClientID    string    `json:"cid"`
	Provider    string    `json:"prv"`
	RedirectURI string    `json:"rdr"`
	Nonce       string    `json:"nce"`
	ExpiresAt   time.Time `json:"exp"`
}

// ExchangePayload is the data encrypted inside an exchange code (AES-GCM).
type ExchangePayload struct {
	ClientID  string    `json:"cid"`
	ExpiresAt time.Time `json:"exp"`
	User      UserInfo  `json:"user"`
}

// ClientApp represents a registered client application.
type ClientApp struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	APIKey           string   `json:"-"`
	AllowedCallbacks []string `json:"allowed_callbacks"`
	AllowedProviders []string `json:"allowed_providers"`
}

package client

import (
	"crypto/hmac"
	"fmt"

	"github.com/BlackMission/centralauth/internal/domain"
)

// Registry holds registered client apps and provides lookup/validation.
type Registry struct {
	byID     map[string]*domain.ClientApp
	byAPIKey map[string]*domain.ClientApp
}

// NewRegistry creates a client registry from the given client app list.
func NewRegistry(clients []domain.ClientApp) (*Registry, error) {
	r := &Registry{
		byID:     make(map[string]*domain.ClientApp, len(clients)),
		byAPIKey: make(map[string]*domain.ClientApp, len(clients)),
	}
	for i := range clients {
		c := &clients[i]
		if _, exists := r.byID[c.ID]; exists {
			return nil, fmt.Errorf("%w: %s", domain.ErrDuplicateClientID, c.ID)
		}
		r.byID[c.ID] = c
		r.byAPIKey[c.APIKey] = c
	}
	return r, nil
}

// Get returns a client app by its ID.
func (r *Registry) Get(clientID string) (*domain.ClientApp, error) {
	c, ok := r.byID[clientID]
	if !ok {
		return nil, domain.ErrClientNotFound
	}
	return c, nil
}

// GetByAPIKey returns a client app by its API key using constant-time comparison.
func (r *Registry) GetByAPIKey(apiKey string) (*domain.ClientApp, error) {
	for key, c := range r.byAPIKey {
		if hmac.Equal([]byte(key), []byte(apiKey)) {
			return c, nil
		}
	}
	return nil, domain.ErrInvalidAPIKey
}

// ValidateCallback checks if the given callback URI is allowed for the client.
func (r *Registry) ValidateCallback(clientID, callbackURI string) error {
	c, err := r.Get(clientID)
	if err != nil {
		return err
	}
	for _, allowed := range c.AllowedCallbacks {
		if allowed == callbackURI {
			return nil
		}
	}
	return domain.ErrCallbackNotAllowed
}

// ValidateProvider checks if the given provider is allowed for the client.
func (r *Registry) ValidateProvider(clientID, provider string) error {
	c, err := r.Get(clientID)
	if err != nil {
		return err
	}
	for _, allowed := range c.AllowedProviders {
		if allowed == provider {
			return nil
		}
	}
	return domain.ErrProviderNotAllowed
}

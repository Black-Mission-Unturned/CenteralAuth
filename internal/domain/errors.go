package domain

import "errors"

var (
	// Client errors
	ErrClientNotFound     = errors.New("client not found")
	ErrInvalidAPIKey      = errors.New("invalid API key")
	ErrCallbackNotAllowed = errors.New("callback URI not allowed")
	ErrProviderNotAllowed = errors.New("provider not allowed for this client")
	ErrDuplicateClientID  = errors.New("duplicate client ID")

	// Provider errors
	ErrProviderNotFound      = errors.New("provider not found")
	ErrDuplicateProvider     = errors.New("duplicate provider registration")
	ErrProviderExchange      = errors.New("provider exchange failed")
	ErrProviderUserFetch     = errors.New("failed to fetch user from provider")
	ErrMissingProviderParams = errors.New("missing required provider parameters")

	// State token errors
	ErrInvalidState  = errors.New("invalid state token")
	ErrExpiredState  = errors.New("expired state token")
	ErrMalformedState = errors.New("malformed state token")

	// Exchange code errors
	ErrInvalidExchangeCode = errors.New("invalid exchange code")
	ErrExpiredExchangeCode = errors.New("expired exchange code")
	ErrClientMismatch      = errors.New("API key does not match client in exchange code")

	// Config errors
	ErrMissingConfig = errors.New("missing required configuration")
	ErrInvalidConfig = errors.New("invalid configuration")
)

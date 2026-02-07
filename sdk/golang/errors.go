package centralauth

import "fmt"

// Error is the base error type for all CentralAuth SDK errors.
type Error struct {
	Message    string
	StatusCode int
}

func (e *Error) Error() string {
	if e.StatusCode != 0 {
		return fmt.Sprintf("centralauth: %s (status %d)", e.Message, e.StatusCode)
	}
	return fmt.Sprintf("centralauth: %s", e.Message)
}

// UnauthorizedError indicates an invalid or missing API key (HTTP 401).
type UnauthorizedError struct {
	Message    string
	StatusCode int
}

func (e *UnauthorizedError) Error() string {
	return fmt.Sprintf("centralauth: %s (status %d)", e.Message, e.StatusCode)
}

func newUnauthorizedError(message string) *UnauthorizedError {
	if message == "" {
		message = "Invalid API key"
	}
	return &UnauthorizedError{Message: message, StatusCode: 401}
}

// ForbiddenError indicates the API key doesn't match the initiating client (HTTP 403).
type ForbiddenError struct {
	Message    string
	StatusCode int
}

func (e *ForbiddenError) Error() string {
	return fmt.Sprintf("centralauth: %s (status %d)", e.Message, e.StatusCode)
}

func newForbiddenError(message string) *ForbiddenError {
	if message == "" {
		message = "Client mismatch"
	}
	return &ForbiddenError{Message: message, StatusCode: 403}
}

// ExchangeExpiredError indicates the exchange code has expired (HTTP 400).
type ExchangeExpiredError struct {
	Message    string
	StatusCode int
}

func (e *ExchangeExpiredError) Error() string {
	return fmt.Sprintf("centralauth: %s (status %d)", e.Message, e.StatusCode)
}

func newExchangeExpiredError(message string) *ExchangeExpiredError {
	if message == "" {
		message = "Exchange code expired"
	}
	return &ExchangeExpiredError{Message: message, StatusCode: 400}
}

// ProviderError indicates the upstream provider exchange failed (HTTP 502).
type ProviderError struct {
	Message    string
	StatusCode int
}

func (e *ProviderError) Error() string {
	return fmt.Sprintf("centralauth: %s (status %d)", e.Message, e.StatusCode)
}

func newProviderError(message string) *ProviderError {
	if message == "" {
		message = "Provider exchange failed"
	}
	return &ProviderError{Message: message, StatusCode: 502}
}

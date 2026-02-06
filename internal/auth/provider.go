package auth

import (
	"context"

	"github.com/BlackMission/centralauth/internal/domain"
)

// Provider defines the interface for an OAuth/OpenID provider.
type Provider interface {
	Name() string
	AuthURL(stateToken string) (string, error)
	Exchange(ctx context.Context, params map[string]string) (*domain.AuthResult, error)
}

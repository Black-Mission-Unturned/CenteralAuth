package handler

import (
	"errors"
	"net/http"

	"github.com/BlackMission/centralauth/internal/auth"
	"github.com/BlackMission/centralauth/internal/client"
	"github.com/BlackMission/centralauth/internal/domain"
	"github.com/BlackMission/centralauth/internal/state"
)

// Authorize handles GET /auth/{provider}.
// It validates the client, redirect_uri, and provider, then redirects to the provider's auth URL.
func Authorize(clients *client.Registry, providers *auth.Registry, stateService *state.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		clientID := r.URL.Query().Get("client_id")
		if clientID == "" {
			writeError(w, http.StatusBadRequest, "missing client_id parameter")
			return
		}

		redirectURI := r.URL.Query().Get("redirect_uri")
		if redirectURI == "" {
			writeError(w, http.StatusBadRequest, "missing redirect_uri parameter")
			return
		}

		providerName := r.PathValue("provider")
		if providerName == "" {
			writeError(w, http.StatusBadRequest, "missing provider")
			return
		}

		// Validate client exists
		if _, err := clients.Get(clientID); err != nil {
			writeError(w, http.StatusBadRequest, "unknown client")
			return
		}

		// Validate redirect_uri is allowed
		if err := clients.ValidateCallback(clientID, redirectURI); err != nil {
			writeError(w, http.StatusBadRequest, "redirect_uri not allowed")
			return
		}

		// Validate provider exists
		provider, err := providers.Get(providerName)
		if err != nil {
			writeError(w, http.StatusBadRequest, "unknown provider")
			return
		}

		// Validate provider is allowed for this client
		if err := clients.ValidateProvider(clientID, providerName); err != nil {
			if errors.Is(err, domain.ErrProviderNotAllowed) {
				writeError(w, http.StatusForbidden, "provider not allowed for this client")
				return
			}
			writeError(w, http.StatusBadRequest, "unknown client")
			return
		}

		// Generate state token
		stateToken, err := stateService.Generate(domain.StatePayload{
			ClientID:    clientID,
			Provider:    providerName,
			RedirectURI: redirectURI,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to generate state token")
			return
		}

		// Get provider auth URL
		authURL, err := provider.AuthURL(stateToken)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to generate auth URL")
			return
		}

		http.Redirect(w, r, authURL, http.StatusFound)
	}
}

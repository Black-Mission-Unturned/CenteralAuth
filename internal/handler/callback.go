package handler

import (
	"errors"
	"net/http"
	"net/url"

	"github.com/BlackMission/centralauth/internal/auth"
	"github.com/BlackMission/centralauth/internal/domain"
	"github.com/BlackMission/centralauth/internal/exchange"
	"github.com/BlackMission/centralauth/internal/state"
)

// Callback handles GET /callback/{provider}.
// It validates the state token, exchanges the code with the provider, encrypts the result,
// and redirects back to the client with an exchange code.
func Callback(providers *auth.Registry, stateService *state.Service, codec *exchange.Codec) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		providerName := r.PathValue("provider")

		// Get state token - could be in query param (Discord) or embedded in return_to (Steam)
		stateToken := r.URL.Query().Get("state")
		if stateToken == "" {
			writeError(w, http.StatusBadRequest, "missing state parameter")
			return
		}

		// Validate state token
		statePayload, err := stateService.Validate(stateToken)
		if err != nil {
			if errors.Is(err, domain.ErrExpiredState) {
				writeError(w, http.StatusBadRequest, "state token expired")
				return
			}
			writeError(w, http.StatusBadRequest, "invalid state token")
			return
		}

		// Get provider
		provider, err := providers.Get(providerName)
		if err != nil {
			writeError(w, http.StatusBadRequest, "unknown provider")
			return
		}

		// Collect all query params for provider exchange
		params := make(map[string]string)
		for key, values := range r.URL.Query() {
			if len(values) > 0 {
				params[key] = values[0]
			}
		}

		// Exchange with provider
		result, err := provider.Exchange(r.Context(), params)
		if err != nil {
			if errors.Is(err, domain.ErrMissingProviderParams) {
				writeError(w, http.StatusBadRequest, "missing provider parameters")
				return
			}
			writeError(w, http.StatusBadGateway, "provider exchange failed")
			return
		}

		// Encrypt auth result as exchange code
		code, err := codec.Encode(domain.ExchangePayload{
			ClientID: statePayload.ClientID,
			User:     result.User,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create exchange code")
			return
		}

		// Redirect back to client with exchange code
		redirectURL, err := url.Parse(statePayload.RedirectURI)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "invalid redirect URI")
			return
		}
		q := redirectURL.Query()
		q.Set("code", code)
		redirectURL.RawQuery = q.Encode()

		http.Redirect(w, r, redirectURL.String(), http.StatusFound)
	}
}

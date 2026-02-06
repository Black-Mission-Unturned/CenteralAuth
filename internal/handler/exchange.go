package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/BlackMission/centralauth/internal/client"
	"github.com/BlackMission/centralauth/internal/domain"
	"github.com/BlackMission/centralauth/internal/exchange"
)

// Exchange handles GET /exchange.
// It decrypts the exchange code, validates the API key, and returns the user info.
func Exchange(clients *client.Registry, codec *exchange.Codec) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			writeError(w, http.StatusBadRequest, "missing code parameter")
			return
		}

		// Extract API key from Authorization header
		authHeader := r.Header.Get("Authorization")
		apiKey := strings.TrimPrefix(authHeader, "Bearer ")
		if apiKey == "" || apiKey == authHeader {
			writeError(w, http.StatusUnauthorized, "missing or invalid Authorization header")
			return
		}

		// Decrypt the exchange code
		payload, err := codec.Decode(code)
		if err != nil {
			if errors.Is(err, domain.ErrExpiredExchangeCode) {
				writeError(w, http.StatusBadRequest, "exchange code expired")
				return
			}
			writeError(w, http.StatusBadRequest, "invalid exchange code")
			return
		}

		// Validate API key
		clientApp, err := clients.GetByAPIKey(apiKey)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid API key")
			return
		}

		// Verify the API key belongs to the client that initiated the flow
		if clientApp.ID != payload.ClientID {
			writeError(w, http.StatusForbidden, "API key does not match the client that initiated the auth flow")
			return
		}

		writeJSON(w, http.StatusOK, domain.AuthResult{User: payload.User})
	}
}

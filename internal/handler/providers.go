package handler

import (
	"net/http"
	"sort"

	"github.com/BlackMission/centralauth/internal/auth"
)

// Providers handles GET /providers.
func Providers(registry *auth.Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		names := registry.Names()
		sort.Strings(names)
		writeJSON(w, http.StatusOK, names)
	}
}

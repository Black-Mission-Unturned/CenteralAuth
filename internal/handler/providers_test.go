package handler

import (
	"net/http"
	"testing"

	"github.com/BlackMission/centralauth/internal/auth"
	"github.com/BlackMission/centralauth/pkg/testutil"
)

func TestProviders(t *testing.T) {
	registry := auth.NewRegistry()
	registry.Register(&stubProvider{name: "discord"})
	registry.Register(&stubProvider{name: "steam"})

	handler := Providers(registry)
	rr := testutil.DoRequest(t, handler, http.MethodGet, "/providers", nil)
	testutil.AssertStatus(t, rr, http.StatusOK)

	var names []string
	testutil.ParseJSON(t, rr, &names)

	if len(names) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(names))
	}
	// Sorted alphabetically
	if names[0] != "discord" || names[1] != "steam" {
		t.Errorf("expected [discord, steam], got %v", names)
	}
}

func TestProviders_Empty(t *testing.T) {
	registry := auth.NewRegistry()
	handler := Providers(registry)
	rr := testutil.DoRequest(t, handler, http.MethodGet, "/providers", nil)
	testutil.AssertStatus(t, rr, http.StatusOK)

	var names []string
	testutil.ParseJSON(t, rr, &names)
	if len(names) != 0 {
		t.Errorf("expected empty list, got %v", names)
	}
}

package handler

import (
	"net/http"
	"testing"

	"github.com/BlackMission/centralauth/pkg/testutil"
)

func TestHealth(t *testing.T) {
	rr := testutil.DoRequest(t, Health(), http.MethodGet, "/health", nil)
	testutil.AssertStatus(t, rr, http.StatusOK)

	var body map[string]string
	testutil.ParseJSON(t, rr, &body)

	if body["status"] != "ok" {
		t.Errorf("expected status 'ok', got %q", body["status"])
	}
}

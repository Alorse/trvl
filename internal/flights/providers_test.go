package flights

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MikkoParkkola/trvl/internal/batchexec"
	"github.com/MikkoParkkola/trvl/internal/models"
)

func TestProviderListed(t *testing.T) {
	opts := SearchOptions{Providers: []string{"duffel", "kiwi"}}
	if !providerListed(opts, "duffel") {
		t.Errorf("duffel should be listed")
	}
	if !providerListed(opts, "DUFFEL") {
		t.Errorf("matching should be case-insensitive")
	}
	if providerListed(opts, "google") {
		t.Errorf("google should not be listed")
	}
	if providerListed(SearchOptions{}, "google") {
		t.Errorf("empty allow-list must not report any provider as explicitly listed")
	}
}

// TestSearchFlights_ProviderDuffelOnly verifies that an explicit --provider
// duffel allow-list queries only Duffel (as primary, bypassing the
// Google-first gating) and returns Duffel-sourced results.
func TestSearchFlights_ProviderDuffelOnly(t *testing.T) {
	// duffelFixture is defined in duffel_test.go (same package).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(duffelFixture))
	}))
	defer srv.Close()

	t.Setenv("DUFFEL_API_KEYS", "")
	t.Setenv("DUFFEL_API_KEY", "test_key")
	restore := duffelSetEndpointForTest(srv.URL)
	defer restore()

	res, err := SearchFlightsWithClient(context.Background(), batchexec.NewClient(), "HAM", "FUK", "2026-09-15",
		SearchOptions{Adults: 1, CabinClass: models.Economy, Providers: []string{"duffel"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success || res.Count == 0 {
		t.Fatalf("expected duffel results, got success=%v count=%d err=%q", res.Success, res.Count, res.Error)
	}
	for _, f := range res.Flights {
		if f.Provider != "duffel" {
			t.Errorf("expected only duffel-sourced flights, got provider %q", f.Provider)
		}
	}
}

// TestSearchFlights_ProviderDuffelOnly_NoKeys verifies a clear error when duffel
// is the only requested provider but no keys are configured.
func TestSearchFlights_ProviderDuffelOnly_NoKeys(t *testing.T) {
	t.Setenv("DUFFEL_API_KEYS", "")
	t.Setenv("DUFFEL_API_KEY", "")

	res, err := SearchFlightsWithClient(context.Background(), batchexec.NewClient(), "HAM", "FUK", "2026-09-15",
		SearchOptions{Adults: 1, Providers: []string{"duffel"}})
	if err == nil {
		t.Fatalf("expected error when duffel requested with no keys, got success=%v", res.Success)
	}
}

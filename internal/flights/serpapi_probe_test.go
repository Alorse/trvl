package flights

import (
	"context"
	"os"
	"testing"
)

// TestSerpApiProbe hits the real serp-key router + SerpApi. Opt-in only.
func TestSerpApiProbe(t *testing.T) {
	if os.Getenv("TRVL_TEST_LIVE_PROBES") != "1" {
		t.Skip("set TRVL_TEST_LIVE_PROBES=1 to run live SerpApi probe")
	}
	if !SerpEnabled() {
		t.Skip("serp-key not available on PATH")
	}
	res, err := SearchSerpApi(context.Background(),
		[]DuffelSlice{{Origin: "FRA", Destination: "NRT", DepartureDate: "2026-08-15"}},
		SearchOptions{Currency: "EUR"})
	if err != nil {
		t.Fatalf("live SerpApi probe failed: %v", err)
	}
	if len(res) == 0 {
		t.Fatal("expected at least one flight result")
	}
	if res[0].Price <= 0 || res[0].Provider != "google_serpapi" {
		t.Fatalf("bad result: price=%v provider=%q", res[0].Price, res[0].Provider)
	}
	t.Logf("probe OK: %d results, cheapest %v %s", len(res), res[0].Price, res[0].Currency)
}

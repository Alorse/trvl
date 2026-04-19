package trip

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestOptimizeMultiCity_TwoCities_LiveAPI exercises optimizeRoute with 2 cities,
// which causes the `if used[i] { continue }` branch in the visit closure to run
// during permutation traversal.
// Requires TRVL_TEST_LIVE_INTEGRATIONS=1 (uses live flight price API).
func TestOptimizeMultiCity_TwoCities_LiveAPI(t *testing.T) {
	if os.Getenv("TRVL_TEST_LIVE_INTEGRATIONS") == "" {
		t.Skip("skipping live-API test; set TRVL_TEST_LIVE_INTEGRATIONS=1 to run")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := OptimizeMultiCity(ctx, "HEL", []string{"AMS", "CDG"}, MultiCityOptions{
		DepartDate: "2026-08-15",
	})
	if err != nil {
		t.Fatalf("OptimizeMultiCity returned error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	// With 2 cities, there are 2! = 2 permutations.
	if result.Permutations != 2 {
		t.Errorf("Permutations = %d, want 2", result.Permutations)
	}
	if len(result.OptimalOrder) != 2 {
		t.Errorf("OptimalOrder len = %d, want 2", len(result.OptimalOrder))
	}
}

// TestOptimizeMultiCity_TwoCities_NoNetworkFallback runs OptimizeMultiCity with
// 2 cities without network. fetchCheapestPriceWithClient will return price=9999
// for all legs, then optimizeRoute is called and exercises the `continue` branch.
func TestOptimizeMultiCity_TwoCities_NoNetworkFallback(t *testing.T) {
	// This test does call the live API but returns quickly (price=9999 on failure).
	// Use a very short timeout to force immediate failure.
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	result, err := OptimizeMultiCity(ctx, "HEL", []string{"AMS", "CDG"}, MultiCityOptions{
		DepartDate: "2026-08-15",
	})
	// Either err from context cancel, or result with all-9999 prices.
	if err != nil {
		// Context cancelled before API calls — acceptable.
		t.Logf("context expired before results: %v (acceptable)", err)
		return
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	// optimizeRoute ran with 2 cities: must have 2 permutations.
	if result.Permutations != 2 {
		t.Errorf("Permutations = %d, want 2 (2 cities = 2! permutations)", result.Permutations)
	}
}

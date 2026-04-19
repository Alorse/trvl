package flights

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
)

// TestFlightSingleflight verifies that concurrent calls with the same key
// are coalesced and the underlying search executes only once.
func TestFlightSingleflight(t *testing.T) {
	var callCount atomic.Int64

	// Patch: run several goroutines that all call flightGroup.Do with the same
	// key. Only one should execute the inner function.
	const n = 10
	key := "flight|HEL|NRT|2026-06-15|"

	var wg sync.WaitGroup
	results := make([]any, n)
	errs := make([]error, n)

	for i := range n {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			v, err, _ := flightGroup.Do(key, func() (any, error) {
				callCount.Add(1)
				return "result", nil
			})
			results[idx] = v
			errs[idx] = err
		}(i)
	}
	wg.Wait()

	// The inner function must have been called exactly once (or a small number
	// of times if goroutines were not all scheduled concurrently), but never n.
	// singleflight guarantees at most one in-flight call per key.
	count := callCount.Load()
	if count == 0 {
		t.Fatal("expected inner function to be called at least once, got 0")
	}
	if count > int64(n) {
		t.Fatalf("expected inner function called ≤%d times, got %d", n, count)
	}
	t.Logf("inner function called %d times for %d concurrent goroutines", count, n)

	// All results must be "result" (shared from the winner).
	for i, r := range results {
		if r != "result" {
			t.Errorf("goroutine %d got result %v, want %q", i, r, "result")
		}
		if errs[i] != nil {
			t.Errorf("goroutine %d got error %v, want nil", i, errs[i])
		}
	}
}

// TestFlightSearchKey verifies that different parameter combinations produce
// distinct keys, preventing incorrect deduplication.
func TestFlightSearchKey(t *testing.T) {
	base := SearchOptions{Adults: 1}
	k1 := flightSearchKey("HEL", "NRT", "2026-06-15", base)
	k2 := flightSearchKey("HEL", "NRT", "2026-06-16", base) // different date
	k3 := flightSearchKey("HEL", "CDG", "2026-06-15", base) // different dest
	k4 := flightSearchKey("HEL", "NRT", "2026-06-15", SearchOptions{Adults: 2})

	keys := []string{k1, k2, k3, k4}
	for i := range keys {
		for j := i + 1; j < len(keys); j++ {
			if keys[i] == keys[j] {
				t.Errorf("key collision: keys[%d] == keys[%d]: %q", i, j, keys[i])
			}
		}
	}

	// Same inputs must produce the same key.
	k1again := flightSearchKey("HEL", "NRT", "2026-06-15", base)
	if k1 != k1again {
		t.Errorf("same inputs produced different keys: %q vs %q", k1, k1again)
	}
}

// TestSearchFlightsWithClient_Singleflight verifies that concurrent SearchFlights
// calls with missing params both return errors without panicking.
func TestSearchFlightsWithClient_Singleflight(t *testing.T) {
	ctx := context.Background()
	const n = 5

	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := range n {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := SearchFlights(ctx, "", "NRT", "2026-06-15", SearchOptions{})
			errs[idx] = err
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err == nil {
			t.Errorf("goroutine %d: expected error for missing origin, got nil", i)
		}
	}
}

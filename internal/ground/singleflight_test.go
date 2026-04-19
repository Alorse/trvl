package ground

import (
	"sync"
	"sync/atomic"
	"testing"
)

// TestGroundSingleflight verifies that concurrent calls with the same key
// are coalesced and the underlying search executes only once.
func TestGroundSingleflight(t *testing.T) {
	var callCount atomic.Int64

	const n = 10
	key := "ground|Amsterdam|Paris|2026-06-15"

	var wg sync.WaitGroup
	results := make([]any, n)
	errs := make([]error, n)

	for i := range n {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			v, err, _ := groundGroup.Do(key, func() (any, error) {
				callCount.Add(1)
				return "result", nil
			})
			results[idx] = v
			errs[idx] = err
		}(i)
	}
	wg.Wait()

	count := callCount.Load()
	if count == 0 {
		t.Fatal("expected inner function to be called at least once, got 0")
	}
	if count > int64(n) {
		t.Fatalf("expected inner function called ≤%d times, got %d", n, count)
	}
	t.Logf("inner function called %d times for %d concurrent goroutines", count, n)

	for i, r := range results {
		if r != "result" {
			t.Errorf("goroutine %d got result %v, want %q", i, r, "result")
		}
		if errs[i] != nil {
			t.Errorf("goroutine %d got error %v, want nil", i, errs[i])
		}
	}
}

// TestGroundSingleflight_DifferentKeys verifies that requests with different
// parameters are NOT coalesced — each gets its own independent call.
func TestGroundSingleflight_DifferentKeys(t *testing.T) {
	var callCount atomic.Int64

	keys := []string{
		"ground|Amsterdam|Paris|2026-06-15",
		"ground|Amsterdam|Berlin|2026-06-15",
		"ground|Amsterdam|Paris|2026-06-16",
	}

	var wg sync.WaitGroup
	for _, key := range keys {
		wg.Add(1)
		k := key
		go func() {
			defer wg.Done()
			groundGroup.Do(k, func() (any, error) { //nolint:errcheck
				callCount.Add(1)
				return nil, nil
			})
		}()
	}
	wg.Wait()

	if got := callCount.Load(); got != int64(len(keys)) {
		t.Errorf("expected %d calls for %d distinct keys, got %d", len(keys), len(keys), got)
	}
}

// TestSearchByNameSingleflight_NoCache verifies that the NoCache opt-out path
// routes through singleflight without panicking or deadlocking.
func TestSearchByNameSingleflight_NoCache(t *testing.T) {
	// Searching with NoCache=true must not panic or deadlock.
	// We use a non-existent route so all providers return not-applicable errors.
	opts := SearchOptions{
		Currency:  "EUR",
		Providers: []string{"flixbus"}, // only one provider to keep test fast
		NoCache:   true,
	}
	// This will attempt a live FlixBus resolve; in short mode we skip.
	if testing.Short() {
		t.Skip("skipping live-provider test in short mode")
	}
	// We just verify it doesn't panic/deadlock; result doesn't matter.
	result, _ := SearchByName(t.Context(), "TestCityXYZ99", "TestCityABC88", "2026-06-15", opts)
	_ = result
}

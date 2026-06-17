package flights

import "testing"

func TestDuffelKeys_EnvParsing(t *testing.T) {
	t.Setenv("DUFFEL_API_KEY", "")
	t.Setenv("DUFFEL_API_KEYS", "k1, k2 ,k3")
	got := duffelKeys()
	if len(got) != 3 || got[0] != "k1" || got[1] != "k2" || got[2] != "k3" {
		t.Fatalf("duffelKeys() = %v, want [k1 k2 k3]", got)
	}

	// Fallback to single key when DUFFEL_API_KEYS is unset.
	t.Setenv("DUFFEL_API_KEYS", "")
	t.Setenv("DUFFEL_API_KEY", "solo")
	if got := duffelKeys(); len(got) != 1 || got[0] != "solo" {
		t.Fatalf("single-key fallback = %v, want [solo]", got)
	}

	// None set → empty.
	t.Setenv("DUFFEL_API_KEY", "")
	if got := duffelKeys(); len(got) != 0 {
		t.Fatalf("no keys = %v, want empty", got)
	}
}

func TestDuffelKeyOrder_RoundRobin(t *testing.T) {
	keys := []string{"a", "b", "c"}
	// Over a full cycle of len(keys) consecutive calls, the starting key must
	// rotate through every key — regardless of the package counter's current
	// offset — and each call must return all keys. This is hermetic: it does not
	// depend on the counter's starting value.
	starts := map[string]bool{}
	for i := 0; i < len(keys); i++ {
		order := duffelKeyOrder(keys)
		if len(order) != len(keys) {
			t.Fatalf("call %d: ordering = %v, want all %d keys", i, order, len(keys))
		}
		seen := map[string]bool{}
		for _, k := range order {
			seen[k] = true
		}
		if len(seen) != len(keys) {
			t.Fatalf("call %d: ordering missing keys: %v", i, order)
		}
		starts[order[0]] = true
	}
	if len(starts) != len(keys) {
		t.Fatalf("round-robin did not rotate through all start keys over a full cycle: got starts %v", starts)
	}
}

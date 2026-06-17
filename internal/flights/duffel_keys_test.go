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
	// Each call starts from the next key; all keys present each time.
	first := duffelKeyOrder(keys)
	second := duffelKeyOrder(keys)
	if first[0] == second[0] {
		t.Errorf("round-robin did not advance: %v then %v", first, second)
	}
	if len(first) != 3 || len(second) != 3 {
		t.Errorf("ordering must contain all keys: %v %v", first, second)
	}
}

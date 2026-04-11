package ground

import (
	"testing"
)

func TestMatchesQuery_Name(t *testing.T) {
	city := regiojetCityRaw{Name: "Prague", Aliases: []string{"Praha"}}
	if !matchesQuery(city, "prague") {
		t.Error("expected match on name")
	}
}

func TestMatchesQuery_Alias(t *testing.T) {
	city := regiojetCityRaw{Name: "Prague", Aliases: []string{"Praha", "Prag"}}
	if !matchesQuery(city, "prah") {
		t.Error("expected match on alias")
	}
}

func TestMatchesQuery_NoMatch(t *testing.T) {
	city := regiojetCityRaw{Name: "Prague", Aliases: nil}
	if matchesQuery(city, "budapest") {
		t.Error("should not match")
	}
}

func TestComputeRegioJetDuration_WithMillis(t *testing.T) {
	got := computeRegioJetDuration(
		"2026-04-10T06:01:00.000+02:00",
		"2026-04-10T10:31:00.000+02:00",
	)
	if got != 270 {
		t.Errorf("got %d, want 270", got)
	}
}

func TestComputeRegioJetDuration_WithoutMillis(t *testing.T) {
	got := computeRegioJetDuration(
		"2026-04-10T06:01:00+02:00",
		"2026-04-10T10:31:00+02:00",
	)
	if got != 270 {
		t.Errorf("got %d, want 270", got)
	}
}

func TestComputeRegioJetDuration_Invalid(t *testing.T) {
	got := computeRegioJetDuration("invalid", "also-invalid")
	if got != 0 {
		t.Errorf("got %d, want 0 for invalid input", got)
	}
}

// RegioJet rate limiter is tested in eurostar_test.go (TestAllLimiterConfigurations).

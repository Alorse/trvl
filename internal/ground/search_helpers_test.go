package ground

import (
	"testing"

	"github.com/MikkoParkkola/trvl/internal/models"
)

func TestBrowserFallbacksEnabled(t *testing.T) {
	t.Setenv("TRVL_ALLOW_BROWSER_FALLBACKS", "")
	if browserFallbacksEnabled(SearchOptions{}) {
		t.Fatal("expected browser fallbacks to be disabled by default")
	}

	t.Setenv("TRVL_ALLOW_BROWSER_FALLBACKS", "true")
	if !browserFallbacksEnabled(SearchOptions{}) {
		t.Fatal("expected environment opt-in to enable browser fallbacks")
	}

	t.Setenv("TRVL_ALLOW_BROWSER_FALLBACKS", "definitely-not-bool")
	if browserFallbacksEnabled(SearchOptions{}) {
		t.Fatal("expected invalid environment value to keep browser fallbacks disabled")
	}

	t.Setenv("TRVL_ALLOW_BROWSER_FALLBACKS", "")
	if !browserFallbacksEnabled(SearchOptions{AllowBrowserFallbacks: true}) {
		t.Fatal("expected explicit option to enable browser fallbacks")
	}
}

func TestDeduplicateGroundRoutes(t *testing.T) {
	routes := []models.GroundRoute{
		{
			Provider: "trainline",
			Price:    49,
			Departure: models.GroundStop{
				Time: "2026-07-01T08:00:00",
			},
			Arrival: models.GroundStop{
				Time: "2026-07-01T10:00:00",
			},
		},
		{
			Provider: "trainline",
			Price:    49,
			Departure: models.GroundStop{
				Time: "2026-07-01T08:00:00",
			},
			Arrival: models.GroundStop{
				Time: "2026-07-01T10:00:00",
			},
		},
		{
			Provider: "trainline",
			Price:    55,
			Departure: models.GroundStop{
				Time: "2026-07-01T08:00:00",
			},
			Arrival: models.GroundStop{
				Time: "2026-07-01T10:00:00",
			},
		},
	}

	deduped := deduplicateGroundRoutes(routes)
	if len(deduped) != 2 {
		t.Fatalf("expected 2 unique routes, got %d", len(deduped))
	}
	if deduped[0].Price != 49 || deduped[1].Price != 55 {
		t.Fatalf("unexpected deduplicated prices: %#v", deduped)
	}
}

func TestFilterUnavailableGroundRoutes(t *testing.T) {
	routes := []models.GroundRoute{
		{Provider: "flixbus", Price: 0},
		{Provider: "transitous", Price: 0},
		{Provider: "db", Price: 0},
		{Provider: "trainline", Price: 39},
	}

	filtered := filterUnavailableGroundRoutes(routes)
	if len(filtered) != 3 {
		t.Fatalf("expected 3 routes after filtering, got %d", len(filtered))
	}
	if filtered[0].Provider != "transitous" {
		t.Fatalf("expected schedule-only transitous route to be kept, got %q", filtered[0].Provider)
	}
	if filtered[1].Provider != "db" {
		t.Fatalf("expected schedule-only db route to be kept, got %q", filtered[1].Provider)
	}
	if filtered[2].Provider != "trainline" {
		t.Fatalf("expected priced route to be kept, got %q", filtered[2].Provider)
	}
}

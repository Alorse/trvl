package hacks

import (
	"context"
	"testing"
)

func TestDetectEU261_emptyInput(t *testing.T) {
	hacks := detectEU261(context.Background(), DetectorInput{})
	if len(hacks) != 0 {
		t.Errorf("expected nil for empty input, got %d hacks", len(hacks))
	}
}

func TestDetectEU261_missingOrigin(t *testing.T) {
	hacks := detectEU261(context.Background(), DetectorInput{
		Destination: "BKK",
		Date:        "2026-06-01",
	})
	if len(hacks) != 0 {
		t.Errorf("expected nil for missing origin, got %d hacks", len(hacks))
	}
}

func TestDetectEU261_missingDestination(t *testing.T) {
	hacks := detectEU261(context.Background(), DetectorInput{
		Origin: "HEL",
		Date:   "2026-06-01",
	})
	if len(hacks) != 0 {
		t.Errorf("expected nil for missing destination, got %d hacks", len(hacks))
	}
}

func TestDetectEU261_missingDate(t *testing.T) {
	hacks := detectEU261(context.Background(), DetectorInput{
		Origin:      "HEL",
		Destination: "BKK",
	})
	if len(hacks) != 0 {
		t.Errorf("expected nil for missing date, got %d hacks", len(hacks))
	}
}

func TestDetectEU261_nonEUOrigin(t *testing.T) {
	hacks := detectEU261(context.Background(), DetectorInput{
		Origin:      "JFK",
		Destination: "HEL",
		Date:        "2026-06-01",
	})
	if len(hacks) != 0 {
		t.Errorf("expected nil for non-EU origin, got %d hacks", len(hacks))
	}
}

func TestDetectEU261_euOrigin(t *testing.T) {
	hacks := detectEU261(context.Background(), DetectorInput{
		Origin:      "HEL",
		Destination: "BKK",
		Date:        "2026-06-01",
	})
	if len(hacks) != 1 {
		t.Fatalf("expected 1 hack, got %d", len(hacks))
	}
	h := hacks[0]
	if h.Type != "eu261_awareness" {
		t.Errorf("type = %q, want eu261_awareness", h.Type)
	}
	if h.Title == "" {
		t.Error("title is empty")
	}
	if h.Description == "" {
		t.Error("description is empty")
	}
	if len(h.Steps) == 0 {
		t.Error("steps are empty")
	}
	if len(h.Risks) == 0 {
		t.Error("risks are empty")
	}
}

func TestDetectEU261_caseInsensitive(t *testing.T) {
	hacks := detectEU261(context.Background(), DetectorInput{
		Origin:      "hel",
		Destination: "bkk",
		Date:        "2026-06-01",
	})
	// Should normalize to uppercase and match
	if len(hacks) != 1 {
		t.Fatalf("expected 1 hack for lowercase hel→bkk, got %d", len(hacks))
	}
}

func TestDetectEU261_variousEUAirports(t *testing.T) {
	airports := []string{"AMS", "CDG", "FRA", "LHR", "BCN", "MUC", "TLL", "PRG"}
	for _, ap := range airports {
		hacks := detectEU261(context.Background(), DetectorInput{
			Origin:      ap,
			Destination: "JFK",
			Date:        "2026-06-01",
		})
		if len(hacks) != 1 {
			t.Errorf("expected 1 hack for EU origin %s, got %d", ap, len(hacks))
		}
	}
}

func TestEUAirports_populated(t *testing.T) {
	if len(euAirports) == 0 {
		t.Fatal("euAirports is empty")
	}
	// Spot-check a few known EU airports
	required := []string{"HEL", "AMS", "CDG", "FRA", "TLL", "PRG"}
	for _, code := range required {
		if !euAirports[code] {
			t.Errorf("%s should be in euAirports", code)
		}
	}
}

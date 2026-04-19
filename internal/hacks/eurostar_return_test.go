package hacks

import (
	"context"
	"testing"
)

func TestDetectEurostarReturn_emptyInput(t *testing.T) {
	hacks := detectEurostarReturn(context.Background(), DetectorInput{})
	if len(hacks) != 0 {
		t.Errorf("expected nil for empty input, got %d hacks", len(hacks))
	}
}

func TestDetectEurostarReturn_missingOrigin(t *testing.T) {
	hacks := detectEurostarReturn(context.Background(), DetectorInput{
		Destination: "PAR",
	})
	if len(hacks) != 0 {
		t.Errorf("expected nil for missing origin, got %d hacks", len(hacks))
	}
}

func TestDetectEurostarReturn_missingDestination(t *testing.T) {
	hacks := detectEurostarReturn(context.Background(), DetectorInput{
		Origin: "LON",
	})
	if len(hacks) != 0 {
		t.Errorf("expected nil for missing destination, got %d hacks", len(hacks))
	}
}

func TestDetectEurostarReturn_nonEurostarRoute(t *testing.T) {
	hacks := detectEurostarReturn(context.Background(), DetectorInput{
		Origin:      "HEL",
		Destination: "PRG",
	})
	if len(hacks) != 0 {
		t.Errorf("expected nil for non-Eurostar route, got %d hacks", len(hacks))
	}
}

func TestDetectEurostarReturn_roundTripSkipped(t *testing.T) {
	// Return date set — hack should not fire (already round-trip)
	hacks := detectEurostarReturn(context.Background(), DetectorInput{
		Origin:      "LON",
		Destination: "PAR",
		ReturnDate:  "2026-05-10",
	})
	if len(hacks) != 0 {
		t.Errorf("expected nil for round-trip search, got %d hacks", len(hacks))
	}
}

func TestDetectEurostarReturn_validOneWay(t *testing.T) {
	hacks := detectEurostarReturn(context.Background(), DetectorInput{
		Origin:      "LON",
		Destination: "PAR",
	})
	if len(hacks) != 1 {
		t.Fatalf("expected 1 hack, got %d", len(hacks))
	}
	h := hacks[0]
	if h.Type != "eurostar_return" {
		t.Errorf("type = %q, want eurostar_return", h.Type)
	}
	if h.Title == "" {
		t.Error("title is empty")
	}
	if len(h.Steps) == 0 {
		t.Error("steps are empty")
	}
	if len(h.Risks) == 0 {
		t.Error("risks are empty")
	}
}

func TestDetectEurostarReturn_airportCodes(t *testing.T) {
	// LHR→CDG should normalize to LON→PAR and fire
	hacks := detectEurostarReturn(context.Background(), DetectorInput{
		Origin:      "LHR",
		Destination: "CDG",
	})
	if len(hacks) != 1 {
		t.Fatalf("expected 1 hack for LHR→CDG, got %d", len(hacks))
	}
	if hacks[0].Type != "eurostar_return" {
		t.Errorf("type = %q, want eurostar_return", hacks[0].Type)
	}
}

func TestDetectEurostarReturn_sameCitySkipped(t *testing.T) {
	// LHR→STN both normalize to LON — should not fire
	hacks := detectEurostarReturn(context.Background(), DetectorInput{
		Origin:      "LHR",
		Destination: "STN",
	})
	if len(hacks) != 0 {
		t.Errorf("expected nil for same-city route, got %d hacks", len(hacks))
	}
}

func TestNormalizeEurostarCity(t *testing.T) {
	tests := []struct {
		code string
		want string
	}{
		{"LHR", "LON"},
		{"LGW", "LON"},
		{"STN", "LON"},
		{"STP", "LON"},
		{"CDG", "PAR"},
		{"ORY", "PAR"},
		{"BRU", "BRU"},
		{"CRL", "BRU"},
		{"AMS", "AMS"},
		{"EIN", "AMS"},
		{"LIL", "LIL"},
		{"HEL", ""},
		{"XYZ", ""},
	}
	for _, tc := range tests {
		got := normalizeEurostarCity(tc.code)
		if got != tc.want {
			t.Errorf("normalizeEurostarCity(%q) = %q, want %q", tc.code, got, tc.want)
		}
	}
}

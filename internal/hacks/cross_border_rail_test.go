package hacks

import (
	"context"
	"testing"
)

func TestDetectCrossBorderRail_emptyInput(t *testing.T) {
	hacks := detectCrossBorderRail(context.Background(), DetectorInput{})
	if len(hacks) != 0 {
		t.Errorf("expected nil for empty input, got %d hacks", len(hacks))
	}
}

func TestDetectCrossBorderRail_missingOrigin(t *testing.T) {
	hacks := detectCrossBorderRail(context.Background(), DetectorInput{
		Destination: "MUC",
	})
	if len(hacks) != 0 {
		t.Errorf("expected nil for missing origin, got %d hacks", len(hacks))
	}
}

func TestDetectCrossBorderRail_missingDestination(t *testing.T) {
	hacks := detectCrossBorderRail(context.Background(), DetectorInput{
		Origin: "PAR",
	})
	if len(hacks) != 0 {
		t.Errorf("expected nil for missing destination, got %d hacks", len(hacks))
	}
}

func TestDetectCrossBorderRail_nonApplicableRoute(t *testing.T) {
	hacks := detectCrossBorderRail(context.Background(), DetectorInput{
		Origin:      "HEL",
		Destination: "PRG",
	})
	if len(hacks) != 0 {
		t.Errorf("expected nil for non-applicable route, got %d hacks", len(hacks))
	}
}

func TestDetectCrossBorderRail_parisMunich(t *testing.T) {
	hacks := detectCrossBorderRail(context.Background(), DetectorInput{
		Origin:      "PAR",
		Destination: "MUC",
	})
	if len(hacks) != 1 {
		t.Fatalf("expected 1 hack, got %d", len(hacks))
	}
	h := hacks[0]
	if h.Type != "cross_border_rail" {
		t.Errorf("type = %q, want cross_border_rail", h.Type)
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
	if len(h.Citations) == 0 {
		t.Error("citations are empty")
	}
}

func TestDetectCrossBorderRail_pragueBerlin(t *testing.T) {
	hacks := detectCrossBorderRail(context.Background(), DetectorInput{
		Origin:      "PRG",
		Destination: "BER",
	})
	if len(hacks) != 1 {
		t.Fatalf("expected 1 hack for PRG→BER, got %d", len(hacks))
	}
	if hacks[0].Type != "cross_border_rail" {
		t.Errorf("type = %q, want cross_border_rail", hacks[0].Type)
	}
	// Should recommend Czech Railways
	found := false
	for _, s := range hacks[0].Steps {
		if containsSubstring(s, "cd.cz") {
			found = true
		}
	}
	if !found {
		t.Error("expected cd.cz in steps for PRG→BER")
	}
}

func TestDetectCrossBorderRail_caseInsensitive(t *testing.T) {
	hacks := detectCrossBorderRail(context.Background(), DetectorInput{
		Origin:      "par",
		Destination: "fra",
	})
	if len(hacks) != 1 {
		t.Fatalf("expected 1 hack for lowercase par→fra, got %d", len(hacks))
	}
}

func TestCrossBorderRoutes_populated(t *testing.T) {
	if len(crossBorderRoutes) == 0 {
		t.Fatal("crossBorderRoutes is empty")
	}
	for origin, dests := range crossBorderRoutes {
		for dest, arb := range dests {
			if arb.cheaperSite == "" {
				t.Errorf("[%s→%s] cheaperSite is empty", origin, dest)
			}
			if arb.cheaperURL == "" {
				t.Errorf("[%s→%s] cheaperURL is empty", origin, dest)
			}
			if arb.savings == "" {
				t.Errorf("[%s→%s] savings is empty", origin, dest)
			}
		}
	}
}

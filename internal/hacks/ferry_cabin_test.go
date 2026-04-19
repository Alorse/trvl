package hacks

import (
	"context"
	"testing"
)

func TestDetectFerryCabin_emptyInput(t *testing.T) {
	hacks := detectFerryCabin(context.Background(), DetectorInput{})
	if len(hacks) != 0 {
		t.Errorf("expected nil for empty input, got %d hacks", len(hacks))
	}
}

func TestDetectFerryCabin_missingOrigin(t *testing.T) {
	hacks := detectFerryCabin(context.Background(), DetectorInput{
		Destination: "ARN",
	})
	if len(hacks) != 0 {
		t.Errorf("expected nil for missing origin, got %d hacks", len(hacks))
	}
}

func TestDetectFerryCabin_missingDestination(t *testing.T) {
	hacks := detectFerryCabin(context.Background(), DetectorInput{
		Origin: "HEL",
	})
	if len(hacks) != 0 {
		t.Errorf("expected nil for missing destination, got %d hacks", len(hacks))
	}
}

func TestDetectFerryCabin_nonApplicableRoute(t *testing.T) {
	hacks := detectFerryCabin(context.Background(), DetectorInput{
		Origin:      "HEL",
		Destination: "PRG",
	})
	if len(hacks) != 0 {
		t.Errorf("expected nil for non-ferry route, got %d hacks", len(hacks))
	}
}

func TestDetectFerryCabin_helsinkiStockholm(t *testing.T) {
	hacks := detectFerryCabin(context.Background(), DetectorInput{
		Origin:      "HEL",
		Destination: "ARN",
	})
	if len(hacks) != 1 {
		t.Fatalf("expected 1 hack, got %d", len(hacks))
	}
	h := hacks[0]
	if h.Type != "ferry_cabin_hotel" {
		t.Errorf("type = %q, want ferry_cabin_hotel", h.Type)
	}
	if h.Title == "" {
		t.Error("title is empty")
	}
	if h.Description == "" {
		t.Error("description is empty")
	}
	if h.Savings <= 0 {
		t.Errorf("savings should be positive, got %.0f", h.Savings)
	}
	if h.Currency != "EUR" {
		t.Errorf("currency = %q, want EUR", h.Currency)
	}
	if len(h.Steps) == 0 {
		t.Error("steps are empty")
	}
	if len(h.Risks) == 0 {
		t.Error("risks are empty")
	}
}

func TestDetectFerryCabin_copenhagenOslo(t *testing.T) {
	hacks := detectFerryCabin(context.Background(), DetectorInput{
		Origin:      "CPH",
		Destination: "OSL",
	})
	if len(hacks) != 1 {
		t.Fatalf("expected 1 hack for CPH→OSL, got %d", len(hacks))
	}
	if hacks[0].Type != "ferry_cabin_hotel" {
		t.Errorf("type = %q, want ferry_cabin_hotel", hacks[0].Type)
	}
}

func TestDetectFerryCabin_caseInsensitive(t *testing.T) {
	hacks := detectFerryCabin(context.Background(), DetectorInput{
		Origin:      "hel",
		Destination: "arn",
	})
	if len(hacks) != 1 {
		t.Fatalf("expected 1 hack for lowercase hel→arn, got %d", len(hacks))
	}
}

func TestOvernightFerries_populated(t *testing.T) {
	if len(overnightFerries) == 0 {
		t.Fatal("overnightFerries is empty")
	}
	for origin, dests := range overnightFerries {
		for dest, ferry := range dests {
			if ferry.operator == "" {
				t.Errorf("[%s→%s] operator is empty", origin, dest)
			}
			if ferry.cabinFromEUR <= 0 {
				t.Errorf("[%s→%s] cabinFromEUR must be positive", origin, dest)
			}
			if ferry.hotelAvgEUR <= 0 {
				t.Errorf("[%s→%s] hotelAvgEUR must be positive", origin, dest)
			}
			if ferry.durationHrs <= 0 {
				t.Errorf("[%s→%s] durationHrs must be positive", origin, dest)
			}
			if ferry.frequency == "" {
				t.Errorf("[%s→%s] frequency is empty", origin, dest)
			}
		}
	}
}

func TestDetectFerryCabin_lowSavingsSkipped(t *testing.T) {
	// ARN→RIX has savings of 50-45=5, which is < 10 threshold
	hacks := detectFerryCabin(context.Background(), DetectorInput{
		Origin:      "ARN",
		Destination: "RIX",
	})
	if len(hacks) != 0 {
		t.Errorf("expected nil for low-savings route, got %d hacks", len(hacks))
	}
}

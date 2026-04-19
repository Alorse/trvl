package hacks

import (
	"context"
	"testing"
)

func TestDetectGroupSplit_emptyInput(t *testing.T) {
	hacks := detectGroupSplit(context.Background(), DetectorInput{})
	if len(hacks) != 0 {
		t.Errorf("expected no hacks for empty input, got %d", len(hacks))
	}
}

func TestDetectGroupSplit_singlePassenger(t *testing.T) {
	hacks := detectGroupSplit(context.Background(), DetectorInput{
		Passengers: 1,
		NaivePrice: 500,
	})
	if len(hacks) != 0 {
		t.Errorf("expected no hacks for 1 passenger, got %d", len(hacks))
	}
}

func TestDetectGroupSplit_twoPassengers(t *testing.T) {
	hacks := detectGroupSplit(context.Background(), DetectorInput{
		Passengers: 2,
		NaivePrice: 400,
	})
	if len(hacks) != 0 {
		t.Errorf("expected no hacks for 2 passengers, got %d", len(hacks))
	}
}

func TestDetectGroupSplit_threePassengers(t *testing.T) {
	hacks := detectGroupSplit(context.Background(), DetectorInput{
		Origin:      "HEL",
		Destination: "PRG",
		Date:        "2026-06-15",
		Passengers:  3,
		NaivePrice:  600,
	})
	if len(hacks) != 1 {
		t.Fatalf("expected 1 hack for 3 passengers, got %d", len(hacks))
	}
	h := hacks[0]
	if h.Type != "group_split" {
		t.Errorf("expected type group_split, got %q", h.Type)
	}
	if h.Title != "Search individually — groups pay more" {
		t.Errorf("unexpected title: %q", h.Title)
	}
	// 3 passengers -> 10% savings rate -> 60 EUR.
	if h.Savings != 60 {
		t.Errorf("expected savings 60, got %.0f", h.Savings)
	}
}

func TestDetectGroupSplit_fourPassengers(t *testing.T) {
	hacks := detectGroupSplit(context.Background(), DetectorInput{
		Passengers: 4,
		NaivePrice: 800,
	})
	if len(hacks) != 1 {
		t.Fatalf("expected 1 hack for 4 passengers, got %d", len(hacks))
	}
	// 4 passengers -> 15% savings rate -> 120 EUR.
	if hacks[0].Savings != 120 {
		t.Errorf("expected savings 120, got %.0f", hacks[0].Savings)
	}
}

func TestDetectGroupSplit_sixPassengers(t *testing.T) {
	hacks := detectGroupSplit(context.Background(), DetectorInput{
		Passengers: 6,
		NaivePrice: 1200,
	})
	if len(hacks) != 1 {
		t.Fatalf("expected 1 hack for 6 passengers, got %d", len(hacks))
	}
	// 6 passengers -> 20% savings rate -> 240 EUR.
	if hacks[0].Savings != 240 {
		t.Errorf("expected savings 240, got %.0f", hacks[0].Savings)
	}
}

func TestDetectGroupSplit_zeroPrice(t *testing.T) {
	hacks := detectGroupSplit(context.Background(), DetectorInput{
		Passengers: 4,
		NaivePrice: 0,
	})
	if len(hacks) != 0 {
		t.Errorf("expected no hacks with zero price, got %d", len(hacks))
	}
}

func TestDetectGroupSplit_negativePrice(t *testing.T) {
	hacks := detectGroupSplit(context.Background(), DetectorInput{
		Passengers: 4,
		NaivePrice: -100,
	})
	if len(hacks) != 0 {
		t.Errorf("expected no hacks with negative price, got %d", len(hacks))
	}
}

func TestDetectGroupSplit_currencyDefault(t *testing.T) {
	hacks := detectGroupSplit(context.Background(), DetectorInput{
		Passengers: 3,
		NaivePrice: 600,
	})
	if len(hacks) != 1 {
		t.Fatalf("expected 1 hack, got %d", len(hacks))
	}
	if hacks[0].Currency != "EUR" {
		t.Errorf("expected EUR default, got %q", hacks[0].Currency)
	}
}

func TestDetectGroupSplit_customCurrency(t *testing.T) {
	hacks := detectGroupSplit(context.Background(), DetectorInput{
		Passengers: 3,
		NaivePrice: 600,
		Currency:   "GBP",
	})
	if len(hacks) != 1 {
		t.Fatalf("expected 1 hack, got %d", len(hacks))
	}
	if hacks[0].Currency != "GBP" {
		t.Errorf("expected GBP, got %q", hacks[0].Currency)
	}
}

func TestDetectGroupSplit_hasRisks(t *testing.T) {
	hacks := detectGroupSplit(context.Background(), DetectorInput{
		Passengers: 4,
		NaivePrice: 800,
	})
	if len(hacks) != 1 {
		t.Fatalf("expected 1 hack, got %d", len(hacks))
	}
	if len(hacks[0].Risks) == 0 {
		t.Error("expected non-empty risks")
	}
	if len(hacks[0].Risks) != 3 {
		t.Errorf("expected 3 risks, got %d", len(hacks[0].Risks))
	}
}

func TestDetectGroupSplit_hasSteps(t *testing.T) {
	hacks := detectGroupSplit(context.Background(), DetectorInput{
		Passengers: 4,
		NaivePrice: 800,
	})
	if len(hacks) != 1 {
		t.Fatalf("expected 1 hack, got %d", len(hacks))
	}
	if len(hacks[0].Steps) != 4 {
		t.Errorf("expected 4 steps, got %d", len(hacks[0].Steps))
	}
}

func TestDetectGroupSplit_description(t *testing.T) {
	hacks := detectGroupSplit(context.Background(), DetectorInput{
		Passengers: 5,
		NaivePrice: 1000,
	})
	if len(hacks) != 1 {
		t.Fatalf("expected 1 hack, got %d", len(hacks))
	}
	if hacks[0].Description == "" {
		t.Error("expected non-empty description")
	}
}

func TestDetectGroupSplit_largGroup(t *testing.T) {
	hacks := detectGroupSplit(context.Background(), DetectorInput{
		Passengers: 10,
		NaivePrice: 2000,
	})
	if len(hacks) != 1 {
		t.Fatalf("expected 1 hack for 10 passengers, got %d", len(hacks))
	}
	// 10 passengers -> 20% rate -> 400 savings.
	if hacks[0].Savings != 400 {
		t.Errorf("expected savings 400, got %.0f", hacks[0].Savings)
	}
}

func TestDetectGroupSplit_boundaryThree(t *testing.T) {
	// Exactly 3 passengers is the minimum.
	hacks := detectGroupSplit(context.Background(), DetectorInput{
		Passengers: 3,
		NaivePrice: 300,
	})
	if len(hacks) != 1 {
		t.Fatalf("expected 1 hack at boundary (3 passengers), got %d", len(hacks))
	}
	// 3 * 10% = 30.
	if hacks[0].Savings != 30 {
		t.Errorf("expected savings 30, got %.0f", hacks[0].Savings)
	}
}

// --- estimatedSavingsRate tests ---

func TestEstimatedSavingsRate(t *testing.T) {
	tests := []struct {
		passengers int
		want       float64
	}{
		{3, 0.10},
		{4, 0.15},
		{5, 0.15},
		{6, 0.20},
		{7, 0.20},
		{10, 0.20},
		{100, 0.20},
	}
	for _, tc := range tests {
		got := estimatedSavingsRate(tc.passengers)
		if got != tc.want {
			t.Errorf("estimatedSavingsRate(%d) = %v, want %v", tc.passengers, got, tc.want)
		}
	}
}

func TestDetectGroupSplit_fivePassengersSavingsRate(t *testing.T) {
	// 5 passengers falls in >= 4 bracket -> 15% rate.
	hacks := detectGroupSplit(context.Background(), DetectorInput{
		Passengers: 5,
		NaivePrice: 1000,
	})
	if len(hacks) != 1 {
		t.Fatalf("expected 1 hack, got %d", len(hacks))
	}
	// 1000 * 0.15 = 150.
	if hacks[0].Savings != 150 {
		t.Errorf("expected savings 150, got %.0f", hacks[0].Savings)
	}
}

func TestDetectGroupSplit_stepsContainPerPerson(t *testing.T) {
	hacks := detectGroupSplit(context.Background(), DetectorInput{
		Passengers: 4,
		NaivePrice: 800,
		Currency:   "EUR",
	})
	if len(hacks) != 1 {
		t.Fatalf("expected 1 hack, got %d", len(hacks))
	}
	// First step should contain "4 passengers x 200 EUR = 800 EUR total".
	step := hacks[0].Steps[0]
	expected := "Current: 4 passengers x 200 EUR = 800 EUR total"
	if step != expected {
		t.Errorf("step[0] = %q, want %q", step, expected)
	}
}

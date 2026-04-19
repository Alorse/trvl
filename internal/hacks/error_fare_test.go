package hacks

import (
	"context"
	"testing"
)

func TestDetectErrorFare_below_threshold(t *testing.T) {
	// HEL→BCN is ~2900 km (long-haul). Floor for one-way long-haul is €60.
	// Error threshold is 50% of floor = €30. Price of €20 should trigger.
	in := DetectorInput{
		Origin:      "HEL",
		Destination: "BCN",
		NaivePrice:  20,
	}
	hacks := detectErrorFare(context.Background(), in)
	if len(hacks) == 0 {
		t.Fatal("expected error_fare hack for €20 HEL→BCN one-way")
	}
	if hacks[0].Type != "error_fare" {
		t.Errorf("type: got %q, want error_fare", hacks[0].Type)
	}
	if hacks[0].Savings <= 0 {
		t.Error("expected positive savings")
	}
}

func TestDetectErrorFare_flash_sale(t *testing.T) {
	// HEL→BCN one-way long-haul. Floor = €60, error threshold = €30.
	// Price of €45 is below floor but above error threshold = flash sale.
	in := DetectorInput{
		Origin:      "HEL",
		Destination: "BCN",
		NaivePrice:  45,
	}
	hacks := detectErrorFare(context.Background(), in)
	if len(hacks) == 0 {
		t.Fatal("expected flash_sale hack for €45 HEL→BCN one-way")
	}
	if hacks[0].Type != "flash_sale" {
		t.Errorf("type: got %q, want flash_sale", hacks[0].Type)
	}
}

func TestDetectErrorFare_normal_price(t *testing.T) {
	// €150 is above the long-haul floor of €60 — no hack should fire.
	in := DetectorInput{
		Origin:      "HEL",
		Destination: "BCN",
		NaivePrice:  150,
	}
	hacks := detectErrorFare(context.Background(), in)
	if len(hacks) != 0 {
		t.Errorf("expected no hack for normal price €150, got %d", len(hacks))
	}
}

func TestDetectErrorFare_roundtrip(t *testing.T) {
	// Round-trip HEL→BCN long-haul. RT floor = €100, error threshold = €50.
	// Price of €40 should trigger error_fare.
	in := DetectorInput{
		Origin:      "HEL",
		Destination: "BCN",
		ReturnDate:  "2026-07-01",
		NaivePrice:  40,
	}
	hacks := detectErrorFare(context.Background(), in)
	if len(hacks) == 0 {
		t.Fatal("expected error_fare for €40 RT HEL→BCN")
	}
	if hacks[0].Type != "error_fare" {
		t.Errorf("type: got %q, want error_fare", hacks[0].Type)
	}
}

func TestDetectErrorFare_short_haul(t *testing.T) {
	// AMS→LHR is ~370 km (short-haul). OW floor = €15, error = €7.50.
	// €5 should trigger error_fare.
	in := DetectorInput{
		Origin:      "AMS",
		Destination: "LHR",
		NaivePrice:  5,
	}
	hacks := detectErrorFare(context.Background(), in)
	if len(hacks) == 0 {
		t.Fatal("expected error_fare for €5 AMS→LHR")
	}
	if hacks[0].Type != "error_fare" {
		t.Errorf("type: got %q, want error_fare", hacks[0].Type)
	}
}

func TestDetectErrorFare_empty_input(t *testing.T) {
	hacks := detectErrorFare(context.Background(), DetectorInput{})
	if len(hacks) != 0 {
		t.Error("expected no hack for empty input")
	}
}

func TestDetectErrorFare_no_price(t *testing.T) {
	in := DetectorInput{
		Origin:      "HEL",
		Destination: "BCN",
		NaivePrice:  0,
	}
	hacks := detectErrorFare(context.Background(), in)
	if len(hacks) != 0 {
		t.Error("expected no hack when NaivePrice is 0")
	}
}

func TestDetectErrorFare_unknown_airports(t *testing.T) {
	in := DetectorInput{
		Origin:      "XYZ",
		Destination: "QQQ",
		NaivePrice:  5,
	}
	hacks := detectErrorFare(context.Background(), in)
	if len(hacks) != 0 {
		t.Error("expected no hack for unknown airports")
	}
}

func TestDetectErrorFare_intercontinental(t *testing.T) {
	// LHR→JFK is ~5500 km. Intercontinental OW floor = €150, error = €75.
	// €50 should trigger error_fare.
	in := DetectorInput{
		Origin:      "LHR",
		Destination: "JFK",
		NaivePrice:  50,
	}
	hacks := detectErrorFare(context.Background(), in)
	if len(hacks) == 0 {
		t.Fatal("expected error_fare for €50 LHR→JFK")
	}
	if hacks[0].Type != "error_fare" {
		t.Errorf("type: got %q, want error_fare", hacks[0].Type)
	}
}

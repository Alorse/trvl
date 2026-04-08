package main

import (
	"context"
	"testing"

	"github.com/MikkoParkkola/trvl/internal/models"
)

func TestConvertRoundedDisplayAmounts_PreservesFallbackCurrency(t *testing.T) {
	orig := convertDisplayCurrencyFunc
	t.Cleanup(func() { convertDisplayCurrencyFunc = orig })

	convertDisplayCurrencyFunc = func(context.Context, float64, string, string) (float64, string) {
		return 123.4, "USD"
	}

	price := 123.4
	total := 456.6

	currency := convertRoundedDisplayAmounts(context.Background(), "USD", "JPY", 0, &price, &total)
	if currency != "USD" {
		t.Fatalf("currency = %q, want %q", currency, "USD")
	}
	if price != 123 {
		t.Fatalf("price = %v, want %v", price, 123.0)
	}
	if total != 123 {
		t.Fatalf("total = %v, want %v", total, 123.0)
	}
}

func TestPrepareGroundRows_PreservesFallbackCurrencyForPriceRange(t *testing.T) {
	orig := convertDisplayCurrencyFunc
	t.Cleanup(func() { convertDisplayCurrencyFunc = orig })

	call := 0
	convertDisplayCurrencyFunc = func(_ context.Context, amount float64, from, to string) (float64, string) {
		call++
		return amount, from
	}

	routes := []models.GroundRoute{
		{
			Price:    100,
			PriceMax: 125,
			Currency: "USD",
		},
	}

	_, _, _ = prepareGroundRows(context.Background(), "JPY", routes)

	if call != 2 {
		t.Fatalf("convertDisplayCurrencyFunc called %d times, want 2", call)
	}
	if routes[0].Currency != "USD" {
		t.Fatalf("routes[0].Currency = %q, want %q", routes[0].Currency, "USD")
	}
	if routes[0].Price != 100 {
		t.Fatalf("routes[0].Price = %v, want %v", routes[0].Price, 100.0)
	}
	if routes[0].PriceMax != 125 {
		t.Fatalf("routes[0].PriceMax = %v, want %v", routes[0].PriceMax, 125.0)
	}
}

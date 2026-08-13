package main

import (
	"testing"

	"github.com/MikkoParkkola/trvl/internal/models"
)

// TestCheapestPricedIgnoresUnpricedResults covers the summary line that read
// "Cheapest: 0" while the table below it listed real fares.
//
// Providers routinely return results with no price — roughly a quarter of a
// typical response — and the default sort puts a zero first. The old code
// seeded the running minimum with Flights[0] and then only replaced it when
// f.Price < cheapest.Price, so a zero seed was unbeatable: no fare is below
// zero. The summary is what an operator reads first, and it was reporting a
// price that no flight had.
func TestCheapestPricedIgnoresUnpricedResults(t *testing.T) {
	flights := []models.FlightResult{
		{Price: 0, Currency: "EUR"},   // unpriced, and sorted first
		{Price: 899, Currency: "EUR"}, // the real cheapest
		{Price: 0, Currency: "EUR"},
		{Price: 1235, Currency: "EUR"},
	}

	got, ok := cheapestPriced(flights)
	if !ok {
		t.Fatal("a set containing priced flights must yield a cheapest")
	}
	if got.Price != 899 {
		t.Errorf("cheapest = %v, want 899 — an unpriced result is not a cheap one", got.Price)
	}
}

// TestCheapestPricedSaysNothingWhenNothingIsPriced pins the other half: when no
// result carries a price there is no cheapest to name, and reporting zero would
// invent one.
func TestCheapestPricedSaysNothingWhenNothingIsPriced(t *testing.T) {
	if _, ok := cheapestPriced([]models.FlightResult{{Price: 0}, {Price: 0}}); ok {
		t.Error("no priced flight means no cheapest flight")
	}
	if _, ok := cheapestPriced(nil); ok {
		t.Error("an empty set has no cheapest flight")
	}
}

// TestCheapestPricedIgnoresOrder pins that the answer does not depend on where
// the cheapest sits, since --sort can order by duration or departure time.
func TestCheapestPricedIgnoresOrder(t *testing.T) {
	got, ok := cheapestPriced([]models.FlightResult{
		{Price: 1800, Currency: "EUR"},
		{Price: 0, Currency: "EUR"},
		{Price: 633, Currency: "EUR"},
		{Price: 947, Currency: "EUR"},
	})
	if !ok || got.Price != 633 {
		t.Errorf("cheapest = %v (ok=%v), want 633 regardless of position", got.Price, ok)
	}
}

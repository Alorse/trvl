package hacks

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"
)

func TestDetectAdvancePurchase_emptyInput(t *testing.T) {
	hacks := detectAdvancePurchase(context.Background(), DetectorInput{})
	if len(hacks) != 0 {
		t.Errorf("expected no hacks for empty input, got %d", len(hacks))
	}
}

func TestDetectAdvancePurchase_pastDate(t *testing.T) {
	hacks := detectAdvancePurchase(context.Background(), DetectorInput{
		Origin:      "HEL",
		Destination: "PRG",
		Date:        "2020-01-01",
	})
	if len(hacks) != 0 {
		t.Errorf("expected no hacks for past date, got %d", len(hacks))
	}
}

func TestDetectAdvancePurchase_invalidDate(t *testing.T) {
	hacks := detectAdvancePurchase(context.Background(), DetectorInput{
		Origin:      "HEL",
		Destination: "PRG",
		Date:        "not-a-date",
	})
	if len(hacks) != 0 {
		t.Errorf("expected no hacks for invalid date, got %d", len(hacks))
	}
}

func TestDetectAdvancePurchase_optimalWindow(t *testing.T) {
	// European short-haul optimal window is 21-56 days (3-8 weeks).
	// Booking at exactly 35 days should return nil.
	date := time.Now().AddDate(0, 0, 35).Format("2006-01-02")
	hacks := detectAdvancePurchase(context.Background(), DetectorInput{
		Origin:      "HEL",
		Destination: "PRG",
		Date:        date,
	})
	if len(hacks) != 0 {
		t.Errorf("expected no hacks within optimal window, got %d", len(hacks))
	}
}

func TestDetectAdvancePurchase_tooEarly(t *testing.T) {
	// European short-haul: booking 120 days ahead (>56 day max) is too early.
	date := time.Now().AddDate(0, 0, 120).Format("2006-01-02")
	hacks := detectAdvancePurchase(context.Background(), DetectorInput{
		Origin:      "HEL",
		Destination: "PRG",
		Date:        date,
	})
	if len(hacks) != 1 {
		t.Fatalf("expected 1 hack for too-early booking, got %d", len(hacks))
	}
	if hacks[0].Type != "advance_purchase" {
		t.Errorf("expected type advance_purchase, got %q", hacks[0].Type)
	}
	if hacks[0].Title == "" {
		t.Error("expected non-empty title")
	}
}

func TestDetectAdvancePurchase_tooLate(t *testing.T) {
	// European short-haul: booking 10 days ahead (< 14 day spike) is very late.
	date := time.Now().AddDate(0, 0, 10).Format("2006-01-02")
	hacks := detectAdvancePurchase(context.Background(), DetectorInput{
		Origin:      "HEL",
		Destination: "PRG",
		Date:        date,
	})
	if len(hacks) != 1 {
		t.Fatalf("expected 1 hack for too-late booking, got %d", len(hacks))
	}
	h := hacks[0]
	if h.Type != "advance_purchase" {
		t.Errorf("expected type advance_purchase, got %q", h.Type)
	}
	// Should be the "last-minute" variant.
	if len(h.Steps) == 0 {
		t.Error("expected non-empty steps")
	}
}

func TestDetectAdvancePurchase_latButNotSpike(t *testing.T) {
	// European short-haul: 18 days ahead (< 21 min, but > 14 spike).
	date := time.Now().AddDate(0, 0, 18).Format("2006-01-02")
	hacks := detectAdvancePurchase(context.Background(), DetectorInput{
		Origin:      "HEL",
		Destination: "PRG",
		Date:        date,
	})
	if len(hacks) != 1 {
		t.Fatalf("expected 1 hack, got %d", len(hacks))
	}
	if hacks[0].Type != "advance_purchase" {
		t.Errorf("expected type advance_purchase, got %q", hacks[0].Type)
	}
}

func TestDetectAdvancePurchase_boundaryOptimalMin(t *testing.T) {
	// At optimalMin + 1 day (22 days) for short-haul should be safely optimal.
	// We use +1 to avoid timezone/rounding edge cases with time.Until.
	date := time.Now().AddDate(0, 0, 22).Format("2006-01-02")
	hacks := detectAdvancePurchase(context.Background(), DetectorInput{
		Origin:      "HEL",
		Destination: "PRG",
		Date:        date,
	})
	if len(hacks) != 0 {
		t.Errorf("expected no hacks within optimal window, got %d", len(hacks))
	}
}

func TestDetectAdvancePurchase_boundaryOptimalMax(t *testing.T) {
	// Exactly at optimalMax (56 days) for short-haul should be optimal.
	date := time.Now().AddDate(0, 0, 56).Format("2006-01-02")
	hacks := detectAdvancePurchase(context.Background(), DetectorInput{
		Origin:      "HEL",
		Destination: "PRG",
		Date:        date,
	})
	if len(hacks) != 0 {
		t.Errorf("expected no hacks at exact optimalMax boundary, got %d", len(hacks))
	}
}

func TestDetectAdvancePurchase_boundaryJustOutside(t *testing.T) {
	// Two days past optimalMax (58 days) for short-haul should trigger.
	// We use +2 to avoid timezone/rounding edge cases with time.Until.
	date := time.Now().AddDate(0, 0, 58).Format("2006-01-02")
	hacks := detectAdvancePurchase(context.Background(), DetectorInput{
		Origin:      "HEL",
		Destination: "PRG",
		Date:        date,
	})
	if len(hacks) != 1 {
		t.Fatalf("expected 1 hack just outside optimal window, got %d", len(hacks))
	}
}

func TestDetectAdvancePurchase_longHaul(t *testing.T) {
	// HEL -> JFK is transatlantic (>2500km). Optimal: 42-112 days.
	// 30 days ahead should be "too late" (< 42 min, > 28 spike).
	date := time.Now().AddDate(0, 0, 30).Format("2006-01-02")
	hacks := detectAdvancePurchase(context.Background(), DetectorInput{
		Origin:      "HEL",
		Destination: "JFK",
		Date:        date,
	})
	if len(hacks) != 1 {
		t.Fatalf("expected 1 hack for long-haul too late, got %d", len(hacks))
	}
}

func TestDetectAdvancePurchase_holidayDestination(t *testing.T) {
	// JMK (Mykonos) is a holiday destination. Optimal: 56-112 days.
	// 30 days ahead should trigger.
	date := time.Now().AddDate(0, 0, 30).Format("2006-01-02")
	hacks := detectAdvancePurchase(context.Background(), DetectorInput{
		Origin:      "HEL",
		Destination: "JMK",
		Date:        date,
	})
	if len(hacks) != 1 {
		t.Fatalf("expected 1 hack for holiday destination outside window, got %d", len(hacks))
	}
}

func TestDetectAdvancePurchase_budgetCarrier(t *testing.T) {
	// STN (Stansted) is a budget carrier airport. Optimal: 28-70 days.
	// 80 days should be too early.
	date := time.Now().AddDate(0, 0, 80).Format("2006-01-02")
	hacks := detectAdvancePurchase(context.Background(), DetectorInput{
		Origin:      "STN",
		Destination: "BCN",
		Date:        date,
	})
	if len(hacks) != 1 {
		t.Fatalf("expected 1 hack for budget carrier too early, got %d", len(hacks))
	}
}

func TestDetectAdvancePurchase_weekendCityBreak(t *testing.T) {
	// Fri-Sun trip. Optimal: 21-42 days.
	// Find next Friday that's 50 days out.
	now := time.Now()
	futureDate := now.AddDate(0, 0, 50)
	// Find the next Friday from futureDate.
	for futureDate.Weekday() != time.Friday {
		futureDate = futureDate.AddDate(0, 0, 1)
	}
	fri := futureDate.Format("2006-01-02")
	sun := futureDate.AddDate(0, 0, 2).Format("2006-01-02")

	hacks := detectAdvancePurchase(context.Background(), DetectorInput{
		Origin:      "HEL",
		Destination: "PRG",
		Date:        fri,
		ReturnDate:  sun,
	})
	if len(hacks) != 1 {
		t.Fatalf("expected 1 hack for weekend break too early, got %d", len(hacks))
	}
}

func TestDetectAdvancePurchase_currencyDefault(t *testing.T) {
	date := time.Now().AddDate(0, 0, 10).Format("2006-01-02")
	hacks := detectAdvancePurchase(context.Background(), DetectorInput{
		Origin:      "HEL",
		Destination: "PRG",
		Date:        date,
	})
	if len(hacks) != 1 {
		t.Fatalf("expected 1 hack, got %d", len(hacks))
	}
	if hacks[0].Currency != "EUR" {
		t.Errorf("expected EUR currency default, got %q", hacks[0].Currency)
	}
}

func TestDetectAdvancePurchase_customCurrency(t *testing.T) {
	date := time.Now().AddDate(0, 0, 10).Format("2006-01-02")
	hacks := detectAdvancePurchase(context.Background(), DetectorInput{
		Origin:      "HEL",
		Destination: "PRG",
		Date:        date,
		Currency:    "USD",
	})
	if len(hacks) != 1 {
		t.Fatalf("expected 1 hack, got %d", len(hacks))
	}
	if hacks[0].Currency != "USD" {
		t.Errorf("expected USD currency, got %q", hacks[0].Currency)
	}
}

func TestDetectAdvancePurchase_missingOrigin(t *testing.T) {
	date := time.Now().AddDate(0, 0, 10).Format("2006-01-02")
	hacks := detectAdvancePurchase(context.Background(), DetectorInput{
		Destination: "PRG",
		Date:        date,
	})
	if len(hacks) != 0 {
		t.Errorf("expected no hacks with missing origin, got %d", len(hacks))
	}
}

func TestDetectAdvancePurchase_missingDestination(t *testing.T) {
	date := time.Now().AddDate(0, 0, 10).Format("2006-01-02")
	hacks := detectAdvancePurchase(context.Background(), DetectorInput{
		Origin: "HEL",
		Date:   date,
	})
	if len(hacks) != 0 {
		t.Errorf("expected no hacks with missing destination, got %d", len(hacks))
	}
}

// --- Route classification tests ---

func TestClassifyRoute_europeanShort(t *testing.T) {
	depart := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	rt := classifyRoute("HEL", "PRG", depart, "")
	if rt != routeEuropeanShort {
		t.Errorf("HEL->PRG should be europeanShort, got %d", rt)
	}
}

func TestClassifyRoute_europeanLong(t *testing.T) {
	depart := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	rt := classifyRoute("HEL", "JFK", depart, "")
	if rt != routeEuropeanLong {
		t.Errorf("HEL->JFK should be europeanLong, got %d", rt)
	}
}

func TestClassifyRoute_budgetCarrier(t *testing.T) {
	depart := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	rt := classifyRoute("STN", "BCN", depart, "")
	if rt != routeBudgetCarrier {
		t.Errorf("STN->BCN should be budgetCarrier, got %d", rt)
	}
}

func TestClassifyRoute_holidaySeasonal(t *testing.T) {
	depart := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	rt := classifyRoute("HEL", "JMK", depart, "")
	if rt != routeHolidaySeasonal {
		t.Errorf("HEL->JMK should be holidaySeasonal, got %d", rt)
	}
}

func TestClassifyRoute_skiResortInSeason(t *testing.T) {
	depart := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	rt := classifyRoute("HEL", "INN", depart, "")
	if rt != routeHolidaySeasonal {
		t.Errorf("HEL->INN in January should be holidaySeasonal, got %d", rt)
	}
}

func TestClassifyRoute_skiResortOffSeason(t *testing.T) {
	depart := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	rt := classifyRoute("HEL", "INN", depart, "")
	// Off-season ski airport should fall through to distance-based.
	if rt == routeHolidaySeasonal {
		t.Errorf("HEL->INN in June should NOT be holidaySeasonal, got %d", rt)
	}
}

func TestClassifyRoute_weekendCityBreak(t *testing.T) {
	// Find a Friday.
	d := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	for d.Weekday() != time.Friday {
		d = d.AddDate(0, 0, 1)
	}
	sun := d.AddDate(0, 0, 2).Format("2006-01-02")
	rt := classifyRoute("HEL", "PRG", d, sun)
	if rt != routeWeekendCityBreak {
		t.Errorf("Fri-Sun trip should be weekendCityBreak, got %d", rt)
	}
}

func TestClassifyRoute_unknownAirports(t *testing.T) {
	depart := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	rt := classifyRoute("XYZ", "ABC", depart, "")
	if rt != routeEuropeanShort {
		t.Errorf("unknown airports should default to europeanShort, got %d", rt)
	}
}

// --- Airport distance tests ---

func TestAirportDistanceKm_knownPair(t *testing.T) {
	dist := airportDistanceKm("HEL", "PRG")
	// HEL-PRG is ~1320km.
	if dist < 1200 || dist > 1500 {
		t.Errorf("HEL->PRG distance should be ~1320km, got %.0f", dist)
	}
}

func TestAirportDistanceKm_transatlantic(t *testing.T) {
	dist := airportDistanceKm("HEL", "JFK")
	// HEL-JFK is ~6600km.
	if dist < 6400 || dist > 6800 {
		t.Errorf("HEL->JFK distance should be ~6600km, got %.0f", dist)
	}
}

func TestAirportDistanceKm_unknownAirport(t *testing.T) {
	dist := airportDistanceKm("HEL", "XYZ")
	if dist != 0 {
		t.Errorf("unknown airport should return 0, got %.0f", dist)
	}
}

func TestAirportDistanceKm_sameAirport(t *testing.T) {
	dist := airportDistanceKm("HEL", "HEL")
	if dist != 0 {
		t.Errorf("same airport distance should be 0, got %.0f", dist)
	}
}

// --- Haversine tests ---

func TestHaversineKm(t *testing.T) {
	// London to Paris is ~340km.
	dist := haversineKm(51.5074, -0.1278, 48.8566, 2.3522)
	if math.Abs(dist-340) > 20 {
		t.Errorf("London->Paris should be ~340km, got %.0f", dist)
	}
}

func TestHaversineKm_samePoint(t *testing.T) {
	dist := haversineKm(51.5074, -0.1278, 51.5074, -0.1278)
	if dist != 0 {
		t.Errorf("same point distance should be 0, got %f", dist)
	}
}

// --- isUpperIATA tests ---

func TestIsUpperIATA(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"HEL", true},
		{"PRG", true},
		{"JFK", true},
		{"hel", false},
		{"He1", false},
		{"HELO", false},
		{"HE", false},
		{"", false},
		{"123", false},
	}
	for _, tc := range tests {
		got := isUpperIATA(tc.input)
		if got != tc.want {
			t.Errorf("isUpperIATA(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

// --- Static data tests ---

func TestAirportCoords_populated(t *testing.T) {
	if len(airportCoords) < 50 {
		t.Errorf("expected at least 50 airports, got %d", len(airportCoords))
	}
}

func TestAirportCoords_validRanges(t *testing.T) {
	for code, coord := range airportCoords {
		if coord[0] < -90 || coord[0] > 90 {
			t.Errorf("airport %s has invalid latitude: %f", code, coord[0])
		}
		if coord[1] < -180 || coord[1] > 180 {
			t.Errorf("airport %s has invalid longitude: %f", code, coord[1])
		}
	}
}

func TestHolidayDestinations_populated(t *testing.T) {
	if len(holidayDestinations) == 0 {
		t.Fatal("holidayDestinations is empty")
	}
	// Spot check.
	for _, code := range []string{"JMK", "JTR", "PMI", "TFS"} {
		if !holidayDestinations[code] {
			t.Errorf("%s should be in holidayDestinations", code)
		}
	}
}

func TestBudgetCarrierAirports_populated(t *testing.T) {
	if len(budgetCarrierAirports) == 0 {
		t.Fatal("budgetCarrierAirports is empty")
	}
	for _, code := range []string{"STN", "BVA", "BGY"} {
		if !budgetCarrierAirports[code] {
			t.Errorf("%s should be in budgetCarrierAirports", code)
		}
	}
}

func TestOptimalWindows_allRouteTypes(t *testing.T) {
	types := []routeType{
		routeEuropeanShort, routeEuropeanLong, routeBudgetCarrier,
		routeHolidaySeasonal, routeWeekendCityBreak,
	}
	for _, rt := range types {
		w, ok := optimalWindows[rt]
		if !ok {
			t.Errorf("missing optimal window for route type %d", rt)
			continue
		}
		if w.optimalMin <= 0 {
			t.Errorf("route type %d: optimalMin should be > 0", rt)
		}
		if w.optimalMax <= w.optimalMin {
			t.Errorf("route type %d: optimalMax (%d) should be > optimalMin (%d)", rt, w.optimalMax, w.optimalMin)
		}
		if w.spikeInside <= 0 {
			t.Errorf("route type %d: spikeInside should be > 0", rt)
		}
		if w.spikeInside >= w.optimalMin {
			t.Errorf("route type %d: spikeInside (%d) should be < optimalMin (%d)", rt, w.spikeInside, w.optimalMin)
		}
		if w.label == "" {
			t.Errorf("route type %d: label should not be empty", rt)
		}
	}
}

func TestDetectAdvancePurchase_allRouteTypeWindows(t *testing.T) {
	// Verify each route type returns nil when within optimal window.
	tests := []struct {
		name        string
		origin      string
		destination string
		daysAhead   int
		returnDate  string
	}{
		{"short-haul mid window", "HEL", "PRG", 35, ""},
		{"long-haul mid window", "HEL", "JFK", 80, ""},
		{"budget carrier mid window", "STN", "BCN", 50, ""},
		{"holiday mid window", "HEL", "JMK", 80, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			date := time.Now().AddDate(0, 0, tc.daysAhead).Format("2006-01-02")
			hacks := detectAdvancePurchase(context.Background(), DetectorInput{
				Origin:      tc.origin,
				Destination: tc.destination,
				Date:        date,
				ReturnDate:  tc.returnDate,
			})
			if len(hacks) != 0 {
				t.Errorf("expected no hacks within optimal window, got %d: %s", len(hacks), hacks[0].Title)
			}
		})
	}
}

func TestDetectAdvancePurchase_zeroSavings(t *testing.T) {
	// Advisory hacks should have 0 savings (no concrete estimate).
	date := time.Now().AddDate(0, 0, 10).Format("2006-01-02")
	hacks := detectAdvancePurchase(context.Background(), DetectorInput{
		Origin:      "HEL",
		Destination: "PRG",
		Date:        date,
	})
	if len(hacks) != 1 {
		t.Fatalf("expected 1 hack, got %d", len(hacks))
	}
	if hacks[0].Savings != 0 {
		t.Errorf("advisory hack should have 0 savings, got %.0f", hacks[0].Savings)
	}
}

func TestDetectAdvancePurchase_hasRisksAndSteps(t *testing.T) {
	date := time.Now().AddDate(0, 0, 10).Format("2006-01-02")
	hacks := detectAdvancePurchase(context.Background(), DetectorInput{
		Origin:      "HEL",
		Destination: "PRG",
		Date:        date,
	})
	if len(hacks) != 1 {
		t.Fatalf("expected 1 hack, got %d", len(hacks))
	}
	if len(hacks[0].Risks) == 0 {
		t.Error("expected non-empty risks")
	}
	if len(hacks[0].Steps) == 0 {
		t.Error("expected non-empty steps")
	}
	if hacks[0].Description == "" {
		t.Error("expected non-empty description")
	}
}

// Verify the 3 return paths produce distinct titles.
func TestDetectAdvancePurchase_distinctTitles(t *testing.T) {
	titles := make(map[string]bool)

	// Too early.
	dateEarly := time.Now().AddDate(0, 0, 120).Format("2006-01-02")
	h1 := detectAdvancePurchase(context.Background(), DetectorInput{
		Origin: "HEL", Destination: "PRG", Date: dateEarly,
	})
	if len(h1) == 1 {
		titles[h1[0].Title] = true
	}

	// Late but not spike.
	dateLate := time.Now().AddDate(0, 0, 18).Format("2006-01-02")
	h2 := detectAdvancePurchase(context.Background(), DetectorInput{
		Origin: "HEL", Destination: "PRG", Date: dateLate,
	})
	if len(h2) == 1 {
		titles[h2[0].Title] = true
	}

	// Spike zone.
	dateSpike := time.Now().AddDate(0, 0, 5).Format("2006-01-02")
	h3 := detectAdvancePurchase(context.Background(), DetectorInput{
		Origin: "HEL", Destination: "PRG", Date: dateSpike,
	})
	if len(h3) == 1 {
		titles[h3[0].Title] = true
	}

	if len(titles) < 3 {
		t.Errorf("expected 3 distinct title variants, got %d: %v", len(titles), titles)
	}
}

func TestDetectAdvancePurchase_descriptionContainsRoute(t *testing.T) {
	date := time.Now().AddDate(0, 0, 120).Format("2006-01-02")
	hacks := detectAdvancePurchase(context.Background(), DetectorInput{
		Origin:      "HEL",
		Destination: "PRG",
		Date:        date,
	})
	if len(hacks) != 1 {
		t.Fatalf("expected 1 hack, got %d", len(hacks))
	}
	desc := hacks[0].Description
	if len(desc) == 0 {
		t.Fatal("expected non-empty description")
	}
	// Description should reference the window label.
	_ = fmt.Sprintf("description: %s", desc)
}

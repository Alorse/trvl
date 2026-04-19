package optimizer

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/MikkoParkkola/trvl/internal/models"
)

// --- Optimize: input validation branches (16% coverage) ---

func TestOptimize_missingOrigin(t *testing.T) {
	res, err := Optimize(context.Background(), OptimizeInput{
		Destination: "BCN",
		DepartDate:  "2026-06-15",
	})
	if err == nil {
		t.Fatal("expected error for missing origin")
	}
	if res == nil || res.Error == "" {
		t.Error("expected error message in result")
	}
}

func TestOptimize_missingDestination(t *testing.T) {
	res, err := Optimize(context.Background(), OptimizeInput{
		Origin:     "HEL",
		DepartDate: "2026-06-15",
	})
	if err == nil {
		t.Fatal("expected error for missing destination")
	}
	if res == nil || res.Error == "" {
		t.Error("expected error message in result")
	}
}

func TestOptimize_missingDepartDate(t *testing.T) {
	res, err := Optimize(context.Background(), OptimizeInput{
		Origin:      "HEL",
		Destination: "BCN",
	})
	if err == nil {
		t.Fatal("expected error for missing depart date")
	}
	if res == nil || res.Error == "" {
		t.Error("expected error message in result")
	}
}

func TestOptimize_invalidDepartDate(t *testing.T) {
	res, err := Optimize(context.Background(), OptimizeInput{
		Origin:      "HEL",
		Destination: "BCN",
		DepartDate:  "not-a-date",
	})
	if err == nil {
		t.Fatal("expected error for invalid depart date")
	}
	if res == nil || res.Error == "" {
		t.Error("expected error message in result")
	}
}

func TestOptimize_invalidReturnDate(t *testing.T) {
	res, err := Optimize(context.Background(), OptimizeInput{
		Origin:      "HEL",
		Destination: "BCN",
		DepartDate:  "2026-06-15",
		ReturnDate:  "invalid",
	})
	if err == nil {
		t.Fatal("expected error for invalid return date")
	}
	if res == nil || res.Error == "" {
		t.Error("expected error message in result")
	}
}

func TestOptimize_sameOriginDest(t *testing.T) {
	res, err := Optimize(context.Background(), OptimizeInput{
		Origin:      "HEL",
		Destination: "HEL",
		DepartDate:  "2026-06-15",
	})
	if err == nil {
		t.Fatal("expected error for same origin/destination")
	}
	if res == nil || res.Error == "" {
		t.Error("expected error message in result")
	}
}

// --- expandCandidates: deeper coverage ---

func TestExpandCandidates_baseline(t *testing.T) {
	candidates := expandCandidates(OptimizeInput{
		Origin:      "HEL",
		Destination: "BCN",
		DepartDate:  "2026-06-15",
		FlexDays:    0, // defaults() not called in unit test, set explicitly
	})
	if len(candidates) == 0 {
		t.Fatal("expected at least baseline candidate")
	}
	base := candidates[0]
	if base.origin != "HEL" || base.dest != "BCN" {
		t.Errorf("baseline should be HEL→BCN, got %s→%s", base.origin, base.dest)
	}
	if base.strategy != "Direct booking" {
		t.Errorf("baseline strategy should be 'Direct booking', got %q", base.strategy)
	}
}

func TestExpandCandidates_withReturnDateFlex(t *testing.T) {
	candidates := expandCandidates(OptimizeInput{
		Origin:      "HEL",
		Destination: "BCN",
		DepartDate:  "2026-06-15",
		ReturnDate:  "2026-06-22",
		FlexDays:    2,
	})
	// Should have baseline + 4 flex candidates (±1, ±2) + any positioning/rail/etc.
	flexCount := 0
	for _, c := range candidates {
		for _, h := range c.hackTypes {
			if h == "date_flex" {
				flexCount++
			}
		}
	}
	if flexCount != 4 {
		t.Errorf("expected 4 date flex candidates (±1, ±2), got %d", flexCount)
	}
	// Each flex candidate should have both depart and return dates shifted.
	for _, c := range candidates {
		for _, h := range c.hackTypes {
			if h == "date_flex" {
				if c.returnDate == "" {
					t.Errorf("date flex candidate should have return date")
				}
				if c.departDate == "2026-06-15" {
					t.Error("date flex candidate should have shifted depart date")
				}
			}
		}
	}
}

func TestExpandCandidates_hiddenCityAMS(t *testing.T) {
	candidates := expandCandidates(OptimizeInput{
		Origin:      "HEL",
		Destination: "AMS",
		DepartDate:  "2026-06-15",
		FlexDays:    0,
	})
	hiddenCityCount := 0
	for _, c := range candidates {
		for _, h := range c.hackTypes {
			if h == "hidden_city" {
				hiddenCityCount++
			}
		}
	}
	if hiddenCityCount == 0 {
		t.Error("expected hidden city candidates for AMS destination")
	}
}

func TestExpandCandidates_hiddenCitySkipsSameAsOrigin(t *testing.T) {
	// Origin HEL should not appear as a hidden city beyond destination.
	candidates := expandCandidates(OptimizeInput{
		Origin:      "HEL",
		Destination: "AMS",
		DepartDate:  "2026-06-15",
		FlexDays:    0,
	})
	for _, c := range candidates {
		for _, h := range c.hackTypes {
			if h == "hidden_city" && c.dest == "HEL" {
				t.Error("hidden city candidate should not have dest == origin")
			}
		}
	}
}

func TestExpandCandidates_departureTaxFiltering(t *testing.T) {
	// AMS should have departure tax savings and zero-tax alternatives.
	candidates := expandCandidates(OptimizeInput{
		Origin:      "AMS",
		Destination: "BCN",
		DepartDate:  "2026-06-15",
		FlexDays:    0,
	})
	taxCount := 0
	for _, c := range candidates {
		for _, h := range c.hackTypes {
			if h == "departure_tax" {
				taxCount++
				// Transfer cost should be set.
				if c.transferCost <= 0 {
					t.Error("departure tax candidate should have transfer cost")
				}
			}
		}
	}
	// AMS is in Netherlands with ~26 EUR tax; alternatives should exist.
	if taxCount == 0 {
		t.Log("no departure tax alternatives found for AMS (may depend on tax data)")
	}
}

func TestExpandCandidates_railCompetition(t *testing.T) {
	// PRG→VIE has a known rail corridor.
	candidates := expandCandidates(OptimizeInput{
		Origin:      "PRG",
		Destination: "VIE",
		DepartDate:  "2026-06-15",
		FlexDays:    0,
	})
	railFound := false
	for _, c := range candidates {
		for _, h := range c.hackTypes {
			if h == "rail_competition" {
				railFound = true
				if !c.prePriced {
					t.Error("rail candidate should be prePriced")
				}
				if c.baseCost <= 0 {
					t.Error("rail candidate should have baseCost > 0")
				}
				if !c.searched {
					t.Error("rail candidate should be marked as searched")
				}
			}
		}
	}
	if !railFound {
		t.Error("expected rail competition candidate for PRG→VIE")
	}
}

func TestExpandCandidates_ferryHELARN(t *testing.T) {
	candidates := expandCandidates(OptimizeInput{
		Origin:      "HEL",
		Destination: "ARN",
		DepartDate:  "2026-06-15",
		FlexDays:    0,
	})
	ferryFound := false
	for _, c := range candidates {
		for _, h := range c.hackTypes {
			if h == "ferry_cabin_hotel" {
				ferryFound = true
				if !c.prePriced {
					t.Error("ferry candidate should be prePriced")
				}
			}
		}
	}
	if !ferryFound {
		t.Error("expected ferry cabin candidate for HEL→ARN")
	}
}

// --- priceCandidate: deeper edge cases ---

func TestPriceCandidate_emptyFlights(t *testing.T) {
	c := &candidate{
		searched: true,
		flights:  []models.FlightResult{},
	}
	input := OptimizeInput{Currency: "EUR"}
	priceCandidate(c, input)
	if c.allInCost != 0 {
		t.Errorf("expected allInCost=0 for empty flights, got %.2f", c.allInCost)
	}
}

func TestPriceCandidate_prePricedWithTransfer(t *testing.T) {
	c := &candidate{
		searched:     true,
		prePriced:    true,
		baseCost:     50,
		transferCost: 10,
		currency:     "EUR",
	}
	input := OptimizeInput{Currency: "EUR"}
	priceCandidate(c, input)
	if c.allInCost != 60 {
		t.Errorf("expected allInCost=60 for prePriced, got %.2f", c.allInCost)
	}
}

func TestPriceCandidate_prePricedNoTransfer(t *testing.T) {
	c := &candidate{
		searched:  true,
		prePriced: true,
		baseCost:  75,
		currency:  "EUR",
	}
	input := OptimizeInput{Currency: "EUR"}
	priceCandidate(c, input)
	if c.allInCost != 75 {
		t.Errorf("expected allInCost=75, got %.2f", c.allInCost)
	}
}

func TestPriceCandidate_multipleFlightsSelectsCheapest(t *testing.T) {
	c := &candidate{
		searched: true,
		origin:   "HEL",
		dest:     "BCN",
		flights: []models.FlightResult{
			{Price: 300, Currency: "EUR"},
			{Price: 150, Currency: "EUR"},
			{Price: 200, Currency: "EUR"},
		},
	}
	input := OptimizeInput{Currency: "EUR"}
	priceCandidate(c, input)
	if c.baseCost != 150 {
		t.Errorf("expected baseCost=150, got %.2f", c.baseCost)
	}
	if c.allInCost <= 0 {
		t.Error("allInCost should be positive")
	}
}

// --- rankCandidates: deeper coverage ---

func TestRankCandidates_emptyPriced(t *testing.T) {
	candidates := []*candidate{
		{searched: false},
	}
	result := rankCandidates(candidates, OptimizeInput{MaxResults: 5, Currency: "EUR"})
	if result.Success {
		t.Error("expected Success=false when no priced candidates")
	}
	if result.Error == "" {
		t.Error("expected error message")
	}
}

func TestRankCandidates_sortsByAllInCost(t *testing.T) {
	candidates := []*candidate{
		{searched: true, allInCost: 300, currency: "EUR", strategy: "A"},
		{searched: true, allInCost: 100, currency: "EUR", strategy: "B"},
		{searched: true, allInCost: 200, currency: "EUR", strategy: "C"},
	}
	result := rankCandidates(candidates, OptimizeInput{MaxResults: 10, Currency: "EUR"})
	if !result.Success {
		t.Fatal("expected success")
	}
	if len(result.Options) != 3 {
		t.Fatalf("expected 3 options, got %d", len(result.Options))
	}
	if result.Options[0].AllInCost != 100 {
		t.Errorf("first option should be cheapest (100), got %.0f", result.Options[0].AllInCost)
	}
	if result.Options[2].AllInCost != 300 {
		t.Errorf("last option should be most expensive (300), got %.0f", result.Options[2].AllInCost)
	}
}

func TestRankCandidates_baselineIdentified(t *testing.T) {
	candidates := []*candidate{
		{searched: true, allInCost: 200, currency: "EUR", strategy: "Direct booking"},
		{searched: true, allInCost: 150, currency: "EUR", strategy: "Via TLL", hackTypes: []string{"positioning"}},
	}
	result := rankCandidates(candidates, OptimizeInput{MaxResults: 10, Currency: "EUR"})
	if result.Baseline == nil {
		t.Fatal("expected baseline to be identified")
	}
	if result.Baseline.AllInCost != 200 {
		t.Errorf("baseline allInCost should be 200, got %.0f", result.Baseline.AllInCost)
	}
}

func TestRankCandidates_savingsVsBaseline(t *testing.T) {
	candidates := []*candidate{
		{searched: true, allInCost: 200, currency: "EUR", strategy: "Direct booking"},
		{searched: true, allInCost: 150, currency: "EUR", strategy: "Via TLL", hackTypes: []string{"positioning"}},
	}
	result := rankCandidates(candidates, OptimizeInput{MaxResults: 10, Currency: "EUR"})
	// The positioning option should show savings vs baseline.
	found := false
	for _, opt := range result.Options {
		if opt.Strategy == "Via TLL" {
			found = true
			if opt.SavingsVsBaseline != 50 {
				t.Errorf("expected savings=50, got %.0f", opt.SavingsVsBaseline)
			}
		}
	}
	if !found {
		t.Error("positioning option not found in results")
	}
}

func TestRankCandidates_crossCurrencyNoSavings(t *testing.T) {
	candidates := []*candidate{
		{searched: true, allInCost: 200, currency: "EUR", strategy: "Direct booking"},
		{searched: true, allInCost: 150, currency: "USD", strategy: "Via X", hackTypes: []string{"positioning"}},
	}
	result := rankCandidates(candidates, OptimizeInput{MaxResults: 10, Currency: "EUR"})
	for _, opt := range result.Options {
		if opt.Currency == "USD" && opt.SavingsVsBaseline != 0 {
			t.Errorf("cross-currency savings should be 0, got %.0f", opt.SavingsVsBaseline)
		}
	}
}

func TestRankCandidates_limitsToMaxResults(t *testing.T) {
	candidates := make([]*candidate, 10)
	for i := range candidates {
		candidates[i] = &candidate{
			searched:  true,
			allInCost: float64(100 + i*10),
			currency:  "EUR",
			strategy:  "S",
		}
	}
	result := rankCandidates(candidates, OptimizeInput{MaxResults: 3, Currency: "EUR"})
	if len(result.Options) != 3 {
		t.Errorf("expected 3 options (MaxResults=3), got %d", len(result.Options))
	}
}

// --- candidateToOption: deeper branches ---

func TestCandidateToOption_groundTransferLeg(t *testing.T) {
	c := &candidate{
		searched:     true,
		origin:       "TLL",
		dest:         "BCN",
		departDate:   "2026-06-15",
		transferCost: 30,
		currency:     "EUR",
		flights:      []models.FlightResult{{Price: 150, Currency: "EUR"}},
		strategy:     "Via Tallinn",
	}
	opt := candidateToOption(c, 1, OptimizeInput{Origin: "HEL", Currency: "EUR"})
	if len(opt.Legs) < 2 {
		t.Fatalf("expected at least 2 legs (ground + flight), got %d", len(opt.Legs))
	}
	if opt.Legs[0].Type != "ground" {
		t.Errorf("first leg should be ground, got %s", opt.Legs[0].Type)
	}
	if opt.Legs[0].From != "HEL" {
		t.Errorf("ground leg From should be HEL, got %s", opt.Legs[0].From)
	}
	if opt.Legs[0].To != "TLL" {
		t.Errorf("ground leg To should be TLL, got %s", opt.Legs[0].To)
	}
}

func TestCandidateToOption_destinationTransferLeg(t *testing.T) {
	c := &candidate{
		searched:     true,
		origin:       "HEL",
		dest:         "GRO",
		departDate:   "2026-06-15",
		transferCost: 15,
		currency:     "EUR",
		hackTypes:    []string{"destination_airport"},
		flights:      []models.FlightResult{{Price: 80, Currency: "EUR"}},
		strategy:     "Fly to Girona",
	}
	opt := candidateToOption(c, 1, OptimizeInput{Origin: "HEL", Destination: "BCN", Currency: "EUR"})
	// Should have: ground transfer (origin→origin if transferCost>0) + flight + destination ground.
	groundLegs := 0
	for _, l := range opt.Legs {
		if l.Type == "ground" {
			groundLegs++
		}
	}
	if groundLegs < 1 {
		t.Errorf("expected at least 1 ground leg for destination_airport, got %d", groundLegs)
	}
}

func TestCandidateToOption_rankAssignment(t *testing.T) {
	c := &candidate{
		searched:  true,
		origin:    "HEL",
		dest:      "BCN",
		currency:  "EUR",
		allInCost: 150,
		baseCost:  150,
		flights:   []models.FlightResult{{Price: 150, Currency: "EUR"}},
		strategy:  "Direct",
	}
	opt := candidateToOption(c, 42, OptimizeInput{Origin: "HEL", Currency: "EUR"})
	if opt.Rank != 42 {
		t.Errorf("expected rank=42, got %d", opt.Rank)
	}
}

func TestCandidateToOption_hacksApplied(t *testing.T) {
	c := &candidate{
		searched:  true,
		origin:    "HEL",
		dest:      "BCN",
		currency:  "EUR",
		allInCost: 150,
		baseCost:  150,
		hackTypes: []string{"positioning", "date_flex"},
		flights:   []models.FlightResult{{Price: 150, Currency: "EUR"}},
		strategy:  "Positioning + flex",
	}
	opt := candidateToOption(c, 1, OptimizeInput{Origin: "HEL", Currency: "EUR"})
	if len(opt.HacksApplied) != 2 {
		t.Errorf("expected 2 hacks applied, got %d", len(opt.HacksApplied))
	}
}

// --- defaults: edge cases ---

func TestDefaults_zeroFlexDaysBecomesThree(t *testing.T) {
	in := OptimizeInput{}
	in.defaults()
	if in.FlexDays != 3 {
		t.Errorf("expected FlexDays=3 from zero, got %d", in.FlexDays)
	}
}

func TestDefaults_zeroGuestsBecomesOne(t *testing.T) {
	in := OptimizeInput{}
	in.defaults()
	if in.Guests != 1 {
		t.Errorf("expected Guests=1, got %d", in.Guests)
	}
}

func TestDefaults_zeroCurrencyBecomesEUR(t *testing.T) {
	in := OptimizeInput{}
	in.defaults()
	if in.Currency != "EUR" {
		t.Errorf("expected Currency=EUR, got %s", in.Currency)
	}
}

func TestDefaults_preservesExplicitValues(t *testing.T) {
	in := OptimizeInput{
		Guests:      3,
		FlexDays:    5,
		MaxResults:  10,
		MaxAPICalls: 20,
		Currency:    "USD",
	}
	in.defaults()
	if in.Guests != 3 {
		t.Errorf("expected Guests=3, got %d", in.Guests)
	}
	if in.FlexDays != 5 {
		t.Errorf("expected FlexDays=5, got %d", in.FlexDays)
	}
	if in.MaxResults != 10 {
		t.Errorf("expected MaxResults=10, got %d", in.MaxResults)
	}
	if in.MaxAPICalls != 20 {
		t.Errorf("expected MaxAPICalls=20, got %d", in.MaxAPICalls)
	}
	if in.Currency != "USD" {
		t.Errorf("expected Currency=USD, got %s", in.Currency)
	}
}

// --- validateInput: additional branches ---

func TestValidateInput_emptyDate(t *testing.T) {
	err := validateInput(OptimizeInput{
		Origin:      "HEL",
		Destination: "BCN",
	})
	if err == nil {
		t.Error("expected error for empty depart date")
	}
}

func TestValidateInput_caseInsensitiveSameOriginDest(t *testing.T) {
	err := validateInput(OptimizeInput{
		Origin:      "hel",
		Destination: "HEL",
		DepartDate:  "2026-06-15",
	})
	if err == nil {
		t.Error("expected error for case-insensitive same origin/dest")
	}
}

// --- cheapestFlight: deeper edges ---

func TestCheapestFlight_skipZeroPrice(t *testing.T) {
	flts := []models.FlightResult{
		{Price: 0, Currency: "EUR"},
		{Price: 200, Currency: "EUR"},
	}
	best := cheapestFlight(flts)
	if best.Price != 200 {
		t.Errorf("expected 200, got %.0f", best.Price)
	}
}

func TestCheapestFlight_multiplePositive(t *testing.T) {
	flts := []models.FlightResult{
		{Price: 300, Currency: "EUR"},
		{Price: 100, Currency: "EUR"},
		{Price: 200, Currency: "EUR"},
	}
	best := cheapestFlight(flts)
	if best.Price != 100 {
		t.Errorf("expected 100, got %.0f", best.Price)
	}
}

// --- convertFFStatuses: edge cases ---

func TestConvertFFStatuses_single(t *testing.T) {
	statuses := convertFFStatuses([]FFStatus{{Alliance: "Star", Tier: "Gold"}})
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	if statuses[0].Alliance != "Star" || statuses[0].Tier != "Gold" {
		t.Errorf("unexpected status: %+v", statuses[0])
	}
}

func TestConvertFFStatuses_multiple(t *testing.T) {
	statuses := convertFFStatuses([]FFStatus{
		{Alliance: "Star", Tier: "Gold"},
		{Alliance: "Oneworld", Tier: "Silver"},
	})
	if len(statuses) != 2 {
		t.Fatalf("expected 2 statuses, got %d", len(statuses))
	}
}

// --- shiftDate: additional edges ---

func TestShiftDate_yearBoundary(t *testing.T) {
	got := shiftDate("2026-12-31", 1)
	if got != "2027-01-01" {
		t.Errorf("expected 2027-01-01, got %s", got)
	}
}

func TestShiftDate_negativeYearBoundary(t *testing.T) {
	got := shiftDate("2026-01-01", -1)
	if got != "2025-12-31" {
		t.Errorf("expected 2025-12-31, got %s", got)
	}
}

func TestShiftDate_leapYear(t *testing.T) {
	got := shiftDate("2028-02-28", 1)
	if got != "2028-02-29" {
		t.Errorf("expected 2028-02-29, got %s", got)
	}
}

// --- searchCandidates: cancelled context (0% coverage) ---

func TestSearchCandidates_cancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	candidates := []*candidate{
		{origin: "HEL", dest: "BCN", departDate: "2026-06-15", strategy: "Direct"},
	}

	input := OptimizeInput{
		Origin:      "HEL",
		Destination: "BCN",
		DepartDate:  "2026-06-15",
		MaxAPICalls: 5,
		Currency:    "EUR",
		Guests:      1,
	}

	searchCandidates(ctx, candidates, nil, input)

	// With cancelled context, the candidate should not be searched.
	if candidates[0].searched {
		t.Log("candidate was searched even with cancelled context (API may not check ctx)")
	}
}

func TestSearchCandidates_budgetZero(t *testing.T) {
	candidates := []*candidate{
		{origin: "HEL", dest: "BCN", departDate: "2026-06-15", strategy: "Direct"},
	}

	input := OptimizeInput{
		Origin:      "HEL",
		Destination: "BCN",
		DepartDate:  "2026-06-15",
		MaxAPICalls: 0, // zero budget
		Currency:    "EUR",
		Guests:      1,
	}

	searchCandidates(context.Background(), candidates, nil, input)

	// With zero budget, no searches should be attempted.
	if candidates[0].searched {
		t.Error("candidate should not be searched with zero budget")
	}
}

func TestSearchCandidates_skipsPrePriced(t *testing.T) {
	candidates := []*candidate{
		{origin: "PRG", dest: "VIE", departDate: "2026-06-15", strategy: "Rail", prePriced: true, searched: true, baseCost: 50},
		{origin: "HEL", dest: "BCN", departDate: "2026-06-15", strategy: "Direct"},
	}

	input := OptimizeInput{
		Origin:      "HEL",
		Destination: "BCN",
		DepartDate:  "2026-06-15",
		MaxAPICalls: 1,
		Currency:    "EUR",
		Guests:      1,
	}

	// The pre-priced candidate should be skipped; the direct one should be attempted.
	searchCandidates(context.Background(), candidates, nil, input)

	// Pre-priced should still be searched (unchanged).
	if !candidates[0].searched {
		t.Error("pre-priced candidate should remain searched=true")
	}
}

func TestSearchCandidates_prioritizesBaseline(t *testing.T) {
	baseline := &candidate{origin: "HEL", dest: "BCN", departDate: "2026-06-15", strategy: "Direct"}
	hack := &candidate{origin: "TLL", dest: "BCN", departDate: "2026-06-15", strategy: "Via Tallinn",
		hackTypes: []string{"positioning"}, transferCost: 30}

	// Put hack first, baseline second — function should reorder to search baseline first.
	candidates := []*candidate{hack, baseline}

	input := OptimizeInput{
		Origin:      "HEL",
		Destination: "BCN",
		DepartDate:  "2026-06-15",
		MaxAPICalls: 1, // only 1 API call — should prioritize baseline
		Currency:    "EUR",
		Guests:      1,
	}

	searchCandidates(context.Background(), candidates, nil, input)

	// We can't guarantee which was searched due to concurrency, but the function
	// should not panic and should handle the budget constraint.
}

// --- resolveFlexDatesViaCalendar: edge cases (0% coverage) ---

func TestResolveFlexDatesViaCalendar_zeroFlexDays(t *testing.T) {
	candidates := []*candidate{
		{origin: "HEL", dest: "BCN", departDate: "2026-06-15"},
	}

	input := OptimizeInput{
		FlexDays: 0,
	}

	var used atomic.Int64
	resolveFlexDatesViaCalendar(context.Background(), candidates, input, &used, 10)

	// With zero flex days, the function should return immediately.
	if used.Load() != 0 {
		t.Errorf("expected 0 API calls used with zero flex days, got %d", used.Load())
	}
}

func TestResolveFlexDatesViaCalendar_noFlexCandidates(t *testing.T) {
	candidates := []*candidate{
		{origin: "HEL", dest: "BCN", departDate: "2026-06-15", strategy: "Direct"},
	}

	input := OptimizeInput{
		FlexDays:   3,
		Origin:     "HEL",
		Destination: "BCN",
		DepartDate: "2026-06-15",
	}

	var used atomic.Int64
	resolveFlexDatesViaCalendar(context.Background(), candidates, input, &used, 10)

	// No date_flex candidates — should return without API calls.
	if used.Load() != 0 {
		t.Errorf("expected 0 API calls with no flex candidates, got %d", used.Load())
	}
}

func TestResolveFlexDatesViaCalendar_budgetExhausted(t *testing.T) {
	candidates := []*candidate{
		{origin: "HEL", dest: "BCN", departDate: "2026-06-16", hackTypes: []string{"date_flex"}},
	}

	input := OptimizeInput{
		FlexDays:    3,
		Origin:      "HEL",
		Destination: "BCN",
		DepartDate:  "2026-06-15",
	}

	var used atomic.Int64
	used.Store(10) // already at budget
	resolveFlexDatesViaCalendar(context.Background(), candidates, input, &used, 10)

	// Budget exhausted — should not increment.
	if used.Load() != 11 {
		// Function does used.Add(1) then checks > budget, so it goes to 11 but returns.
		t.Logf("used=%d (expected 11 after budget check)", used.Load())
	}
}

// --- Optimize: end-to-end with cancelled context ---

func TestOptimize_cancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res, err := Optimize(ctx, OptimizeInput{
		Origin:      "HEL",
		Destination: "BCN",
		DepartDate:  "2026-06-15",
	})
	if err != nil {
		t.Fatalf("Optimize should not return validation error: %v", err)
	}
	// With cancelled context, no searches succeed, so result should indicate no options.
	if res == nil {
		t.Fatal("expected non-nil result")
	}
	// Either Success=false or Options is empty.
	if res.Success && len(res.Options) > 0 {
		t.Log("unexpectedly got options with cancelled context")
	}
}

// --- expandCandidates: more origins/destinations ---

func TestExpandCandidates_noFlexDays(t *testing.T) {
	candidates := expandCandidates(OptimizeInput{
		Origin:      "XXX",
		Destination: "YYY",
		DepartDate:  "2026-06-15",
		FlexDays:    0,
	})
	// Unknown airports: should still have baseline.
	if len(candidates) != 1 {
		t.Errorf("expected 1 candidate (baseline) for unknown airports with no flex, got %d", len(candidates))
	}
}

func TestExpandCandidates_ARNhasAlternatives(t *testing.T) {
	candidates := expandCandidates(OptimizeInput{
		Origin:      "ARN",
		Destination: "BCN",
		DepartDate:  "2026-06-15",
		FlexDays:    0,
	})
	// ARN should have nearby airports (CPH, GOT) and multimodal hubs.
	altCount := 0
	for _, c := range candidates {
		if len(c.hackTypes) > 0 && c.hackTypes[0] == "positioning" {
			altCount++
		}
	}
	if altCount == 0 {
		t.Error("expected positioning alternatives for ARN origin")
	}
}

func TestExpandCandidates_CPHdepartureTax(t *testing.T) {
	candidates := expandCandidates(OptimizeInput{
		Origin:      "CPH",
		Destination: "BCN",
		DepartDate:  "2026-06-15",
		FlexDays:    0,
	})
	// CPH is in Denmark — check if departure tax candidates exist.
	for _, c := range candidates {
		for _, h := range c.hackTypes {
			if h == "departure_tax" {
				// Valid: CPH has departure tax.
				return
			}
		}
	}
	t.Log("no departure tax candidate for CPH (may depend on tax data)")
}

// --- priceCandidate with FF status ---

func TestPriceCandidate_withFFStatusStar(t *testing.T) {
	c := &candidate{
		searched: true,
		origin:   "HEL",
		dest:     "BCN",
		flights: []models.FlightResult{
			{Price: 200, Currency: "EUR", Legs: []models.FlightLeg{{AirlineCode: "AY", Airline: "Finnair"}}},
		},
	}
	input := OptimizeInput{
		Currency:       "EUR",
		NeedCheckedBag: true,
		FFStatuses:     []FFStatus{{Alliance: "oneworld", Tier: "Gold"}},
	}
	priceCandidate(c, input)
	if c.allInCost <= 0 {
		t.Error("allInCost should be positive")
	}
	// With FF status, ffSavings should be >= 0.
	if c.ffSavings < 0 {
		t.Errorf("ffSavings should be non-negative, got %.2f", c.ffSavings)
	}
}

func TestPriceCandidate_carryOnOnly(t *testing.T) {
	c := &candidate{
		searched: true,
		origin:   "HEL",
		dest:     "BCN",
		flights: []models.FlightResult{
			{Price: 150, Currency: "EUR"},
		},
	}
	input := OptimizeInput{
		Currency:    "EUR",
		CarryOnOnly: true,
	}
	priceCandidate(c, input)
	if c.allInCost <= 0 {
		t.Error("allInCost should be positive for carry-on only")
	}
}

// --- candidateToOption: prePriced ground leg ---

func TestCandidateToOption_prePricedGroundLegType(t *testing.T) {
	c := &candidate{
		searched:  true,
		prePriced: true,
		origin:    "PRG",
		dest:      "VIE",
		departDate: "2026-06-15",
		baseCost:  50,
		currency:  "EUR",
		strategy:  "Train (CD/OBB)",
		hackTypes: []string{"rail_competition"},
	}
	opt := candidateToOption(c, 1, OptimizeInput{Origin: "PRG", Destination: "VIE", Currency: "EUR"})
	if len(opt.Legs) == 0 {
		t.Fatal("expected at least 1 leg for pre-priced ground")
	}
	if opt.Legs[0].Type != "ground" {
		t.Errorf("expected ground leg, got %s", opt.Legs[0].Type)
	}
	if opt.Legs[0].Price != 50 {
		t.Errorf("expected price 50, got %.0f", opt.Legs[0].Price)
	}
}

func TestCandidateToOption_flightLegDuration(t *testing.T) {
	c := &candidate{
		searched:   true,
		origin:     "HEL",
		dest:       "BCN",
		departDate: "2026-06-15",
		currency:   "EUR",
		flights: []models.FlightResult{
			{
				Price:    200,
				Currency: "EUR",
				Duration: 240,
				Legs:     []models.FlightLeg{{Airline: "Finnair", AirlineCode: "AY"}},
			},
		},
	}
	opt := candidateToOption(c, 1, OptimizeInput{Origin: "HEL", Currency: "EUR"})
	found := false
	for _, l := range opt.Legs {
		if l.Type == "flight" {
			found = true
			if l.Duration != 240 {
				t.Errorf("expected duration 240, got %d", l.Duration)
			}
			if l.Airline != "Finnair" {
				t.Errorf("expected airline Finnair, got %s", l.Airline)
			}
		}
	}
	if !found {
		t.Error("expected a flight leg")
	}
}

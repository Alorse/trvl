package optimizer

import (
	"testing"

	"github.com/MikkoParkkola/trvl/internal/models"
)

func TestValidateInput_missing_origin(t *testing.T) {
	err := validateInput(OptimizeInput{
		Destination: "BCN",
		DepartDate:  "2026-06-01",
	})
	if err == nil {
		t.Fatal("expected error for missing origin")
	}
}

func TestValidateInput_missing_destination(t *testing.T) {
	err := validateInput(OptimizeInput{
		Origin:     "HEL",
		DepartDate: "2026-06-01",
	})
	if err == nil {
		t.Fatal("expected error for missing destination")
	}
}

func TestValidateInput_missing_depart_date(t *testing.T) {
	err := validateInput(OptimizeInput{
		Origin:      "HEL",
		Destination: "BCN",
	})
	if err == nil {
		t.Fatal("expected error for missing departure date")
	}
}

func TestValidateInput_invalid_depart_date(t *testing.T) {
	err := validateInput(OptimizeInput{
		Origin:      "HEL",
		Destination: "BCN",
		DepartDate:  "not-a-date",
	})
	if err == nil {
		t.Fatal("expected error for invalid departure date")
	}
}

func TestValidateInput_invalid_return_date(t *testing.T) {
	err := validateInput(OptimizeInput{
		Origin:      "HEL",
		Destination: "BCN",
		DepartDate:  "2026-06-01",
		ReturnDate:  "bad",
	})
	if err == nil {
		t.Fatal("expected error for invalid return date")
	}
}

func TestValidateInput_same_origin_dest(t *testing.T) {
	err := validateInput(OptimizeInput{
		Origin:      "HEL",
		Destination: "HEL",
		DepartDate:  "2026-06-01",
	})
	if err == nil {
		t.Fatal("expected error when origin == destination")
	}
}

func TestValidateInput_same_origin_dest_case_insensitive(t *testing.T) {
	err := validateInput(OptimizeInput{
		Origin:      "hel",
		Destination: "HEL",
		DepartDate:  "2026-06-01",
	})
	if err == nil {
		t.Fatal("expected error when origin == destination (case insensitive)")
	}
}

func TestValidateInput_valid(t *testing.T) {
	err := validateInput(OptimizeInput{
		Origin:      "HEL",
		Destination: "BCN",
		DepartDate:  "2026-06-01",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateInput_valid_roundtrip(t *testing.T) {
	err := validateInput(OptimizeInput{
		Origin:      "HEL",
		Destination: "BCN",
		DepartDate:  "2026-06-01",
		ReturnDate:  "2026-06-08",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDefaults(t *testing.T) {
	in := OptimizeInput{
		Origin:      "HEL",
		Destination: "BCN",
		DepartDate:  "2026-06-01",
	}
	in.defaults()

	if in.Guests != 1 {
		t.Errorf("expected Guests=1, got %d", in.Guests)
	}
	if in.FlexDays != 3 {
		t.Errorf("expected FlexDays=3, got %d", in.FlexDays)
	}
	if in.MaxResults != 5 {
		t.Errorf("expected MaxResults=5, got %d", in.MaxResults)
	}
	if in.MaxAPICalls != 15 {
		t.Errorf("expected MaxAPICalls=15, got %d", in.MaxAPICalls)
	}
	if in.Currency != "EUR" {
		t.Errorf("expected Currency=EUR, got %s", in.Currency)
	}
}

func TestDefaults_preserves_explicit(t *testing.T) {
	in := OptimizeInput{
		Origin:      "HEL",
		Destination: "BCN",
		DepartDate:  "2026-06-01",
		Guests:      2,
		FlexDays:    5,
		MaxResults:  10,
		MaxAPICalls: 20,
		Currency:    "USD",
	}
	in.defaults()

	if in.Guests != 2 {
		t.Errorf("expected Guests=2, got %d", in.Guests)
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

func TestExpandCandidates_baseline_always_present(t *testing.T) {
	input := OptimizeInput{
		Origin:      "HEL",
		Destination: "BCN",
		DepartDate:  "2026-06-01",
	}
	input.defaults()
	candidates := expandCandidates(input)

	if len(candidates) == 0 {
		t.Fatal("expected at least baseline candidate")
	}

	baseline := candidates[0]
	if baseline.origin != "HEL" {
		t.Errorf("baseline origin: got %s, want HEL", baseline.origin)
	}
	if baseline.dest != "BCN" {
		t.Errorf("baseline dest: got %s, want BCN", baseline.dest)
	}
	if baseline.strategy != "Direct booking" {
		t.Errorf("baseline strategy: got %q, want %q", baseline.strategy, "Direct booking")
	}
	if len(baseline.hackTypes) != 0 {
		t.Errorf("baseline should have no hackTypes, got %v", baseline.hackTypes)
	}
}

func TestExpandCandidates_HEL_has_alternatives(t *testing.T) {
	input := OptimizeInput{
		Origin:      "HEL",
		Destination: "BCN",
		DepartDate:  "2026-06-01",
	}
	input.defaults()
	candidates := expandCandidates(input)

	// HEL has nearby airports (TLL, RIX, VNO) and multimodal hubs (TLL, RIX, ARN).
	// Must have more than just the baseline.
	if len(candidates) < 3 {
		t.Errorf("expected at least 3 candidates for HEL->BCN, got %d", len(candidates))
	}

	// Check that TLL appears as a positioning alternative.
	found := false
	for _, c := range candidates {
		if c.origin == "TLL" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected TLL as alternative origin for HEL")
	}
}

func TestExpandCandidates_BCN_destination_alternatives(t *testing.T) {
	input := OptimizeInput{
		Origin:      "HEL",
		Destination: "BCN",
		DepartDate:  "2026-06-01",
	}
	input.defaults()
	candidates := expandCandidates(input)

	// BCN has destination alternative: GRO (Girona).
	found := false
	for _, c := range candidates {
		if c.dest == "GRO" {
			found = true
			if c.transferCost <= 0 {
				t.Error("expected transfer cost for GRO destination alternative")
			}
			break
		}
	}
	if !found {
		t.Error("expected GRO as alternative destination for BCN")
	}
}

func TestExpandCandidates_AMS_rail_fly(t *testing.T) {
	input := OptimizeInput{
		Origin:      "AMS",
		Destination: "BCN",
		DepartDate:  "2026-06-01",
	}
	input.defaults()
	candidates := expandCandidates(input)

	// AMS is a KLM hub with rail+fly stations: ZWE (Antwerp), ZYR (Brussels-Midi).
	foundZWE := false
	for _, c := range candidates {
		if c.origin == "ZWE" {
			foundZWE = true
			if c.transferCost != 0 {
				t.Errorf("rail+fly transfer cost should be 0 (included in ticket), got %f", c.transferCost)
			}
			hackFound := false
			for _, h := range c.hackTypes {
				if h == "rail_fly_arbitrage" {
					hackFound = true
				}
			}
			if !hackFound {
				t.Error("expected rail_fly_arbitrage hack type for ZWE")
			}
			break
		}
	}
	if !foundZWE {
		t.Error("expected ZWE as rail+fly origin for AMS hub")
	}
}

func TestExpandCandidates_unknown_origin_unknown_dest(t *testing.T) {
	input := OptimizeInput{
		Origin:      "XYZ",
		Destination: "QQQ",
		DepartDate:  "2026-06-01",
	}
	input.defaults()
	candidates := expandCandidates(input)

	// Unknown origin + unknown destination: baseline + date-flex candidates only.
	// FlexDays defaults to 3, so we get 1 baseline + 6 date-flex = 7.
	if len(candidates) != 7 {
		t.Errorf("expected 7 candidates for unknown origin+dest (1 baseline + 6 date-flex), got %d", len(candidates))
	}
}

func TestExpandCandidates_skips_self_referential(t *testing.T) {
	input := OptimizeInput{
		Origin:      "HEL",
		Destination: "TLL",
		DepartDate:  "2026-06-01",
	}
	input.defaults()
	candidates := expandCandidates(input)

	// TLL should not appear as alternative origin when TLL is the destination.
	for _, c := range candidates {
		if c.origin == "TLL" && c.dest == "TLL" {
			t.Error("candidate should not have same origin and destination")
		}
	}
}

func TestPriceCandidate_no_flights(t *testing.T) {
	c := &candidate{searched: true}
	input := OptimizeInput{}
	input.defaults()
	priceCandidate(c, input)

	if c.allInCost != 0 {
		t.Errorf("expected allInCost=0 for no flights, got %f", c.allInCost)
	}
}

func TestPriceCandidate_with_transfer_cost(t *testing.T) {
	c := &candidate{
		searched:     true,
		transferCost: 30,
		flights: []models.FlightResult{
			{Price: 100, Currency: "EUR"},
		},
	}
	input := OptimizeInput{}
	input.defaults()
	priceCandidate(c, input)

	// AllInCost should include transfer cost.
	if c.allInCost < 130 {
		t.Errorf("expected allInCost >= 130 (100 flight + 30 transfer), got %f", c.allInCost)
	}
}

func TestPriceCandidate_selects_cheapest(t *testing.T) {
	c := &candidate{
		searched: true,
		flights: []models.FlightResult{
			{Price: 200, Currency: "EUR"},
			{Price: 150, Currency: "EUR"},
			{Price: 180, Currency: "EUR"},
		},
	}
	input := OptimizeInput{}
	input.defaults()
	priceCandidate(c, input)

	if c.baseCost != 150 {
		t.Errorf("expected baseCost=150 (cheapest), got %f", c.baseCost)
	}
}

func TestRankCandidates_sorts_by_allInCost(t *testing.T) {
	candidates := []*candidate{
		{searched: true, allInCost: 200, baseCost: 200, currency: "EUR", strategy: "A"},
		{searched: true, allInCost: 100, baseCost: 100, currency: "EUR", strategy: "B"},
		{searched: true, allInCost: 150, baseCost: 150, currency: "EUR", strategy: "C"},
	}

	input := OptimizeInput{MaxResults: 3}
	result := rankCandidates(candidates, input)

	if !result.Success {
		t.Fatal("expected success")
	}
	if len(result.Options) != 3 {
		t.Fatalf("expected 3 options, got %d", len(result.Options))
	}
	if result.Options[0].AllInCost != 100 {
		t.Errorf("rank 1 allInCost: want 100, got %f", result.Options[0].AllInCost)
	}
	if result.Options[1].AllInCost != 150 {
		t.Errorf("rank 2 allInCost: want 150, got %f", result.Options[1].AllInCost)
	}
	if result.Options[2].AllInCost != 200 {
		t.Errorf("rank 3 allInCost: want 200, got %f", result.Options[2].AllInCost)
	}
}

func TestRankCandidates_limits_results(t *testing.T) {
	candidates := []*candidate{
		{searched: true, allInCost: 300, baseCost: 300, currency: "EUR", strategy: "A"},
		{searched: true, allInCost: 100, baseCost: 100, currency: "EUR", strategy: "B"},
		{searched: true, allInCost: 200, baseCost: 200, currency: "EUR", strategy: "C"},
	}

	input := OptimizeInput{MaxResults: 2}
	result := rankCandidates(candidates, input)

	if len(result.Options) != 2 {
		t.Fatalf("expected 2 options (MaxResults=2), got %d", len(result.Options))
	}
}

func TestRankCandidates_baseline_savings(t *testing.T) {
	candidates := []*candidate{
		{searched: true, allInCost: 200, baseCost: 200, currency: "EUR", strategy: "Direct"},
		{searched: true, allInCost: 150, baseCost: 150, currency: "EUR", strategy: "Alt", hackTypes: []string{"positioning"}},
	}

	input := OptimizeInput{MaxResults: 5}
	result := rankCandidates(candidates, input)

	if !result.Success {
		t.Fatal("expected success")
	}
	if result.Baseline == nil {
		t.Fatal("expected baseline to be set")
	}
	if result.Baseline.AllInCost != 200 {
		t.Errorf("baseline allInCost: want 200, got %f", result.Baseline.AllInCost)
	}

	// The cheapest option (150) should show savings of 50 vs baseline (200).
	if result.Options[0].SavingsVsBaseline != 50 {
		t.Errorf("savings vs baseline: want 50, got %f", result.Options[0].SavingsVsBaseline)
	}
}

func TestRankCandidates_no_results(t *testing.T) {
	candidates := []*candidate{
		{searched: false}, // not searched
	}

	input := OptimizeInput{MaxResults: 5}
	result := rankCandidates(candidates, input)

	if result.Success {
		t.Error("expected failure when no results")
	}
	if result.Error == "" {
		t.Error("expected error message")
	}
}

func TestRankCandidates_assigns_ranks(t *testing.T) {
	candidates := []*candidate{
		{searched: true, allInCost: 200, baseCost: 200, currency: "EUR", strategy: "A"},
		{searched: true, allInCost: 100, baseCost: 100, currency: "EUR", strategy: "B"},
	}

	input := OptimizeInput{MaxResults: 5}
	result := rankCandidates(candidates, input)

	for i, opt := range result.Options {
		if opt.Rank != i+1 {
			t.Errorf("option %d rank: want %d, got %d", i, i+1, opt.Rank)
		}
	}
}

func TestCandidateToOption_legs(t *testing.T) {
	c := &candidate{
		origin:       "TLL",
		dest:         "BCN",
		departDate:   "2026-06-01",
		strategy:     "Fly from Tallinn",
		hackTypes:    []string{"positioning"},
		transferCost: 30,
		baseCost:     150,
		currency:     "EUR",
		allInCost:    180,
		flights: []models.FlightResult{
			{Price: 150, Currency: "EUR", Duration: 240},
		},
	}

	input := OptimizeInput{Origin: "HEL", Destination: "BCN"}
	opt := candidateToOption(c, 1, input)

	if len(opt.Legs) < 2 {
		t.Fatalf("expected at least 2 legs (ground + flight), got %d", len(opt.Legs))
	}

	if opt.Legs[0].Type != "ground" {
		t.Errorf("first leg type: want ground, got %s", opt.Legs[0].Type)
	}
	if opt.Legs[0].From != "HEL" {
		t.Errorf("first leg from: want HEL, got %s", opt.Legs[0].From)
	}
	if opt.Legs[0].To != "TLL" {
		t.Errorf("first leg to: want TLL, got %s", opt.Legs[0].To)
	}

	if opt.Legs[1].Type != "flight" {
		t.Errorf("second leg type: want flight, got %s", opt.Legs[1].Type)
	}
}

func TestConvertFFStatuses(t *testing.T) {
	statuses := []FFStatus{
		{Alliance: "skyteam", Tier: "gold"},
		{Alliance: "oneworld", Tier: "sapphire"},
	}
	converted := convertFFStatuses(statuses)

	if len(converted) != 2 {
		t.Fatalf("expected 2 converted statuses, got %d", len(converted))
	}
	if converted[0].Alliance != "skyteam" || converted[0].Tier != "gold" {
		t.Errorf("first status: got %+v", converted[0])
	}
	if converted[1].Alliance != "oneworld" || converted[1].Tier != "sapphire" {
		t.Errorf("second status: got %+v", converted[1])
	}
}

func TestCheapestFlight(t *testing.T) {
	flights := []models.FlightResult{
		{Price: 200, Currency: "EUR"},
		{Price: 0, Currency: "EUR"},   // invalid
		{Price: 150, Currency: "EUR"},
		{Price: 180, Currency: "EUR"},
	}
	best := cheapestFlight(flights)
	if best.Price != 150 {
		t.Errorf("cheapestFlight: want 150, got %f", best.Price)
	}
}

func TestOptimize_validation_error(t *testing.T) {
	result, err := Optimize(t.Context(), OptimizeInput{})
	if err == nil {
		t.Fatal("expected error for empty input")
	}
	if result == nil {
		t.Fatal("expected non-nil result even on error")
	}
	if result.Error == "" {
		t.Error("expected error message in result")
	}
}

func TestExpandCandidates_departure_tax_AMS(t *testing.T) {
	// AMS is in NL (€26 tax). Some nearby airports may be in zero-tax countries.
	input := OptimizeInput{
		Origin:      "AMS",
		Destination: "BCN",
		DepartDate:  "2026-06-01",
	}
	input.defaults()
	candidates := expandCandidates(input)

	// Check for departure_tax + positioning candidates.
	var taxCandidates []*candidate
	for _, c := range candidates {
		for _, h := range c.hackTypes {
			if h == "departure_tax" {
				taxCandidates = append(taxCandidates, c)
				break
			}
		}
	}

	// Whether we get candidates depends on whether any zero-tax alternative
	// has ground cost < €26. We verify the ones that appear are correct.
	for _, c := range taxCandidates {
		if c.origin == c.dest {
			t.Errorf("departure_tax candidate has same origin and dest: %s", c.origin)
		}
		// Must have both departure_tax and positioning hack types.
		hasTax, hasPos := false, false
		for _, h := range c.hackTypes {
			if h == "departure_tax" {
				hasTax = true
			}
			if h == "positioning" {
				hasPos = true
			}
		}
		if !hasTax || !hasPos {
			t.Errorf("departure_tax candidate %s missing hack types: tax=%v pos=%v", c.origin, hasTax, hasPos)
		}
	}
}

func TestExpandCandidates_rail_competition(t *testing.T) {
	// MAD→BCN is a competitive rail corridor.
	input := OptimizeInput{
		Origin:      "MAD",
		Destination: "BCN",
		DepartDate:  "2026-06-01",
		FlexDays:    -1, // disable date flex to simplify
	}
	input.defaults()
	candidates := expandCandidates(input)

	var railCandidate *candidate
	for _, c := range candidates {
		for _, h := range c.hackTypes {
			if h == "rail_competition" {
				railCandidate = c
				break
			}
		}
		if railCandidate != nil {
			break
		}
	}

	if railCandidate == nil {
		t.Fatal("expected rail_competition candidate for MAD→BCN")
	}
	if !railCandidate.prePriced {
		t.Error("rail_competition candidate should be prePriced")
	}
	if !railCandidate.searched {
		t.Error("rail_competition candidate should be marked searched")
	}
	if railCandidate.baseCost != 7 {
		t.Errorf("rail baseCost: got %.0f, want 7", railCandidate.baseCost)
	}

	// Verify both hack types.
	hasRail, hasGround := false, false
	for _, h := range railCandidate.hackTypes {
		if h == "rail_competition" {
			hasRail = true
		}
		if h == "ground_alternative" {
			hasGround = true
		}
	}
	if !hasRail || !hasGround {
		t.Errorf("expected hackTypes [rail_competition, ground_alternative], got %v", railCandidate.hackTypes)
	}
}

func TestExpandCandidates_ferry_cabin(t *testing.T) {
	// HEL→ARN has an overnight ferry.
	input := OptimizeInput{
		Origin:      "HEL",
		Destination: "ARN",
		DepartDate:  "2026-06-01",
		FlexDays:    -1, // disable date flex
	}
	input.defaults()
	candidates := expandCandidates(input)

	var ferryCandidate *candidate
	for _, c := range candidates {
		for _, h := range c.hackTypes {
			if h == "ferry_cabin_hotel" {
				ferryCandidate = c
				break
			}
		}
		if ferryCandidate != nil {
			break
		}
	}

	if ferryCandidate == nil {
		t.Fatal("expected ferry_cabin_hotel candidate for HEL→ARN")
	}
	if !ferryCandidate.prePriced {
		t.Error("ferry_cabin_hotel candidate should be prePriced")
	}
	if !ferryCandidate.searched {
		t.Error("ferry_cabin_hotel candidate should be marked searched")
	}
	if ferryCandidate.baseCost != 35 {
		t.Errorf("ferry baseCost: got %.0f, want 35", ferryCandidate.baseCost)
	}
}

func TestExpandCandidates_no_rail_for_non_corridor(t *testing.T) {
	input := OptimizeInput{
		Origin:      "HEL",
		Destination: "BCN",
		DepartDate:  "2026-06-01",
		FlexDays:    -1,
	}
	input.defaults()
	candidates := expandCandidates(input)

	for _, c := range candidates {
		for _, h := range c.hackTypes {
			if h == "rail_competition" {
				t.Error("unexpected rail_competition candidate for HEL→BCN")
			}
		}
	}
}

func TestExpandCandidates_no_ferry_for_non_route(t *testing.T) {
	input := OptimizeInput{
		Origin:      "HEL",
		Destination: "BCN",
		DepartDate:  "2026-06-01",
		FlexDays:    -1,
	}
	input.defaults()
	candidates := expandCandidates(input)

	for _, c := range candidates {
		for _, h := range c.hackTypes {
			if h == "ferry_cabin_hotel" {
				t.Error("unexpected ferry_cabin_hotel candidate for HEL→BCN")
			}
		}
	}
}

func TestPriceCandidate_prePriced(t *testing.T) {
	c := &candidate{
		searched:     true,
		prePriced:    true,
		baseCost:     7,
		transferCost: 0,
		currency:     "EUR",
	}
	input := OptimizeInput{}
	input.defaults()
	priceCandidate(c, input)

	if c.allInCost != 7 {
		t.Errorf("prePriced allInCost: got %.0f, want 7", c.allInCost)
	}
	if c.bagCost != 0 {
		t.Errorf("prePriced bagCost: got %.0f, want 0", c.bagCost)
	}
}

func TestPriceCandidate_prePriced_with_transfer(t *testing.T) {
	c := &candidate{
		searched:     true,
		prePriced:    true,
		baseCost:     35,
		transferCost: 10,
		currency:     "EUR",
	}
	input := OptimizeInput{}
	input.defaults()
	priceCandidate(c, input)

	if c.allInCost != 45 {
		t.Errorf("prePriced allInCost: got %.0f, want 45 (35+10)", c.allInCost)
	}
}

func TestCandidateToOption_prePriced_ground_leg(t *testing.T) {
	c := &candidate{
		origin:     "MAD",
		dest:       "BCN",
		departDate: "2026-06-01",
		strategy:   "Take train (AVE, AVLO, Ouigo, Iryo) — fares from €7",
		hackTypes:  []string{"rail_competition", "ground_alternative"},
		prePriced:  true,
		searched:   true,
		baseCost:   7,
		currency:   "EUR",
		allInCost:  7,
	}

	input := OptimizeInput{Origin: "MAD", Destination: "BCN"}
	opt := candidateToOption(c, 1, input)

	if len(opt.Legs) == 0 {
		t.Fatal("expected at least 1 leg for pre-priced candidate")
	}
	if opt.Legs[0].Type != "ground" {
		t.Errorf("pre-priced leg type: want ground, got %s", opt.Legs[0].Type)
	}
	if opt.Legs[0].Price != 7 {
		t.Errorf("pre-priced leg price: want 7, got %.0f", opt.Legs[0].Price)
	}
}

func TestRankCandidates_cross_currency_no_savings(t *testing.T) {
	// When baseline is RUB and option is EUR, savings should be 0 (not cross-currency nonsense).
	candidates := []*candidate{
		{searched: true, allInCost: 7000, baseCost: 7000, currency: "RUB", strategy: "Direct"},
		{searched: true, prePriced: true, allInCost: 7, baseCost: 7, currency: "EUR", strategy: "Train", hackTypes: []string{"rail_competition"}},
	}

	input := OptimizeInput{MaxResults: 5, Currency: "RUB"}
	result := rankCandidates(candidates, input)

	if !result.Success {
		t.Fatal("expected success")
	}

	// The RUB baseline should rank first (same currency as input).
	if result.Options[0].Currency != "RUB" {
		t.Errorf("rank 1 should be same-currency (RUB), got %s", result.Options[0].Currency)
	}

	// The EUR option should have zero savings (can't compare cross-currency).
	for _, opt := range result.Options {
		if opt.Currency == "EUR" && opt.SavingsVsBaseline != 0 {
			t.Errorf("cross-currency option should have 0 savings, got %.0f", opt.SavingsVsBaseline)
		}
	}
}

func TestRankCandidates_same_currency_savings(t *testing.T) {
	candidates := []*candidate{
		{searched: true, allInCost: 200, baseCost: 200, currency: "EUR", strategy: "Direct"},
		{searched: true, allInCost: 150, baseCost: 150, currency: "EUR", strategy: "Alt", hackTypes: []string{"positioning"}},
	}

	input := OptimizeInput{MaxResults: 5, Currency: "EUR"}
	result := rankCandidates(candidates, input)

	// Same currency — savings should be computed.
	if result.Options[0].SavingsVsBaseline != 50 {
		t.Errorf("same-currency savings: want 50, got %.0f", result.Options[0].SavingsVsBaseline)
	}
}

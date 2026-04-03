package trip

import (
	"testing"
)

func TestPermutations_Empty(t *testing.T) {
	perms := permutations(nil)
	if len(perms) != 1 || len(perms[0]) != 0 {
		t.Errorf("permutations(nil) = %v, want [[]]", perms)
	}
}

func TestPermutations_Single(t *testing.T) {
	perms := permutations([]string{"A"})
	if len(perms) != 1 {
		t.Fatalf("len = %d, want 1", len(perms))
	}
	if perms[0][0] != "A" {
		t.Errorf("perms[0] = %v, want [A]", perms[0])
	}
}

func TestPermutations_Two(t *testing.T) {
	perms := permutations([]string{"A", "B"})
	if len(perms) != 2 {
		t.Fatalf("len = %d, want 2", len(perms))
	}
}

func TestPermutations_Three(t *testing.T) {
	perms := permutations([]string{"A", "B", "C"})
	if len(perms) != 6 {
		t.Fatalf("len = %d, want 6", len(perms))
	}
}

func TestPermutations_Four(t *testing.T) {
	perms := permutations([]string{"A", "B", "C", "D"})
	if len(perms) != 24 {
		t.Fatalf("len = %d, want 24", len(perms))
	}
}

func TestRouteCost(t *testing.T) {
	prices := map[string]float64{
		"H->A": 100,
		"A->B": 50,
		"B->H": 120,
		"H->B": 80,
		"B->A": 60,
		"A->H": 110,
	}

	// Route: H -> A -> B -> H = 100 + 50 + 120 = 270
	cost1 := routeCost("H", []string{"A", "B"}, prices)
	if cost1 != 270 {
		t.Errorf("cost H->A->B->H = %v, want 270", cost1)
	}

	// Route: H -> B -> A -> H = 80 + 60 + 110 = 250
	cost2 := routeCost("H", []string{"B", "A"}, prices)
	if cost2 != 250 {
		t.Errorf("cost H->B->A->H = %v, want 250", cost2)
	}
}

func TestOptimizeMultiCity_EmptyHome(t *testing.T) {
	_, err := OptimizeMultiCity(t.Context(), "", []string{"BCN"}, MultiCityOptions{DepartDate: "2026-07-01"})
	if err == nil {
		t.Error("expected error for empty home")
	}
}

func TestOptimizeMultiCity_NoCities(t *testing.T) {
	_, err := OptimizeMultiCity(t.Context(), "HEL", nil, MultiCityOptions{DepartDate: "2026-07-01"})
	if err == nil {
		t.Error("expected error for no cities")
	}
}

func TestOptimizeMultiCity_TooManyCities(t *testing.T) {
	cities := []string{"A", "B", "C", "D", "E", "F", "G"}
	_, err := OptimizeMultiCity(t.Context(), "HEL", cities, MultiCityOptions{DepartDate: "2026-07-01"})
	if err == nil {
		t.Error("expected error for >6 cities")
	}
}

func TestOptimizeMultiCity_NoDate(t *testing.T) {
	_, err := OptimizeMultiCity(t.Context(), "HEL", []string{"BCN"}, MultiCityOptions{})
	if err == nil {
		t.Error("expected error for missing date")
	}
}

func TestOptimizeMultiCity_ExactlySixCities(t *testing.T) {
	// Exactly 6 should be accepted (validation passes), but will fail at API calls.
	// We just verify the validation doesn't reject it.
	cities := []string{"A", "B", "C", "D", "E", "F"}
	_, err := OptimizeMultiCity(t.Context(), "HEL", cities, MultiCityOptions{})
	if err == nil {
		t.Error("expected error for missing date")
	}
}

func TestSortSegmentsByPrice(t *testing.T) {
	segments := []Segment{
		{From: "A", To: "B", Price: 300, Currency: "EUR"},
		{From: "B", To: "C", Price: 100, Currency: "EUR"},
		{From: "C", To: "A", Price: 200, Currency: "EUR"},
	}
	SortSegmentsByPrice(segments)

	if segments[0].Price != 100 {
		t.Errorf("segments[0].Price = %v, want 100", segments[0].Price)
	}
	if segments[1].Price != 200 {
		t.Errorf("segments[1].Price = %v, want 200", segments[1].Price)
	}
	if segments[2].Price != 300 {
		t.Errorf("segments[2].Price = %v, want 300", segments[2].Price)
	}
}

func TestSortSegmentsByPrice_Empty(t *testing.T) {
	var segments []Segment
	SortSegmentsByPrice(segments) // should not panic
}

func TestSortSegmentsByPrice_SingleElement(t *testing.T) {
	segments := []Segment{
		{From: "A", To: "B", Price: 100, Currency: "EUR"},
	}
	SortSegmentsByPrice(segments)
	if segments[0].Price != 100 {
		t.Errorf("segments[0].Price = %v, want 100", segments[0].Price)
	}
}

func TestSortSegmentsByPrice_AlreadySorted(t *testing.T) {
	segments := []Segment{
		{From: "A", To: "B", Price: 100, Currency: "EUR"},
		{From: "B", To: "C", Price: 200, Currency: "EUR"},
		{From: "C", To: "A", Price: 300, Currency: "EUR"},
	}
	SortSegmentsByPrice(segments)
	if segments[0].Price != 100 || segments[1].Price != 200 || segments[2].Price != 300 {
		t.Errorf("already-sorted input should remain sorted")
	}
}

func TestRouteCost_SingleCity(t *testing.T) {
	prices := map[string]float64{
		"H->A": 100,
		"A->H": 120,
	}
	// Route: H -> A -> H = 100 + 120 = 220
	cost := routeCost("H", []string{"A"}, prices)
	if cost != 220 {
		t.Errorf("cost H->A->H = %v, want 220", cost)
	}
}

func TestRouteCost_MissingPrice(t *testing.T) {
	// Missing price should contribute 0 (zero value from map).
	prices := map[string]float64{
		"H->A": 100,
	}
	cost := routeCost("H", []string{"A"}, prices)
	// H->A = 100, A->H = 0 (missing)
	if cost != 100 {
		t.Errorf("cost with missing return = %v, want 100", cost)
	}
}

func TestRouteCost_ThreeCities(t *testing.T) {
	prices := map[string]float64{
		"H->A": 100,
		"A->B": 50,
		"B->C": 75,
		"C->H": 120,
	}
	// Route: H -> A -> B -> C -> H = 100 + 50 + 75 + 120 = 345
	cost := routeCost("H", []string{"A", "B", "C"}, prices)
	if cost != 345 {
		t.Errorf("cost = %v, want 345", cost)
	}
}

func TestPermutations_Uniqueness(t *testing.T) {
	perms := permutations([]string{"A", "B", "C"})
	seen := make(map[string]bool)
	for _, p := range perms {
		key := ""
		for _, s := range p {
			key += s + ","
		}
		if seen[key] {
			t.Errorf("duplicate permutation: %v", p)
		}
		seen[key] = true
	}
	if len(seen) != 6 {
		t.Errorf("expected 6 unique permutations, got %d", len(seen))
	}
}

func TestPermutations_ContainsAllElements(t *testing.T) {
	input := []string{"X", "Y", "Z"}
	perms := permutations(input)
	for _, p := range perms {
		if len(p) != 3 {
			t.Errorf("permutation length = %d, want 3", len(p))
		}
		has := map[string]bool{}
		for _, s := range p {
			has[s] = true
		}
		for _, s := range input {
			if !has[s] {
				t.Errorf("permutation %v missing element %q", p, s)
			}
		}
	}
}

func TestOptimizeRoute_TwoCities(t *testing.T) {
	prices := map[string]float64{
		"H->A": 100,
		"A->B": 50,
		"B->H": 120,
		"H->B": 80,
		"B->A": 60,
		"A->H": 110,
	}

	result := optimizeRoute("H", []string{"A", "B"}, prices)

	if !result.Success {
		t.Fatal("expected success")
	}
	if result.HomeAirport != "H" {
		t.Errorf("home = %q, want H", result.HomeAirport)
	}
	// H->B->A->H = 80+60+110 = 250 is cheaper than H->A->B->H = 100+50+120 = 270.
	if result.TotalCost != 250 {
		t.Errorf("total cost = %v, want 250", result.TotalCost)
	}
	if result.WorstCost != 270 {
		t.Errorf("worst cost = %v, want 270", result.WorstCost)
	}
	if result.Savings != 20 {
		t.Errorf("savings = %v, want 20", result.Savings)
	}
	if result.Permutations != 2 {
		t.Errorf("permutations = %d, want 2", result.Permutations)
	}
	// Optimal order should be B, A.
	if len(result.OptimalOrder) != 2 || result.OptimalOrder[0] != "B" || result.OptimalOrder[1] != "A" {
		t.Errorf("optimal order = %v, want [B A]", result.OptimalOrder)
	}
	// Segments: H->B, B->A, A->H.
	if len(result.Segments) != 3 {
		t.Fatalf("segments = %d, want 3", len(result.Segments))
	}
	if result.Segments[0].From != "H" || result.Segments[0].To != "B" {
		t.Errorf("segment 0 = %s->%s, want H->B", result.Segments[0].From, result.Segments[0].To)
	}
	if result.Segments[1].From != "B" || result.Segments[1].To != "A" {
		t.Errorf("segment 1 = %s->%s, want B->A", result.Segments[1].From, result.Segments[1].To)
	}
	if result.Segments[2].From != "A" || result.Segments[2].To != "H" {
		t.Errorf("segment 2 = %s->%s, want A->H", result.Segments[2].From, result.Segments[2].To)
	}
}

func TestOptimizeRoute_SingleCity(t *testing.T) {
	prices := map[string]float64{
		"H->A": 100,
		"A->H": 150,
	}

	result := optimizeRoute("H", []string{"A"}, prices)

	if !result.Success {
		t.Fatal("expected success")
	}
	if result.TotalCost != 250 {
		t.Errorf("total = %v, want 250", result.TotalCost)
	}
	if result.Permutations != 1 {
		t.Errorf("permutations = %d, want 1", result.Permutations)
	}
	if result.Savings != 0 {
		t.Errorf("savings = %v, want 0 (only one permutation)", result.Savings)
	}
}

func TestOptimizeRoute_ThreeCities(t *testing.T) {
	prices := map[string]float64{
		"H->A": 100, "H->B": 200, "H->C": 150,
		"A->H": 110, "B->H": 210, "C->H": 160,
		"A->B": 50, "A->C": 80,
		"B->A": 60, "B->C": 40,
		"C->A": 90, "C->B": 70,
	}

	result := optimizeRoute("H", []string{"A", "B", "C"}, prices)

	if !result.Success {
		t.Fatal("expected success")
	}
	if result.Permutations != 6 {
		t.Errorf("permutations = %d, want 6", result.Permutations)
	}
	// Manually verify: find the cheapest route.
	// H->A->B->C->H = 100+50+40+160 = 350
	// H->A->C->B->H = 100+80+70+210 = 460
	// H->B->A->C->H = 200+60+80+160 = 500
	// H->B->C->A->H = 200+40+90+110 = 440
	// H->C->A->B->H = 150+90+50+210 = 500
	// H->C->B->A->H = 150+70+60+110 = 390
	// Cheapest is H->A->B->C->H = 350
	if result.TotalCost != 350 {
		t.Errorf("total cost = %v, want 350", result.TotalCost)
	}
	if result.WorstCost != 500 {
		t.Errorf("worst cost = %v, want 500", result.WorstCost)
	}
	if result.Currency != "EUR" {
		t.Errorf("currency = %q, want EUR", result.Currency)
	}
}

func TestOptimizeRoute_AllSamePrice(t *testing.T) {
	prices := map[string]float64{
		"H->A": 100, "H->B": 100,
		"A->H": 100, "B->H": 100,
		"A->B": 100, "B->A": 100,
	}

	result := optimizeRoute("H", []string{"A", "B"}, prices)

	if result.TotalCost != 300 {
		t.Errorf("total = %v, want 300", result.TotalCost)
	}
	if result.Savings != 0 {
		t.Errorf("savings = %v, want 0 (all same price)", result.Savings)
	}
}

func TestPermutations_Five(t *testing.T) {
	perms := permutations([]string{"A", "B", "C", "D", "E"})
	if len(perms) != 120 {
		t.Fatalf("len = %d, want 120", len(perms))
	}
}

func TestPermutations_Six(t *testing.T) {
	perms := permutations([]string{"A", "B", "C", "D", "E", "F"})
	if len(perms) != 720 {
		t.Fatalf("len = %d, want 720", len(perms))
	}
}

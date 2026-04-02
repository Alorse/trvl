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

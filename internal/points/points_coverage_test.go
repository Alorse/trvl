package points

import (
	"math"
	"testing"
)

// ============================================================
// evaluate — all branches (83.3% coverage to 100%)
// ============================================================

func TestEvaluate_AllBranches(t *testing.T) {
	prog := &Program{
		Slug:       "test-program",
		Name:       "Test Program",
		FloorCPP:   1.0,
		CeilingCPP: 2.0,
		Category:   "airline",
	}

	tests := []struct {
		name        string
		cpp         float64
		wantVerdict string
	}{
		{"above ceiling use points", 2.5, "use points"},
		{"at ceiling use points", 2.0, "use points"},
		{"above midpoint use points", 1.6, "use points"},
		{"at midpoint use points", 1.5, "use points"},
		{"below midpoint borderline", 1.2, "borderline"},
		{"at floor borderline", 1.0, "borderline"},
		{"below floor pay cash", 0.5, "pay cash"},
		{"zero pay cash", 0.0, "pay cash"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verdict, explanation := evaluate(tt.cpp, prog)
			if verdict != tt.wantVerdict {
				t.Errorf("evaluate(%.2f) verdict = %q, want %q", tt.cpp, verdict, tt.wantVerdict)
			}
			if explanation == "" {
				t.Error("expected non-empty explanation")
			}
		})
	}
}

// ============================================================
// estimateFlyingBlue — all branches (75% coverage to higher)
// ============================================================

func TestEstimateFlyingBlue_HomeCarrier_Economy(t *testing.T) {
	est := estimateFlyingBlue("KL", "economy", 200, 1000)
	if est.Miles != 800 {
		t.Errorf("KL economy: miles = %d, want 800", est.Miles)
	}
	if est.Method != "revenue" {
		t.Errorf("method = %q, want revenue", est.Method)
	}
}

func TestEstimateFlyingBlue_HomeCarrier_Business(t *testing.T) {
	est := estimateFlyingBlue("AF", "business", 1000, 5000)
	if est.Miles != 8000 {
		t.Errorf("AF business: miles = %d, want 8000", est.Miles)
	}
}

func TestEstimateFlyingBlue_HomeCarrier_First(t *testing.T) {
	est := estimateFlyingBlue("KL", "first", 500, 3000)
	if est.Miles != 4000 {
		t.Errorf("KL first: miles = %d, want 4000 (8 * 500)", est.Miles)
	}
}

func TestEstimateFlyingBlue_HomeCarrier_PremiumEconomy(t *testing.T) {
	est := estimateFlyingBlue("AF", "premium_economy", 300, 2000)
	if est.Miles != 1800 {
		t.Errorf("AF premium_economy: miles = %d, want 1800 (6 * 300)", est.Miles)
	}
}

func TestEstimateFlyingBlue_Partner_Business(t *testing.T) {
	est := estimateFlyingBlue("DL", "business", 800, 3000)
	if est.Miles != 3200 {
		t.Errorf("DL business: miles = %d, want 3200 (4 * 800)", est.Miles)
	}
	if est.Note == "" {
		t.Error("expected note for partner airline")
	}
}

func TestEstimateFlyingBlue_Partner_PremiumEconomy(t *testing.T) {
	est := estimateFlyingBlue("KE", "premium_economy", 400, 2000)
	if est.Miles != 1200 {
		t.Errorf("KE premium_economy: miles = %d, want 1200 (3 * 400)", est.Miles)
	}
}

func TestEstimateFlyingBlue_NonAlliance(t *testing.T) {
	est := estimateFlyingBlue("EK", "economy", 500, 3000)
	if est.Miles != 500 {
		t.Errorf("EK economy: miles = %d, want 500 (1 * 500)", est.Miles)
	}
	if est.Note == "" {
		t.Error("expected note for non-alliance airline")
	}
}

func TestEstimateFlyingBlue_NoPrice_Business(t *testing.T) {
	est := estimateFlyingBlue("KL", "business", 0, 3000)
	if est.Method != "distance" {
		t.Errorf("method = %q, want distance", est.Method)
	}
	expected := int(math.Round(float64(3000) * 1.5))
	if est.Miles != expected {
		t.Errorf("miles = %d, want %d", est.Miles, expected)
	}
}

func TestEstimateFlyingBlue_NoPrice_First(t *testing.T) {
	est := estimateFlyingBlue("AF", "first", 0, 3000)
	if est.Method != "distance" {
		t.Errorf("method = %q, want distance", est.Method)
	}
	expected := int(math.Round(float64(3000) * 2.0))
	if est.Miles != expected {
		t.Errorf("miles = %d, want %d", est.Miles, expected)
	}
}

func TestEstimateFlyingBlue_NoPrice_PremiumEconomy(t *testing.T) {
	est := estimateFlyingBlue("KL", "premium_economy", 0, 3000)
	if est.Method != "distance" {
		t.Errorf("method = %q, want distance", est.Method)
	}
	expected := int(math.Round(float64(3000) * 0.75))
	if est.Miles != expected {
		t.Errorf("miles = %d, want %d", est.Miles, expected)
	}
}

func TestEstimateFlyingBlue_NoPrice_Economy(t *testing.T) {
	est := estimateFlyingBlue("KL", "economy", 0, 3000)
	if est.Method != "distance" {
		t.Errorf("method = %q, want distance", est.Method)
	}
	expected := int(math.Round(float64(3000) * 0.5))
	if est.Miles != expected {
		t.Errorf("miles = %d, want %d", est.Miles, expected)
	}
}

// ============================================================
// estimateDistanceBased — all branches (72.2% coverage to higher)
// ============================================================

func TestEstimateDistanceBased_Oneworld_HomeCarrier(t *testing.T) {
	est := estimateDistanceBased("oneworld", "BA", "economy", 5000)
	if est.Program != "Avios" {
		t.Errorf("program = %q, want Avios", est.Program)
	}
	if est.Miles != 2500 {
		t.Errorf("miles = %d, want 2500", est.Miles)
	}
}

func TestEstimateDistanceBased_Oneworld_NonHomeCarrier(t *testing.T) {
	est := estimateDistanceBased("oneworld", "ZZ", "economy", 5000)
	if est.Program != "Oneworld programme" {
		t.Errorf("program = %q, want 'Oneworld programme'", est.Program)
	}
	if est.Miles != 1250 {
		t.Errorf("miles = %d, want 1250", est.Miles)
	}
}

func TestEstimateDistanceBased_Oneworld_Business(t *testing.T) {
	est := estimateDistanceBased("oneworld", "BA", "business", 4000)
	if est.Miles != 6000 {
		t.Errorf("miles = %d, want 6000", est.Miles)
	}
}

func TestEstimateDistanceBased_Oneworld_First(t *testing.T) {
	est := estimateDistanceBased("oneworld", "QF", "first", 3000)
	if est.Miles != 6000 {
		t.Errorf("miles = %d, want 6000", est.Miles)
	}
}

func TestEstimateDistanceBased_Oneworld_PremiumEconomy(t *testing.T) {
	est := estimateDistanceBased("oneworld", "AY", "premium_economy", 3000)
	if est.Miles != 3000 {
		t.Errorf("miles = %d, want 3000", est.Miles)
	}
}

func TestEstimateDistanceBased_StarAlliance(t *testing.T) {
	est := estimateDistanceBased("star_alliance", "LH", "economy", 5000)
	if est.Program != "Star Alliance programme" {
		t.Errorf("program = %q, want 'Star Alliance programme'", est.Program)
	}
	if est.Miles != 2500 {
		t.Errorf("miles = %d, want 2500", est.Miles)
	}
}

func TestEstimateDistanceBased_OtherAlliance(t *testing.T) {
	est := estimateDistanceBased("other", "ZZ", "economy", 5000)
	if est.Program != "Frequent flyer programme" {
		t.Errorf("program = %q, want 'Frequent flyer programme'", est.Program)
	}
}

func TestEstimateDistanceBased_MinimumEarning(t *testing.T) {
	est := estimateDistanceBased("star_alliance", "LH", "economy", 200)
	if est.Miles != 500 {
		t.Errorf("miles = %d, want 500 (minimum earning)", est.Miles)
	}
}

func TestEstimateDistanceBased_ZeroDistance(t *testing.T) {
	est := estimateDistanceBased("star_alliance", "LH", "economy", 0)
	if est.Miles != 0 {
		t.Errorf("miles = %d, want 0 for zero distance", est.Miles)
	}
}

// ============================================================
// programNameForOneworld — all branches (50% coverage to 100%)
// ============================================================

func TestProgramNameForOneworld_AllCoverage(t *testing.T) {
	tests := map[string]string{
		"BA": "Avios",
		"AY": "Finnair Plus",
		"QF": "Qantas Frequent Flyer",
		"AA": "AAdvantage",
		"CX": "Asia Miles",
		"JL": "JAL Mileage Bank",
		"QR": "Privilege Club",
		"IB": "Iberia Plus",
		"RJ": "Royal Plus",
		"MH": "Enrich",
		"ZZ": "Oneworld programme",
	}
	for code, want := range tests {
		got := programNameForOneworld(code)
		if got != want {
			t.Errorf("programNameForOneworld(%q) = %q, want %q", code, got, want)
		}
	}
}

// ============================================================
// CalculateValue — error paths
// ============================================================

func TestCalculateValue_ZeroCashPrice(t *testing.T) {
	_, err := CalculateValue(0, 10000, "finnair-plus")
	if err == nil {
		t.Error("expected error for zero cash price")
	}
}

func TestCalculateValue_NegativeCashPrice(t *testing.T) {
	_, err := CalculateValue(-100, 10000, "finnair-plus")
	if err == nil {
		t.Error("expected error for negative cash price")
	}
}

func TestCalculateValue_ZeroPoints(t *testing.T) {
	_, err := CalculateValue(100, 0, "finnair-plus")
	if err == nil {
		t.Error("expected error for zero points")
	}
}

func TestCalculateValue_NegativePoints(t *testing.T) {
	_, err := CalculateValue(100, -5000, "finnair-plus")
	if err == nil {
		t.Error("expected error for negative points")
	}
}

func TestCalculateValue_MissingProgram(t *testing.T) {
	_, err := CalculateValue(100, 10000, "does-not-exist-program")
	if err == nil {
		t.Error("expected error for unrecognized program")
	}
}

func TestCalculateValue_WhitespaceSlug(t *testing.T) {
	_, err := CalculateValue(100, 10000, "  finnair-plus  ")
	if err != nil {
		t.Errorf("expected no error for slug with whitespace, got: %v", err)
	}
}

func TestCalculateValue_AllVerdicts(t *testing.T) {
	tests := []struct {
		name        string
		cash        float64
		points      int
		slug        string
		wantVerdict string
	}{
		// Finnair Plus: floor 1.0, ceiling 2.2
		// cpp = (cash * 100) / points
		{"excellent above ceiling", 250, 10000, "finnair-plus", "use points"},
		{"good above midpoint", 180, 10000, "finnair-plus", "use points"},
		{"borderline below midpoint", 130, 10000, "finnair-plus", "borderline"},
		{"poor below floor", 50, 10000, "finnair-plus", "pay cash"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec, err := CalculateValue(tt.cash, tt.points, tt.slug)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if rec.Verdict != tt.wantVerdict {
				t.Errorf("verdict = %q (cpp=%.2f), want %q", rec.Verdict, rec.CPP, tt.wantVerdict)
			}
			if rec.Explanation == "" {
				t.Error("expected non-empty explanation")
			}
			if rec.ProgramSlug != tt.slug {
				t.Errorf("ProgramSlug = %q, want %q", rec.ProgramSlug, tt.slug)
			}
		})
	}
}

// ============================================================
// ListPrograms — category filtering
// ============================================================

func TestListPrograms_AllCategories(t *testing.T) {
	all := ListPrograms("")
	if len(all) != len(Programs) {
		t.Errorf("ListPrograms('') returned %d, want %d", len(all), len(Programs))
	}

	airlines := ListPrograms("airline")
	hotels := ListPrograms("hotel")
	transferable := ListPrograms("transferable")

	total := len(airlines) + len(hotels) + len(transferable)
	if total != len(Programs) {
		t.Errorf("sum of categories = %d, want %d", total, len(Programs))
	}

	for _, p := range airlines {
		if p.Category != "airline" {
			t.Errorf("airline category contains %q with category %q", p.Slug, p.Category)
		}
	}
	for _, p := range hotels {
		if p.Category != "hotel" {
			t.Errorf("hotel category contains %q with category %q", p.Slug, p.Category)
		}
	}
}

func TestListPrograms_CaseInsensitive(t *testing.T) {
	result := ListPrograms("AIRLINE")
	if len(result) == 0 {
		t.Error("expected results for case-insensitive category match")
	}
}

func TestListPrograms_NoMatchCategory(t *testing.T) {
	result := ListPrograms("bogus-category")
	if len(result) != 0 {
		t.Errorf("expected 0 results for bogus category, got %d", len(result))
	}
}

// ============================================================
// LookupProgram
// ============================================================

func TestLookupProgram_FoundCoverage(t *testing.T) {
	p := LookupProgram("finnair-plus")
	if p == nil {
		t.Fatal("expected to find finnair-plus")
	}
	if p.Name != "Finnair Plus" {
		t.Errorf("Name = %q, want 'Finnair Plus'", p.Name)
	}
}

func TestLookupProgram_NotFoundCoverage(t *testing.T) {
	p := LookupProgram("bogus-slug")
	if p != nil {
		t.Errorf("expected nil for bogus slug, got %+v", p)
	}
}

// ============================================================
// EstimateMilesEarned — integration with alliance routing
// ============================================================

func TestEstimateMilesEarned_StarAllianceIntegration(t *testing.T) {
	est := EstimateMilesEarned("FRA", "NRT", "business", "LH", "star_alliance", 0)
	if est.Program != "Star Alliance programme" {
		t.Errorf("program = %q, want 'Star Alliance programme'", est.Program)
	}
	if est.Method != "distance" {
		t.Errorf("method = %q, want 'distance'", est.Method)
	}
	if est.Miles < 5000 {
		t.Errorf("expected > 5000 miles for FRA-NRT business, got %d", est.Miles)
	}
}

func TestEstimateMilesEarned_OtherAllianceIntegration(t *testing.T) {
	est := EstimateMilesEarned("HEL", "AMS", "economy", "ZZ", "other", 0)
	if est.Program != "Frequent flyer programme" {
		t.Errorf("program = %q, want 'Frequent flyer programme'", est.Program)
	}
}

func TestEstimateMilesEarned_Oneworld_NonHome_PremiumEconomy(t *testing.T) {
	est := EstimateMilesEarned("HEL", "NRT", "premium_economy", "ZZ", "oneworld", 0)
	if est.Program != "Oneworld programme" {
		t.Errorf("program = %q, want 'Oneworld programme'", est.Program)
	}
	if est.Miles < 2000 || est.Miles > 3000 {
		t.Errorf("expected around 2400 miles for HEL-NRT premium_economy non-home, got %d", est.Miles)
	}
}

// ============================================================
// TransferPartners — data integrity
// ============================================================

func TestTransferPartners_AllReferencesValid(t *testing.T) {
	for _, tp := range TransferPartners {
		from := LookupProgram(tp.From)
		if from == nil {
			t.Errorf("TransferPartner.From %q not found in programs", tp.From)
		}
		to := LookupProgram(tp.To)
		if to == nil {
			t.Errorf("TransferPartner.To %q not found in programs", tp.To)
		}
		if tp.Ratio <= 0 {
			t.Errorf("TransferPartner %s->%s has invalid ratio %.2f", tp.From, tp.To, tp.Ratio)
		}
	}
}

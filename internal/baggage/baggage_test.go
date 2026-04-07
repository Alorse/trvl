package baggage

import (
	"strings"
	"testing"
)

func TestGet_KnownAirline(t *testing.T) {
	ab, ok := Get("KL")
	if !ok {
		t.Fatal("expected KL to be found")
	}
	if ab.Code != "KL" {
		t.Errorf("Code = %q, want KL", ab.Code)
	}
	if ab.Name != "KLM" {
		t.Errorf("Name = %q, want KLM", ab.Name)
	}
	if ab.CarryOnMaxKg != 12 {
		t.Errorf("CarryOnMaxKg = %v, want 12", ab.CarryOnMaxKg)
	}
	if !ab.PersonalItem {
		t.Error("expected PersonalItem = true for KLM")
	}
	if ab.CheckedIncluded != 1 {
		t.Errorf("CheckedIncluded = %d, want 1", ab.CheckedIncluded)
	}
}

func TestGet_UnknownAirline(t *testing.T) {
	_, ok := Get("XX")
	if ok {
		t.Error("expected XX to not be found")
	}
}

func TestGet_LCCOverheadOnly(t *testing.T) {
	ab, ok := Get("FR")
	if !ok {
		t.Fatal("expected FR (Ryanair) to be found")
	}
	if !ab.OverheadOnly {
		t.Error("expected Ryanair OverheadOnly = true")
	}
	if ab.CheckedIncluded != 0 {
		t.Errorf("Ryanair CheckedIncluded = %d, want 0", ab.CheckedIncluded)
	}
}

func TestGet_WizzAir(t *testing.T) {
	ab, ok := Get("W6")
	if !ok {
		t.Fatal("expected W6 (Wizz Air) to be found")
	}
	if !ab.OverheadOnly {
		t.Error("expected Wizz Air OverheadOnly = true")
	}
}

func TestAll_ReturnsSorted(t *testing.T) {
	all := All()
	if len(all) == 0 {
		t.Fatal("All() returned empty slice")
	}
	for i := 1; i < len(all); i++ {
		if all[i].Code < all[i-1].Code {
			t.Errorf("All() not sorted: %q < %q at index %d", all[i].Code, all[i-1].Code, i)
		}
	}
}

func TestAll_ContainsExpectedAirlines(t *testing.T) {
	all := All()
	codes := make(map[string]bool, len(all))
	for _, ab := range all {
		codes[ab.Code] = true
	}

	required := []string{"KL", "AY", "AF", "LH", "BA", "IB", "LX", "OS", "LO", "SK", "AZ", "TP", "TK", "QR", "EK", "SQ", "FR", "W6", "U2", "DY", "BT", "VY", "F9", "B6"}
	for _, code := range required {
		if !codes[code] {
			t.Errorf("All() missing airline %q", code)
		}
	}
}

func TestBaggageNote_OverheadOnly(t *testing.T) {
	note := BaggageNote("FR")
	if !strings.Contains(note, "⚠️") {
		t.Errorf("expected warning emoji in Ryanair note, got: %q", note)
	}
	if !strings.Contains(note, "under-seat") {
		t.Errorf("expected 'under-seat' in Ryanair note, got: %q", note)
	}
}

func TestBaggageNote_NoWeightLimit(t *testing.T) {
	note := BaggageNote("BA")
	if !strings.Contains(note, "no weight limit") {
		t.Errorf("expected 'no weight limit' in BA note, got: %q", note)
	}
}

func TestBaggageNote_Normal(t *testing.T) {
	note := BaggageNote("KL")
	if !strings.Contains(note, "12kg") {
		t.Errorf("expected '12kg' in KL note, got: %q", note)
	}
	if !strings.Contains(note, "hidden city") {
		t.Errorf("expected 'hidden city' in KL note, got: %q", note)
	}
}

func TestBaggageNote_Unknown(t *testing.T) {
	note := BaggageNote("ZZ")
	if note != "" {
		t.Errorf("expected empty note for unknown airline, got: %q", note)
	}
}

func TestAirlineNotesAvoidSubjectiveQualityClaims(t *testing.T) {
	for _, airline := range All() {
		if strings.Contains(airline.Notes, "world's best airline") {
			t.Fatalf("%s note should avoid unsupported ranking claims", airline.Code)
		}
		if strings.Contains(airline.Notes, "excellent business class") {
			t.Fatalf("%s note should avoid subjective cabin-quality claims", airline.Code)
		}
		if strings.Contains(airline.Notes, "generous allowances across all classes") {
			t.Fatalf("%s note should avoid subjective baggage-allowance claims", airline.Code)
		}
	}
}

func TestFormatKg(t *testing.T) {
	tests := []struct {
		kg   float64
		want string
	}{
		{12, "12kg"},
		{8, "8kg"},
		{7.5, "7.5kg"},
		{10, "10kg"},
	}
	for _, tc := range tests {
		got := formatKg(tc.kg)
		if got != tc.want {
			t.Errorf("formatKg(%v) = %q, want %q", tc.kg, got, tc.want)
		}
	}
}

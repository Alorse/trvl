package visa

import "testing"

func TestLookup_SameCountry(t *testing.T) {
	r := Lookup("FI", "FI")
	if !r.Success {
		t.Fatal("expected success")
	}
	if r.Requirement.Status != "freedom-of-movement" {
		t.Errorf("got status %q, want freedom-of-movement", r.Requirement.Status)
	}
	if r.Requirement.MaxStay != "unlimited" {
		t.Errorf("got max_stay %q, want unlimited", r.Requirement.MaxStay)
	}
}

func TestLookup_EUFreedom(t *testing.T) {
	r := Lookup("FI", "DE")
	if !r.Success {
		t.Fatal("expected success")
	}
	if r.Requirement.Status != "freedom-of-movement" {
		t.Errorf("got status %q, want freedom-of-movement", r.Requirement.Status)
	}
}

func TestLookup_BilateralFI_JP(t *testing.T) {
	r := Lookup("FI", "JP")
	if !r.Success {
		t.Fatal("expected success")
	}
	if r.Requirement.Status != "visa-free" {
		t.Errorf("got status %q, want visa-free", r.Requirement.Status)
	}
	if r.Requirement.MaxStay != "90 days" {
		t.Errorf("got max_stay %q, want 90 days", r.Requirement.MaxStay)
	}
}

func TestLookup_ESTA(t *testing.T) {
	r := Lookup("FI", "US")
	if !r.Success {
		t.Fatal("expected success")
	}
	if r.Requirement.Status != "e-visa" {
		t.Errorf("got status %q, want e-visa", r.Requirement.Status)
	}
}

func TestLookup_VisaRequired(t *testing.T) {
	r := Lookup("FI", "CN")
	if !r.Success {
		t.Fatal("expected success")
	}
	if r.Requirement.Status != "visa-required" {
		t.Errorf("got status %q, want visa-required", r.Requirement.Status)
	}
}

func TestLookup_VisaOnArrival(t *testing.T) {
	r := Lookup("FI", "ID") // Indonesia
	if !r.Success {
		t.Fatal("expected success")
	}
	if r.Requirement.Status != "visa-on-arrival" {
		t.Errorf("got status %q, want visa-on-arrival", r.Requirement.Status)
	}
}

func TestLookup_EVisa(t *testing.T) {
	r := Lookup("US", "IN") // India
	if !r.Success {
		t.Fatal("expected success")
	}
	if r.Requirement.Status != "e-visa" {
		t.Errorf("got status %q, want e-visa", r.Requirement.Status)
	}
}

func TestLookup_GroupRule_UStoJP(t *testing.T) {
	r := Lookup("US", "JP")
	if !r.Success {
		t.Fatal("expected success")
	}
	if r.Requirement.Status != "visa-free" {
		t.Errorf("got status %q, want visa-free", r.Requirement.Status)
	}
}

func TestLookup_GroupRule_UStoTH(t *testing.T) {
	r := Lookup("US", "TH")
	if !r.Success {
		t.Fatal("expected success")
	}
	if r.Requirement.Status != "visa-free" {
		t.Errorf("got status %q, want visa-free", r.Requirement.Status)
	}
}

func TestLookup_TransTasman(t *testing.T) {
	r := Lookup("AU", "NZ")
	if !r.Success {
		t.Fatal("expected success")
	}
	if r.Requirement.Status != "visa-free" {
		t.Errorf("got status %q, want visa-free", r.Requirement.Status)
	}
	if r.Requirement.MaxStay != "unlimited" {
		t.Errorf("got max_stay %q, want unlimited", r.Requirement.MaxStay)
	}
}

func TestLookup_UnknownPassport(t *testing.T) {
	r := Lookup("XX", "FI")
	if r.Success {
		t.Fatal("expected failure for unknown passport")
	}
	if r.Error == "" {
		t.Error("expected error message")
	}
}

func TestLookup_UnknownDestination(t *testing.T) {
	r := Lookup("FI", "ZZ")
	if r.Success {
		t.Fatal("expected failure for unknown destination")
	}
}

func TestLookup_EmptyInput(t *testing.T) {
	r := Lookup("", "JP")
	if r.Success {
		t.Fatal("expected failure for empty passport")
	}
}

func TestLookup_CaseInsensitive(t *testing.T) {
	r := Lookup("fi", "jp")
	if !r.Success {
		t.Fatal("expected success with lowercase")
	}
	if r.Requirement.Status != "visa-free" {
		t.Errorf("got status %q, want visa-free", r.Requirement.Status)
	}
}

func TestCountryName(t *testing.T) {
	if got := CountryName("FI"); got != "Finland" {
		t.Errorf("got %q, want Finland", got)
	}
	if got := CountryName("XX"); got != "XX" {
		t.Errorf("got %q, want XX", got)
	}
}

func TestListCountries(t *testing.T) {
	codes := ListCountries()
	if len(codes) < 50 {
		t.Errorf("expected 50+ countries, got %d", len(codes))
	}
	// Verify sorted.
	for i := 1; i < len(codes); i++ {
		if codes[i] < codes[i-1] {
			t.Errorf("not sorted: %s before %s", codes[i-1], codes[i])
		}
	}
}

func TestStatusEmoji(t *testing.T) {
	tests := []struct {
		status string
		want   string
	}{
		{"visa-free", "✅"},
		{"freedom-of-movement", "✅"},
		{"visa-on-arrival", "🟡"},
		{"e-visa", "🟠"},
		{"visa-required", "🔴"},
		{"unknown", "❓"},
	}
	for _, tt := range tests {
		if got := StatusEmoji(tt.status); got != tt.want {
			t.Errorf("StatusEmoji(%q) = %q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestLookup_UStoSchengen(t *testing.T) {
	r := Lookup("US", "FI")
	if !r.Success {
		t.Fatal("expected success")
	}
	if r.Requirement.Status != "visa-free" {
		t.Errorf("got status %q, want visa-free", r.Requirement.Status)
	}
}

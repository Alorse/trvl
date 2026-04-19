package hacks

import "testing"

func TestDepartureTaxSavings_high_tax_country(t *testing.T) {
	// AMS is in the Netherlands (NL) which has €26 departure tax.
	tax, country, ok := DepartureTaxSavings("AMS")
	if !ok {
		t.Fatal("expected ok=true for AMS (NL has departure tax)")
	}
	if country != "NL" {
		t.Errorf("country: got %q, want NL", country)
	}
	if tax != 26 {
		t.Errorf("tax: got %.0f, want 26", tax)
	}
}

func TestDepartureTaxSavings_zero_tax_country(t *testing.T) {
	// HEL is in Finland (FI) which has zero departure tax.
	_, _, ok := DepartureTaxSavings("HEL")
	if ok {
		t.Error("expected ok=false for HEL (FI has zero departure tax)")
	}
}

func TestDepartureTaxSavings_unknown_airport(t *testing.T) {
	_, _, ok := DepartureTaxSavings("XYZ")
	if ok {
		t.Error("expected ok=false for unknown airport")
	}
}

func TestDepartureTaxSavings_case_insensitive(t *testing.T) {
	tax, _, ok := DepartureTaxSavings("ams")
	if !ok {
		t.Fatal("expected ok=true for lowercase ams")
	}
	if tax != 26 {
		t.Errorf("tax: got %.0f, want 26", tax)
	}
}

func TestZeroTaxAlternatives_AMS(t *testing.T) {
	// AMS (NL, €26 tax) has nearby airports; some should be in zero-tax countries.
	alts := ZeroTaxAlternatives("AMS")
	// BRU is in Belgium (€3 tax, not zero) — should NOT appear.
	// EIN is in NL (€26) — should NOT appear.
	// We need to check what's actually zero-tax among AMS alternatives.
	for _, alt := range alts {
		cc := iataToCountry[alt.IATA]
		tax, has := departureTaxEUR[cc]
		if !has || tax != 0 {
			t.Errorf("alternative %s (%s) has tax %.0f, expected 0", alt.IATA, cc, tax)
		}
	}
}

func TestZeroTaxAlternatives_HEL(t *testing.T) {
	// HEL is in Finland (FI, zero tax) — NearbyAirports exist but
	// DepartureTaxSavings returns ok=false, so the optimizer won't call
	// ZeroTaxAlternatives. But the function itself should still work.
	alts := ZeroTaxAlternatives("HEL")
	// HEL nearby: TLL (EE, zero), RIX (LV, zero), VNO (LT, zero) — all zero-tax.
	if len(alts) == 0 {
		t.Error("expected at least one zero-tax alternative for HEL")
	}
	for _, alt := range alts {
		cc := iataToCountry[alt.IATA]
		tax := departureTaxEUR[cc]
		if tax != 0 {
			t.Errorf("alternative %s (%s) has tax %.0f, expected 0", alt.IATA, cc, tax)
		}
	}
}

func TestZeroTaxAlternatives_unknown(t *testing.T) {
	alts := ZeroTaxAlternatives("XYZ")
	if len(alts) != 0 {
		t.Errorf("expected no alternatives for unknown airport, got %d", len(alts))
	}
}

func TestCompetitiveRailRoute_match(t *testing.T) {
	minFare, operators, ok := CompetitiveRailRoute("MAD", "BCN")
	if !ok {
		t.Fatal("expected ok=true for MAD→BCN")
	}
	if minFare != 7 {
		t.Errorf("minFare: got %.0f, want 7", minFare)
	}
	if len(operators) != 4 {
		t.Errorf("operators: got %d, want 4", len(operators))
	}
}

func TestCompetitiveRailRoute_reverse_direction(t *testing.T) {
	_, operators, ok := CompetitiveRailRoute("BCN", "MAD")
	if !ok {
		t.Fatal("expected ok=true for BCN→MAD (reverse)")
	}
	if len(operators) == 0 {
		t.Error("expected operators for reverse direction")
	}
}

func TestCompetitiveRailRoute_no_match(t *testing.T) {
	_, _, ok := CompetitiveRailRoute("HEL", "BCN")
	if ok {
		t.Error("expected ok=false for HEL→BCN (no rail corridor)")
	}
}

func TestCompetitiveRailRoute_case_insensitive(t *testing.T) {
	_, _, ok := CompetitiveRailRoute("mad", "bcn")
	if !ok {
		t.Fatal("expected ok=true for lowercase mad→bcn")
	}
}

func TestOvernightFerryRoute_match(t *testing.T) {
	cabinEUR, hotelSavings, operator, ok := OvernightFerryRoute("HEL", "ARN")
	if !ok {
		t.Fatal("expected ok=true for HEL→ARN")
	}
	if cabinEUR != 35 {
		t.Errorf("cabinEUR: got %.0f, want 35", cabinEUR)
	}
	if hotelSavings != 85 {
		t.Errorf("hotelSavings: got %.0f, want 85 (120-35)", hotelSavings)
	}
	if operator == "" {
		t.Error("expected non-empty operator")
	}
}

func TestOvernightFerryRoute_no_match(t *testing.T) {
	_, _, _, ok := OvernightFerryRoute("HEL", "BCN")
	if ok {
		t.Error("expected ok=false for HEL→BCN (no ferry)")
	}
}

func TestOvernightFerryRoute_case_insensitive(t *testing.T) {
	_, _, _, ok := OvernightFerryRoute("hel", "arn")
	if !ok {
		t.Fatal("expected ok=true for lowercase hel→arn")
	}
}

func TestOvernightFerryRoute_low_savings_excluded(t *testing.T) {
	// ARN→RIX has cabin €45, hotel €50 — savings only €5, below threshold of 10.
	_, _, _, ok := OvernightFerryRoute("ARN", "RIX")
	if ok {
		t.Error("expected ok=false for ARN→RIX (savings < 10)")
	}
}

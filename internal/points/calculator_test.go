package points_test

import (
	"testing"

	"github.com/MikkoParkkola/trvl/internal/points"
)

func TestCalculateValue_UsePoints(t *testing.T) {
	// 450 USD / 60 000 points = 0.75 cpp  — below floor, should be "pay cash"
	// 450 USD / 20 000 points = 2.25 cpp  — at/above ceiling, should be "use points"
	r, err := points.CalculateValue(450, 20000, "finnair-plus")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Verdict != "use points" {
		t.Errorf("expected 'use points', got %q (cpp=%.4f)", r.Verdict, r.CPP)
	}
}

func TestCalculateValue_PayCash(t *testing.T) {
	// 100 USD / 100 000 points = 0.1 cpp — well below any floor
	r, err := points.CalculateValue(100, 100000, "finnair-plus")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Verdict != "pay cash" {
		t.Errorf("expected 'pay cash', got %q (cpp=%.4f)", r.Verdict, r.CPP)
	}
}

func TestCalculateValue_Borderline(t *testing.T) {
	// 450 USD / 60 000 points = 0.75 cpp  — below floor of finnair-plus (1.0)
	// Use a redemption that falls between floor and midpoint.
	// delta-skymiles floor=1.0, ceiling=1.8, midpoint=1.4
	// 100 USD / 8 500 points ≈ 1.18 cpp — above floor but below midpoint
	r, err := points.CalculateValue(100, 8500, "delta-skymiles")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Verdict != "borderline" {
		t.Errorf("expected 'borderline', got %q (cpp=%.4f)", r.Verdict, r.CPP)
	}
}

func TestCalculateValue_FinnairExample(t *testing.T) {
	// The canonical example from the issue.
	// 450 USD / 60 000 points = 0.75 cpp — below finnair floor (1.0) → pay cash
	r, err := points.CalculateValue(450, 60000, "finnair-plus")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.ProgramSlug != "finnair-plus" {
		t.Errorf("wrong program slug: %s", r.ProgramSlug)
	}
	if r.CashPrice != 450 {
		t.Errorf("wrong cash price: %f", r.CashPrice)
	}
	if r.PointsRequired != 60000 {
		t.Errorf("wrong points: %d", r.PointsRequired)
	}
	// CPP should be (450 * 100) / 60000 = 0.75
	wantCPP := 0.75
	if r.CPP < wantCPP-0.001 || r.CPP > wantCPP+0.001 {
		t.Errorf("expected cpp≈%.3f, got %.3f", wantCPP, r.CPP)
	}
	if r.Verdict != "pay cash" {
		t.Errorf("expected 'pay cash', got %q", r.Verdict)
	}
}

func TestCalculateValue_UnknownProgram(t *testing.T) {
	_, err := points.CalculateValue(100, 1000, "nonexistent-program")
	if err == nil {
		t.Fatal("expected error for unknown program")
	}
}

func TestCalculateValue_InvalidCashPrice(t *testing.T) {
	_, err := points.CalculateValue(0, 1000, "finnair-plus")
	if err == nil {
		t.Fatal("expected error for zero cash price")
	}
	_, err = points.CalculateValue(-50, 1000, "finnair-plus")
	if err == nil {
		t.Fatal("expected error for negative cash price")
	}
}

func TestCalculateValue_InvalidPoints(t *testing.T) {
	_, err := points.CalculateValue(100, 0, "finnair-plus")
	if err == nil {
		t.Fatal("expected error for zero points")
	}
	_, err = points.CalculateValue(100, -1, "finnair-plus")
	if err == nil {
		t.Fatal("expected error for negative points")
	}
}

func TestLookupProgram_AllSlugs(t *testing.T) {
	for _, prog := range points.Programs {
		got := points.LookupProgram(prog.Slug)
		if got == nil {
			t.Errorf("LookupProgram(%q) returned nil", prog.Slug)
			continue
		}
		if got.Slug != prog.Slug {
			t.Errorf("slug mismatch: want %q, got %q", prog.Slug, got.Slug)
		}
	}
}

func TestProgramCount(t *testing.T) {
	if len(points.Programs) < 20 {
		t.Errorf("expected at least 20 programs, got %d", len(points.Programs))
	}
}

func TestTransferPartnerCount(t *testing.T) {
	if len(points.TransferPartners) < 4 {
		t.Errorf("expected at least 4 transfer partners, got %d", len(points.TransferPartners))
	}
}

func TestListPrograms_AllCategories(t *testing.T) {
	all := points.ListPrograms("")
	if len(all) != len(points.Programs) {
		t.Errorf("ListPrograms(\"\") returned %d, want %d", len(all), len(points.Programs))
	}
}

func TestListPrograms_AirlineOnly(t *testing.T) {
	airlines := points.ListPrograms("airline")
	for _, p := range airlines {
		if p.Category != "airline" {
			t.Errorf("expected airline, got %q for %s", p.Category, p.Slug)
		}
	}
	if len(airlines) == 0 {
		t.Error("expected at least one airline program")
	}
}

func TestCalculateValue_HotelProgram(t *testing.T) {
	// World of Hyatt: floor=1.5, ceiling=2.5
	// 250 USD / 15 000 points = 1.67 cpp — above floor, below midpoint (2.0) → borderline
	r, err := points.CalculateValue(250, 15000, "world-of-hyatt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.ProgramName == "" {
		t.Error("expected non-empty program name")
	}
	if r.FloorCPP <= 0 || r.CeilingCPP <= 0 {
		t.Errorf("invalid floor/ceiling: %.2f / %.2f", r.FloorCPP, r.CeilingCPP)
	}
	if r.Explanation == "" {
		t.Error("expected non-empty explanation")
	}
}

func TestCalculateValue_TransferableCurrency(t *testing.T) {
	// Amex MR: floor=1.0, ceiling=2.0
	// 300 USD / 15 000 = 2.0 cpp → at ceiling → "use points"
	r, err := points.CalculateValue(300, 15000, "amex-mr")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Verdict != "use points" {
		t.Errorf("expected 'use points', got %q", r.Verdict)
	}
}

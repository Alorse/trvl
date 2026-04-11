package lounges

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestSearchLounges_StaticFallback verifies that known hub airports return results
// without any external API call.
func TestSearchLounges_StaticFallback(t *testing.T) {
	// Point LoungeBuddy at an address that always fails so the static fallback runs.
	orig := loungebuddyBaseURL
	loungebuddyBaseURL = "http://127.0.0.1:0" // unreachable
	defer func() { loungebuddyBaseURL = orig }()

	result, err := SearchLounges(context.Background(), "HEL")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}
	if result.Source != "static" {
		t.Errorf("source: got %q, want static", result.Source)
	}
	if result.Airport != "HEL" {
		t.Errorf("airport: got %q, want HEL", result.Airport)
	}
	if result.Count == 0 {
		t.Error("expected at least 1 lounge for HEL")
	}
	if len(result.Lounges) != result.Count {
		t.Errorf("count mismatch: Count=%d len(Lounges)=%d", result.Count, len(result.Lounges))
	}
	// Verify all HEL lounges have required fields.
	for _, l := range result.Lounges {
		if l.Name == "" {
			t.Error("lounge has empty name")
		}
		if l.Airport != "HEL" {
			t.Errorf("lounge airport: got %q, want HEL", l.Airport)
		}
		if len(l.Cards) == 0 {
			t.Errorf("lounge %q has no access cards", l.Name)
		}
	}
}

// TestSearchLounges_UnknownAirport verifies that an unknown airport returns a
// successful empty result (graceful degradation, not an error).
func TestSearchLounges_UnknownAirport(t *testing.T) {
	orig := loungebuddyBaseURL
	loungebuddyBaseURL = "http://127.0.0.1:0"
	defer func() { loungebuddyBaseURL = orig }()

	result, err := SearchLounges(context.Background(), "XYZ")
	if err != nil {
		t.Fatalf("unexpected error for unknown airport: %v", err)
	}
	if !result.Success {
		t.Error("expected success even for unknown airport")
	}
	if result.Count != 0 {
		t.Errorf("expected 0 lounges for XYZ, got %d", result.Count)
	}
}

// TestSearchLounges_InvalidIATA verifies that invalid codes are rejected.
func TestSearchLounges_InvalidIATA(t *testing.T) {
	for _, code := range []string{"", "HE", "HELL", "12", "123"} {
		_, err := SearchLounges(context.Background(), code)
		if err == nil {
			t.Errorf("expected error for code %q, got nil", code)
		}
	}
}

// TestSearchLounges_LoungebuddyAPI tests the live-API path against a mock server.
func TestSearchLounges_LoungebuddyAPI(t *testing.T) {
	mockBody := loungebuddyLoungesResponse{
		Lounges: []struct {
			Name      string   `json:"name"`
			Terminal  string   `json:"terminal"`
			Cards     []string `json:"cards"`
			Amenities []string `json:"amenities"`
			OpenHours string   `json:"hours"`
		}{
			{
				Name:      "Test Lounge",
				Terminal:  "Terminal 1",
				Cards:     []string{"Priority Pass", "LoungeKey"},
				Amenities: []string{"Wi-Fi", "Bar"},
				OpenHours: "06:00–22:00",
			},
		},
	}
	body, _ := json.Marshal(mockBody)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/lounges" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("airport") != "TST" {
			http.Error(w, "bad airport", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	orig := loungebuddyBaseURL
	loungebuddyBaseURL = srv.URL
	defer func() { loungebuddyBaseURL = orig }()

	result, err := SearchLounges(context.Background(), "TST")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got: %s", result.Error)
	}
	if result.Source != "loungebuddy" {
		t.Errorf("source: got %q, want loungebuddy", result.Source)
	}
	if result.Count != 1 {
		t.Fatalf("count: got %d, want 1", result.Count)
	}
	lounge := result.Lounges[0]
	if lounge.Name != "Test Lounge" {
		t.Errorf("name: got %q, want Test Lounge", lounge.Name)
	}
	if lounge.Terminal != "Terminal 1" {
		t.Errorf("terminal: got %q, want Terminal 1", lounge.Terminal)
	}
	if len(lounge.Cards) != 2 {
		t.Errorf("cards: got %d, want 2", len(lounge.Cards))
	}
	if lounge.Airport != "TST" {
		t.Errorf("airport: got %q, want TST", lounge.Airport)
	}
}

// TestAnnotateAccess verifies that user cards are matched case-insensitively.
func TestAnnotateAccess(t *testing.T) {
	result := &SearchResult{
		Lounges: []Lounge{
			{Name: "Lounge A", Cards: []string{"Priority Pass", "LoungeKey"}},
			{Name: "Lounge B", Cards: []string{"Amex Platinum", "Diners Club"}},
			{Name: "Lounge C", Cards: []string{"Dragon Pass"}},
		},
	}

	// User has Priority Pass and Diners Club (different casing).
	AnnotateAccess(result, []string{"priority pass", "Diners Club"})

	// Lounge A: user has Priority Pass.
	if len(result.Lounges[0].AccessibleWith) != 1 || result.Lounges[0].AccessibleWith[0] != "priority pass" {
		t.Errorf("Lounge A AccessibleWith: got %v, want [priority pass]", result.Lounges[0].AccessibleWith)
	}
	// Lounge B: user has Diners Club.
	if len(result.Lounges[1].AccessibleWith) != 1 || result.Lounges[1].AccessibleWith[0] != "Diners Club" {
		t.Errorf("Lounge B AccessibleWith: got %v, want [Diners Club]", result.Lounges[1].AccessibleWith)
	}
	// Lounge C: user has no matching card.
	if len(result.Lounges[2].AccessibleWith) != 0 {
		t.Errorf("Lounge C AccessibleWith: got %v, want []", result.Lounges[2].AccessibleWith)
	}
}

// TestAnnotateAccess_Nil verifies nil safety.
func TestAnnotateAccess_Nil(t *testing.T) {
	// Should not panic.
	AnnotateAccess(nil, []string{"Priority Pass"})
	AnnotateAccess(&SearchResult{}, nil)
	AnnotateAccess(&SearchResult{Lounges: []Lounge{{Name: "X"}}}, nil)
}

// TestStaticFallback_AllAirports verifies that all airports in the static dataset
// return valid data (no empty names, all have access cards).
func TestStaticFallback_AllAirports(t *testing.T) {
	for airport, entries := range staticData {
		t.Run(airport, func(t *testing.T) {
			result := staticFallback(airport)
			if !result.Success {
				t.Fatal("expected success")
			}
			if result.Count != len(entries) {
				t.Errorf("count: got %d, want %d", result.Count, len(entries))
			}
			for _, l := range result.Lounges {
				if strings.TrimSpace(l.Name) == "" {
					t.Error("lounge has empty name")
				}
				if len(l.Cards) == 0 {
					t.Errorf("lounge %q has no access cards", l.Name)
				}
			}
		})
	}
}

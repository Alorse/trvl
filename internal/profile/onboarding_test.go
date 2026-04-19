package profile

import "testing"

// helper: find question by key.
func findQuestion(qs []Question, key string) *Question {
	for i := range qs {
		if qs[i].Key == key {
			return &qs[i]
		}
	}
	return nil
}

func TestPhase1EmptyProfile(t *testing.T) {
	qs, instr := OnboardingQuestions(1, &TravelProfile{}, nil)
	if len(qs) == 0 {
		t.Fatal("expected questions for empty profile")
	}
	if instr == "" {
		t.Error("expected non-empty LLM instructions")
	}
	// All core phase-1 keys should be present.
	for _, key := range []string{"home", "travel_frequency", "travel_companions", "loyalty"} {
		if findQuestion(qs, key) == nil {
			t.Errorf("missing question %q", key)
		}
	}
}

func TestPhase1SkipsHomeWhenDetected(t *testing.T) {
	prof := &TravelProfile{HomeDetected: []string{"HEL"}}
	qs, _ := OnboardingQuestions(1, prof, nil)
	if findQuestion(qs, "home") != nil {
		t.Error("expected home question to be skipped when HomeDetected is set")
	}
	// Other questions should still appear.
	if findQuestion(qs, "travel_frequency") == nil {
		t.Error("travel_frequency should still be asked")
	}
}

func TestPhase1NilProfile(t *testing.T) {
	qs, _ := OnboardingQuestions(1, nil, nil)
	if len(qs) == 0 {
		t.Fatal("expected questions even with nil profile")
	}
}

func TestPhase1SkipsKidsForSolo(t *testing.T) {
	qs, _ := OnboardingQuestions(1, &TravelProfile{}, map[string]string{
		"travel_companions": "solo",
	})
	if findQuestion(qs, "kids") != nil {
		t.Error("expected kids question to be skipped for solo traveller")
	}
}

func TestPhase1KidsAskedWhenNotSolo(t *testing.T) {
	qs, _ := OnboardingQuestions(1, &TravelProfile{}, map[string]string{
		"travel_companions": "partner",
	})
	if findQuestion(qs, "kids") == nil {
		t.Error("expected kids question when travelling with partner")
	}
}

func TestPhase1SkipsAnsweredQuestions(t *testing.T) {
	answers := map[string]string{
		"home":              "Helsinki",
		"travel_frequency":  "monthly",
		"travel_companions": "solo",
		"loyalty":           "Finnair Plus",
	}
	qs, _ := OnboardingQuestions(1, &TravelProfile{}, answers)
	// All answered + solo (no kids) => nothing left.
	if len(qs) != 0 {
		t.Errorf("expected no questions when all answered, got %d", len(qs))
	}
}

func TestPhase2ReturnsAccommodationQuestions(t *testing.T) {
	qs, _ := OnboardingQuestions(2, &TravelProfile{}, nil)
	if len(qs) == 0 {
		t.Fatal("expected phase-2 questions")
	}
	if findQuestion(qs, "accom_type") == nil {
		t.Error("expected accom_type question in phase 2")
	}
	if findQuestion(qs, "nightly_budget") == nil {
		t.Error("expected nightly_budget question in phase 2")
	}
}

func TestPhase2SkipsAccomTypeWhenProfileHasPreferredType(t *testing.T) {
	prof := &TravelProfile{PreferredType: "hotel"}
	qs, _ := OnboardingQuestions(2, prof, nil)
	if findQuestion(qs, "accom_type") != nil {
		t.Error("expected accom_type to be skipped when profile has PreferredType")
	}
}

func TestPhase2SkipsBudgetWhenProfileHasRate(t *testing.T) {
	prof := &TravelProfile{AvgNightlyRate: 120}
	qs, _ := OnboardingQuestions(2, prof, nil)
	if findQuestion(qs, "nightly_budget") != nil {
		t.Error("expected nightly_budget to be skipped when profile has AvgNightlyRate")
	}
}

func TestPhase2SkipsGroundWhenProfileHasGroundData(t *testing.T) {
	prof := &TravelProfile{TopGroundModes: []ModeStats{{Mode: "train", Count: 5}}}
	qs, _ := OnboardingQuestions(2, prof, nil)
	if findQuestion(qs, "transport_modes") != nil {
		t.Error("expected transport_modes to be skipped when profile has ground mode data")
	}
}

func TestPhase3ReturnsQuestionsForEmptyProfile(t *testing.T) {
	qs, _ := OnboardingQuestions(3, &TravelProfile{}, nil)
	if len(qs) == 0 {
		t.Fatal("expected phase-3 questions for empty profile")
	}
	if findQuestion(qs, "favourite_destinations") == nil {
		t.Error("expected favourite_destinations question")
	}
	if findQuestion(qs, "food_style") == nil {
		t.Error("expected food_style question")
	}
	if findQuestion(qs, "travel_hacks") == nil {
		t.Error("expected travel_hacks question")
	}
}

func TestPhase3SkipsFavouriteDestinationsWhenProfileHasThem(t *testing.T) {
	prof := &TravelProfile{TopDestinations: []string{"Prague", "Amsterdam"}}
	qs, _ := OnboardingQuestions(3, prof, nil)
	if findQuestion(qs, "favourite_destinations") != nil {
		t.Error("expected favourite_destinations to be skipped when profile has TopDestinations")
	}
}

func TestPhase3SkipsLoungesWhenProfileHasAlliance(t *testing.T) {
	prof := &TravelProfile{PreferredAlliance: "Star Alliance"}
	qs, _ := OnboardingQuestions(3, prof, nil)
	if findQuestion(qs, "lounges") != nil {
		t.Error("expected lounges question to be skipped when profile has PreferredAlliance")
	}
}

func TestPhase4ReturnsWishlistQuestion(t *testing.T) {
	qs, _ := OnboardingQuestions(4, &TravelProfile{}, nil)
	if findQuestion(qs, "wishlist") == nil {
		t.Error("expected wishlist question in phase 4")
	}
}

func TestPhase4SkipsCompanionDetailsForSolo(t *testing.T) {
	qs, _ := OnboardingQuestions(4, &TravelProfile{}, map[string]string{
		"travel_companions": "solo",
	})
	if findQuestion(qs, "companion_details") != nil {
		t.Error("expected companion_details to be skipped for solo traveller")
	}
}

func TestPhase4CompanionDetailsAskedForNonSolo(t *testing.T) {
	qs, _ := OnboardingQuestions(4, &TravelProfile{}, map[string]string{
		"travel_companions": "partner",
	})
	if findQuestion(qs, "companion_details") == nil {
		t.Error("expected companion_details when not solo")
	}
}

func TestPhase4ReturnsMotivationQuestion(t *testing.T) {
	qs, _ := OnboardingQuestions(4, &TravelProfile{}, nil)
	q := findQuestion(qs, "motivation")
	if q == nil {
		t.Fatal("expected motivation question in phase 4")
	}
	if len(q.Options) == 0 {
		t.Error("motivation question should have options")
	}
}

func TestFullProfileReturnsFewerQuestions(t *testing.T) {
	full := &TravelProfile{
		HomeDetected:    []string{"HEL"},
		TopDestinations: []string{"Prague", "Amsterdam", "Barcelona"},
		PreferredType:   "hotel",
		AvgNightlyRate:  120,
		TopGroundModes:  []ModeStats{{Mode: "train", Count: 5}},
		PreferredDays:   []string{"Friday", "Monday"},
		TopHotelChains:  []HotelChainStats{{Name: "Marriott", Nights: 10}},
		PreferredAlliance: "Star Alliance",
	}

	emptyQs1, _ := OnboardingQuestions(1, &TravelProfile{}, nil)
	fullQs1, _ := OnboardingQuestions(1, full, nil)
	if len(fullQs1) >= len(emptyQs1) {
		t.Errorf("full profile phase 1: expected fewer questions (%d) than empty (%d)", len(fullQs1), len(emptyQs1))
	}

	emptyQs2, _ := OnboardingQuestions(2, &TravelProfile{}, nil)
	fullQs2, _ := OnboardingQuestions(2, full, nil)
	if len(fullQs2) >= len(emptyQs2) {
		t.Errorf("full profile phase 2: expected fewer questions (%d) than empty (%d)", len(fullQs2), len(emptyQs2))
	}

	emptyQs3, _ := OnboardingQuestions(3, &TravelProfile{}, nil)
	fullQs3, _ := OnboardingQuestions(3, full, nil)
	if len(fullQs3) >= len(emptyQs3) {
		t.Errorf("full profile phase 3: expected fewer questions (%d) than empty (%d)", len(fullQs3), len(emptyQs3))
	}
}

func TestInvalidPhaseReturnsEmpty(t *testing.T) {
	qs, instr := OnboardingQuestions(0, &TravelProfile{}, nil)
	if len(qs) != 0 {
		t.Errorf("expected empty questions for phase 0, got %d", len(qs))
	}
	if instr == "" {
		t.Error("expected non-empty instructions even for invalid phase")
	}

	qs, _ = OnboardingQuestions(5, &TravelProfile{}, nil)
	if len(qs) != 0 {
		t.Errorf("expected empty questions for phase 5, got %d", len(qs))
	}
}

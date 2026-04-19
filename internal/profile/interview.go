package profile

import (
	"github.com/MikkoParkkola/trvl/internal/preferences"
)

// TripContext captures per-trip context gathered from the pre-search interview.
// Unlike the persistent TravelProfile (patterns) and Preferences (desires),
// this is ephemeral — it applies to a single search session.
type TripContext struct {
	Travellers     int      `json:"travellers"`
	TravellerNames []string `json:"traveller_names,omitempty"`
	DatesFixed     bool     `json:"dates_fixed"`
	DateFlexDays   int      `json:"date_flex_days"`
	BudgetTotal    float64  `json:"budget_total,omitempty"`
	BudgetFlight   float64  `json:"budget_flight,omitempty"`
	BudgetPerNight float64  `json:"budget_per_night,omitempty"`
	LuggageNeeds   string   `json:"luggage_needs"`
	AccomNeeds     []string `json:"accom_needs,omitempty"`
	Purpose        string   `json:"purpose"`
	TransportFlex  string   `json:"transport_flex"`
	SpecialNeeds   []string `json:"special_needs,omitempty"`
	Priority       string   `json:"priority"`
}

// Question represents a single interview question for the pre-search flow.
type Question struct {
	Key       string   `json:"key"`
	Text      string   `json:"text"`
	Type      string   `json:"type"` // "number", "choice", "multi_choice", "text"
	Options   []string `json:"options,omitempty"`
	Default   string   `json:"default,omitempty"`
	Required  bool     `json:"required"`
	Inference string   `json:"inference,omitempty"` // LLM's inferred value for phase-0 confirmation questions
}

// InterviewQuestions returns the minimal set of questions to ask before searching,
// given what we already know from the user's profile and preferences. Questions
// that can be inferred from existing data are skipped.
func InterviewQuestions(prof *TravelProfile, prefs *preferences.Preferences) []Question {
	var questions []Question

	// 1. Travellers — skip if profile consistently shows solo travel.
	if !isConsistentSolo(prof) {
		q := Question{
			Key:      "travellers",
			Text:     "Just you, or others joining?",
			Type:     "number",
			Default:  "1",
			Required: true,
		}
		if prof != nil && prefs != nil && prefs.DefaultCompanions > 0 {
			// Profile suggests a default.
		} else if prefs != nil && prefs.DefaultCompanions > 0 {
			q.Default = intToStr(prefs.DefaultCompanions + 1) // companions + self
		}
		questions = append(questions, q)
	}

	// 2. Dates — always ask (no way to infer from history).
	questions = append(questions, Question{
		Key:      "dates_fixed",
		Text:     "Are your dates fixed, or flexible?",
		Type:     "choice",
		Options:  []string{"fixed", "flexible_1_day", "flexible_3_days", "flexible_week", "anytime"},
		Required: true,
	})

	// 3. Budget — skip if prefs already has budget set.
	if prefs == nil || (prefs.BudgetPerNightMax == 0 && prefs.BudgetFlightMax == 0) {
		q := Question{
			Key:      "budget",
			Text:     "Any budget limit for this trip?",
			Type:     "text",
			Required: false,
		}
		// Use profile average as smart default.
		if prof != nil && prof.AvgTripCost > 0 {
			q.Default = formatCurrency(prof.AvgTripCost)
		}
		questions = append(questions, q)
	}

	// 4. Luggage — skip if prefs has carry_on_only set explicitly.
	if prefs == nil || !hasLuggagePreference(prefs) {
		q := Question{
			Key:      "luggage",
			Text:     "Carry-on only or checking bags?",
			Type:     "choice",
			Options:  []string{"carry_on", "checked", "heavy"},
			Required: false,
		}
		if prof != nil && prof.BudgetTier == "budget" {
			q.Default = "carry_on"
		}
		questions = append(questions, q)
	}

	// 5. Accommodation needs — skip if prefs has accommodation details.
	if prefs == nil || !hasAccomPreference(prefs) {
		questions = append(questions, Question{
			Key:      "accom_needs",
			Text:     "Any must-haves for accommodation?",
			Type:     "multi_choice",
			Options:  []string{"laundry", "kitchen", "workspace", "parking", "pool", "gym", "pet_friendly"},
			Required: false,
		})
	}

	// 6. Purpose — affects search strategy.
	questions = append(questions, Question{
		Key:      "purpose",
		Text:     "What kind of trip?",
		Type:     "choice",
		Options:  []string{"leisure", "remote_work", "business", "city_break", "beach", "adventure"},
		Required: false,
		Default:  inferDefaultPurpose(prefs),
	})

	// 7. Priority — skip if prefs has deal_tolerance.
	if prefs == nil || prefs.DealTolerance == "" {
		q := Question{
			Key:      "priority",
			Text:     "What matters most?",
			Type:     "choice",
			Options:  []string{"price", "comfort", "speed", "experience"},
			Required: false,
		}
		if prof != nil {
			q.Default = inferPriority(prof)
		}
		questions = append(questions, q)
	}

	return questions
}

// isConsistentSolo returns true if the profile consistently shows solo travel.
func isConsistentSolo(prof *TravelProfile) bool {
	if prof == nil || len(prof.Bookings) < 3 {
		return false
	}
	// If all flight bookings are single-passenger (heuristic: we don't track
	// passengers explicitly, so look at booking notes/price patterns).
	// For now, we can't reliably detect this, so default to asking.
	return false
}

// hasLuggagePreference returns true if the user has explicitly set luggage prefs.
func hasLuggagePreference(prefs *preferences.Preferences) bool {
	// CarryOnOnly is the only luggage-related pref; its zero value (false)
	// could mean "not set" or "allows checked". We consider it set if
	// the user has home airports configured (meaning they've done the setup).
	return prefs.CarryOnOnly && len(prefs.HomeAirports) > 0
}

// hasAccomPreference returns true if the user has accommodation preferences set.
func hasAccomPreference(prefs *preferences.Preferences) bool {
	return prefs.NoDormitories || prefs.EnSuiteOnly || prefs.FastWifiNeeded || prefs.MinHotelStars > 0
}

// inferDefaultPurpose guesses the trip purpose from preferences.
func inferDefaultPurpose(prefs *preferences.Preferences) string {
	if prefs == nil {
		return ""
	}
	if prefs.FastWifiNeeded {
		return "remote_work"
	}
	if len(prefs.TripTypes) > 0 {
		return prefs.TripTypes[0]
	}
	return ""
}

// inferPriority guesses priority from profile budget tier.
func inferPriority(prof *TravelProfile) string {
	switch prof.BudgetTier {
	case "budget":
		return "price"
	case "premium":
		return "comfort"
	default:
		return ""
	}
}

// intToStr converts int to string without importing strconv.
func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	if neg {
		s = "-" + s
	}
	return s
}

// formatCurrency formats a float as a simple currency string.
func formatCurrency(amount float64) string {
	whole := int(amount)
	return intToStr(whole)
}

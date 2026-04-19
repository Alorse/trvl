package profile

import "strings"

// FlightSearchHints carries profile-derived defaults for a flight search.
// Every field is a soft default: the MCP handler applies it only when the
// caller has not already set that parameter explicitly.
type FlightSearchHints struct {
	// PreferredAirlines is a ranked list of IATA codes the user flies most.
	// Empty slice means no preference.
	PreferredAirlines []string

	// PreferredAlliance is the user's statistically dominant alliance
	// ("STAR_ALLIANCE", "ONEWORLD", "SKYTEAM"), or "" if none detected.
	PreferredAlliance string

	// CabinClass is the default cabin derived from booking history and budget
	// tier. Zero value means economy (caller default).
	// 1=Economy, 2=Premium Economy, 3=Business, 4=First
	CabinClass int

	// MaxPrice is the user's typical flight budget ceiling (AvgFlightPrice × 1.5).
	// 0 means apply no price hint.
	MaxPrice int

	// Adults is the companion count for this travel mode (+1 for the traveller
	// themselves). 0 means the caller should use their own default (1).
	Adults int
}

// HotelSearchHints carries profile-derived defaults for a hotel search.
type HotelSearchHints struct {
	// MinStars is the minimum star rating from history. 0 = no hint.
	MinStars int

	// MinRating is the minimum guest-rating score (0-10). 0 = no hint.
	MinRating float64

	// MaxPrice is the typical nightly ceiling (AvgNightlyRate × BudgetFlex).
	// 0 = no hint.
	MaxPrice float64

	// PropertyType is the preferred property type from booking history:
	// "hotel", "apartment", etc.  "" = no hint.
	PropertyType string

	// PreferredNeighbourhoods lists neighbourhoods the user has mentioned for
	// this city (from CityIntelligence). Empty = no hint.
	PreferredNeighbourhoods []string

	// Guests is the companion count for this travel mode (+1 for the traveller).
	// 0 means the caller should use their own default.
	Guests int
}

// GroundSearchHints carries profile-derived defaults for a ground transport search.
type GroundSearchHints struct {
	// PreferredType is the statistically dominant transport mode ("bus", "train",
	// "ferry"). "" = no hint (all modes returned).
	PreferredType string
}

// DetectTravelMode returns the TravelMode that best matches the companion count.
// Returns nil when no TravelModes are defined in the profile.
//
// Matching rules (first match wins):
//  1. Exact companions count match.
//  2. Mode with companions count closest to the supplied value.
//  3. First mode in the list.
func DetectTravelMode(prof *TravelProfile, companions int) *TravelMode {
	if prof == nil || len(prof.TravelModes) == 0 {
		return nil
	}

	// Exact match first.
	for i := range prof.TravelModes {
		if prof.TravelModes[i].Companions == companions {
			return &prof.TravelModes[i]
		}
	}

	// Closest match.
	best := &prof.TravelModes[0]
	bestDiff := abs(best.Companions - companions)
	for i := 1; i < len(prof.TravelModes); i++ {
		d := abs(prof.TravelModes[i].Companions - companions)
		if d < bestDiff {
			bestDiff = d
			best = &prof.TravelModes[i]
		}
	}
	return best
}

// FlightHints returns profile-derived flight search defaults for the given
// origin → destination pair. When the profile is nil or empty, all hints are
// zero values (no-ops in the MCP handler).
func FlightHints(prof *TravelProfile, origin, dest string) FlightSearchHints {
	if prof == nil {
		return FlightSearchHints{}
	}

	h := FlightSearchHints{}

	// Top airlines — take the three most-flown.
	for i, a := range prof.TopAirlines {
		if i >= 3 {
			break
		}
		if a.Code != "" {
			h.PreferredAirlines = append(h.PreferredAirlines, a.Code)
		}
	}

	h.PreferredAlliance = prof.PreferredAlliance

	// Cabin class from budget tier.
	switch prof.BudgetTier {
	case "budget":
		h.CabinClass = 1 // Economy
	case "mid-range", "":
		h.CabinClass = 1 // Economy
	case "premium":
		h.CabinClass = 2 // Premium Economy
	case "luxury":
		h.CabinClass = 3 // Business
	}

	// Price hint: 1.5× average flight price, rounded down.
	if prof.AvgFlightPrice > 0 {
		h.MaxPrice = int(prof.AvgFlightPrice * 1.5)
	}

	// Check if there is a known travel mode for this route (use default
	// companions = solo = 0 as baseline; the MCP handler can override if the
	// guests arg is explicit).
	mode := DetectTravelMode(prof, 0)
	if mode != nil {
		h.Adults = mode.Companions + 1
	}

	// Route-specific override: if we have a RouteStats entry for this pair,
	// use its average price as a tighter ceiling.
	ou := strings.ToUpper(origin)
	du := strings.ToUpper(dest)
	for _, r := range prof.TopRoutes {
		if strings.ToUpper(r.From) == ou && strings.ToUpper(r.To) == du {
			if r.AvgPrice > 0 {
				h.MaxPrice = int(r.AvgPrice * 1.5)
			}
			break
		}
	}

	return h
}

// HotelHints returns profile-derived hotel search defaults for the given
// location (city name or airport code). When the profile is nil or empty,
// all hints are zero values (no-ops in the MCP handler).
func HotelHints(prof *TravelProfile, location string) HotelSearchHints {
	if prof == nil {
		return HotelSearchHints{}
	}

	h := HotelSearchHints{}

	// Star rating from history.
	if prof.AvgStarRating >= 1 {
		h.MinStars = int(prof.AvgStarRating)
		// Don't demand more than they've averaged.
		if float64(h.MinStars) > prof.AvgStarRating {
			h.MinStars--
		}
	}

	// Property type from history.
	h.PropertyType = prof.PreferredType

	// Price ceiling: average nightly rate, optionally stretched by budget
	// flexibility from the matching travel mode.
	if prof.AvgNightlyRate > 0 {
		flex := 1.3 // default: allow 30% above average
		mode := DetectTravelMode(prof, 0)
		if mode != nil && mode.BudgetFlex > 0 {
			flex = mode.BudgetFlex
		}
		h.MaxPrice = prof.AvgNightlyRate * flex
	}

	// City-specific intelligence.
	locationLower := strings.ToLower(location)
	for _, ci := range prof.CityIntelligence {
		if strings.ToLower(ci.City) == locationLower {
			h.PreferredNeighbourhoods = ci.Neighbourhoods
			break
		}
	}

	// Guests from default travel mode.
	mode := DetectTravelMode(prof, 0)
	if mode != nil {
		h.Guests = mode.Companions + 1
	}

	// Budget tier can tighten or loosen the price ceiling.
	switch prof.BudgetTier {
	case "budget":
		if h.MaxPrice > 0 {
			h.MaxPrice = h.MaxPrice * 0.8
		}
	case "luxury":
		h.MaxPrice = 0 // no ceiling for luxury travellers
		if h.MinStars < 4 {
			h.MinStars = 4
		}
	}

	// Elasticity: if the user has "location" as a dealbreaker or will_pay_more
	// factor, relax the max price so we don't filter out centrally located gems.
	for _, e := range prof.PriceElasticity {
		if e.Factor == "location" && e.Impact == "will_pay_more" && e.PriceDelta > 1 {
			if h.MaxPrice > 0 {
				h.MaxPrice = h.MaxPrice * e.PriceDelta
			}
		}
	}

	return h
}

// GroundHints returns profile-derived ground transport search defaults for the
// from → to pair. When the profile is nil or empty, all hints are zero values.
func GroundHints(prof *TravelProfile, from, to string) GroundSearchHints {
	if prof == nil || len(prof.TopGroundModes) == 0 {
		return GroundSearchHints{}
	}

	// Pick the most-used mode.
	top := prof.TopGroundModes[0]
	for _, m := range prof.TopGroundModes[1:] {
		if m.Count > top.Count {
			top = m
		}
	}

	// Normalise mode names to the values search_ground accepts.
	mode := strings.ToLower(top.Mode)
	switch mode {
	case "bus", "train", "ferry":
		// Already valid.
	default:
		mode = "" // unknown → no hint
	}

	return GroundSearchHints{PreferredType: mode}
}

// abs returns the absolute value of an int.
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

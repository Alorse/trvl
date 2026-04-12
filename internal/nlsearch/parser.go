// Package nlsearch parses free-form natural-language travel queries into
// structured search parameters. It is shared with the MCP search_natural
// tool path so the CLI and the AI surface use identical parsing semantics
// for the basic intent + weekend resolution. The CLI surface additionally
// extracts IATA codes and "from X to Y" patterns so it can dispatch real
// searches without an LLM.
package nlsearch

import (
	"regexp"
	"strings"
	"time"
)

// Params holds the structured parameters extracted from a free-form query.
type Params struct {
	Intent        string   `json:"intent"`          // "route", "flight", "hotel", "deals"
	Origin        string   `json:"origin"`          // IATA code (uppercase) when extractable
	Destination   string   `json:"destination"`     // IATA code (uppercase) when extractable
	Date          string   `json:"date"`            // YYYY-MM-DD or empty
	ReturnDate    string   `json:"return_date"`     // YYYY-MM-DD or empty
	CheckIn       string   `json:"check_in"`        // YYYY-MM-DD (hotels)
	CheckOut      string   `json:"check_out"`       // YYYY-MM-DD (hotels)
	MaxBudget     float64  `json:"max_budget"`      // 0 = unlimited
	TravelerCount int      `json:"traveler_count"`  // 0 = unspecified (default 1 or 2)
	Modes         []string `json:"transport_modes"` // "flight", "train", "bus", "ferry"
	Location      string   `json:"location"`        // hotel location when intent=hotel
}

// iataPattern matches a 3-letter uppercase IATA airport code.
var iataPattern = regexp.MustCompile(`\b([A-Z]{3})\b`)

// fromToPattern captures "from X to Y" with X and Y being IATA codes.
var fromToPattern = regexp.MustCompile(`(?i)from\s+([A-Z]{3})\s+to\s+([A-Z]{3})`)

// isoDatePattern captures YYYY-MM-DD date literals.
var isoDatePattern = regexp.MustCompile(`\b(\d{4})-(\d{2})-(\d{2})\b`)

// Heuristic extracts travel intent and parameters from a free-form query
// using keyword matching and simple date resolution. `today` must be in
// YYYY-MM-DD format.
//
// Extraction order:
//  1. Intent detection (hotel/flight/deals/route default).
//  2. IATA codes via fromToPattern, then bare 3-letter uppercase tokens.
//  3. ISO 8601 dates (first match → Date, second → ReturnDate).
//  4. "next weekend" / "this weekend" relative dates (only if no ISO date found).
func Heuristic(query, today string) Params {
	lower := strings.ToLower(query)
	upper := strings.ToUpper(query)

	p := Params{Intent: "route"}

	// 1. Intent detection.
	switch {
	case strings.Contains(lower, "hotel") || strings.Contains(lower, "hostel") ||
		strings.Contains(lower, "accommodation") || strings.Contains(lower, "stay") ||
		strings.Contains(lower, "sleep") || strings.Contains(lower, "room") ||
		strings.Contains(lower, "check-in") || strings.Contains(lower, "check in"):
		p.Intent = "hotel"
	case strings.Contains(lower, "fly ") || strings.Contains(lower, "flying") ||
		strings.Contains(lower, "flight") || strings.Contains(lower, "airport"):
		p.Intent = "flight"
	case strings.Contains(lower, "deal") || strings.Contains(lower, "inspiration"):
		p.Intent = "deals"
	}

	// 2. IATA extraction. Prefer "from X to Y" if present.
	if m := fromToPattern.FindStringSubmatch(query); len(m) == 3 {
		p.Origin = strings.ToUpper(m[1])
		p.Destination = strings.ToUpper(m[2])
	} else if codes := iataPattern.FindAllString(upper, -1); len(codes) > 0 {
		// Skip 3-letter false positives that are common English words.
		filtered := filterFalsePositiveIATA(codes)
		if len(filtered) >= 2 {
			p.Origin = filtered[0]
			p.Destination = filtered[1]
		} else if len(filtered) == 1 {
			// Single IATA: assume it's the destination for hotel/route intent.
			p.Destination = filtered[0]
		}
	}

	// 3. ISO 8601 dates.
	if dates := isoDatePattern.FindAllString(query, -1); len(dates) > 0 {
		p.Date = dates[0]
		p.CheckIn = dates[0]
		if len(dates) >= 2 {
			p.ReturnDate = dates[1]
			p.CheckOut = dates[1]
		}
	}

	// 4. Relative weekend dates — only if no ISO date was extracted.
	if p.Date == "" && (strings.Contains(lower, "next weekend") || strings.Contains(lower, "this weekend")) {
		t, _ := time.Parse("2006-01-02", today)
		daysUntilSat := (6 - int(t.Weekday()) + 7) % 7
		if daysUntilSat == 0 {
			daysUntilSat = 7
		}
		sat := t.AddDate(0, 0, daysUntilSat)
		mon := sat.AddDate(0, 0, 2)
		p.Date = sat.Format("2006-01-02")
		p.CheckIn = p.Date
		p.CheckOut = mon.Format("2006-01-02")
	}

	// 5. Hotel location: when intent=hotel and we have a destination IATA,
	// surface it as Location too so the dispatcher has one source of truth.
	if p.Intent == "hotel" && p.Location == "" && p.Destination != "" {
		p.Location = p.Destination
	}

	return p
}

// commonEnglishUppercase lists three-letter words that look like IATA codes
// but are common English words. This is intentionally narrow — we only need
// to filter the words that actually appear in travel queries.
var commonEnglishUppercase = map[string]bool{
	"THE": true, "AND": true, "FOR": true, "ARE": true, "BUT": true,
	"NOT": true, "YOU": true, "ALL": true, "CAN": true, "HER": true,
	"WAS": true, "ONE": true, "OUR": true, "OUT": true, "DAY": true,
	"GET": true, "HAS": true, "HIM": true, "HIS": true, "HOW": true,
	"MAN": true, "NEW": true, "NOW": true, "OLD": true, "SEE": true,
	"TWO": true, "WAY": true, "WHO": true, "BOY": true, "DID": true,
	"ITS": true, "LET": true, "PUT": true, "SAY": true, "SHE": true,
	"TOO": true, "USE": true, "DAD": true, "MOM": true, "MAY": true,
	"USA": true, // ambiguous: could be a country code, not an airport
}

func filterFalsePositiveIATA(codes []string) []string {
	out := make([]string, 0, len(codes))
	for _, c := range codes {
		if !commonEnglishUppercase[c] {
			out = append(out, c)
		}
	}
	return out
}

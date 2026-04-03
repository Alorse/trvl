package deals

import (
	"strings"
	"time"
)

// cityAliases maps common IATA codes to city name prefixes for flexible matching.
var cityAliases = map[string]string{
	"HEL": "helsinki", "AMS": "amsterdam", "PRG": "prague",
	"BCN": "barcelona", "ROM": "rome", "FCO": "rome",
	"PAR": "paris", "CDG": "paris", "ORY": "paris",
	"LHR": "london", "LGW": "london", "STN": "london",
	"BER": "berlin", "VIE": "vienna", "BUD": "budapest",
	"WAW": "warsaw", "LIS": "lisbon", "ATH": "athens",
	"DUB": "dublin", "CPH": "copenhagen", "OSL": "oslo",
	"ARN": "stockholm", "ZRH": "zurich", "BRU": "brussels",
	"MIL": "milan", "MXP": "milan", "MAD": "madrid",
	"JFK": "new york", "LAX": "los angeles", "SFO": "san francisco",
	"NRT": "tokyo", "HND": "tokyo", "ICN": "seoul",
	"BKK": "bangkok", "SIN": "singapore", "DXB": "dubai",
	"IST": "istanbul", "TLL": "tallinn", "RIX": "riga",
	"DBV": "dubrovnik", "SPU": "split",
}

// originMatchesDeal checks whether a deal's origin matches any of the filter origins
// using substring matching and IATA-to-city alias resolution.
func originMatchesDeal(dealOrigin string, filterOrigins []string) bool {
	dOrigin := strings.ToLower(strings.TrimSpace(dealOrigin))
	if dOrigin == "" {
		return false
	}
	for _, filter := range filterOrigins {
		f := strings.ToLower(strings.TrimSpace(filter))
		// Direct substring match.
		if strings.Contains(dOrigin, f) || strings.Contains(f, dOrigin) {
			return true
		}
		// IATA alias match: if filter is "HEL", check if deal origin contains "helsinki".
		if alias, ok := cityAliases[strings.ToUpper(filter)]; ok {
			if strings.Contains(dOrigin, alias) {
				return true
			}
		}
		// Reverse: if deal origin looks like an IATA code, resolve it.
		if alias, ok := cityAliases[strings.ToUpper(dealOrigin)]; ok {
			if strings.Contains(alias, f) || strings.Contains(f, alias) {
				return true
			}
		}
	}
	return false
}

// FilterDeals applies the given filter to a slice of deals.
func FilterDeals(deals []Deal, f DealFilter) []Deal {
	hoursAgo := f.HoursAgo
	if hoursAgo <= 0 {
		hoursAgo = 48
	}
	cutoff := time.Now().Add(-time.Duration(hoursAgo) * time.Hour)

	var result []Deal
	for _, d := range deals {
		// Time filter.
		if !d.Published.IsZero() && d.Published.Before(cutoff) {
			continue
		}
		// Origin filter.
		if len(f.Origins) > 0 {
			if d.Origin == "" || !originMatchesDeal(d.Origin, f.Origins) {
				continue
			}
		}
		// Price filter.
		if f.MaxPrice > 0 && d.Price > 0 && d.Price > f.MaxPrice {
			continue
		}
		// Type filter.
		if f.Type != "" && !strings.EqualFold(d.Type, f.Type) {
			continue
		}
		result = append(result, d)
	}
	return result
}

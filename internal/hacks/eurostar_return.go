package hacks

import (
	"context"
	"fmt"
	"strings"
)

// detectEurostarReturn advises booking a Eurostar return even for one-way
// travel, because the return premium is typically only €5-10.
func detectEurostarReturn(_ context.Context, in DetectorInput) []Hack {
	if !in.valid() {
		return nil
	}

	// Only fire for one-way searches (no return date)
	if in.ReturnDate != "" {
		return nil
	}

	originCity := normalizeEurostarCity(in.Origin)
	destCity := normalizeEurostarCity(in.Destination)

	if originCity == "" || destCity == "" {
		return nil
	}
	if originCity == destCity {
		return nil
	}

	// Only fire for Eurostar routes
	eurostarRoutes := map[string]map[string]bool{
		"LON": {"PAR": true, "BRU": true, "AMS": true, "LIL": true},
		"PAR": {"LON": true, "BRU": true, "AMS": true, "LIL": true},
		"BRU": {"LON": true, "PAR": true, "AMS": true},
		"AMS": {"LON": true, "PAR": true, "BRU": true},
		"LIL": {"LON": true, "PAR": true},
	}

	dests, ok := eurostarRoutes[originCity]
	if !ok || !dests[destCity] {
		return nil
	}

	return []Hack{{
		Type:  "eurostar_return",
		Title: "Eurostar return is almost free — book round-trip",
		Description: fmt.Sprintf(
			"Eurostar return fares are typically only €5-10 more than one-way. "+
				"Book a return ticket %s↔%s even if you only need one-way — the return leg is essentially free.",
			in.Origin, in.Destination),
		Steps: []string{
			"Compare Eurostar one-way vs return price on eurostar.com",
			"If the return premium is under €15, book the return",
			"Use or discard the return leg — either way, you save vs booking two one-ways later",
		},
		Risks: []string{
			"Return leg must be used within the ticket validity period",
		},
	}}
}

// normalizeEurostarCity maps airport/station codes to Eurostar city codes.
func normalizeEurostarCity(code string) string {
	switch strings.ToUpper(code) {
	case "LHR", "LGW", "STN", "LTN", "SEN", "LON", "STP":
		return "LON"
	case "CDG", "ORY", "BVA", "PAR":
		return "PAR"
	case "BRU", "CRL":
		return "BRU"
	case "AMS", "EIN":
		return "AMS"
	case "LIL":
		return "LIL"
	default:
		return ""
	}
}

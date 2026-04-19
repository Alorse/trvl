package hacks

import (
	"context"
	"fmt"
	"strings"
)

// arbitrageRoute describes a cross-border train where one national railway's
// website consistently sells the same seat for less.
type arbitrageRoute struct {
	cheaperSite string
	cheaperURL  string
	savings     string
}

// crossBorderRoutes maps origin→destination to the cheaper booking site.
var crossBorderRoutes = map[string]map[string]arbitrageRoute{
	// Paris→Germany: book on DB, not SNCF
	"PAR": {
		"MUC": {cheaperSite: "bahn.de (DB)", cheaperURL: "https://www.bahn.de", savings: "€20-40"},
		"FRA": {cheaperSite: "bahn.de (DB)", cheaperURL: "https://www.bahn.de", savings: "€15-30"},
	},
	"CDG": {
		"MUC": {cheaperSite: "bahn.de (DB)", cheaperURL: "https://www.bahn.de", savings: "€20-40"},
		"FRA": {cheaperSite: "bahn.de (DB)", cheaperURL: "https://www.bahn.de", savings: "€15-30"},
	},
	// Vienna→Italy/Munich: book on ÖBB, not Trenitalia/DB
	"VIE": {
		"VCE": {cheaperSite: "oebb.at (ÖBB)", cheaperURL: "https://www.oebb.at", savings: "€20-40"},
		"MXP": {cheaperSite: "oebb.at (ÖBB)", cheaperURL: "https://www.oebb.at", savings: "€20-30"},
		"MUC": {cheaperSite: "oebb.at (ÖBB)", cheaperURL: "https://www.oebb.at", savings: "€15-30"},
	},
	// Zurich→Italy: book on SBB, not Trenitalia
	"ZRH": {
		"MXP": {cheaperSite: "sbb.ch (SBB)", cheaperURL: "https://www.sbb.ch", savings: "€15-25"},
	},
	// Prague→Germany: book on CD, not DB
	"PRG": {
		"MUC": {cheaperSite: "cd.cz (Czech Railways)", cheaperURL: "https://www.cd.cz", savings: "€10-20"},
		"BER": {cheaperSite: "cd.cz (Czech Railways)", cheaperURL: "https://www.cd.cz", savings: "€15-25"},
	},
	// Brussels→Paris: book on b-europe
	"BRU": {
		"PAR": {cheaperSite: "b-europe.com", cheaperURL: "https://www.b-europe.com", savings: "€10-20"},
	},
	// Berlin→Prague: book on CD (reverse of PRG→BER)
	"BER": {
		"PRG": {cheaperSite: "cd.cz (Czech Railways)", cheaperURL: "https://www.cd.cz", savings: "€10-20"},
	},
	// Munich→Venice: book on ÖBB
	"MUC": {
		"VCE": {cheaperSite: "oebb.at (ÖBB)", cheaperURL: "https://www.oebb.at", savings: "€20-35"},
	},
}

// detectCrossBorderRail fires when the search matches a known cross-border
// rail route where booking on a different national railway's website is
// consistently cheaper for the same train and seat.
func detectCrossBorderRail(_ context.Context, in DetectorInput) []Hack {
	if !in.valid() {
		return nil
	}

	origin := strings.ToUpper(in.Origin)
	dest := strings.ToUpper(in.Destination)

	destRoutes, ok := crossBorderRoutes[origin]
	if !ok {
		return nil
	}
	arb, ok := destRoutes[dest]
	if !ok {
		return nil
	}

	return []Hack{{
		Type:  "cross_border_rail",
		Title: fmt.Sprintf("Book on %s — same train, cheaper price", arb.cheaperSite),
		Description: fmt.Sprintf(
			"Cross-border trains between %s and %s are sold by multiple railway companies. "+
				"Booking on %s is typically %s cheaper than the other operator for the same seat.",
			in.Origin, in.Destination, arb.cheaperSite, arb.savings),
		Steps: []string{
			fmt.Sprintf("Check prices on %s", arb.cheaperURL),
			"Compare with the other national railway's website",
			"Book on whichever is cheaper — it's the same train, same seat",
		},
		Risks: []string{
			"Cancellation/change policies may differ between operators",
		},
		Citations: []string{arb.cheaperURL},
	}}
}

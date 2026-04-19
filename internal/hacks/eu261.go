package hacks

import (
	"context"
	"strings"
)

// euAirports is the set of airports where EU Regulation 261/2004 applies
// (EU/EEA departures).
var euAirports = map[string]bool{
	"HEL": true, "AMS": true, "CDG": true, "FRA": true, "LHR": true, "BCN": true,
	"MAD": true, "FCO": true, "MXP": true, "BER": true, "VIE": true, "CPH": true,
	"ARN": true, "OSL": true, "PRG": true, "WAW": true, "BUD": true, "ATH": true,
	"LIS": true, "DUB": true, "BRU": true, "ZRH": true, "MUC": true, "HAM": true,
	"ORY": true, "LGW": true, "STN": true, "BGY": true, "CIA": true, "TLL": true,
	"RIX": true, "VNO": true, "SOF": true, "OTP": true, "ZAG": true, "LJU": true,
}

// detectEU261 reminds travellers on EU-departing flights about their
// compensation rights under EU Regulation 261/2004.
func detectEU261(_ context.Context, in DetectorInput) []Hack {
	if !in.valid() || in.Date == "" {
		return nil
	}

	if !euAirports[strings.ToUpper(in.Origin)] {
		return nil
	}

	return []Hack{{
		Type:  "eu261_awareness",
		Title: "EU261: you're entitled to €250-600 if delayed 3+ hours",
		Description: "Under EU Regulation 261/2004, passengers on EU-departing flights " +
			"are entitled to compensation for delays over 3 hours, cancellations, or denied boarding. " +
			"This applies regardless of ticket price.",
		Steps: []string{
			"If your flight is delayed 3+ hours: claim €250 (short-haul) to €600 (long-haul)",
			"Airlines must provide meals and accommodation during delays",
			"File via airline website or use AirHelp/Flightright/ClaimCompass for enforcement",
			"Claim deadline: up to 6 years after the flight (varies by country)",
		},
		Risks: []string{
			"Extraordinary circumstances (weather, strikes, security) exempt the airline",
			"Compensation agencies charge 25-35% commission",
		},
	}}
}

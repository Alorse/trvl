// Package baggage provides a static database of airline baggage allowances.
package baggage

import (
	"fmt"
	"sort"
)

// AirlineBaggage holds carry-on and checked baggage rules for an airline.
type AirlineBaggage struct {
	Code              string  `json:"code"`                // IATA airline code, e.g. "KL"
	Name              string  `json:"name"`                // Full name, e.g. "KLM"
	CarryOnMaxKg      float64 `json:"carry_on_max_kg"`     // Max carry-on weight in kg (0 = no weight limit)
	CarryOnDimensions string  `json:"carry_on_dimensions"` // e.g. "55x35x25 cm"
	PersonalItem      bool    `json:"personal_item"`       // True if extra personal/handbag item allowed
	CheckedIncluded   int     `json:"checked_included"`    // Number of checked bags included (0 = none)
	CheckedFee        float64 `json:"checked_fee_eur"`     // EUR for first checked bag (0 = included or unknown)
	OverheadOnly      bool    `json:"overhead_only"`       // True if only small under-seat bag is free in base fare
	Notes             string  `json:"notes"`
}

// database holds all known airline baggage rules, keyed by IATA code.
var database = map[string]AirlineBaggage{
	// --- Full-service European carriers ---
	"KL": {
		Code:              "KL",
		Name:              "KLM",
		CarryOnMaxKg:      12,
		CarryOnDimensions: "55x35x25 cm",
		PersonalItem:      true,
		CheckedIncluded:   1,
		CheckedFee:        0,
		Notes:             "1x23kg checked bag included; personal item (handbag/laptop bag) in addition to cabin bag.",
	},
	"AY": {
		Code:              "AY",
		Name:              "Finnair",
		CarryOnMaxKg:      8,
		CarryOnDimensions: "55x40x23 cm",
		PersonalItem:      true,
		CheckedIncluded:   1,
		CheckedFee:        0,
		Notes:             "1x23kg checked bag included on most fares; personal item allowed.",
	},
	"AF": {
		Code:              "AF",
		Name:              "Air France",
		CarryOnMaxKg:      12,
		CarryOnDimensions: "55x35x25 cm",
		PersonalItem:      true,
		CheckedIncluded:   1,
		CheckedFee:        0,
		Notes:             "1x23kg checked bag included; personal item in addition to cabin bag.",
	},
	"LH": {
		Code:              "LH",
		Name:              "Lufthansa",
		CarryOnMaxKg:      8,
		CarryOnDimensions: "55x40x23 cm",
		PersonalItem:      true,
		CheckedIncluded:   1,
		CheckedFee:        0,
		Notes:             "1x23kg checked bag included on most fares.",
	},
	"BA": {
		Code:              "BA",
		Name:              "British Airways",
		CarryOnMaxKg:      0,
		CarryOnDimensions: "56x45x25 cm",
		PersonalItem:      false,
		CheckedIncluded:   1,
		CheckedFee:        0,
		Notes:             "No weight limit on carry-on; 1x23kg checked bag included on most fare types.",
	},
	"IB": {
		Code:              "IB",
		Name:              "Iberia",
		CarryOnMaxKg:      10,
		CarryOnDimensions: "56x45x25 cm",
		PersonalItem:      true,
		CheckedIncluded:   1,
		CheckedFee:        0,
		Notes:             "1x23kg checked bag included on most fares.",
	},
	"LX": {
		Code:              "LX",
		Name:              "Swiss",
		CarryOnMaxKg:      8,
		CarryOnDimensions: "55x40x23 cm",
		PersonalItem:      true,
		CheckedIncluded:   1,
		CheckedFee:        0,
		Notes:             "1x23kg checked bag included on most fares.",
	},
	"OS": {
		Code:              "OS",
		Name:              "Austrian Airlines",
		CarryOnMaxKg:      8,
		CarryOnDimensions: "55x40x23 cm",
		PersonalItem:      true,
		CheckedIncluded:   1,
		CheckedFee:        0,
		Notes:             "1x23kg checked bag included on most fares.",
	},
	"LO": {
		Code:              "LO",
		Name:              "LOT Polish Airlines",
		CarryOnMaxKg:      8,
		CarryOnDimensions: "55x40x23 cm",
		PersonalItem:      true,
		CheckedIncluded:   1,
		CheckedFee:        0,
		Notes:             "1x23kg checked bag included on most economy fares.",
	},
	"SK": {
		Code:              "SK",
		Name:              "SAS",
		CarryOnMaxKg:      8,
		CarryOnDimensions: "55x40x23 cm",
		PersonalItem:      true,
		CheckedIncluded:   1,
		CheckedFee:        0,
		Notes:             "1x23kg checked bag included; personal item allowed.",
	},
	"AZ": {
		Code:              "AZ",
		Name:              "ITA Airways",
		CarryOnMaxKg:      8,
		CarryOnDimensions: "55x35x25 cm",
		PersonalItem:      true,
		CheckedIncluded:   1,
		CheckedFee:        0,
		Notes:             "1x23kg checked bag included on most fares.",
	},
	"TP": {
		Code:              "TP",
		Name:              "TAP Portugal",
		CarryOnMaxKg:      8,
		CarryOnDimensions: "55x40x20 cm",
		PersonalItem:      true,
		CheckedIncluded:   1,
		CheckedFee:        0,
		Notes:             "1x23kg checked bag included on most fares.",
	},
	"TK": {
		Code:              "TK",
		Name:              "Turkish Airlines",
		CarryOnMaxKg:      8,
		CarryOnDimensions: "55x40x23 cm",
		PersonalItem:      true,
		CheckedIncluded:   1,
		CheckedFee:        0,
		Notes:             "1x23kg checked bag included; excellent business class.",
	},

	// --- Long-haul Gulf/Asia carriers ---
	"QR": {
		Code:              "QR",
		Name:              "Qatar Airways",
		CarryOnMaxKg:      7,
		CarryOnDimensions: "50x37x25 cm",
		PersonalItem:      true,
		CheckedIncluded:   1,
		CheckedFee:        0,
		Notes:             "1x30kg checked bag included on most economy fares.",
	},
	"EK": {
		Code:              "EK",
		Name:              "Emirates",
		CarryOnMaxKg:      7,
		CarryOnDimensions: "55x38x20 cm",
		PersonalItem:      true,
		CheckedIncluded:   1,
		CheckedFee:        0,
		Notes:             "1x30kg checked bag included on economy; generous allowances across all classes.",
	},
	"SQ": {
		Code:              "SQ",
		Name:              "Singapore Airlines",
		CarryOnMaxKg:      7,
		CarryOnDimensions: "54x38x23 cm",
		PersonalItem:      true,
		CheckedIncluded:   1,
		CheckedFee:        0,
		Notes:             "1x30kg checked bag included; consistently rated world's best airline.",
	},

	// --- Low-cost carriers (LCC) ---
	"FR": {
		Code:              "FR",
		Name:              "Ryanair",
		CarryOnMaxKg:      10,
		CarryOnDimensions: "55x40x20 cm",
		PersonalItem:      false,
		CheckedIncluded:   0,
		CheckedFee:        35,
		OverheadOnly:      true,
		Notes:             "Free small bag (40x20x25 cm) fits under seat only. 10kg overhead cabin bag requires Priority Boarding (~EUR 6-20). Checked bag from ~EUR 35.",
	},
	"W6": {
		Code:              "W6",
		Name:              "Wizz Air",
		CarryOnMaxKg:      10,
		CarryOnDimensions: "55x40x23 cm",
		PersonalItem:      false,
		CheckedIncluded:   0,
		CheckedFee:        30,
		OverheadOnly:      true,
		Notes:             "Free small bag (40x30x20 cm) fits under seat only. 10kg overhead bag requires WIZZ Priority (~EUR 10-18). Checked bag from ~EUR 30.",
	},
	"U2": {
		Code:              "U2",
		Name:              "easyJet",
		CarryOnMaxKg:      15,
		CarryOnDimensions: "56x45x25 cm",
		PersonalItem:      false,
		CheckedIncluded:   0,
		CheckedFee:        33,
		Notes:             "15kg cabin bag included; small bag (45x36x20 cm) also allowed for free under seat. Checked bag from ~EUR 33.",
	},
	"DY": {
		Code:              "DY",
		Name:              "Norwegian",
		CarryOnMaxKg:      10,
		CarryOnDimensions: "55x40x23 cm",
		PersonalItem:      false,
		CheckedIncluded:   0,
		CheckedFee:        30,
		Notes:             "10kg carry-on included on LowFare+ and flex fares. LowFare base: small bag only. Checked bag from ~EUR 30.",
	},
	"BT": {
		Code:              "BT",
		Name:              "airBaltic",
		CarryOnMaxKg:      8,
		CarryOnDimensions: "55x40x20 cm",
		PersonalItem:      true,
		CheckedIncluded:   0,
		CheckedFee:        25,
		Notes:             "8kg cabin bag + small personal item included. Checked bag fee from ~EUR 25.",
	},
	"VY": {
		Code:              "VY",
		Name:              "Vueling",
		CarryOnMaxKg:      10,
		CarryOnDimensions: "55x40x20 cm",
		PersonalItem:      false,
		CheckedIncluded:   0,
		CheckedFee:        30,
		Notes:             "10kg cabin bag on Optima/TimeFlex fares; Basic fare includes small bag only. Checked bag from ~EUR 30.",
	},

	// --- US low-cost carriers ---
	"F9": {
		Code:              "F9",
		Name:              "Frontier Airlines",
		CarryOnMaxKg:      16,
		CarryOnDimensions: "24x16x10 in (61x41x25 cm)",
		PersonalItem:      false,
		CheckedIncluded:   0,
		CheckedFee:        45,
		Notes:             "35 lb (~16 kg) carry-on; personal item (18x14x8 in) free. Carry-on fee ~USD 45 unless bundled.",
	},
	"B6": {
		Code:              "B6",
		Name:              "JetBlue",
		CarryOnMaxKg:      0,
		CarryOnDimensions: "22x14x9 in (56x36x23 cm)",
		PersonalItem:      true,
		CheckedIncluded:   0,
		CheckedFee:        35,
		Notes:             "No weight limit on carry-on; 1 personal item free. Checked bag from USD 35 (Blue Basic: no carry-on overhead).",
	},
}

// Get returns baggage rules for an airline by its IATA code.
// Returns the rules and true if found, or a zero-value struct and false if not.
func Get(airlineCode string) (AirlineBaggage, bool) {
	ab, ok := database[airlineCode]
	return ab, ok
}

// All returns all known airline baggage rules, sorted by airline code.
func All() []AirlineBaggage {
	result := make([]AirlineBaggage, 0, len(database))
	for _, ab := range database {
		result = append(result, ab)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Code < result[j].Code
	})
	return result
}

// BaggageNote returns a short human-readable note about carry-on rules for use
// in hack descriptions. Highlights restrictions relevant to hidden-city / throwaway.
func BaggageNote(airlineCode string) string {
	ab, ok := database[airlineCode]
	if !ok {
		return ""
	}
	if ab.OverheadOnly {
		return fmt.Sprintf("⚠️  %s base fare: only small under-seat bag free — overhead cabin bag costs extra", ab.Name)
	}
	if ab.CarryOnMaxKg == 0 {
		return fmt.Sprintf("✓ %s allows carry-on with no weight limit", ab.Name)
	}
	return fmt.Sprintf("✓ %s allows %s carry-on — fits hidden city restriction", ab.Name, formatKg(ab.CarryOnMaxKg))
}

func formatKg(kg float64) string {
	if kg == float64(int(kg)) {
		return fmt.Sprintf("%.0fkg", kg)
	}
	return fmt.Sprintf("%.1fkg", kg)
}

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
	// CheckedIncluded is the number of checked bags included in the CHEAPEST
	// long-haul economy brand — Light/Basic/Saver — because that is the fare a
	// search surfaces. Encoding the standard brand instead reads as "bag
	// included" for fares that carry none: Air France Light and Finnair Light
	// both include zero while their standard brands include one.
	CheckedIncluded int `json:"checked_included"`
	// CheckedSource cites the airline page the inclusion figure was read from,
	// and CheckedVerified dates that reading. Empty means the figure has no
	// citation and is reported as an unsourced estimate.
	CheckedSource   string  `json:"checked_source,omitempty"`
	CheckedVerified string  `json:"checked_verified,omitempty"` // YYYY-MM
	CheckedFee      float64 `json:"checked_fee_eur"`            // EUR for first checked bag (0 = included or unknown)
	// Published fees are a range, not a point: route, booking channel, timing
	// and weight tier move them 3x to 9.5x within one carrier and fare brand
	// (Lufthansa Economy Light is EUR 15-100 for the same 23 kg bag). Min/Max
	// are only set where a primary source states them; FeeSource cites it and
	// FeeVerified dates it. Zero Min and Max means we have no sourced range.
	CheckedFeeMin float64 `json:"checked_fee_min,omitempty"`
	CheckedFeeMax float64 `json:"checked_fee_max,omitempty"`
	FeeCurrency   string  `json:"fee_currency,omitempty"` // ISO code for the range; "" means EUR
	FeeVaries     bool    `json:"fee_varies,omitempty"`   // carrier publishes no figure at all
	FeeSource     string  `json:"fee_source,omitempty"`   // URL the range came from
	FeeVerified   string  `json:"fee_verified,omitempty"` // YYYY-MM the range was checked
	OverheadOnly  bool    `json:"overhead_only"`          // True if only small under-seat bag is free in base fare
	Notes         string  `json:"notes"`
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
		CheckedIncluded:   0,
		CheckedFee:        0,
		Notes:             "Economy Light/Superlight include NO checked bag. Classic/Flex include 1x23kg, or 2 on Japan-Europe.",
		CheckedSource:     "https://www.finnair.com/us-en/baggage-on-finnair-flights/checked-baggage (Light and Superlight: 0 PC on every band; Classic/Flex: 1 PC, 2 PC Japan-Europe)",
		CheckedVerified:   "2026-08",
	},
	"AF": {
		Code:              "AF",
		Name:              "Air France",
		CarryOnMaxKg:      12,
		CarryOnDimensions: "55x35x25 cm",
		PersonalItem:      true,
		CheckedIncluded:   0,
		CheckedFee:        0,
		Notes:             "Economy Light includes NO checked bag; standard brands include 1x23kg. Air France publishes no fixed excess-bag fee.",
		CheckedSource:     "https://wwws.airfrance.co.uk/information/bagages/bagage-cabine-soute (Light fare: no checked baggage; standard brands include 1)",
		CheckedVerified:   "2026-08",
	},
	"LH": {
		Code:              "LH",
		CheckedFeeMin:     15,
		CheckedFeeMax:     100,
		FeeCurrency:       "EUR",
		FeeSource:         "https://business.lufthansagroup.com/content/dam/b2b/experts/files/LHG_FBAG_EcoLight_EN.pdf (Economy Light only; PDF stamped \"As at 11/2023\")",
		FeeVerified:       "2023-11",
		Name:              "Lufthansa",
		CarryOnMaxKg:      8,
		CarryOnDimensions: "55x40x23 cm",
		PersonalItem:      true,
		CheckedIncluded:   1,
		CheckedFee:        0,
		Notes:             "1x23kg on long-haul Economy. The zero-bag Light brand is confined to Scandinavia-US routes.",
		CheckedSource:     "https://www.lufthansa.com/us/en/baggage-and-other-fees (Economy 1x23kg; the zero-bag Light brand is sold only ex-DK/NO/SE to the US) - INFERRED: no per-brand row published",
		CheckedVerified:   "2026-08",
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
		Notes:             "1x23kg verified only on US routes (swiss.com/us/en/current-fees). Economy Light is sold internationally and likely includes none, but SWISS publishes no intercontinental brand table - UNVERIFIED.",
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
		FeeVaries:         true,
		FeeSource:         "https://www.turkishairlines.com/en-int/any-questions/baggage-allowance/",
		FeeVerified:       "2026-08",
		Name:              "Turkish Airlines",
		CarryOnMaxKg:      8,
		CarryOnDimensions: "55x40x23 cm",
		PersonalItem:      true,
		CheckedIncluded:   0,
		CheckedFee:        0,
		Notes:             "Turkish publishes no route allowance table and no fixed fee. Its own text confirms EcoFly fares exist with no free checked bag, without naming the routes - UNVERIFIED, assumed none.",
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
		Notes:             "Economy Special: 20kg on weight-concept routes, 1x23kg to/from the Americas and Africa. Saver and above carry more.",
		CheckedSource:     "https://www.emirates.com/us/english/before-you-fly/baggage/checked-baggage/ (Economy Special: 20kg weight-concept routes, 1x23kg piece-concept routes)",
		CheckedVerified:   "2026-08",
	},
	"SQ": {
		Code:              "SQ",
		Name:              "Singapore Airlines",
		CarryOnMaxKg:      7,
		CarryOnDimensions: "54x38x23 cm",
		PersonalItem:      true,
		CheckedIncluded:   1,
		CheckedFee:        0,
		Notes:             "1x30kg checked bag included on most fares.",
	},

	// --- Added 2026-08 from primary sources; figures are for the CHEAPEST
	// long-haul economy brand, which is what a flight search surfaces. ---
	"CX": {
		Code:              "CX",
		Name:              "Cathay Pacific",
		CarryOnMaxKg:      7,
		CarryOnDimensions: "56x36x23 cm",
		PersonalItem:      true,
		CheckedIncluded:   1,
		CheckedSource:     "https://www.cathaypacific.com/cx/en_US/book-a-trip/book-flights/new-economy-fares.html (Economy Light: 1 piece 23kg; Essential and Flex: 2x23kg)",
		CheckedVerified:   "2026-08",
		Notes:             "Even the Light fare includes 1x23kg. Essential and Flex include two.",
	},
	"AC": {
		Code:              "AC",
		Name:              "Air Canada",
		CarryOnDimensions: "23x40x55 cm",
		PersonalItem:      true,
		CheckedIncluded:   0,
		CheckedSource:     "https://www.aircanada.com/ca/en/aco/home/plan/baggage/checked.html (calculator, YYZ-LHR Economy, no status: Basic 0 bags, Standard/Flex 1, Comfort/Latitude 2)",
		CheckedVerified:   "2026-08",
		CheckedFeeMin:     90,
		CheckedFeeMax:     90,
		FeeCurrency:       "USD",
		FeeSource:         "https://www.aircanada.com/ca/en/aco/home/plan/baggage/checked.html (CA/US$90 first bag on Economy Basic; varies by direction and ticketing date)",
		FeeVerified:       "2026-08",
		Notes:             "Economy Basic includes no checked bag. Comfort and Latitude include two.",
	},
	"AI": {
		Code:            "AI",
		Name:            "Air India",
		CheckedIncluded: 1,
		CheckedSource:   "https://www.airindia.com/in/en/travel-information/baggage-guidelines/checked-baggage-allowance/europe-uk-israel.html (Economy Value: 1x23kg; Classic and Flex: 2x23kg)",
		CheckedVerified: "2026-08",
		Notes:           "Directional: Value is 2 pieces leaving India for the US/Canada, 1 piece inbound and on Europe/Asia/Gulf routes. Air India's own US/Canada page contradicts itself on the inbound case.",
	},
	"EY": {
		Code:              "EY",
		Name:              "Etihad Airways",
		CarryOnMaxKg:      7,
		CarryOnDimensions: "56x36x23 cm",
		PersonalItem:      true,
		CheckedIncluded:   0,
		CheckedSource:     "https://www.etihad.com/en/help/baggage-information (Economy Basic: cabin bag only, no checked allowance; Value: 1x25kg to Europe, 2x23kg to the USA)",
		CheckedVerified:   "2026-08",
		FeeVaries:         true,
		FeeSource:         "https://www.etihad.com/en/help/baggage-information (extra baggage priced per region in 5kg increments, e.g. Europe/Asia USD 44-125 per 5kg)",
		FeeVerified:       "2026-08",
		Notes:             "Economy Basic includes no checked bag. Value switches between weight and piece concepts by region.",
	},
	"KE": {
		Code:            "KE",
		Name:            "Korean Air",
		CheckedIncluded: 1,
		CheckedSource:   "https://www.koreanair.com/contents/plan-your-travel/baggage/checked-baggage/free-baggage (Economy Saver: 1x23kg; non-Saver: 2x23kg to/from the Americas)",
		CheckedVerified: "2026-08",
		Notes:           "Korean sells no zero-bag economy fare. Non-Saver Economy carries two bags to/from the Americas.",
	},
	"TG": {
		Code:            "TG",
		Name:            "Thai Airways",
		CheckedIncluded: 1,
		CheckedSource:   "https://www.thaiairways.com/en-th/content/baggage/checked-baggage/ (Economy Saver W/L and Standard K/S/V: 1x23kg; Flexi and Full Flex: 2x23kg)",
		CheckedVerified: "2026-08",
		Notes:           "Piece concept since 02 Mar 2026 travel. Thai footnotes that some Saver destinations carry no allowance at all, without naming them.",
	},
	"UA": {
		Code:            "UA",
		Name:            "United Airlines",
		CheckedIncluded: 0,
		CheckedSource:   "https://www.united.com/en/us/checked-bag-fee-calculator/any-flights (Basic Economy: 0 bags transatlantic EWR-LHR, 1 bag transpacific SFO-NRT; standard Economy: 1 and 2)",
		CheckedVerified: "2026-08",
		CheckedFeeMin:   90,
		CheckedFeeMax:   90,
		FeeCurrency:     "USD",
		FeeSource:       "https://www.united.com/en/us/checked-bag-fee-calculator/any-flights (Basic Economy EWR-LHR first bag USD 90; four regional increases during 2026 make this ticketing-date sensitive)",
		FeeVerified:     "2026-08",
		Notes:           "Route-dependent and the route flips the answer: Basic Economy carries no bag transatlantic but one transpacific. The conservative transatlantic figure is used.",
	},

	// --- Added 2026-08 from primary sources: the carriers that dominate
	// Europe-Japan/Korea/China and Europe-South America, previously absent and
	// therefore discarded by the bag filter as unknown. ---
	"JL": {
		Code:            "JL",
		Name:            "Japan Airlines",
		CheckedIncluded: 2,
		CheckedSource:   "https://www.jal.co.jp/jp/en/inter/baggage/checked/ (\"The first two bags are free\"; Economy and Premium Economy 2 pieces, 23kg each)",
		CheckedVerified: "2026-08",
		Notes:           "2x23kg on every international economy fare; JAL publishes no baggage split between its fare types. Pieces are 203cm total, not the usual 158cm. Read from JAL's Japan-market English pages: its European sites return 403.",
	},
	"NH": {
		Code:              "NH",
		Name:              "ANA",
		CarryOnMaxKg:      10,
		CarryOnDimensions: "55x40x25 cm",
		PersonalItem:      true,
		CheckedIncluded:   1,
		CheckedSource:     "https://www.ana.co.jp/en/jp/guide/plan/fare/international/branded-fare/ plus ANA's notice \"Changes in rules applied to checked baggage for fares departing from Japan\" (Japan to Europe, Light and Value: 2 pieces reduced to 1 from 2024-11-01)",
		CheckedVerified:   "2026-08",
		Notes:             "1 piece on Light/Value for fares DEPARTING JAPAN to Europe; Value Plus and above carry 2. ANA publishes nothing equivalent for Europe-origin fares, and its Basic brand is not named in the Europe row - both unresolved. ANA does sell economy fares with a zero allowance.",
	},
	"OZ": {
		Code:            "OZ",
		Name:            "Asiana Airlines",
		CarryOnMaxKg:    10,
		PersonalItem:    true,
		CheckedIncluded: 1,
		CheckedSource:   "https://flyasiana.com/C/GB/EN/contents/free-baggage (US routes 2x23kg; all non-US routes including Europe 1x23kg)",
		CheckedVerified: "2026-08",
		Notes:           "Allowance is cabin x route only - Asiana publishes no fare brands and no cheap-brand carve-out. Europe is 1x23kg; the Americas get 2.",
	},
	"MU": {
		Code:              "MU",
		Name:              "China Eastern",
		CarryOnMaxKg:      8,
		CarryOnDimensions: "55x40x20 cm",
		PersonalItem:      true,
		CheckedIncluded:   0,
		CheckedSource:     "https://www.ceair.com/global/en_static/Announcement/BaggageService/FreeBaggageAllowanceandSpecifications/ (Europe band: Basic Economy 0, Standard/Flexible 1)",
		CheckedVerified:   "2026-08",
		CheckedFeeMin:     1000,
		CheckedFeeMax:     1000,
		FeeCurrency:       "CNY",
		FeeSource:         "https://www.ceair.com/global/en_USD/Announcement/BaggageService/ExcessBaggageFees/ (first extra piece RMB 1000 on Mainland-Europe/Africa/Japan/Korea)",
		FeeVerified:       "2026-08",
		Notes:             "Six published route bands and Europe is the least generous: Basic Economy 0, Standard/Flexible 1. Africa, the Americas, Australia, NZ and Japan give Basic 1 and the rest 2. A single global figure would be wrong in four of the six bands; this entry is keyed to Europe.",
	},
	"UX": {
		Code:              "UX",
		Name:              "Air Europa",
		CarryOnMaxKg:      10,
		CarryOnDimensions: "55x35x25 cm",
		PersonalItem:      true,
		CheckedIncluded:   0,
		CheckedSource:     "https://www.aireuropa.com/us/en/aea/aexperience/our-fares.html (Long-haul Economy: LITE in-hold included \"--\"; STANDARD and FLEX 1x23kg)",
		CheckedVerified:   "2026-08",
		FeeVaries:         true,
		FeeSource:         "https://www.aireuropa.com/us/en/aea/travel-information/baggage/checked-baggage.html (fare table says only \"Paid\"; no figure published)",
		FeeVerified:       "2026-08",
		Notes:             "LITE includes no checked bag; STANDARD and FLEX include 1x23kg. All three keep the cabin allowance. Exception: flights originating in Bolivia to MAD or BCN include 2x23kg both ways.",
	},

	// --- Added 2026-08 from primary sources: Europe to Latin America, plus the
	// Swiss and Taiwanese long-haul carriers on those routes. ---
	"LA": {
		Code:              "LA",
		Name:              "LATAM Airlines",
		CarryOnMaxKg:      12,
		CarryOnDimensions: "55x35x25 cm",
		PersonalItem:      true,
		CheckedIncluded:   0,
		CheckedSource:     "https://www.latamairlines.com/us/en/experience/prepare-your-trip/baggage/checked-baggage (Basic and Light listed under \"Fares that do not include checked bags\"; Standard and Full include 1x23kg) - confirmed word-for-word on the US and Spain market sites",
		CheckedVerified:   "2026-08",
		CheckedFeeMin:     35,
		CheckedFeeMax:     150,
		FeeCurrency:       "USD",
		FeeSource:         "https://www.latamairlines.com/us/en/experience/prepare-your-trip/baggage/checked-baggage-fees (first bag USD 35-150 more than 48h before departure; Europe-South America falls under \"Other destinations\")",
		FeeVerified:       "2026-08",
		Notes:             "Basic and Light include no checked bag; Standard and Full include 1x23kg. Basic also drops the 12kg cabin bag, keeping only the 10kg personal item.",
	},
	"AV": {
		Code:              "AV",
		Name:              "Avianca",
		CarryOnMaxKg:      10,
		CarryOnDimensions: "55x35x25 cm",
		PersonalItem:      true,
		CheckedIncluded:   0,
		CheckedSource:     "https://ayuda.avianca.com/hc/en-us/articles/13080382443803 (Basic and Light pay for the first bag; Classic and Flex show \"Included\" across all eight route bands)",
		CheckedVerified:   "2026-08",
		CheckedFeeMin:     95,
		CheckedFeeMax:     95,
		FeeCurrency:       "EUR",
		FeeSource:         "https://ayuda.avianca.com/hc/en-us/articles/13080382443803 (from EUR/GBP 95 departing Europe, more than 48h before departure; \"from\", so treat as a floor)",
		FeeVerified:       "2026-08",
		Notes:             "Basic and Light include no checked bag, and Basic requires buying the carry-on too. Classic and Flex include 1x23kg. Read from Avianca's own help centre: www.avianca.com is Akamai-blocked on every content path.",
	},
	"WK": {
		Code:            "WK",
		Name:            "Edelweiss Air",
		CarryOnMaxKg:    8,
		PersonalItem:    true,
		CheckedIncluded: 1,
		CheckedSource:   "https://www.flyedelweiss.com/ch/en/prepare/baggage/free-baggage/checked-baggage.html (Economy Class and Economy Max: 1x23kg)",
		CheckedVerified: "2026-08",
		Notes:           "Edelweiss organises its site by cabin, not fare brand, and publishes no Light/Classic/Flex table at all. So this is correct per published policy, but a long-haul zero-bag bundle sold only in the booking engine cannot be ruled out.",
	},
	"BR": {
		Code:              "BR",
		Name:              "EVA Air",
		CarryOnMaxKg:      7,
		CarryOnDimensions: "56x36x23 cm",
		PersonalItem:      true,
		CheckedIncluded:   2,
		CheckedSource:     "https://www.evaair.com/en-global/fly-prepare/baggage/free-baggage/checked-baggage/ (long-haul to Europe, North America and Australia/NZ: Up, Standard and Basic all 2x23kg; Discount class A: 1)",
		CheckedVerified:   "2026-08",
		Notes:             "2 pieces on every named long-haul brand including Basic. Discount (A) booking class gets 1, and short-haul within Asia drops Basic and Discount to 1 - so this figure is the long-haul one.",
	},
	"TS": {
		Code:              "TS",
		Name:              "Air Transat",
		CarryOnDimensions: "40x23x55 cm",
		PersonalItem:      true,
		CheckedIncluded:   0,
		CheckedSource:     "https://www.airtransat.com/en-US/travel-information/fare-options (Eco Budget: personal item plus a carry-on to Europe, Africa, Peru and Brazil, no checked bag; Eco Standard adds one checked bag on those routes)",
		CheckedVerified:   "2026-08",
		FeeVaries:         true,
		FeeSource:         "https://www.airtransat.com/en-IE/travel-information/baggage/weight-and-size-what-you-can-bring (advance price cell renders as a bare currency symbol; only the within-24h price is published, EUR 80 for the first bag)",
		FeeVerified:       "2026-08",
		Notes:             "Eco Budget includes no checked bag. The one bag on Eco Standard is scoped to Europe, Africa, Peru and Brazil and does not extend to Air Transat's Sun destinations.",
	},

	// --- Low-cost carriers (LCC) ---
	"FR": {
		Code:              "FR",
		CheckedFeeMin:     9.49,
		CheckedFeeMax:     60,
		FeeCurrency:       "EUR",
		FeeSource:         "https://www.ryanair.com/ie/en/useful-info/help-centre/fees (10 kg tier: EUR 9.49 online to EUR 46-60 at the gate)",
		FeeVerified:       "2026-08",
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
		FeeVaries:         true,
		FeeSource:         "https://wizzair.com/en-gb/help-centre/baggage",
		FeeVerified:       "2026-08",
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
		CheckedFeeMin:     6.99,
		CheckedFeeMax:     60,
		FeeCurrency:       "GBP",
		FeeSource:         "https://www.easyjet.com/en/help-centre/policy-terms-and-conditions/fees-charges (airport bag drop is a flat GBP 60 at any weight tier)",
		FeeVerified:       "2026-08",
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
		CheckedFeeMin:     18,
		CheckedFeeMax:     120,
		FeeCurrency:       "EUR",
		FeeSource:         "https://www.vueling.com/en/vueling-services/supplementary-service-rates (25 kg: EUR 18-99 online, EUR 50-120 airport, EUR 100-160 Africa/Middle East)",
		FeeVerified:       "2026-08",
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

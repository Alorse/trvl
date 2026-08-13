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
		CheckedIncluded:   0,
		CheckedFee:        0,
		Notes:             "Light is the cheapest intercontinental brand and includes no checked bag. KLM also states that an Economy multi-city booking on klm.com can only be sold as a Light ticket - which is exactly the search shape used here.",
		CheckedSource:     "https://www.klm.com/information/baggage/checked-baggage-allowance (\"Basic ticket: no checked baggage included. Light ticket: no checked baggage included. Standard or Flex ticket: 1 item\")",
		CheckedVerified:   "2026-08",
		FeeSource:         "Two KLM pages. Floor from klm.com/information/legal/fees-paid-options, the per-piece table valid from 2026-07-01: 1st extra piece EUR 85 standard, EUR 70 US/Canada-Europe, EUR 110 US/Canada-rest of world. Since a Light ticket carries zero pieces, the page's own rule puts the first bag in the \"1st\" column - a documented step, not a guess. Ceiling from klm.com/information/baggage/checked-baggage-allowance, which states EUR 30-240 intercontinental when bought at least 24h ahead; that floor of 30 is not used because it understates a long-haul first bag and the floor is what drives ranking",
		FeeVerified:       "2026-08",
		CheckedFeeMin:     70,
		CheckedFeeMax:     240,
		FeeCurrency:       "EUR",
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
		CheckedFeeMin:     75,
		CheckedFeeMax:     120,
		FeeCurrency:       "EUR",
		FeeSource:         "https://www.finnair.com/en/baggage-on-finnair-flights/extra-baggage-fees (Europe-North America EUR 75 online / 90 airport; Europe-Asia EUR 75-100 online / 120 airport; Oceania reaches 140. Priced by ticket-purchase date since 2025-04, which is why the published figures are ranges)",
		FeeVerified:       "2026-08",
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
		FeeSource:         "https://wwws.airfrance.us/information/legal/fees-and-paid-options - the same per-piece table Air France shares with KLM, same validity dates: 1st extra piece EUR 85 standard, EUR 70 US/Canada-Europe, EUR 110 US/Canada-rest of world. Air France's own baggage help page says rates are shown at ticket purchase, so the two pages disagree and this figure may be a reference or airport rate rather than the online one. Found only on a noindex legal page, which is why three other passes concluded Air France published nothing",
		FeeVerified:       "2026-08",
		CheckedFeeMin:     70,
		CheckedFeeMax:     110,
		FeeCurrency:       "EUR",
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
		CheckedIncluded:   0,
		CheckedFee:        0,
		Notes:             "Basic in World Traveller is hand baggage only. BA's own baggage calculator has no fare-brand input and silently returns the bundled Standard figure, which is how the optimistic value survived.",
		CheckedSource:     "https://www.britishairways.com/travel-partner-connect/our-products/fares/longhaul-fares (World Traveller Basic: \"an unbundled fare which includes a hand baggage allowance only\"; Standard, Select and Select Pro include checked baggage)",
		CheckedVerified:   "2026-08",
		CheckedFeeMin:     65,
		CheckedFeeMax:     90,
		FeeCurrency:       "GBP",
		FeeSource:         "https://www.britishairways.com/content/information/baggage-essentials (LHR-JFK second bag GBP 65 online, GBP 90 at the airport; BA publishes no first-bag price for Basic)",
		FeeVerified:       "2026-08",
	},
	"IB": {
		Code:              "IB",
		Name:              "Iberia",
		CarryOnMaxKg:      10,
		CarryOnDimensions: "56x45x25 cm",
		PersonalItem:      true,
		CheckedIncluded:   0,
		CheckedFee:        0,
		Notes:             "Basic includes no checked bag on long-haul; the other four brands include 1 piece. Fees are zone-banded and seasonal, not a single figure.",
		CheckedSource:     "https://www.iberia.com/us/fare-classes/economy/ (Long-haul tab: Basic checked baggage cell reads \"Purchase\"; Optimal, Executive, Comfort and Flexible include 1 piece)",
		CheckedVerified:   "2026-08",
		CheckedFeeMin:     50,
		CheckedFeeMax:     135,
		FeeCurrency:       "EUR",
		FeeSource:         "https://www.iberia.com/us/luggage/allowance-in-hold/ (America/Asia zone, first 23kg bag: EUR 50-135 online, EUR 70-164 at the airport; five fee zones and seasonal variation)",
		FeeVerified:       "2026-08",
	},
	"LX": {
		Code:              "LX",
		Name:              "Swiss",
		CarryOnMaxKg:      8,
		CarryOnDimensions: "55x40x23 cm",
		PersonalItem:      true,
		CheckedIncluded:   0,
		CheckedFee:        0,
		Notes:             "Economy Light on long-haul includes no checked bag. The prose brand table on swiss.com is scoped to European routes; the answer lives in the baggage calculator's fare selector, which appears only on the results screen - which is why an earlier pass concluded SWISS did not publish it.",
		CheckedSource:     "https://www.swiss.com/us/en/prepare/baggage/checked-baggage/baggage-calculator (JFK-ZRH, Economy Light: carry-on and personal item only, no checked bag; Comfort returns 1x23kg)",
		CheckedVerified:   "2026-08",
		CheckedFeeMin:     70,
		CheckedFeeMax:     105,
		FeeCurrency:       "EUR",
		FeeSource:         "https://www.swiss.com/us/en/prepare/baggage/excess-baggage (first bag on the Light fare: EUR 70 on swiss.com for any intercontinental route; EUR 70 at check-in to the USA and Canada, EUR 105 at check-in elsewhere. Published as \"from\" floors, so no ceiling is stated)",
		FeeVerified:       "2026-08",
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
		CheckedIncluded:   1,
		CheckedFee:        0,
		Notes:             "EcoFly does carry an allowance on Europe-Asia long-haul, contrary to the earlier assumption of none. Turkish publishes no static route table and its calculator declines to break out EcoFly on Europe-Americas routes, so those remain unknown.",
		CheckedSource:     "https://www.turkishairlines.com/en-int/any-questions/checked-baggage/baggage-calculator/ (FRA-BKK Economy: EcoFly 20kg, ExtraFly 25kg, FlexFly 30kg, PrimeFly 40kg)",
		CheckedVerified:   "2026-08",
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
		Notes:             "Even the cheapest brand carries a bag. Piece concept applies only to itineraries to/from the Americas and originating in Africa; weight concept elsewhere. Read from Qatar's trade portal: the consumer allowance widget renders empty.",
		CheckedSource:     "https://www.qatarairways.com/tradeportal/en-us/NewFareFamilies.html (Economy Lite, the cheapest brand: 1x23kg on piece-concept routes, 20kg on weight-concept routes; Classic and above carry more)",
		CheckedVerified:   "2026-08",
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
		FeeSource:         "https://www.etihad.com/en/help/baggage-information - Etihad prices extra baggage by weight, not per bag: USD 44-125 per 5kg from Europe and Asia, USD 26-75 from Africa, the Indian subcontinent and the Middle East, USD 70-200 from Australia. The only per-bag figures are airport purchases in a few markets (USD 260 up to 23kg). Multiplying a 5kg band by five would manufacture a first-bag price the airline does not state, so no figure is stored",
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
		Notes:             "LITE includes no checked bag; STANDARD and FLEX include 1x23kg. All three keep the cabin allowance. Exception: flights originating in Bolivia to MAD or BCN include 2x23kg both ways.",
		CheckedFeeMin:     120,
		CheckedFeeMax:     140,
		FeeCurrency:       "EUR",
		FeeSource:         "https://www.aireuropa.com/us/en/aea/travel-information/baggage/checked-baggage.html — route selector set to Frankfurt-Lima: first bag EUR 120 in advance, EUR 140 close to departure. Identical across every fare brand, so the figure is banded by route rather than by brand. Read manually; the selector computes client-side and no automated pass could drive it",
		FeeVerified:       "2026-08",
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
		Notes:             "Eco Budget includes no checked bag. The one bag on Eco Standard is scoped to Europe, Africa, Peru and Brazil and does not extend to Air Transat's Sun destinations.",
		CheckedFeeMin:     76,
		CheckedFeeMax:     80,
		FeeCurrency:       "EUR",
		FeeSource:         "https://www.airtransat.com/en-IE/news/changes-baggage-fees - first bag to/from Europe and Africa, effective 2026-06-01, charged in euros for departures from Europe. WITHIN 24 HOURS OF DEPARTURE ONLY, the same price online or at the counter; this is the last-minute price and therefore a ceiling. Air Transat quotes the cheaper advance price during booking and publishes it nowhere - its fee tables render client-side and come back empty",
		FeeVerified:       "2026-08",
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

	// --- Added 2026-08: the carriers that fly intra-Asia, where the table had
	// no coverage at all. On KIX-ICN two thirds of priced results resolved to
	// unknown and the cheapest flight was unknown on every route measured, so a
	// downstream price cache was discarding the cheap carrier and storing a
	// full-service fare in its place. Fees here are published in HKD, JPY and
	// KRW, which only became usable once bag fees started converting into the
	// fare's currency. ---
	"UO": {
		Code:            "UO",
		Name:            "HK Express",
		CheckedIncluded: 0,
		CheckedSource:   "https://www.hkexpress.com/en/Plan/Extras/Baggage/Checked-Baggage (\"Checked Baggage is only included in the Essential and Max fare categories\"; the cheaper bundles carry none)",
		CheckedVerified: "2026-08",
		CheckedFeeMin:   310,
		CheckedFeeMax:   600,
		FeeCurrency:     "HKD",
		FeeSource:       "https://www.hkexpress.com/-/media/Plan/Extras/Baggage/Baggage%20Fee/202603%20Price%20Table%20-%20Baggage%20-%20EN.pdf, the airline's own price table, last updated 4 November 2025. Range is one 20 kg piece across booking channels: HKD 310 at initial booking, 380 via Manage My Booking, 460 at online check-in, 600 at the airport counter. The floor is the advance online price because the floor is what drives ranking",
		FeeVerified:     "2026-08",
		Notes:           "Fee is stated per passenger PER SEGMENT, which is the same shape as charging it per direction. A 32 kg piece is HKD 430-740. The table is a single grid with no route dimension, so unlike the network carriers there is nothing route-dependent to approximate.",
	},
	"7C": {
		Code:            "7C",
		Name:            "Jeju Air",
		CheckedIncluded: 0,
		CheckedSource:   "https://www.jejuair.net/en/linkService/boardingProcessGuide/trustBaggage.do, the airline's own baggage calculator with Japan-Korea selected: \"BASIC passengers : Free baggage service 0KG\". STANDARD carries 15 kg and BIZ LITE 30 kg, so the zero belongs to the cheapest brand exactly as the search surfaces it",
		CheckedVerified: "2026-08",
		CheckedFeeMin:   40,
		CheckedFeeMax:   60,
		FeeCurrency:     "USD",
		FeeSource:       "Same calculator: first 15 kg is USD 40 booked online, USD 60 booked offline at an airport or branch. The floor is the online price because the floor is what drives ranking",
		FeeVerified:     "2026-08",
		Notes:           "Jeju charges in KRW for departures from Korea (KRW 40,000 online / 60,000 offline) and in USD from everywhere else, converting any local currency to USD. USD is stored because it is the currency for the Japan-origin routes this covers. Jeju also states charges apply per itinerary regardless of transfers, which matches charging per direction rather than per ticket.",
	},
	"MM": {
		Code:            "MM",
		Name:            "Peach Aviation",
		CheckedIncluded: 0,
		CheckedSource:   "https://www.flypeach.com/en/lm/fares/fees_and_charges — the international checked-baggage table prices a first bag under Minimum and marks it free under Standard and Standard Plus. Minimum is the cheapest of the three brands, so the fare a search surfaces carries none",
		CheckedVerified: "2026-08",
		CheckedFeeMin:   2600,
		CheckedFeeMax:   7500,
		FeeCurrency:     "JPY",
		FeeSource:       "Same page, the Minimum first-bag row bought over the internet, which is priced by route zone: Zone A (Japan-Seoul) JPY 2,600, Zone B (Japan-Taipei/Kaohsiung/Hong Kong) JPY 3,600, Zone D (Kansai-Bangkok/Singapore) JPY 6,100, Zone C (Japan-Shanghai) JPY 7,500",
		FeeVerified:     "2026-08",
		Notes:           "Four published zones spanning nearly 3x, so a single carrier-level range is unusually coarse here — Kansai-Seoul really is JPY 2,600 and Kansai-Shanghai really is JPY 7,500. Booking through the contact centre or the airport counter raises all of them (Zone C reaches JPY 8,900). Domestic Japan is a flat JPY 2,000. Peach charges in the currency of the point of origin. This is the clearest case yet for keying the table by (carrier, region) rather than by carrier alone.",
	},
	"BX": {
		Code:            "BX",
		Name:            "Air Busan",
		CarryOnMaxKg:    10,
		PersonalItem:    true,
		CheckedIncluded: 1,
		CheckedSource:   "https://en.airbusan.com free baggage page, International Flight table: \"Economy/Regular Airfare (Non-America Routes): 15 kg, maximum size 203 cm\". Americas routes get 2x23 kg to Guam and 1x23 kg to Saipan",
		CheckedVerified: "2026-08",
		Notes:           "An exception to the cheapest-brand rule, and the entry here most likely to be wrong. Air Busan splits by ticket kind rather than by fare brand: the same table says \"Special/Event Flights (including routes to the Americas): Not applicable\", so a promotional ticket carries nothing. Unlike Iberia Basic or KLM Light, Special/Event reads as an occasional promotion rather than the permanently-available cheap brand, so the regular 15 kg is recorded as the normal case. If it turns out these routes routinely sell as Special/Event tickets, this must become 0 — it is the one claim in the intra-Asia block that a real booking would settle faster than any page.",
	},
	"HX": {
		Code:            "HX",
		CarryOnMaxKg:    7,
		PersonalItem:    true,
		Name:            "Hong Kong Airlines",
		CheckedIncluded: 1,
		CheckedSource:   "https://www.hongkongairlines.com/en_HK/fly-with-us/baggage/checkedbaggage, free allowance for tickets flying 26 October 2025 or later: \"Standard, FlexiPlus and Main Economy Class: 2 pieces of 23 kg each. Value Economy Class: 1 piece of 23 kg\"",
		CheckedVerified: "2026-08",
		Notes:           "The reassuring shape: a published brand ladder whose CHEAPEST rung still carries a bag, so the cheapest-brand rule and the generous answer agree. Value Economy gets one 23 kg piece and everything above it two — the same pattern as Cathay's Light fare. One piece is recorded rather than two because Value is what a price search surfaces. Fortune Wing Club Platinum, Gold and Silver each add a piece on top, which the frequent-flyer path handles separately. Zones are drawn from Hong Kong, and Japan sits in Zone 1.",
	},
	"TW": {
		Code:            "TW",
		Name:            "T'Way Air",
		CheckedIncluded: 0,
		CheckedSource:   "https://www.twayair.com fee regulations PDF, revision 2026-05-08, section 3-1 Excess Baggage Fee. The zone tables set a free allowance per brand by naming the weight it must exceed — Smart 15 kg, Normal 20 kg, Business 30 kg — while Event fare instead has its own row: \"~15 kilograms or less | You need to pay a rate up to 15 kilograms regardless of the weight of checked baggage\". A flat charge from the first gram is a zero allowance",
		CheckedVerified: "2026-08",
		CheckedFeeMin:   60000,
		CheckedFeeMax:   80000,
		FeeCurrency:     "KRW",
		FeeSource:       "Same PDF, Event-fare flat rate for travel dates from 30 March 2026: Zone 2 (Japan, China short-haul) KRW 60,000; Zone 3 (Hong Kong, Taiwan, Macao, China long-haul) KRW 80,000. The earlier 50,000/70,000 figures apply only to travel before that date and are not used",
		FeeVerified:     "2026-08",
		Notes:           "Recorded at the cheapest brand, unlike Air Busan above, and the difference is real rather than a coin flip: T'Way's Event fare sits in a published brand ladder (Event, Smart, Normal, Business) as the cheapest rung, whereas Air Busan's Special/Event names a class of flight. Where a brand ladder exists, the rule that burned us on Iberia and KLM applies unchanged. Zones are measured from Korea, so a Japan-Korea leg is Zone 2 and Japan-Hong Kong is Zone 3. Fees are stated per one-way trip per person, matching per-direction charging.",
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

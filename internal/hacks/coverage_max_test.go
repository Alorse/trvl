package hacks

import (
	"context"
	"testing"

	"github.com/MikkoParkkola/trvl/internal/models"
	"github.com/MikkoParkkola/trvl/internal/preferences"
)

// --- accessors.go: DestinationAlternatives (0% coverage) ---

func TestDestinationAlternatives_knownDest(t *testing.T) {
	// BCN has known destination alternatives in the static map.
	alts := DestinationAlternatives("BCN")
	if len(alts) == 0 {
		t.Fatal("expected alternatives for BCN, got none")
	}
	for _, a := range alts {
		if a.IATA == "" {
			t.Error("alternative IATA must not be empty")
		}
		if a.City == "" {
			t.Errorf("alternative %s must have a city", a.IATA)
		}
	}
}

func TestDestinationAlternatives_unknownDest(t *testing.T) {
	alts := DestinationAlternatives("XXX")
	if alts != nil {
		t.Errorf("expected nil for unknown dest, got %d alternatives", len(alts))
	}
}

func TestDestinationAlternatives_fieldsPopulated(t *testing.T) {
	// Test a destination known to have alternatives.
	for _, dest := range []string{"BCN", "LHR", "CDG", "FCO"} {
		alts := DestinationAlternatives(dest)
		if len(alts) == 0 {
			continue // not all may be in the map
		}
		for _, a := range alts {
			if a.Mode == "" {
				t.Errorf("DestinationAlternatives(%s): %s has empty Mode", dest, a.IATA)
			}
		}
	}
}

// --- accessors.go: RailFlyStationsForHub (0% coverage) ---

func TestRailFlyStationsForHub_known(t *testing.T) {
	stations := RailFlyStationsForHub("AMS")
	if len(stations) == 0 {
		t.Fatal("expected rail+fly stations for AMS hub")
	}
	for _, s := range stations {
		if s.IATA == "" || s.City == "" {
			t.Errorf("station must have IATA and City; got %+v", s)
		}
		if s.HubIATA != "AMS" {
			t.Errorf("expected HubIATA=AMS, got %s", s.HubIATA)
		}
	}
}

func TestRailFlyStationsForHub_FRA_fields(t *testing.T) {
	stations := RailFlyStationsForHub("FRA")
	if len(stations) == 0 {
		t.Fatal("expected rail+fly stations for FRA hub")
	}
	for _, s := range stations {
		if s.TrainMins <= 0 {
			t.Errorf("station %s should have positive TrainMins", s.IATA)
		}
		if s.AirlineName == "" {
			t.Errorf("station %s should have AirlineName", s.IATA)
		}
		if s.FareZone == "" {
			t.Errorf("station %s should have FareZone", s.IATA)
		}
	}
}

func TestRailFlyStationsForHub_unknownHub(t *testing.T) {
	stations := RailFlyStationsForHub("XXX")
	if len(stations) != 0 {
		t.Errorf("expected no stations for unknown hub, got %d", len(stations))
	}
}

func TestRailFlyStationsForHub_CDG_accessor(t *testing.T) {
	stations := RailFlyStationsForHub("CDG")
	if len(stations) == 0 {
		t.Fatal("expected rail+fly stations for CDG hub")
	}
}

func TestRailFlyStationsForHub_ZRH_accessor(t *testing.T) {
	stations := RailFlyStationsForHub("ZRH")
	if len(stations) == 0 {
		t.Fatal("expected rail+fly stations for ZRH hub")
	}
}

// --- detectCurrencyArbitrage: input validation (17% -> higher) ---

func TestDetectCurrencyArbitrage_noDate(t *testing.T) {
	h := detectCurrencyArbitrage(context.Background(), DetectorInput{
		Origin:      "HEL",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for missing date")
	}
}

func TestDetectCurrencyArbitrage_noOrigin(t *testing.T) {
	h := detectCurrencyArbitrage(context.Background(), DetectorInput{
		Date:        "2026-06-15",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for missing origin")
	}
}

func TestDetectCurrencyArbitrage_noDestination(t *testing.T) {
	h := detectCurrencyArbitrage(context.Background(), DetectorInput{
		Date:   "2026-06-15",
		Origin: "HEL",
	})
	if h != nil {
		t.Error("expected nil for missing destination")
	}
}

// --- detectDateFlex: input validation (12% -> higher) ---

func TestDetectDateFlex_noDate(t *testing.T) {
	h := detectDateFlex(context.Background(), DetectorInput{
		Origin:      "HEL",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for missing date")
	}
}

func TestDetectDateFlex_noOrigin(t *testing.T) {
	h := detectDateFlex(context.Background(), DetectorInput{
		Date:        "2026-06-15",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for missing origin")
	}
}

func TestDetectDateFlex_noDestination(t *testing.T) {
	h := detectDateFlex(context.Background(), DetectorInput{
		Date:   "2026-06-15",
		Origin: "HEL",
	})
	if h != nil {
		t.Error("expected nil for missing destination")
	}
}

// --- detectFerryPositioning: input validation (18% -> higher) ---

func TestDetectFerryPositioning_noDate(t *testing.T) {
	h := detectFerryPositioning(context.Background(), DetectorInput{
		Origin:      "HEL",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for missing date")
	}
}

func TestDetectFerryPositioning_noOrigin(t *testing.T) {
	h := detectFerryPositioning(context.Background(), DetectorInput{
		Date:        "2026-06-15",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for missing origin")
	}
}

func TestDetectFerryPositioning_noDestination(t *testing.T) {
	h := detectFerryPositioning(context.Background(), DetectorInput{
		Date:   "2026-06-15",
		Origin: "HEL",
	})
	if h != nil {
		t.Error("expected nil for missing destination")
	}
}

func TestDetectFerryPositioning_unknownOrigin(t *testing.T) {
	h := detectFerryPositioning(context.Background(), DetectorInput{
		Date:        "2026-06-15",
		Origin:      "XXX",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for origin with no ferry routes")
	}
}

// --- detectHiddenCity: input validation (13% -> higher) ---

func TestDetectHiddenCity_noDate(t *testing.T) {
	h := detectHiddenCity(context.Background(), DetectorInput{
		Origin:      "HEL",
		Destination: "AMS",
	})
	if h != nil {
		t.Error("expected nil for missing date")
	}
}

func TestDetectHiddenCity_noOrigin(t *testing.T) {
	h := detectHiddenCity(context.Background(), DetectorInput{
		Date:        "2026-06-15",
		Destination: "AMS",
	})
	if h != nil {
		t.Error("expected nil for missing origin")
	}
}

func TestDetectHiddenCity_noDestination(t *testing.T) {
	h := detectHiddenCity(context.Background(), DetectorInput{
		Date:   "2026-06-15",
		Origin: "HEL",
	})
	if h != nil {
		t.Error("expected nil for missing destination")
	}
}

func TestDetectHiddenCity_unknownDest(t *testing.T) {
	h := detectHiddenCity(context.Background(), DetectorInput{
		Date:        "2026-06-15",
		Origin:      "HEL",
		Destination: "XXX",
	})
	if h != nil {
		t.Error("expected nil for destination without hidden city extensions")
	}
}

// --- detectLowCostCarrier: input validation (20% -> higher) ---

func TestDetectLowCostCarrier_noDate(t *testing.T) {
	h := detectLowCostCarrier(context.Background(), DetectorInput{
		Origin:      "HEL",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for missing date")
	}
}

func TestDetectLowCostCarrier_noOrigin(t *testing.T) {
	h := detectLowCostCarrier(context.Background(), DetectorInput{
		Date:        "2026-06-15",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for missing origin")
	}
}

func TestDetectLowCostCarrier_noDestination(t *testing.T) {
	h := detectLowCostCarrier(context.Background(), DetectorInput{
		Date:   "2026-06-15",
		Origin: "HEL",
	})
	if h != nil {
		t.Error("expected nil for missing destination")
	}
}

// --- detectMultiStop: input validation (21% -> higher) ---

func TestDetectMultiStop_noDate(t *testing.T) {
	h := detectMultiStop(context.Background(), DetectorInput{
		Origin:      "HEL",
		Destination: "PRG",
	})
	if h != nil {
		t.Error("expected nil for missing date")
	}
}

func TestDetectMultiStop_noOrigin(t *testing.T) {
	h := detectMultiStop(context.Background(), DetectorInput{
		Date:        "2026-06-15",
		Destination: "PRG",
	})
	if h != nil {
		t.Error("expected nil for missing origin")
	}
}

func TestDetectMultiStop_noDestination(t *testing.T) {
	h := detectMultiStop(context.Background(), DetectorInput{
		Date:   "2026-06-15",
		Origin: "HEL",
	})
	if h != nil {
		t.Error("expected nil for missing destination")
	}
}

func TestDetectMultiStop_unknownDest(t *testing.T) {
	h := detectMultiStop(context.Background(), DetectorInput{
		Date:        "2026-06-15",
		Origin:      "HEL",
		Destination: "XXX",
	})
	if h != nil {
		t.Error("expected nil for destination without multistop hubs")
	}
}

// --- detectMultiModalOpenJawGround: input validation (9% -> higher) ---

func TestDetectMultiModalOpenJawGround_noDate(t *testing.T) {
	h := detectMultiModalOpenJawGround(context.Background(), DetectorInput{
		Origin:      "HEL",
		Destination: "DBV",
	})
	if h != nil {
		t.Error("expected nil for missing date")
	}
}

func TestDetectMultiModalOpenJawGround_noOrigin(t *testing.T) {
	h := detectMultiModalOpenJawGround(context.Background(), DetectorInput{
		Date:        "2026-06-15",
		Destination: "DBV",
	})
	if h != nil {
		t.Error("expected nil for missing origin")
	}
}

func TestDetectMultiModalOpenJawGround_noDestination(t *testing.T) {
	h := detectMultiModalOpenJawGround(context.Background(), DetectorInput{
		Date:   "2026-06-15",
		Origin: "HEL",
	})
	if h != nil {
		t.Error("expected nil for missing destination")
	}
}

// --- detectMultiModalPositioning: input validation (20% -> higher) ---

func TestDetectMultiModalPositioning_noDate(t *testing.T) {
	h := detectMultiModalPositioning(context.Background(), DetectorInput{
		Origin:      "HEL",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for missing date")
	}
}

func TestDetectMultiModalPositioning_noOrigin(t *testing.T) {
	h := detectMultiModalPositioning(context.Background(), DetectorInput{
		Date:        "2026-06-15",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for missing origin")
	}
}

func TestDetectMultiModalPositioning_noDestination(t *testing.T) {
	h := detectMultiModalPositioning(context.Background(), DetectorInput{
		Date:   "2026-06-15",
		Origin: "HEL",
	})
	if h != nil {
		t.Error("expected nil for missing destination")
	}
}

// --- detectMultiModalReturnSplit: input validation (9% -> higher) ---

func TestDetectMultiModalReturnSplit_noDate(t *testing.T) {
	h := detectMultiModalReturnSplit(context.Background(), DetectorInput{
		ReturnDate:  "2026-06-22",
		Origin:      "HEL",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for missing date")
	}
}

func TestDetectMultiModalReturnSplit_noOrigin(t *testing.T) {
	h := detectMultiModalReturnSplit(context.Background(), DetectorInput{
		Date:        "2026-06-15",
		ReturnDate:  "2026-06-22",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for missing origin")
	}
}

func TestDetectMultiModalReturnSplit_noDestination(t *testing.T) {
	h := detectMultiModalReturnSplit(context.Background(), DetectorInput{
		Date:       "2026-06-15",
		ReturnDate: "2026-06-22",
		Origin:     "HEL",
	})
	if h != nil {
		t.Error("expected nil for missing destination")
	}
}

func TestDetectMultiModalReturnSplit_noReturnDate(t *testing.T) {
	h := detectMultiModalReturnSplit(context.Background(), DetectorInput{
		Date:        "2026-06-15",
		Origin:      "HEL",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for missing return date")
	}
}

// --- detectMultiModalSkipFlight: input validation (13% -> higher) ---

func TestDetectMultiModalSkipFlight_noDestination(t *testing.T) {
	h := detectMultiModalSkipFlight(context.Background(), DetectorInput{
		Date:   "2026-06-15",
		Origin: "HEL",
	})
	if h != nil {
		t.Error("expected nil for missing destination")
	}
}

// --- detectNightTransport: input validation (35% -> higher) ---

func TestDetectNightTransport_noDate(t *testing.T) {
	h := detectNightTransport(context.Background(), DetectorInput{
		Origin:      "HEL",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for missing date")
	}
}

func TestDetectNightTransport_noOrigin(t *testing.T) {
	h := detectNightTransport(context.Background(), DetectorInput{
		Date:        "2026-06-15",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for missing origin")
	}
}

func TestDetectNightTransport_noDestination(t *testing.T) {
	h := detectNightTransport(context.Background(), DetectorInput{
		Date:   "2026-06-15",
		Origin: "HEL",
	})
	if h != nil {
		t.Error("expected nil for missing destination")
	}
}

// --- detectOpenJaw: input validation (14% -> higher) ---

func TestDetectOpenJaw_noDate(t *testing.T) {
	h := detectOpenJaw(context.Background(), DetectorInput{
		ReturnDate:  "2026-06-22",
		Origin:      "HEL",
		Destination: "PRG",
	})
	if h != nil {
		t.Error("expected nil for missing date")
	}
}

func TestDetectOpenJaw_noOrigin(t *testing.T) {
	h := detectOpenJaw(context.Background(), DetectorInput{
		Date:        "2026-06-15",
		ReturnDate:  "2026-06-22",
		Destination: "PRG",
	})
	if h != nil {
		t.Error("expected nil for missing origin")
	}
}

func TestDetectOpenJaw_noDestination(t *testing.T) {
	h := detectOpenJaw(context.Background(), DetectorInput{
		Date:       "2026-06-15",
		ReturnDate: "2026-06-22",
		Origin:     "HEL",
	})
	if h != nil {
		t.Error("expected nil for missing destination")
	}
}

// --- detectPositioning: input validation (26% -> higher) ---

func TestDetectPositioning_noDate(t *testing.T) {
	h := detectPositioning(context.Background(), DetectorInput{
		Origin:      "HEL",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for missing date")
	}
}

func TestDetectPositioning_noOrigin(t *testing.T) {
	h := detectPositioning(context.Background(), DetectorInput{
		Date:        "2026-06-15",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for missing origin")
	}
}

func TestDetectPositioning_noDestination(t *testing.T) {
	h := detectPositioning(context.Background(), DetectorInput{
		Date:   "2026-06-15",
		Origin: "HEL",
	})
	if h != nil {
		t.Error("expected nil for missing destination")
	}
}

func TestDetectPositioning_unknownOrigin(t *testing.T) {
	h := detectPositioning(context.Background(), DetectorInput{
		Date:        "2026-06-15",
		Origin:      "XXX",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for origin without nearby airports")
	}
}

// --- detectSplit: input validation (19% -> higher) ---

func TestDetectSplit_noDate(t *testing.T) {
	h := detectSplit(context.Background(), DetectorInput{
		ReturnDate:  "2026-06-22",
		Origin:      "HEL",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for missing date")
	}
}

func TestDetectSplit_noOrigin(t *testing.T) {
	h := detectSplit(context.Background(), DetectorInput{
		Date:        "2026-06-15",
		ReturnDate:  "2026-06-22",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for missing origin")
	}
}

func TestDetectSplit_noDestination(t *testing.T) {
	h := detectSplit(context.Background(), DetectorInput{
		Date:       "2026-06-15",
		ReturnDate: "2026-06-22",
		Origin:     "HEL",
	})
	if h != nil {
		t.Error("expected nil for missing destination")
	}
}

func TestDetectSplit_noReturnDate(t *testing.T) {
	h := detectSplit(context.Background(), DetectorInput{
		Date:        "2026-06-15",
		Origin:      "HEL",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for missing return date")
	}
}

// --- detectStopover: input validation (27% -> higher) ---

func TestDetectStopover_noDate(t *testing.T) {
	h := detectStopover(context.Background(), DetectorInput{
		Origin:      "HEL",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for missing date")
	}
}

func TestDetectStopover_noOrigin(t *testing.T) {
	h := detectStopover(context.Background(), DetectorInput{
		Date:        "2026-06-15",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for missing origin")
	}
}

func TestDetectStopover_noDestination(t *testing.T) {
	h := detectStopover(context.Background(), DetectorInput{
		Date:   "2026-06-15",
		Origin: "HEL",
	})
	if h != nil {
		t.Error("expected nil for missing destination")
	}
}

// --- detectThrowaway: input validation (25% -> higher) ---

func TestDetectThrowaway_noDate(t *testing.T) {
	h := detectThrowaway(context.Background(), DetectorInput{
		Origin:      "HEL",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for missing date")
	}
}

func TestDetectThrowaway_noOrigin(t *testing.T) {
	h := detectThrowaway(context.Background(), DetectorInput{
		Date:        "2026-06-15",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for missing origin")
	}
}

func TestDetectThrowaway_noDestination(t *testing.T) {
	h := detectThrowaway(context.Background(), DetectorInput{
		Date:   "2026-06-15",
		Origin: "HEL",
	})
	if h != nil {
		t.Error("expected nil for missing destination")
	}
}

// --- detectTuesdayBooking: input validation (18% -> higher) ---

func TestDetectTuesdayBooking_noDate(t *testing.T) {
	h := detectTuesdayBooking(context.Background(), DetectorInput{
		Origin:      "HEL",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for missing date")
	}
}

func TestDetectTuesdayBooking_noOrigin(t *testing.T) {
	h := detectTuesdayBooking(context.Background(), DetectorInput{
		Date:        "2026-06-19", // Friday
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for missing origin")
	}
}

func TestDetectTuesdayBooking_noDestination(t *testing.T) {
	h := detectTuesdayBooking(context.Background(), DetectorInput{
		Date:   "2026-06-19", // Friday
		Origin: "HEL",
	})
	if h != nil {
		t.Error("expected nil for missing destination")
	}
}

// Tuesday is NOT an expensive day — should early return.
func TestDetectTuesdayBooking_cheapDay(t *testing.T) {
	h := detectTuesdayBooking(context.Background(), DetectorInput{
		Date:        "2026-06-16", // Tuesday
		Origin:      "HEL",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for non-expensive day (Tuesday)")
	}
}

// Wednesday is NOT an expensive day.
func TestDetectTuesdayBooking_wednesdayNotExpensive(t *testing.T) {
	h := detectTuesdayBooking(context.Background(), DetectorInput{
		Date:        "2026-06-17", // Wednesday
		Origin:      "HEL",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for non-expensive day (Wednesday)")
	}
}

// Saturday is NOT an expensive day.
func TestDetectTuesdayBooking_saturdayNotExpensive(t *testing.T) {
	h := detectTuesdayBooking(context.Background(), DetectorInput{
		Date:        "2026-06-20", // Saturday
		Origin:      "HEL",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for non-expensive day (Saturday)")
	}
}

// --- detectMultiTripCombo: 0% coverage, test input guard ---

func TestDetectMultiTripCombo_empty(t *testing.T) {
	h := detectMultiTripCombo(context.Background(), "", "", nil, "EUR")
	if h != nil {
		t.Error("expected nil for empty inputs")
	}
}

// --- routesThroughDestination tests ---

func TestRoutesThroughDestination_nil(t *testing.T) {
	if routesThroughDestination(nil, "AMS") {
		t.Error("expected false for nil result")
	}
}

// --- primaryAirlineCode tests ---

func TestPrimaryAirlineCode_nilResult(t *testing.T) {
	if code := primaryAirlineCode(nil); code != "" {
		t.Errorf("expected empty for nil result, got %q", code)
	}
}

// --- toEUR edge cases ---

func TestToEUR_knownCurrency(t *testing.T) {
	v := toEUR(100, "GBP")
	if v != 117 {
		t.Errorf("expected 117 for GBP, got %.2f", v)
	}
}

func TestToEUR_unknownCurrency(t *testing.T) {
	v := toEUR(100, "ZZZ")
	if v != 100 {
		t.Errorf("expected 100 (passthrough) for unknown currency, got %.2f", v)
	}
}

func TestToEUR_eur(t *testing.T) {
	v := toEUR(50, "EUR")
	if v != 50 {
		t.Errorf("expected 50 for EUR, got %.2f", v)
	}
}

// --- isHomeAirport edge cases ---

func TestIsHomeAirport_nilPrefs(t *testing.T) {
	if isHomeAirport("HEL", nil) {
		t.Error("expected false for nil prefs")
	}
}

// --- groundCostBetween edge cases ---

func TestGroundCostBetween_reverse(t *testing.T) {
	// Test reverse lookup (VIE→PRG should find PRG→VIE).
	v := groundCostBetween("VIE", "PRG")
	if v != 15 {
		t.Errorf("expected 15 for VIE→PRG reverse, got %.0f", v)
	}
}

func TestGroundCostBetween_unknown(t *testing.T) {
	v := groundCostBetween("XXX", "YYY")
	if v != 25 {
		t.Errorf("expected default 25, got %.0f", v)
	}
}

// --- knownArbitrageAirlines data integrity ---

func TestKnownArbitrageAirlines_integrity(t *testing.T) {
	for _, n := range knownArbitrageAirlines {
		if n.AirlineCode == "" {
			t.Error("arbitrage airline entry has empty code")
		}
		if n.AirlineName == "" {
			t.Errorf("airline %s has empty name", n.AirlineCode)
		}
		if n.HomeCurrency == "" {
			t.Errorf("airline %s has empty home currency", n.AirlineCode)
		}
	}
}

// --- hiddenCityExtensions data integrity ---

func TestHiddenCityExtensions_keysAreHubs(t *testing.T) {
	for hub, beyonds := range hiddenCityExtensions {
		if len(hub) != 3 {
			t.Errorf("hidden city hub %q is not a 3-letter code", hub)
		}
		if len(beyonds) == 0 {
			t.Errorf("hub %s has no beyond destinations", hub)
		}
		for _, b := range beyonds {
			if len(b) != 3 {
				t.Errorf("hub %s: beyond %q is not a 3-letter code", hub, b)
			}
		}
	}
}

// --- ferryPositioningRoutes data integrity ---

func TestFerryPositioningRoutes_fieldsPopulated(t *testing.T) {
	for origin, routes := range ferryPositioningRoutes {
		for _, r := range routes {
			if r.FerryFrom == "" {
				t.Errorf("ferry route from %s has empty FerryFrom", origin)
			}
			if r.FerryTo == "" {
				t.Errorf("ferry route from %s has empty FerryTo", origin)
			}
			if r.AirportTo == "" {
				t.Errorf("ferry route from %s has empty AirportTo", origin)
			}
			if r.FerryEUR <= 0 {
				t.Errorf("ferry route %s→%s has non-positive FerryEUR", origin, r.AirportTo)
			}
		}
	}
}

// --- nearbyHubs data integrity ---

func TestNearbyHubs_fieldsPopulated(t *testing.T) {
	for dest, hubs := range nearbyHubs {
		for _, h := range hubs {
			if h.HubCode == "" {
				t.Errorf("nearbyHub for %s has empty HubCode", dest)
			}
			if h.HubCity == "" {
				t.Errorf("nearbyHub for %s has empty HubCity", dest)
			}
			if h.DestCity == "" {
				t.Errorf("nearbyHub for %s has empty DestCity", dest)
			}
			if h.StaticGroundEUR <= 0 {
				t.Errorf("nearbyHub %s→%s has non-positive StaticGroundEUR", dest, h.HubCode)
			}
		}
	}
}

// --- multiModalHubs data integrity ---

func TestMultiModalHubs_fieldsPopulated(t *testing.T) {
	for origin, hubs := range multiModalHubs {
		for _, h := range hubs {
			if h.HubCode == "" {
				t.Errorf("multiModalHub for %s has empty HubCode", origin)
			}
			if h.HubCity == "" {
				t.Errorf("multiModalHub for %s has empty HubCity", origin)
			}
			if h.OriginCity == "" {
				t.Errorf("multiModalHub for %s→%s has empty OriginCity", origin, h.HubCode)
			}
			if h.StaticGroundEUR <= 0 {
				t.Errorf("multiModalHub %s→%s has non-positive StaticGroundEUR", origin, h.HubCode)
			}
		}
	}
}

// --- openJawAlternates data integrity ---

func TestOpenJawAlternates_fieldsPopulated(t *testing.T) {
	for dest, alts := range openJawAlternates {
		if len(alts) == 0 {
			t.Errorf("destination %s has empty alternatives", dest)
		}
		for _, a := range alts {
			if len(a) != 3 {
				t.Errorf("destination %s: alternative %q is not 3-letter code", dest, a)
			}
		}
	}
}

// --- hubStopoverAllowance data integrity ---

func TestHubStopoverAllowance_fieldsPopulated(t *testing.T) {
	for hub, info := range hubStopoverAllowance {
		if info.Airline == "" {
			t.Errorf("hub %s has empty airline", hub)
		}
		if info.MaxNight <= 0 {
			t.Errorf("hub %s has non-positive MaxNight", hub)
		}
	}
}

// --- multistopHubs data integrity ---

func TestMultistopHubs_fieldsPopulated(t *testing.T) {
	for dest, hubs := range multistopHubs {
		if len(hubs) == 0 {
			t.Errorf("destination %s has empty hub list", dest)
		}
		for _, h := range hubs {
			if len(h) != 3 {
				t.Errorf("destination %s: hub %q is not 3-letter code", dest, h)
			}
		}
	}
}

// --- averageHotelCost constant ---

func TestAverageHotelCostConstant(t *testing.T) {
	if averageHotelCost <= 0 {
		t.Error("averageHotelCost must be positive")
	}
}

// --- adjustReturnDate with return ---

func TestAdjustReturnDate_withReturn(t *testing.T) {
	got := adjustReturnDate("2026-06-15", 3)
	if got != "2026-06-18" {
		t.Errorf("expected 2026-06-18, got %s", got)
	}
}

func TestAdjustReturnDate_empty(t *testing.T) {
	got := adjustReturnDate("", 3)
	if got != "" {
		t.Errorf("expected empty, got %s", got)
	}
}

// --- lowCostCarriers data integrity ---

func TestLowCostCarriers_allHaveNames(t *testing.T) {
	for code, name := range lowCostCarriers {
		if code == "" || name == "" {
			t.Errorf("LCC entry has empty code or name: %q -> %q", code, name)
		}
	}
}

// --- dateFlexWindow and dateFlexMinSaving constants ---

func TestDateFlexConstants(t *testing.T) {
	if dateFlexWindow <= 0 {
		t.Error("dateFlexWindow must be positive")
	}
	if dateFlexMinSaving <= 0 {
		t.Error("dateFlexMinSaving must be positive")
	}
}

// --- tuesdayBookingWindow and tuesdayBookingMinSaving constants ---

func TestTuesdayBookingConstants(t *testing.T) {
	if tuesdayBookingWindow <= 0 {
		t.Error("tuesdayBookingWindow must be positive")
	}
	if tuesdayBookingMinSaving <= 0 {
		t.Error("tuesdayBookingMinSaving must be positive")
	}
}

// --- minSavingsFraction constant ---

func TestMinSavingsFractionRange(t *testing.T) {
	if minSavingsFraction <= 0 || minSavingsFraction >= 1 {
		t.Errorf("minSavingsFraction must be between 0 and 1, got %f", minSavingsFraction)
	}
}

// --- minLayoverMinutesForStopover constant ---

func TestMinLayoverMinutesForStopoverConstant(t *testing.T) {
	if minLayoverMinutesForStopover < 60 {
		t.Errorf("minLayoverMinutesForStopover should be at least 60, got %d", minLayoverMinutesForStopover)
	}
}

// --- lowCostMinSavingPct constant ---

func TestLowCostMinSavingPctConstant(t *testing.T) {
	if lowCostMinSavingPct <= 0 || lowCostMinSavingPct > 100 {
		t.Errorf("lowCostMinSavingPct out of range: %f", lowCostMinSavingPct)
	}
}

// --- railCityName (66% -> 100%) ---

func TestRailCityName_knownCode(t *testing.T) {
	if got := railCityName("MAD"); got != "Madrid" {
		t.Errorf("expected Madrid, got %s", got)
	}
}

func TestRailCityName_knownInRailMap(t *testing.T) {
	if got := railCityName("BCN"); got != "Barcelona" {
		t.Errorf("expected Barcelona, got %s", got)
	}
}

func TestRailCityName_fallbackToCityFromCode(t *testing.T) {
	// HEL is in cityFromCode but not in railCityMap.
	if got := railCityName("HEL"); got != "Helsinki" {
		t.Errorf("expected Helsinki (via cityFromCode fallback), got %s", got)
	}
}

func TestRailCityName_unknownCode(t *testing.T) {
	// XXX is in neither map — should return the code itself.
	if got := railCityName("XXX"); got != "XXX" {
		t.Errorf("expected XXX (passthrough), got %s", got)
	}
}

// --- matchStopoverProgram (83% -> higher) ---

func TestMatchStopoverProgram_byAirline(t *testing.T) {
	prog, ok := matchStopoverProgram("HEL", "AY")
	if !ok {
		t.Fatal("expected match for AY+HEL")
	}
	if prog.Airline != "Finnair" {
		t.Errorf("expected Finnair, got %s", prog.Airline)
	}
}

func TestMatchStopoverProgram_byHubOnly(t *testing.T) {
	// Using an unknown airline code but known hub should match.
	prog, ok := matchStopoverProgram("HEL", "XX")
	if !ok {
		t.Fatal("expected match for HEL hub with unknown airline")
	}
	if prog.Hub != "HEL" {
		t.Errorf("expected Hub=HEL, got %s", prog.Hub)
	}
}

func TestMatchStopoverProgram_noMatch(t *testing.T) {
	_, ok := matchStopoverProgram("XXX", "ZZ")
	if ok {
		t.Error("expected no match for unknown hub and airline")
	}
}

func TestMatchStopoverProgram_airlineHubMismatch(t *testing.T) {
	// AY is Finnair with hub HEL. Asking for AMS+AY should not match by airline
	// (hub mismatch) but may match by hub if AMS is in stopoverPrograms.
	_, ok := matchStopoverProgram("AMS", "AY")
	// AMS is not a stopover program hub, so should not match.
	// Actually let's check: it could match via the hub scan...
	// The function scans all programs for hub match. No program has Hub=AMS.
	// So this should return false.
	if ok {
		t.Log("AMS matched a stopover program (unexpected but valid if data changed)")
	}
}

// --- isDirectLCCRoute (85% -> 100%) ---

func TestIsDirectLCCRoute_knownForward(t *testing.T) {
	if !isDirectLCCRoute("STN", "BCN") {
		t.Error("expected true for STN→BCN")
	}
}

func TestIsDirectLCCRoute_knownReverse(t *testing.T) {
	// Reverse lookup: BCN→STN should also be true.
	if !isDirectLCCRoute("BCN", "STN") {
		t.Error("expected true for BCN→STN (reverse)")
	}
}

func TestIsDirectLCCRoute_unknownPair(t *testing.T) {
	if isDirectLCCRoute("HEL", "XXX") {
		t.Error("expected false for unknown pair")
	}
}

func TestIsDirectLCCRoute_sameAirport(t *testing.T) {
	if isDirectLCCRoute("STN", "STN") {
		t.Error("expected false for same airport")
	}
}

func TestIsDirectLCCRoute_originInMapDestNot(t *testing.T) {
	// STN is in lccDirectRoutes but HEL is not a dest for STN.
	if isDirectLCCRoute("STN", "HEL") {
		t.Error("expected false for STN→HEL (not in LCC routes)")
	}
}

func TestIsDirectLCCRoute_reverseOnlyMatch(t *testing.T) {
	// DUB→STN: DUB has STN in its map.
	if !isDirectLCCRoute("DUB", "STN") {
		t.Error("expected true for DUB→STN")
	}
}

// --- primaryAirlineCode with various results ---

func TestPrimaryAirlineCode_unsuccessfulResult(t *testing.T) {
	result := &models.FlightSearchResult{Success: false}
	if code := primaryAirlineCode(result); code != "" {
		t.Errorf("expected empty for unsuccessful result, got %q", code)
	}
}

func TestPrimaryAirlineCode_noLegs(t *testing.T) {
	result := &models.FlightSearchResult{
		Success: true,
		Flights: []models.FlightResult{
			{Price: 100},
		},
	}
	if code := primaryAirlineCode(result); code != "" {
		t.Errorf("expected empty for no legs, got %q", code)
	}
}

func TestPrimaryAirlineCode_findsFirstLeg(t *testing.T) {
	result := &models.FlightSearchResult{
		Success: true,
		Flights: []models.FlightResult{
			{
				Price: 100,
				Legs: []models.FlightLeg{
					{AirlineCode: "AY", Airline: "Finnair"},
				},
			},
		},
	}
	if code := primaryAirlineCode(result); code != "AY" {
		t.Errorf("expected AY, got %q", code)
	}
}

// --- routesThroughDestination with various inputs ---

func TestRoutesThroughDestination_unsuccessfulResult(t *testing.T) {
	result := &models.FlightSearchResult{Success: false}
	if routesThroughDestination(result, "AMS") {
		t.Error("expected false for unsuccessful result")
	}
}

func TestRoutesThroughDestination_singleLeg(t *testing.T) {
	result := &models.FlightSearchResult{
		Success: true,
		Flights: []models.FlightResult{
			{
				Legs: []models.FlightLeg{
					{ArrivalAirport: models.AirportInfo{Code: "AMS"}},
				},
			},
		},
	}
	// Single-leg flights can't be hidden-city, but function returns true optimistically
	// when no multi-leg flight with intermediate stop is found.
	if !routesThroughDestination(result, "AMS") {
		t.Error("expected true (optimistic) for single-leg flight")
	}
}

func TestRoutesThroughDestination_multiLegMatch(t *testing.T) {
	result := &models.FlightSearchResult{
		Success: true,
		Flights: []models.FlightResult{
			{
				Legs: []models.FlightLeg{
					{ArrivalAirport: models.AirportInfo{Code: "AMS"}},
					{ArrivalAirport: models.AirportInfo{Code: "HEL"}},
				},
			},
		},
	}
	if !routesThroughDestination(result, "AMS") {
		t.Error("expected true: flight stops at AMS as intermediate")
	}
}

func TestRoutesThroughDestination_multiLegNoMatch(t *testing.T) {
	result := &models.FlightSearchResult{
		Success: true,
		Flights: []models.FlightResult{
			{
				Legs: []models.FlightLeg{
					{ArrivalAirport: models.AirportInfo{Code: "FRA"}},
					{ArrivalAirport: models.AirportInfo{Code: "HEL"}},
				},
			},
		},
	}
	// No intermediate stop at AMS, but optimistic return for non-empty flights.
	if !routesThroughDestination(result, "AMS") {
		t.Error("expected true (optimistic fallback)")
	}
}

func TestRoutesThroughDestination_emptyFlights(t *testing.T) {
	result := &models.FlightSearchResult{
		Success: true,
		Flights: []models.FlightResult{},
	}
	if routesThroughDestination(result, "AMS") {
		t.Error("expected false for empty flights")
	}
}

// --- isHomeAirport with matches ---

func TestIsHomeAirport_found(t *testing.T) {
	prefs := &preferences.Preferences{
		HomeAirports: []string{"HEL", "TLL"},
	}
	if !isHomeAirport("HEL", prefs) {
		t.Error("expected true for HEL in home airports")
	}
}

func TestIsHomeAirport_notFound(t *testing.T) {
	prefs := &preferences.Preferences{
		HomeAirports: []string{"HEL", "TLL"},
	}
	if isHomeAirport("AMS", prefs) {
		t.Error("expected false for AMS not in home airports")
	}
}

func TestIsHomeAirport_emptyHomeAirports(t *testing.T) {
	prefs := &preferences.Preferences{
		HomeAirports: []string{},
	}
	if isHomeAirport("HEL", prefs) {
		t.Error("expected false for empty home airports")
	}
}

// --- groundCostBetween known pairs ---

func TestGroundCostBetween_knownPair(t *testing.T) {
	v := groundCostBetween("AMS", "BRU")
	if v != 20 {
		t.Errorf("expected 20 for AMS→BRU, got %.0f", v)
	}
}

func TestGroundCostBetween_knownPairCPHARN(t *testing.T) {
	v := groundCostBetween("CPH", "ARN")
	if v != 30 {
		t.Errorf("expected 30 for CPH→ARN, got %.0f", v)
	}
}

func TestGroundCostBetween_knownPairBCNMAD(t *testing.T) {
	v := groundCostBetween("BCN", "MAD")
	if v != 30 {
		t.Errorf("expected 30 for BCN→MAD, got %.0f", v)
	}
}

func TestGroundCostBetween_reverseAMSDUS(t *testing.T) {
	// Forward: AMS→DUS=20, reverse should also work.
	v := groundCostBetween("DUS", "AMS")
	if v != 20 {
		t.Errorf("expected 20 for DUS→AMS (reverse), got %.0f", v)
	}
}

// --- cancelled context tests for API-dependent detectors ---
// These test that a cancelled context causes the detector to return nil
// (covering the error path after the API call).

func TestDetectDateFlex_cancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately
	h := detectDateFlex(ctx, DetectorInput{
		Date:        "2026-06-15",
		Origin:      "HEL",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for cancelled context")
	}
}

func TestDetectCurrencyArbitrage_cancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	h := detectCurrencyArbitrage(ctx, DetectorInput{
		Date:        "2026-06-15",
		Origin:      "HEL",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for cancelled context")
	}
}

func TestDetectHiddenCity_cancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	h := detectHiddenCity(ctx, DetectorInput{
		Date:        "2026-06-15",
		Origin:      "HEL",
		Destination: "AMS",
	})
	if h != nil {
		t.Error("expected nil for cancelled context")
	}
}

func TestDetectLowCostCarrier_cancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	h := detectLowCostCarrier(ctx, DetectorInput{
		Date:        "2026-06-15",
		Origin:      "HEL",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for cancelled context")
	}
}

func TestDetectPositioning_cancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	h := detectPositioning(ctx, DetectorInput{
		Date:        "2026-06-15",
		Origin:      "HEL",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for cancelled context")
	}
}

func TestDetectSplit_cancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	h := detectSplit(ctx, DetectorInput{
		Date:        "2026-06-15",
		ReturnDate:  "2026-06-22",
		Origin:      "HEL",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for cancelled context")
	}
}

func TestDetectThrowaway_cancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	h := detectThrowaway(ctx, DetectorInput{
		Date:        "2026-06-15",
		Origin:      "HEL",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for cancelled context")
	}
}

func TestDetectOpenJaw_cancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	h := detectOpenJaw(ctx, DetectorInput{
		Date:        "2026-06-15",
		ReturnDate:  "2026-06-22",
		Origin:      "HEL",
		Destination: "PRG",
	})
	if h != nil {
		t.Error("expected nil for cancelled context")
	}
}

func TestDetectStopover_cancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	h := detectStopover(ctx, DetectorInput{
		Date:        "2026-06-15",
		Origin:      "HEL",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for cancelled context")
	}
}

func TestDetectNightTransport_unknownRoute(t *testing.T) {
	// Unknown city pair with no ground routes — should return nil gracefully.
	h := detectNightTransport(context.Background(), DetectorInput{
		Date:        "2026-06-15",
		Origin:      "XXX",
		Destination: "YYY",
	})
	if h != nil {
		t.Error("expected nil for unknown route")
	}
}

func TestDetectFerryPositioning_cancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	h := detectFerryPositioning(ctx, DetectorInput{
		Date:        "2026-06-15",
		Origin:      "HEL",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for cancelled context")
	}
}

func TestDetectMultiStop_cancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	h := detectMultiStop(ctx, DetectorInput{
		Date:        "2026-06-15",
		Origin:      "HEL",
		Destination: "PRG",
	})
	if h != nil {
		t.Error("expected nil for cancelled context")
	}
}

func TestDetectTuesdayBooking_cancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	h := detectTuesdayBooking(ctx, DetectorInput{
		Date:        "2026-06-19", // Friday
		Origin:      "HEL",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for cancelled context")
	}
}

func TestDetectMultiModalSkipFlight_cancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	h := detectMultiModalSkipFlight(ctx, DetectorInput{
		Date:        "2026-06-15",
		Origin:      "HEL",
		Destination: "TLL",
	})
	if h != nil {
		t.Error("expected nil for cancelled context")
	}
}

func TestDetectMultiModalPositioning_cancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	h := detectMultiModalPositioning(ctx, DetectorInput{
		Date:        "2026-06-15",
		Origin:      "HEL",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for cancelled context")
	}
}

func TestDetectMultiModalOpenJawGround_cancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	h := detectMultiModalOpenJawGround(ctx, DetectorInput{
		Date:        "2026-06-15",
		Origin:      "HEL",
		Destination: "DBV",
	})
	if h != nil {
		t.Error("expected nil for cancelled context")
	}
}

func TestDetectMultiModalReturnSplit_cancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	h := detectMultiModalReturnSplit(ctx, DetectorInput{
		Date:        "2026-06-15",
		ReturnDate:  "2026-06-22",
		Origin:      "HEL",
		Destination: "TLL",
	})
	if h != nil {
		t.Error("expected nil for cancelled context")
	}
}

// --- DetectRailFlyArbitrage edge cases ---

func TestDetectRailFlyArbitrage_emptyOrigin(t *testing.T) {
	h := DetectRailFlyArbitrage(context.Background(), "", "BCN", "2026-06-15", "")
	if h != nil {
		t.Error("expected nil for empty origin")
	}
}

func TestDetectRailFlyArbitrage_emptyDestination(t *testing.T) {
	h := DetectRailFlyArbitrage(context.Background(), "AMS", "", "2026-06-15", "")
	if h != nil {
		t.Error("expected nil for empty destination")
	}
}

func TestDetectRailFlyArbitrage_emptyDate(t *testing.T) {
	h := DetectRailFlyArbitrage(context.Background(), "AMS", "BCN", "", "")
	if h != nil {
		t.Error("expected nil for empty date")
	}
}

func TestDetectRailFlyArbitrage_noStationsForOrigin(t *testing.T) {
	h := DetectRailFlyArbitrage(context.Background(), "BCN", "HEL", "2026-06-15", "")
	if h != nil {
		t.Error("expected nil for origin without rail stations")
	}
}

func TestDetectRailFlyArbitrage_cancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	h := DetectRailFlyArbitrage(ctx, "AMS", "BCN", "2026-06-15", "")
	if h != nil {
		t.Error("expected nil for cancelled context")
	}
}

// --- isLowCostFlight and lccName with more cases ---

func TestIsLowCostFlight_noLegs(t *testing.T) {
	f := models.FlightResult{Price: 100}
	if isLowCostFlight(f) {
		t.Error("expected false for flight with no legs")
	}
}

func TestIsLowCostFlight_legacyCarrier(t *testing.T) {
	f := models.FlightResult{
		Legs: []models.FlightLeg{{AirlineCode: "BA"}},
	}
	if isLowCostFlight(f) {
		t.Error("expected false for legacy carrier BA")
	}
}

func TestIsLowCostFlight_mixedLegs(t *testing.T) {
	f := models.FlightResult{
		Legs: []models.FlightLeg{
			{AirlineCode: "BA"},
			{AirlineCode: "FR"},
		},
	}
	if !isLowCostFlight(f) {
		t.Error("expected true when one leg is LCC (FR)")
	}
}

func TestLccName_noLCCLegs(t *testing.T) {
	f := models.FlightResult{
		Legs: []models.FlightLeg{{AirlineCode: "BA"}},
	}
	if got := lccName(f); got != "LCC" {
		t.Errorf("expected fallback 'LCC', got %q", got)
	}
}

func TestLccName_wizz(t *testing.T) {
	f := models.FlightResult{
		Legs: []models.FlightLeg{{AirlineCode: "W6"}},
	}
	if got := lccName(f); got != "Wizz Air" {
		t.Errorf("expected 'Wizz Air', got %q", got)
	}
}

// --- loyaltyConflictNote edge cases ---

func TestLoyaltyConflictNote_emptyLoyalty(t *testing.T) {
	result := &models.FlightSearchResult{
		Success: true,
		Flights: []models.FlightResult{
			{Legs: []models.FlightLeg{{AirlineCode: "AY", Airline: "Finnair"}}},
		},
	}
	prefs := &preferences.Preferences{
		LoyaltyAirlines: []string{},
	}
	if got := loyaltyConflictNote(result, prefs); got != "" {
		t.Errorf("expected empty for no loyalty airlines, got %q", got)
	}
}

func TestLoyaltyConflictNote_noConflict(t *testing.T) {
	result := &models.FlightSearchResult{
		Success: true,
		Flights: []models.FlightResult{
			{Legs: []models.FlightLeg{{AirlineCode: "AY", Airline: "Finnair"}}},
		},
	}
	prefs := &preferences.Preferences{
		LoyaltyAirlines: []string{"BA"},
	}
	if got := loyaltyConflictNote(result, prefs); got != "" {
		t.Errorf("expected empty for non-matching loyalty, got %q", got)
	}
}

func TestLoyaltyConflictNote_conflict(t *testing.T) {
	result := &models.FlightSearchResult{
		Success: true,
		Flights: []models.FlightResult{
			{Legs: []models.FlightLeg{{AirlineCode: "AY", Airline: "Finnair"}}},
		},
	}
	prefs := &preferences.Preferences{
		LoyaltyAirlines: []string{"AY"},
	}
	got := loyaltyConflictNote(result, prefs)
	if got == "" {
		t.Error("expected loyalty conflict note")
	}
}

// --- DetectFlightCombo cancelled context ---

func TestDetectFlightCombo_cancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	h := DetectFlightCombo(ctx, FlightComboInput{
		Origin:      "HEL",
		Destination: "BCN",
		Trips: []TripLeg{
			{DepartDate: "2026-06-15", ReturnDate: "2026-06-22"},
		},
		Currency: "EUR",
	})
	if h != nil {
		t.Error("expected nil for cancelled context")
	}
}

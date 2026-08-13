package baggage

import (
	"testing"

	"github.com/MikkoParkkola/trvl/internal/models"
)

func TestResolveCheckedBag(t *testing.T) {
	one, two, zero := 1, 2, 0

	cases := []struct {
		name        string
		provider    *int // what the flight provider reported (nil = did not state)
		airline     string
		wantIncl    bool
		wantSource  models.BagSource
		wantAmtMin  float64
		description string
	}{
		{
			name: "provider states one bag", provider: &one, airline: "LH",
			wantIncl: true, wantSource: models.BagSourceProvider,
			description: "hard data always wins over the table",
		},
		{
			name: "provider states two bags", provider: &two, airline: "AA",
			wantIncl: true, wantSource: models.BagSourceProvider,
		},
		{
			name: "provider states none, table has a sourced range", provider: &zero, airline: "FR",
			wantIncl: false, wantSource: models.BagSourceProvider, wantAmtMin: 9.49,
			description: "inclusion is hard data; the fee to add is still an estimate",
		},
		{
			name: "provider silent, table says included", provider: nil, airline: "LH",
			wantIncl: true, wantSource: models.BagSourceTableSourced,
			description: "Lufthansa's allowance was read off its own page, so the claim is cited",
		},
		{
			name: "provider silent, table says not included", provider: nil, airline: "VY",
			wantIncl: false, wantSource: models.BagSourceTableUnsourced, wantAmtMin: 18,
			description: "Vueling's fee has a source but its allowance does not; the weaker claim governs",
		},
		{
			name: "provider silent, airline not in table", provider: nil, airline: "JU",
			wantIncl: false, wantSource: models.BagSourceUnknown,
			description: "Air Serbia is not covered; assume no bag but never invent a fee",
		},
		{
			name: "no airline code at all", provider: nil, airline: "",
			wantIncl: false, wantSource: models.BagSourceUnknown,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveCheckedBag(tc.provider, tc.airline, nil)
			if got.Included != tc.wantIncl {
				t.Errorf("Included = %v, want %v (%s)", got.Included, tc.wantIncl, tc.description)
			}
			if got.Source != tc.wantSource {
				t.Errorf("Source = %q, want %q", got.Source, tc.wantSource)
			}
			if got.AmountMin != tc.wantAmtMin {
				t.Errorf("AmountMin = %v, want %v", got.AmountMin, tc.wantAmtMin)
			}
			if got.Source == models.BagSourceUnknown && got.AmountMin != 0 {
				t.Errorf("unknown terms must never carry an amount, got %v", got.AmountMin)
			}
		})
	}
}

// TestResolveCheckedBagVariableFee covers carriers that publish no figure at
// all: we must state that the fee varies rather than emit a number.
func TestResolveCheckedBagVariableFee(t *testing.T) {
	got := ResolveCheckedBag(nil, "W6", nil)
	if got.Included {
		t.Error("Wizz Air includes no checked bag")
	}
	if got.AmountMin != 0 || got.AmountMax != 0 {
		t.Errorf("a carrier that publishes no fee must carry no amount, got %v-%v", got.AmountMin, got.AmountMax)
	}
	if got.Reference == "" {
		t.Error("expected the variable-fee situation to be stated in Reference")
	}
}

// TestResolveCheckedBagFrequentFlyer pins that alliance status grants a bag the
// fare itself does not include. This has to happen before the bag filter runs:
// applied afterwards, as it used to be, it could never rescue a flight the
// filter had already dropped.
func TestResolveCheckedBagFrequentFlyer(t *testing.T) {
	gold := []FFStatus{{Alliance: "star_alliance", Tier: "gold"}}

	// Ryanair is in no alliance, so status changes nothing.
	if got := ResolveCheckedBag(nil, "FR", gold); got.Included {
		t.Error("Ryanair is in no alliance; status must not grant a bag")
	}

	// Swiss is Star Alliance: Gold grants a checked bag even on a fare without one.
	got := ResolveCheckedBag(nil, "LX", gold)
	if !got.Included {
		t.Error("Star Alliance Gold must grant a checked bag on Swiss")
	}
	if got.Source != models.BagSourceFrequentFlyer {
		t.Errorf("source = %q, want the entitlement to be credited", got.Source)
	}
}

// TestResolveCheckedBagCitesInclusion covers the inclusion claim's own
// provenance. Until now only the fee carried a source, so even an airline whose
// allowance we had read off its own page came back table_unsourced — every
// verdict on a real route did, which is what made the whole filter rest on
// uncited data.
func TestResolveCheckedBagCitesInclusion(t *testing.T) {
	// Finnair: Economy Light carries no checked bag, read off Finnair's table.
	ay := ResolveCheckedBag(nil, "AY", nil)
	if ay.Included {
		t.Error("Finnair's cheapest long-haul brand includes no checked bag")
	}
	if ay.Source != models.BagSourceTableSourced {
		t.Errorf("source = %q, want table_sourced — the claim has a citation", ay.Source)
	}

	// Cathay: Economy Light does include one, so it must survive a bag filter.
	cx := ResolveCheckedBag(nil, "CX", nil)
	if !cx.Included {
		t.Error("Cathay's Light fare includes 1x23 kg; dropping it is a false negative")
	}
	if cx.Source != models.BagSourceTableSourced {
		t.Errorf("source = %q, want table_sourced", cx.Source)
	}
	if cx.Reference == "" || cx.Verified == "" {
		t.Errorf("a sourced claim must carry its reference and date, got %q / %q", cx.Reference, cx.Verified)
	}

	// Turkish publishes no route table, so its value stays explicitly uncited.
	tk := ResolveCheckedBag(nil, "TK", nil)
	if tk.Source != models.BagSourceTableUnsourced {
		t.Errorf("source = %q, want table_unsourced — Turkish publishes no table", tk.Source)
	}
}

// TestResolveCheckedBagJapanRoutes covers the carriers that dominate Europe-Japan
// and were absent from the table, so the bag filter discarded every one of them
// as "unknown" rather than on their merits. Two of the five genuinely carry no
// bag on their cheapest brand, which is why adding them wholesale as "includes"
// would have been the same mistake in the other direction.
func TestResolveCheckedBagJapanRoutes(t *testing.T) {
	cases := []struct {
		airline  string
		wantIncl bool
		why      string
	}{
		{"JL", true, "JAL gives 2 pieces on every international economy fare; it publishes no zero-bag brand"},
		{"NH", true, "ANA Light/Value carry 1 piece ex-Japan to Europe"},
		{"OZ", true, "Asiana is 1 piece on all non-US routes, Europe included"},
		{"MU", false, "China Eastern Basic Economy carries none on Europe routes"},
		{"UX", false, "Air Europa LITE carries none"},
	}

	for _, tc := range cases {
		t.Run(tc.airline, func(t *testing.T) {
			got := ResolveCheckedBag(nil, tc.airline, nil)
			if got.Included != tc.wantIncl {
				t.Errorf("Included = %v, want %v — %s", got.Included, tc.wantIncl, tc.why)
			}
			if got.Source != models.BagSourceTableSourced {
				t.Errorf("Source = %q, want table_sourced — each was read off the airline's own page", got.Source)
			}
			if got.Verified == "" {
				t.Error("a sourced claim must carry the date it was checked")
			}
		})
	}
}

// TestResolveCheckedBagLatinAmerica covers the carriers that dominate Europe to
// Peru, Costa Rica and Colombia. The expectation going in was that adding them
// would rescue cheap bagged options the filter was discarding as unknown. It
// does the opposite for the two flag carriers: LATAM and Avianca both sell
// their cheapest long-haul brands with no checked bag at all, so the higher
// prices those routes were showing are real, not an artefact of missing data.
func TestResolveCheckedBagLatinAmerica(t *testing.T) {
	cases := []struct {
		airline  string
		wantIncl bool
		why      string
	}{
		{"LA", false, "LATAM lists Basic and Light under 'fares that do not include checked bags'"},
		{"AV", false, "Avianca Basic and Light pay from EUR 95; only Classic and Flex include one"},
		{"WK", true, "Edelweiss publishes 1x23 kg for Economy and sells no long-haul Light brand"},
		{"BR", true, "EVA gives 2 pieces on every named long-haul brand, Basic included"},
		{"TS", false, "Air Transat Eco Budget includes a carry-on to Europe but no checked bag"},
	}

	for _, tc := range cases {
		t.Run(tc.airline, func(t *testing.T) {
			got := ResolveCheckedBag(nil, tc.airline, nil)
			if got.Included != tc.wantIncl {
				t.Errorf("Included = %v, want %v — %s", got.Included, tc.wantIncl, tc.why)
			}
			if got.Source != models.BagSourceTableSourced {
				t.Errorf("Source = %q, want table_sourced", got.Source)
			}
		})
	}

	// LATAM's fee is a wide published range, not a point, and must survive as one.
	la := ResolveCheckedBag(nil, "LA", nil)
	if la.AmountMin != 35 || la.AmountMax != 150 {
		t.Errorf("LATAM fee = %v-%v, want the published 35-150", la.AmountMin, la.AmountMax)
	}
}

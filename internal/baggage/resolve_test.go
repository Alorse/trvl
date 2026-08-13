package baggage

import (
	"fmt"
	"testing"
	"time"

	"github.com/MikkoParkkola/trvl/internal/models"
)

// The three states Included can now hold. Unknown is nil, and inclEq keeps a
// nil from quietly comparing equal to "no bag" — the exact conflation the
// pointer was introduced to stop.
var (
	inclYes = models.BagIncluded()
	inclNo  = models.BagNotIncluded()
)

func inclEq(got, want *bool) bool {
	if got == nil || want == nil {
		return got == nil && want == nil
	}
	return *got == *want
}

func inclStr(v *bool) string {
	if v == nil {
		return "unknown"
	}
	return fmt.Sprintf("%v", *v)
}

func TestResolveCheckedBag(t *testing.T) {
	one, two, zero := 1, 2, 0

	cases := []struct {
		name        string
		provider    *int // what the flight provider reported (nil = did not state)
		airline     string
		wantIncl    *bool
		wantSource  models.BagSource
		wantAmtMin  float64
		description string
	}{
		{
			name: "provider states one bag", provider: &one, airline: "LH",
			wantIncl: inclYes, wantSource: models.BagSourceProvider,
			description: "hard data always wins over the table",
		},
		{
			name: "provider states two bags", provider: &two, airline: "AA",
			wantIncl: inclYes, wantSource: models.BagSourceProvider,
		},
		{
			name: "provider states none, table has a sourced range", provider: &zero, airline: "FR",
			wantIncl: inclNo, wantSource: models.BagSourceProvider, wantAmtMin: 9.49,
			description: "inclusion is hard data; the fee to add is still an estimate",
		},
		{
			name: "provider silent, table says included", provider: nil, airline: "CX",
			wantIncl: inclYes, wantSource: models.BagSourceTableSourced,
			description: "Cathay's Light fare carries 1x23kg, read off its own page. This case used to name Lufthansa, until Lufthansa's own calculator showed Economy Light carrying none",
		},
		{
			name: "provider silent, table says not included", provider: nil, airline: "VY",
			wantIncl: inclNo, wantSource: models.BagSourceTableUnsourced, wantAmtMin: 18,
			description: "Vueling's fee has a source but its allowance does not; the weaker claim governs",
		},
		{
			name: "provider silent, airline not in table", provider: nil, airline: "JU",
			wantIncl: nil, wantSource: models.BagSourceUnknown,
			description: "Air Serbia is not covered: no verdict either way, and never invent a fee",
		},
		{
			name: "no airline code at all", provider: nil, airline: "",
			wantIncl: nil, wantSource: models.BagSourceUnknown,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveCheckedBag(tc.provider, tc.airline, nil)
			if !inclEq(got.Included, tc.wantIncl) {
				t.Errorf("Included = %s, want %s (%s)", inclStr(got.Included), inclStr(tc.wantIncl), tc.description)
			}
			if got.Source != tc.wantSource {
				t.Errorf("Source = %q, want %q", got.Source, tc.wantSource)
			}
			if got.AmountMin != tc.wantAmtMin {
				t.Errorf("AmountMin = %v, want %v", got.AmountMin, tc.wantAmtMin)
			}
			if got.Source == models.BagSourceUnknown && got.Included != nil {
				t.Errorf("an unknown source must not assert an inclusion verdict, got %s", inclStr(got.Included))
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
	if got.HasBag() {
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
	if got := ResolveCheckedBag(nil, "FR", gold); got.HasBag() {
		t.Error("Ryanair is in no alliance; status must not grant a bag")
	}

	// Swiss is Star Alliance: Gold grants a checked bag even on a fare without one.
	got := ResolveCheckedBag(nil, "LX", gold)
	if !got.HasBag() {
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
	if ay.HasBag() {
		t.Error("Finnair's cheapest long-haul brand includes no checked bag")
	}
	if ay.Source != models.BagSourceTableSourced {
		t.Errorf("source = %q, want table_sourced — the claim has a citation", ay.Source)
	}

	// Cathay: Economy Light does include one, so it must survive a bag filter.
	cx := ResolveCheckedBag(nil, "CX", nil)
	if !cx.HasBag() {
		t.Error("Cathay's Light fare includes 1x23 kg; dropping it is a false negative")
	}
	if cx.Source != models.BagSourceTableSourced {
		t.Errorf("source = %q, want table_sourced", cx.Source)
	}
	if cx.Reference == "" || cx.Verified == "" {
		t.Errorf("a sourced claim must carry its reference and date, got %q / %q", cx.Reference, cx.Verified)
	}

	// Singapore Airlines has never been researched. An uncited claim that an
	// airline INCLUDES a bag is the exact shape that proved wrong for Iberia,
	// KLM, British Airways and SWISS, so it carries no date, never counts as
	// fresh, and is reported as unknown rather than asserted.
	sq := ResolveCheckedBag(nil, "SQ", nil)
	if sq.HasBag() {
		t.Error("an uncited positive claim must not assert a bag")
	}
	if !sq.IsUnknown() {
		t.Errorf("Included = %s, want unknown: dropping an uncited claim leaves us without a verdict, not with a negative one", inclStr(sq.Included))
	}
	if sq.Source != models.BagSourceUnknown {
		t.Errorf("source = %q, want unknown", sq.Source)
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
			if got.IsUnknown() {
				t.Fatalf("no verdict at all — %s", tc.why)
			}
			if got.HasBag() != tc.wantIncl {
				t.Errorf("Included = %s, want %v — %s", inclStr(got.Included), tc.wantIncl, tc.why)
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
			if got.IsUnknown() {
				t.Fatalf("no verdict at all — %s", tc.why)
			}
			if got.HasBag() != tc.wantIncl {
				t.Errorf("Included = %s, want %v — %s", inclStr(got.Included), tc.wantIncl, tc.why)
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

// TestResolveCheckedBagEuropeanNetworkCarriers pins the six that had been
// claiming a checked bag with no citation behind the claim. Four of them turned
// out to sell a long-haul brand carrying none — Iberia Basic, KLM Light,
// British Airways Basic and SWISS Light — and between them they carried 38 of
// 127 results on a single Munich-Lima search, every one of which had been
// passing a bag filter on an optimistic guess.
func TestResolveCheckedBagEuropeanNetworkCarriers(t *testing.T) {
	cases := []struct {
		airline  string
		wantIncl bool
		why      string
	}{
		{"IB", false, "Iberia's long-haul tab shows Basic checked baggage as 'Purchase'"},
		{"KL", false, "KLM: 'Light ticket ... Checked baggage: not included, but can be added for a fee'"},
		{"BA", false, "British Airways Basic in World Traveller is 'hand bag and cabin bag only'"},
		{"LX", false, "SWISS Economy Light on JFK-ZRH returns carry-on and personal item only"},
		{"QR", true, "Qatar's cheapest brand, Economy Lite, still carries a bag"},
		{"TK", true, "Turkish EcoFly returns a 20 kg allowance on Europe-Asia long-haul"},
	}

	for _, tc := range cases {
		t.Run(tc.airline, func(t *testing.T) {
			got := ResolveCheckedBag(nil, tc.airline, nil)
			if got.IsUnknown() {
				t.Fatalf("no verdict at all — %s", tc.why)
			}
			if got.HasBag() != tc.wantIncl {
				t.Errorf("Included = %s, want %v — %s", inclStr(got.Included), tc.wantIncl, tc.why)
			}
			if got.Source != models.BagSourceTableSourced {
				t.Errorf("Source = %q, want table_sourced — all six now carry a citation", got.Source)
			}
		})
	}
}

// TestResolveCheckedBagDecaysWithAge covers the staleness guard. Baggage
// unbundling is a ratchet: every change we have documented removed an
// allowance, none restored one, so a stale entry drifts toward claiming a bag
// that no longer exists. Downstream that is the expensive direction — a fare
// without a bag passes the filter, gets cached as the day's price, and a
// ratcheting cache pins it there.
//
// Only positive claims decay. A stale "no bag" cannot cause that failure.
func TestResolveCheckedBagDecaysWithAge(t *testing.T) {
	orig := nowFunc
	defer func() { nowFunc = orig }()

	fresh := AirlineBaggage{Code: "XX", CheckedIncluded: 1, CheckedSource: "https://example.test/bags", CheckedVerified: "2026-08"}

	at := func(ym string) { nowFunc = func() time.Time { t, _ := time.Parse("2006-01", ym); return t } }

	at("2026-10") // 2 months on — still trusted
	if got := resolveFromTable(fresh); got.Source != models.BagSourceTableSourced || !got.HasBag() {
		t.Errorf("a recent claim must stand: %+v", got)
	}

	at("2027-08") // 12 months on — past its shelf life, still believed but no longer cited
	got := resolveFromTable(fresh)
	if !got.HasBag() {
		t.Error("a year on, the claim is doubted but not yet discarded")
	}
	if got.Source != models.BagSourceTableUnsourced {
		t.Errorf("Source = %q, want the citation to lapse", got.Source)
	}

	at("2028-06") // ~22 months on — no longer good enough to pass a bag filter
	got = resolveFromTable(fresh)
	if got.HasBag() {
		t.Error("an expired positive claim must stop asserting a bag")
	}
	if !got.IsUnknown() {
		t.Errorf("Included = %s, want unknown: an expired allowance tells us nothing about what the airline now sells", inclStr(got.Included))
	}
	if got.Source != models.BagSourceUnknown {
		t.Errorf("Source = %q, want unknown", got.Source)
	}

	// A negative claim never expires: it cannot cause the cached-price failure,
	// and airlines do not quietly start including bags again.
	none := AirlineBaggage{Code: "YY", CheckedIncluded: 0, CheckedSource: "https://example.test/none", CheckedVerified: "2020-01"}
	at("2028-06")
	if got := resolveFromTable(none); got.HasBag() || got.Source != models.BagSourceTableSourced {
		t.Errorf("a negative claim must survive unchanged: %+v", got)
	}
}

// TestResolveCheckedBagIntraAsia covers the carriers flying intra-Asian routes,
// where the table previously had no coverage at all. The gap was not academic:
// on KIX-ICN two thirds of priced results resolved to unknown, and on every
// route measured the CHEAPEST flight was one of them — so a price cache that
// drops flights it cannot total was discarding the cheap carrier and storing a
// full-service fare two to three times the price.
//
// HK Express publishes in HKD, which is only usable because bag fees now
// convert into the fare's currency.
func TestResolveCheckedBagIntraAsia(t *testing.T) {
	uo := ResolveCheckedBag(nil, "UO", nil)
	if uo.HasBag() {
		t.Error("HK Express includes a checked bag only in Essential and Max, not in its cheapest bundle")
	}
	if !uo.LacksBag() {
		t.Errorf("this is a cited negative, not an absence of evidence: %+v", uo)
	}
	if uo.Source != models.BagSourceTableSourced {
		t.Errorf("Source = %q, want table_sourced — read off the airline's own page", uo.Source)
	}
	if uo.Currency != "HKD" {
		t.Errorf("Currency = %q, want the fee kept in the currency HK Express publishes", uo.Currency)
	}
	if uo.AmountMin != 310 || uo.AmountMax != 600 {
		t.Errorf("fee = %v-%v, want the published 310-600 for a 20 kg piece", uo.AmountMin, uo.AmountMax)
	}
	if uo.Verified == "" {
		t.Error("a sourced claim must carry the date it was checked")
	}

	// Jeju's cheapest brand is explicit about it: "BASIC passengers : Free
	// baggage service 0KG". Its STANDARD brand carries 15 kg, which is why
	// reading the standard brand would have got this backwards.
	jj := ResolveCheckedBag(nil, "7C", nil)
	if !jj.LacksBag() {
		t.Errorf("Jeju BASIC carries no free allowance: %+v", jj)
	}
	if jj.Source != models.BagSourceTableSourced {
		t.Errorf("Source = %q, want table_sourced", jj.Source)
	}
	if jj.Currency != "USD" || jj.AmountMin != 40 || jj.AmountMax != 60 {
		t.Errorf("fee = %s %v-%v, want USD 40-60 for the first 15 kg", jj.Currency, jj.AmountMin, jj.AmountMax)
	}

	// Peach prices a first bag under Minimum and gives it free under Standard.
	// Minimum is the cheapest of its three brands, so the surfaced fare has none.
	mm := ResolveCheckedBag(nil, "MM", nil)
	if !mm.LacksBag() {
		t.Errorf("Peach Minimum pays for its first bag: %+v", mm)
	}
	if mm.Currency != "JPY" || mm.AmountMin != 2600 || mm.AmountMax != 7500 {
		t.Errorf("fee = %s %v-%v, want the published JPY 2600-7500 zone span", mm.Currency, mm.AmountMin, mm.AmountMax)
	}

	// Air Busan goes the other way, and the direction matters: assuming a
	// low-cost carrier includes nothing would have added a fee to a fare that
	// already carries 15 kg, inflating the cheapest flight on the route and
	// causing the same substitution from the opposite side.
	bx := ResolveCheckedBag(nil, "BX", nil)
	if !bx.HasBag() {
		t.Errorf("Air Busan's regular economy carries 15 kg on non-America routes: %+v", bx)
	}
	if bx.Source != models.BagSourceTableSourced {
		t.Errorf("Source = %q, want table_sourced — read off the airline's own table", bx.Source)
	}

	// T'Way goes back the other way, and the two must not be confused. Its
	// Event fare is the cheapest rung of a published brand ladder, so the
	// cheapest-brand rule applies exactly as it does to Iberia Basic; Air
	// Busan's Special/Event names a class of flight, not a brand.
	tw := ResolveCheckedBag(nil, "TW", nil)
	if !tw.LacksBag() {
		t.Errorf("T'Way's Event fare pays a flat rate from the first gram: %+v", tw)
	}
	if tw.Currency != "KRW" || tw.AmountMin != 60000 || tw.AmountMax != 80000 {
		t.Errorf("fee = %s %v-%v, want KRW 60000-80000 for travel from 2026-03-30", tw.Currency, tw.AmountMin, tw.AmountMax)
	}

	// Hong Kong Airlines is the case where the rule and the generous answer
	// agree: a brand ladder exists, and even its cheapest rung carries a bag.
	hx := ResolveCheckedBag(nil, "HX", nil)
	if !hx.HasBag() {
		t.Errorf("Value Economy carries 1x23 kg; dropping it is a false negative: %+v", hx)
	}
	if hx.Source != models.BagSourceTableSourced || hx.Verified == "" {
		t.Errorf("want a dated citation, got source %q verified %q", hx.Source, hx.Verified)
	}
}

// TestResolveCheckedBagAustrian pins a correction rather than an addition.
// Austrian was already in the table claiming "1x23kg checked bag included on
// most fares" with nothing behind it. Its own baggage calculator, run on a
// long-haul route, returns a carry-on and a personal item and prices a first
// checked bag as an extra — so the claim was not merely uncited, it was
// backwards, the same way it had been for Iberia, KLM, British Airways and
// SWISS.
//
// The staleness guard had already demoted the undated claim to unknown, so trvl
// was not asserting it. That is the guard working: it converted a wrong answer
// into an honest absence while nobody was looking, and the absence is what
// prompted someone to go and read the page.
func TestResolveCheckedBagAustrian(t *testing.T) {
	os := ResolveCheckedBag(nil, "OS", nil)
	if os.HasBag() {
		t.Error("Austrian's cheapest intercontinental brand carries no checked bag")
	}
	if !os.LacksBag() {
		t.Errorf("this is now a cited verdict, not an absence of evidence: %+v", os)
	}
	if os.Source != models.BagSourceTableSourced {
		t.Errorf("Source = %q, want table_sourced — read off Austrian's own calculator", os.Source)
	}
	if os.Currency != "EUR" || os.AmountMin != 65 || os.AmountMax != 110 {
		t.Errorf("fee = %s %v-%v, want EUR 65-110 (online floor to counter plus the stated surcharge)",
			os.Currency, os.AmountMin, os.AmountMax)
	}
}

// TestResolveCheckedBagAmerican covers the carrier whose answer depends on
// region so strongly that it flips: American gives Basic Economy a free checked
// bag to Asia, Qatar and Australia, charges USD 70 to South America and USD 85
// across the Atlantic.
//
// The Europe figure is encoded because American's own table applies that row to
// anything "connecting via Europe to another destination", which is nearly
// every itinerary a Europe-origin search surfaces. It is the clearest argument
// yet for keying this table by (carrier, region) rather than by carrier.
func TestResolveCheckedBagAmerican(t *testing.T) {
	aa := ResolveCheckedBag(nil, "AA", nil)
	if aa.HasBag() {
		t.Error("American's Basic Economy pays for the first bag on Europe routes")
	}
	if aa.Source != models.BagSourceTableSourced {
		t.Errorf("Source = %q, want table_sourced", aa.Source)
	}
	if aa.Currency != "USD" || aa.AmountMin != 85 || aa.AmountMax != 85 {
		t.Errorf("fee = %s %v-%v, want the single published USD 85", aa.Currency, aa.AmountMin, aa.AmountMax)
	}

	// A provider verdict still overrides the table, which is what rescues the
	// regions where American does include a bag.
	one := 1
	if got := ResolveCheckedBag(&one, "AA", nil); !got.HasBag() || got.Source != models.BagSourceProvider {
		t.Errorf("hard provider data must win over a region-averaged table entry: %+v", got)
	}
}

// TestResolveCheckedBagLufthansaGroup pins the correction that cost the most.
//
// Lufthansa's entry asserted an included bag on the strength of an inference —
// its citation literally ended "INFERRED: no per-brand row published" — and
// explained away the zero-bag Economy Light brand as "confined to
// Scandinavia-US routes". Lufthansa's own baggage calculator, run on FRA-LIM,
// returns Economy Light with a carry-on and a personal item and prices a first
// checked bag as an extra.
//
// This one was worse than the uncited claims elsewhere in the table. Those
// carry no date, so the staleness guard demotes them to unknown and they merely
// cost results. This one was dated and cited, so it PASSED a bag filter and
// reported its all-in as the bare fare: every Lufthansa fare understated by a
// bag, in the direction a ratcheting price cache pins and never corrects.
func TestResolveCheckedBagLufthansaGroup(t *testing.T) {
	for _, tc := range []struct {
		airline, currency string
		min, max          float64
		why               string
	}{
		{"LH", "USD", 75, 120, "Lufthansa Economy Light on FRA-LIM carries none"},
		{"OS", "EUR", 65, 110, "Austrian Economy Light on FRA-LIM carries none"},
		{"LX", "EUR", 70, 105, "SWISS Light was already known to carry none"},
	} {
		t.Run(tc.airline, func(t *testing.T) {
			got := ResolveCheckedBag(nil, tc.airline, nil)
			if !got.LacksBag() {
				t.Fatalf("%s: %s, got %+v", tc.airline, tc.why, got)
			}
			if got.Source != models.BagSourceTableSourced {
				t.Errorf("Source = %q, want table_sourced", got.Source)
			}
			if got.Currency != tc.currency || got.AmountMin != tc.min || got.AmountMax != tc.max {
				t.Errorf("fee = %s %v-%v, want %s %v-%v", got.Currency, got.AmountMin, got.AmountMax, tc.currency, tc.min, tc.max)
			}
		})
	}
}

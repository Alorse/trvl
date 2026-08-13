# TODO

Open work, ordered by measured impact. Each item says what is wrong, why it
matters, and how to tell when it is fixed.

Context: trvl resolves a flight's checked-bag situation from the best evidence
available and records which evidence that was — provider payload, a cited table
entry, an uncited one, a frequent-flyer entitlement, or nothing. See
`internal/baggage/resolve.go`. The airline table is a fallback for the routes
where Google publishes no allowance, which is every European route tested.

**Baseline, measured 2026-08-13** across ZRH–HND, MUC–LIM, FRA–SJO and MAD–BOG,
477 priced flights:

| | count | share |
|---|---|---|
| No bag verdict at all (`unknown`) | 81 | 16% |
| No all-in total | 126 | 26% |
| …of which the fee exists but the currency does not match the fare | 45 | 9% |

Re-run that sweep after any change here; it is the only honest measure of
whether the table is getting better.

---

## 1. Convert bag fees into the fare's currency

**Biggest single cause of a missing all-in: 45 of the 477 flights.** British
Airways alone accounts for 18, United 10, LATAM 8, Air Canada 6, Etihad 3.

These airlines have a perfectly good fee — BA in GBP, UA and LA in USD — but the
fare is quoted in EUR, and `allInRange` refuses to add across currencies rather
than invent a rate. Refusing is right; having no rate is the gap.

The consequence is the one the whole baggage effort exists to prevent: a flight
with no all-in drops out of price comparison, and a downstream selection can
substitute a much more expensive flight in its place.

There is an FX cache at `internal/providers/fx.go` (Frankfurter API, daily ECB
rates, hardcoded fallbacks) but it is unexported and does network I/O from a
hotels package. Options: export it, or store a rate with its own date alongside
the fee — which matches how the rest of this data records provenance.

**Done when:** the "currency does not match" row above is zero, and a converted
figure carries the rate and the date it was taken.

## 2. Cite the six airlines whose claims have expired

`OS` Austrian · `LO` LOT · `SK` SAS · `AZ` ITA · `TP` TAP · `SQ` Singapore

Together **33 of the 81 unknowns** — Austrian 13, SAS 7, TAP 7, LOT 6. Each
claims one included bag with no source behind it, and since an undated positive
claim never counts as fresh, all six now resolve to `unknown` and are dropped by
`--require-checked-bag`.

This is deliberate. The same uncited "includes one bag" proved wrong for Iberia,
KLM, British Airways and SWISS — all four sell a long-haul brand carrying none.
But it costs real results until the figures are sourced.

**Done when:** each has `CheckedSource` and `CheckedVerified` in
`internal/baggage/baggage.go`, read off the airline's own page.

## 3. Add the five airlines that close most of the coverage gap

`DL` Delta · `VL` · `AA` American · `SN` Brussels · `AM` Aeroméxico

**41 of the 81 unknowns.** Adding these plus item 2 would take unknowns from 16%
to roughly 1%. The remaining tail (`4Y`, `EN`, `OU`, `AR`, `9B`) appears once or
twice each.

---

## What we learned about sourcing this data

Worth reading before the next pass — it cost a day to work out.

**Ask for the CHEAPEST long-haul economy brand, not the standard one.** That is
the fare a search surfaces. "Airline X includes a bag" has now proved false for
thirteen carriers whose standard brand includes one: Air France Light, Finnair
Light, Iberia Basic, KLM Light, British Airways Basic, SWISS Light, Air Canada
Basic, Etihad Basic, United Basic Economy, China Eastern Basic, Air Europa LITE,
LATAM Basic and Avianca Basic.

**Where a carrier publishes no static table, its calculator usually does.** SWISS
and Turkish both hide a fare selector on the calculator's *results* screen, not
the input form — which is why one pass wrongly concluded they published nothing.

**"Publishes no fixed fee" must mean the airline says so, not that an automated
pass could not reach it.** Two independent agents concluded Air Europa published
nothing; it publishes EUR 120–140, reachable by hand through a client-side route
selector neither could drive. Two agents failing the same way is one failure,
not two confirmations. Three agents concluded Air France published nothing; a
fourth found its figures on a `noindex` legal page.

**Sources that do not survive a click are worthless.** One pass returned a Google
search URL as a citation and bare domains for the rest, presented derived numbers
as read, and reported figures for Air Europa that match its *infant* baggage
charges — which another pass had explicitly flagged as a trap.

**Some airlines genuinely publish nothing usable.** Etihad prices extra baggage
by 5 kg band, not per bag. Multiplying a band by five would manufacture a
first-bag figure the airline never states, so no figure is stored. That is a
property of the airline, correctly recorded.

---

## Known limitations, recorded rather than fixed

**Air France's two pages disagree.** Its legal fees page publishes EUR 70–110;
its baggage help page says rates are shown at booking. Both are current. The
stored figure may be a reference or airport rate rather than the online one.

**A single integer per airline cannot be right.** Qatar switches between piece
and weight concepts by continent; Turkish publishes EcoFly figures for
Europe–Asia but not Europe–Americas; Iberia has five fee zones; SWISS's Basic
brand exists only on European routes; China Eastern publishes six route bands;
Air India differs by direction of travel. The table can at best be right about
the most common case. A `(carrier, brand)` or `(carrier, brand, region)` key
would be maintainable in a way `(carrier)` is not, and the brand names are
stable and published.

**The airline is read from the first leg.** A multi-city itinerary flown out on
one carrier and back on another is charged twice at the first carrier's rate.
Closer to the truth than charging once, but an approximation.

**Claims expire on a timer that needs someone to notice.** Positive claims lapse
to `table_unsourced` after ~9 months and stop being asserted after ~18
(`bagClaimStaleAfter` / `bagClaimExpiresAfter` in `internal/baggage/resolve.go`).
Entries stamped `2026-08` start degrading around **May 2027** and stop asserting
around **February 2028**. The table decaying toward "unknown" rather than toward
a false claim is the safety net working — but it wants a calendar reminder, or a
probe test that fails when an entry is within 60 days of lapsing.

---

## Providers

### Why does SerpApi return far fewer results than Google?

2–12 results per date via SerpApi against 13–16 via Google on the same routes.
With fewer candidates the cheapest bagged option may simply be absent, which
biases a downstream price cache upward, silently.

This is about result *count*, not baggage data: since SerpApi switched to
`output=html` it returns Google's raw payload, so bag figures are identical
whichever provider answers.

### Reconsider the reduced Google retry budget

`searchFlightsCore` drops Google's anti-bot retries from 3 to 1 when SerpApi is
available, which mechanically pushes traffic to SerpApi. That trade-off was made
when SerpApi was the cheaper fallback; combined with the result-count gap it may
now cost more than it saves.

---

## Pre-existing test failures

Three tests fail on `main` and predate the baggage work — each confirmed by
stashing the changes and re-running:

- `TestParseFlightLocations_CityNames` — `ParseFlightLocations("Paris")` returns
  `[BVA CDG ORY]`, the test wants `[CDG ORY]`. Someone added Beauvais; the test
  was not updated. Decide which is correct.
- `TestParseFlightLocations_Mixed` — same cause.
- `TestSearchDigitransit_MockHappyPath2` — in `internal/ground`, unrelated.

---

## Known-wrong data elsewhere

Found while sourcing the baggage table, not yet fixed:

- `internal/baggage/alliance.go` lists **SAS under Star Alliance**. SAS moved to
  SkyTeam on 1 September 2024, so frequent-flyer benefit resolution is wrong for
  every SAS flight.
- The same file lists **Czech Airlines**, which ceased operations in October 2024.
- Alliance rosters are incomplete: SkyTeam is missing Virgin Atlantic, Star is
  missing Aegean, Avianca, Copa and EVA, oneworld is missing Alaska. An airline
  absent from its alliance silently loses its frequent-flyer bag waiver.
- The same alliance benefit data exists twice, independently, in
  `internal/baggage/alliance.go` and `internal/flights/loyalty.go`, with
  different shapes and different tier spellings.
- `internal/baggage/baggage.go` stores `F9` Frontier's **carry-on** fee, in USD,
  in `CheckedFee` — a field documented as EUR and as the checked-bag price.

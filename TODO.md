# TODO

Open work, ordered by how much it costs to leave undone. Each item says what is
wrong, why it matters, and how to tell when it is fixed.

Context for everything under "Checked baggage": trvl resolves a flight's
checked-bag situation from the best evidence available and records which
evidence that was — provider payload, a cited table entry, an uncited one, a
frequent-flyer entitlement, or nothing at all. See `internal/baggage/resolve.go`.
The airline table is a fallback for the routes where Google publishes no
allowance, which is every European route tested.

---

## Checked baggage

### Cite the six airlines that currently report as unknown

`OS` Austrian · `LO` LOT · `SK` SAS · `AZ` ITA · `TP` TAP · `SQ` Singapore

Each claims one included bag with no source behind it. Since an undated positive
claim never counts as fresh, all six now resolve to `unknown` and are dropped by
`--require-checked-bag`. Austrian alone appeared 4 times in a single Munich–Lima
search.

This is deliberate, not a bug: the same uncited "includes one bag" proved wrong
for Iberia, KLM, British Airways and SWISS. But it costs real results until the
figures are sourced.

**Done when:** each has `CheckedSource` and `CheckedVerified` in
`internal/baggage/baggage.go`, read off the airline's own page, and
`ResolveCheckedBag` returns `table_sourced` for it.

**Method that worked:** get the figure for the airline's *cheapest* long-haul
economy brand, not its standard one — that is the fare a search surfaces. Where a
carrier publishes no static table, its baggage calculator usually does; SWISS and
Turkish both hide a fare selector on the calculator's *results* screen, which is
why an earlier pass wrongly concluded they published nothing.

### Add the five airlines that close the coverage gap

`DL` Delta · `VL` (as seen on Europe–Latin America) · `AA` American ·
`SN` Brussels · `AM` Aeroméxico

Measured across four live routes (ZRH–HND, MUC–LIM, FRA–SJO, MAD–BOG, 485
flights): 91% of flights resolve against the table, 9% do not. These five are 39
of the 46 uncovered flights — about 85% of the remaining gap. Adding them takes
coverage to roughly 98%. The rest of the tail (`4Y`, `EN`, `OU`, `AR`, `9B`)
appears once each.

**Done when:** a re-run of that sweep reports ≥97% resolved.

### Re-verify claims before they expire

Positive claims lapse to `table_unsourced` after ~9 months and stop being
asserted after ~18 (`bagClaimStaleAfter` / `bagClaimExpiresAfter` in
`internal/baggage/resolve.go`). Entries stamped `2026-08` therefore start
degrading around **May 2027** and stop asserting around **February 2028**.

This is the safety net working — the table decays toward "unknown" rather than
toward a false claim — but it needs someone to notice. Worth a calendar reminder
or a probe test that fails when any entry is within 60 days of lapsing.

### Consider keying allowances to `(carrier, brand, region)`

A single integer per airline is structurally incapable of being right. Qatar
switches between piece and weight concepts depending on continent; Turkish
publishes EcoFly figures for Europe–Asia but not Europe–Americas; Iberia has five
fee zones; SWISS's Basic brand exists only on European routes; China Eastern
publishes six route bands; Air India differs by direction of travel. Today's
table can at best be right about the most common case.

The fare brand does travel in the search result, and the brand names are stable
and published, so a `(carrier, brand)` or `(carrier, brand, region)` key is
maintainable in a way `(carrier)` is not.

---

## Providers

### Understand why SerpApi returns far fewer results than Google

Observed 2–12 results per date via SerpApi against 13–16 via Google on the same
routes. With fewer candidates, the cheapest bagged option may simply be absent
from the set — which biases a downstream price cache upward, silently and
systematically.

Worth checking whether it is a `sort_by` we do not send, a filter applied
differently, or simply what SerpApi returns.

**Note:** this is about result *count*, not baggage data. Since SerpApi switched
to `output=html` it returns Google's raw payload, so the bag figures are
identical whichever provider answers.

### Reconsider the reduced Google retry budget

`searchFlightsCore` drops Google's anti-bot retries from 3 to 1 when SerpApi is
available, which mechanically pushes traffic to SerpApi. That trade-off was made
when SerpApi was the cheaper fallback; combined with the result-count gap above
it may now cost more than it saves.

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

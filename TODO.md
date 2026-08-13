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

## 1. Cover the Asian carriers — the whole remaining gap

Surfaced by a question from Sebas in the MyVentura daily: had the price cache
ever been checked on internal/domestic routes? It had not, and the answer is
bad. Measured 2026-08-13 on the two routes the internal cron actually runs:

| route | priced | `unknown` | cheapest flight |
|---|---|---|---|
| KIX–HKG one way | 123 | 42 (34%) | Peach 115 EUR, unknown |
| KIX–ICN one way | 61 | 41 (**67%**) | Air Busan 113 EUR, unknown |
| KIX–HKG round trip | 71 | 27 (38%) | Peach 222 EUR, unknown |

Two things the framing got wrong on the way in, both worth keeping straight:

**It is not a one-way bug.** The same route as a round trip drops the same
carriers. The variable is the region, not the trip type — the table covers
European long-haul and four Asian full-service carriers, and these routes are
flown by neither.

**It is not five LCCs.** `7C` Jeju, `UO` HK Express, `HX` Hong Kong Airlines,
`MM` Peach, `TW` T'Way, `BX` Air Busan, `LJ` Jin Air, `ZE` Eastar, `HB` Greater
Bay, `SC` Shandong, `CI` China Airlines, `PR` Philippine, `MH` Malaysia. The
largest single contributor is China Airlines, a full-service carrier.

`UO`, `7C`, `MM`, `BX`, `TW` and `HX` are now in the table, all cited from the
airline's own page or fee PDF. That took flights with no all-in on the internal
routes from 37 to 16, and **the cheapest flight is now priced on every route
measured** — which was the whole point.

Still open, by measured damage: `LJ` Jin Air 7, `ZE` Eastar 4, `HB` Greater Bay
2, `RS` Air Seoul 2, `GK` Jetstar Japan 1. Then `SC`, `CI`, `PR`, `MH` on the
wider sweep. None of them holds the cheapest flight on any route measured.

**`LJ` Jin Air — do not restart from scratch.** Its excess-baggage page has been
read and is *not* the one needed. It gives pre-purchase and airport rates only
(Japan/Shanghai KRW 45,000, Hong Kong/Taiwan KRW 55,000, both **per 5 kg**) and
never states the free allowance, which is the half that decides the verdict.
Two consequences:

- The missing page is the free-allowance one (`무료 수하물` / "Free Baggage
  Allowance"), specifically the international economy row.
- Even with it, no fee should be stored unless Jin Air publishes a per-bag
  price. A 5 kg band is the Etihad case: multiplying it up would manufacture a
  first-bag figure the airline never states. So if the allowance turns out to be
  15 kg the entry needs no fee at all and is complete; if it turns out to be
  zero, the entry is a cited verdict with no figure and the flight still has no
  all-in.

**Do not assume LCC means no bag.** Air Busan includes 15 kg in regular
international economy — the opposite of the assumption that opened this
investigation. Adding a fee there would have inflated the cheapest flight on
the route and caused the same substitution from the other side.

What makes this the priority is not the count but the *position*: the cheapest
flight was `unknown` in every measurement taken. Twenty-four correctly resolved
Korean Air results do not compensate for the one flight the cache will actually
store being the one we cannot price. **Rank this work by whether the cheapest
flight is covered, not by how many results are.**

Downstream the failure is concrete: with no all-in the cron discards the cheap
flight and stores a full-service one instead — 115 EUR becoming 341, 113
becoming 282. That is not a bag being added, it is a different airline being
substituted.

**Blocked on reading, not on code.** Conversion (below) is done, so a fee in
yen or won is now usable. Getting the figures is the work: `flypeach.com`,
`jejuair.net`, `hkexpress.com` and `support.flypeach.com` variously return 404,
time out, exceed the redirect limit, or fail the TLS handshake to an automated
fetch. Search snippets and a Scribd copy of an HK Express price table are
available and are *not* good enough — see the sourcing rules below. This needs
a browser session or a manual read.

**Done when:** each carrier has `CheckedIncluded`, `CheckedSource` and
`CheckedVerified` read off the airline's own page, and the two routes above
show the cheapest flight priced.

## 2. Persist FX rates across runs

Every `trvl` invocation starts with an empty rate cache and makes three HTTP
calls to Frankfurter. When those fail the fallback covers only EUR, USD and
GBP — so HKD, JPY and KRW lose their rate, and the flights priced in them
silently lose their all-in.

The failure mode is nasty because it is selective and quiet: USD and GBP keep
resolving from the hardcoded fallbacks, so only the Asian carriers stop
producing a total, and downstream that reads as "cannot price this flight"
rather than "could not reach the ECB".

The acute cause is fixed: the client pointed at `api.frankfurter.app`, which now
answers 301 to `api.frankfurter.dev/v1`. Go followed the redirect, so it worked
— at double the round trips, and under a burst that exhausted the 5s timeout.
Pointing straight at the versioned host made JPY conversion reproducible across
runs where it had been intermittent.

That removes the trigger, not the fragility: a network blip still costs the same
currencies. Writing the rates under `~/.trvl/` with the date already attached
would fix it properly and without weakening provenance — a rate from yesterday
is still a real published rate, and `conversion_as_of` already says which day.

**Done when:** a run with no network still converts HKD, JPY and KRW from the
last stored rates, and says how old they are.

## 3. Cite the six airlines whose claims have expired

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

## 4. Add the five airlines that close most of the coverage gap

`DL` Delta · `VL` · `AA` American · `SN` Brussels · `AM` Aeroméxico

**41 of the 81 unknowns.** Adding these plus item 2 would take unknowns from 16%
to roughly 1%. The remaining tail (`4Y`, `EN`, `OU`, `AR`, `9B`) appears once or
twice each.

---

## Done

**Bag fees convert into the fare's currency** (`internal/fx`). Was the largest
cause of a missing all-in — 45 of 477 flights, BA in GBP, UA and LATAM in USD
against EUR fares. `allInRange` now converts rather than dropping the flight,
and records the rate and its ECB publication date on the estimate
(`conversion_rate`, `conversion_as_of`) so a derived figure can be audited. The
published fee stays in the airline's own currency. Where no rate exists for the
pair the total is still withheld — converting is not licence to invent.

The FX cache moved out of `internal/providers` into its own package so both
hotels and flights can reach it, and it now inverts the ECB's EUR table, which
is what makes JPY, KRW and HKD reachable at all. Measured after: the European
long-haul routes went to zero flights without an all-in.

**`included` is three-valued** (`*bool`, JSON `null`). trvl used to emit
`"included": false` alongside `"source": "unknown"`, collapsing "this fare
carries no checked bag" into "we do not know what this fare carries". The
second is not the first, and any consumer reading `included` without also
reading `source` was being told something trvl cannot support. Raised by the
downstream cache's maintainer.

Note for consumers: `included` may now be `null`. Nothing that reads it as
falsy changes behaviour; anything that distinguishes the two states now can.
`HasBag()` and `LacksBag()` are deliberately not each other's negation.

---

## What we learned about sourcing this data

Worth reading before the next pass — it cost a day to work out.

**Ask for the CHEAPEST long-haul economy brand, not the standard one.** That is
the fare a search surfaces. "Airline X includes a bag" has now proved false for
thirteen carriers whose standard brand includes one: Air France Light, Finnair
Light, Iberia Basic, KLM Light, British Airways Basic, SWISS Light, Air Canada
Basic, Etihad Basic, United Basic Economy, China Eastern Basic, Air Europa LITE,
LATAM Basic and Avianca Basic.

**A brand ladder and a ticket kind are not the same thing, and they decide the
answer in opposite directions.** T'Way publishes Event, Smart, Normal and
Business as rungs of one ladder, so Event is the cheapest brand and the rule
above applies unchanged: it carries no allowance. Air Busan's "Special/Event
Flights" instead names a *class of flight* alongside "Economy/Regular Airfare",
which carries 15 kg. Reading both the same way would have been wrong about one
of them whichever way it went. Ask what the label is a member of — a set of
fares, or a set of flights.

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

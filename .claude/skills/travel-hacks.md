---
name: travel-hacks
description: Advanced travel optimization. Hidden city, nearby airports, throw-away returns, hotel splits, positioning flights, timing hacks. Applies automatically when planning trips.
triggers:
  - cheap flight
  - cheapest
  - optimize
  - save money
  - travel hack
  - hidden city
  - nearby airport
  - positioning flight
  - split ticket
  - throw-away
  - hotel split
  - budget trip
  - best price
  - trip planning
  - find deals
allowed-tools:
  - Bash
  - mcp__gateway__gateway_invoke
  - mcp__gateway__gateway_search_tools
---

# Travel Hacks — Optimization Playbook

When the user asks for cheap flights, optimized trips, or travel deals, apply these strategies AUTOMATICALLY using trvl's MCP tools.

## FIRST: Understand the Trip Before Optimizing

Before running any optimization, make sure you understand:
1. **What's fixed vs flexible?** — fixed dates = optimize price for those dates. Flexible = optimize dates too.
2. **What are the constraints?** — "must be in Barcelona June 15-18" is different from "want to visit Barcelona sometime in summer"
3. **What's the priority?** — cheapest possible? best value? minimum travel time? comfort?
4. **How hacky are they willing to go?** — hidden city and throw-away returns carry risks. Some travelers are fine with it, others want conventional bookings only.

Ask ONE focused question if critical info is missing, then start searching. Show initial results fast, then offer optimizations: "I found flights from €235. Want me to check nearby airports and flexible dates? That often saves 20-40%."

## Strategy 1: Nearby Airport Arbitrage (ALWAYS CHECK)

Search from ALL airports within reasonable distance. Many cities have 2-5 airports with wildly different prices.

**How**: Run `explore_destinations` from each nearby airport, or `search_flights` for the same route from each.

Common clusters:
- Helsinki: HEL (main), TMP (Tampere, 2h train), TKU (Turku, 2h train)
- London: LHR, LGW, STN, LTN, SEN
- Paris: CDG, ORY, BVA (Beauvais)
- New York: JFK, EWR, LGA
- Tokyo: NRT, HND
- Milan: MXP, LIN, BGY (Bergamo/Ryanair)
- Stockholm: ARN, BMA, NYO, VST
- Berlin: BER (but nearby: SXF was, now LEJ Leipzig 2h train)
- Barcelona: BCN (but nearby: GRO Girona for Ryanair)

**Say**: "Let me check nearby airports too..." then search 2-3 alternatives.

## Strategy 2: Throw-Away Return Detection (CHECK ON INTERNATIONAL)

One-way often costs MORE than round-trip on legacy carriers. Check both.

**How**:
1. `search_flights(origin, dest, date)` — one-way price
2. `search_flights(origin, dest, date, return_date=date+14d)` — round-trip price
3. If round-trip < one-way: "Round-trip is €X cheaper than one-way. You could book round-trip and skip the return."

Especially effective on: Lufthansa, BA, AF/KLM, transatlantic, business class.

## Strategy 3: Split Ticketing (ALWAYS COMPARE)

Two separate one-ways on different airlines can beat a round-trip on one airline.

**How**: Always search each direction independently:
1. `search_flights(A, B, outbound_date)` — cheapest outbound
2. `search_flights(B, A, return_date)` — cheapest return
3. Compare sum vs round-trip price

Especially effective when: low-cost carriers serve one direction, different airlines dominate each direction, asymmetric demand.

## Strategy 4: Flexible Date Optimization (ALWAYS OFFER)

±3 days can save 30-50%. Midweek (Tue-Wed) is almost always cheaper than Fri-Sun.

**How**: `search_dates(origin, dest, start_date-3, end_date+3)` or `search_price_grid` for departure×return matrix.

**Say**: "Flying Tuesday instead of Friday saves €180" with the specific data.

## Strategy 5: Positioning Flights

Cheap flight to a hub → cheap long-haul from the hub. Total < direct from home.

**How**:
1. `explore_destinations(home_airport)` — find cheap routes to major hubs
2. For each hub: `search_flights(hub, final_destination, date)` 
3. Compare: positioning + long-haul vs direct

Key hubs: IST (Turkish, cheap from EU), DOH (Qatar), DXB (Emirates), AMS (KLM), FRA (Lufthansa), HEL (Finnair Asia), WAW (LOT, cheap from Nordics).

## Strategy 6: Hidden City Detection (SUGGEST WITH WARNINGS)

A→C via B can be cheaper than A→B direct. Check connecting flights.

**How**:
1. User wants A→B. `search_flights(A, B, date)` — get direct price
2. Look at which airports are common connections (from leg data in results)
3. `search_flights(A, C, date)` for C beyond B (e.g., B=FRA, try C=MUC, C=VIE)
4. If A→C-via-B < A→B: flag it

**ALWAYS warn about risks**: "⚠️ Hidden city ticketing: carry-on only, airline may enforce against this, never on round-trips."

## Strategy 7: Hotel Split Optimization (CHECK FOR 4+ NIGHT STAYS)

Different hotels have different pricing for weekdays vs weekends. Splitting a stay can save significantly.

**How**: For a 5-night stay (Mon-Sat):
1. `search_hotels(location, checkin=Mon, checkout=Thu)` — weekday rates
2. `search_hotels(location, checkin=Thu, checkout=Sat)` — weekend rates
3. Compare cheapest combos vs single 5-night stay

Also: boutique/business hotel on weekdays (low demand), hostel/apartment on weekends.

## Strategy 8: Airline-Specific Tricks

**KLM/Air France**: Often cheaper to add a short European connection than fly direct. Search with `max_stops=one_stop` alongside `nonstop`.

**Icelandair**: Free stopover in Reykjavik on transatlantic flights. Search HEL→KEF→JFK and check if price matches HEL→JFK.

**Turkish Airlines**: IST connections are often the cheapest Europe↔Asia/Africa routing. Always check.

**Norwegian/Ryanair/Wizzair**: One-way pricing = exactly half round-trip. No throw-away benefit. But nearby airports (BGY, GRO, BVA) can be 50% cheaper.

**Qatar/Emirates**: Positioning to DOH/DXB for cheap onward connections. Check `search_flights(home, DOH, date)` then `search_flights(DOH, final_dest, date+1)`.

## Strategy 9: Open-Jaw Routing

Fly into city A, travel overland, fly out of city B. Often same price as round-trip but saves backtracking.

**How**: 
1. `search_flights(home, A, outbound_date)` — inbound
2. `search_flights(B, home, return_date)` — return from different city
3. Sum and compare vs round-trip to A

Great for: Italy (fly into Rome, train to Florence, fly out of Florence), Japan (into Tokyo, out of Osaka), Spain (into Barcelona, out of Madrid).

## Strategy 10: Train+Flight Combo

For European travel, a train to a cheaper departure city can save more than the train costs.

**How**:
1. Check nearby airports with `explore_destinations`
2. If a city 2-3h by train has significantly cheaper flights, recommend the combo
3. Reference: Helsinki→Tampere (2h, €15), Paris→Brussels (1.5h, €30), London→Paris (2.5h, €50)

## Strategy 11: Airline Service & Loyalty Awareness

When comparing flights, price alone doesn't tell the full story. Factor in:

**Luggage allowances vary massively:**
- **Full-service** (Finnair, Lufthansa, BA, SAS): Usually 23kg checked bag + 8kg cabin included
- **Low-cost** (Ryanair, Wizzair, easyJet): Cabin bag only. Checked bag = +€20-50/direction. Priority boarding for overhead cabin bag = +€6-10. Total extras can add €40-100 round-trip.
- **Norwegian**: Large cabin bag included on most fares, checked bag extra (~€15-30)
- **Turkish Airlines**: 23kg checked included even on economy, generous cabin allowance

**When to flag**: If the user mentions luggage or is comparing low-cost vs full-service, add the luggage cost to the comparison: "Ryanair is €73 + €46 bags = €119 total vs Finnair at €113 all-in."

**Frequent flyer status benefits** (ask if the user has status):
- **Star Alliance Gold** (Finnair Plus, SAS EuroBonus, etc.): lounge access, priority boarding, extra baggage, same-day flight changes on some airlines
- **oneworld Sapphire/Emerald** (Finnair, BA, Iberia): lounge, priority, extra bag
- **SkyTeam Elite Plus** (KLM, AF): lounge, priority, extra bag
- Status affects the TRUE cost — a slightly more expensive flight on your status airline saves lounge fees (€30-50), bags, and seat selection

**Ask about**: "Do you have frequent flyer status with any airline/alliance? That might affect which flights give best value."

**Fare class awareness:**
- Business class one-ways are often only 20-30% more than economy on short-haul (1-2h) but include lounge, bags, seat, flexibility
- Premium economy on long-haul (8h+) is often 40-60% more than economy but includes: 2x baggage, better seat pitch, better food, lounge on some airlines
- Light/basic economy fares (Lufthansa Light, BA Basic): NO cabin bag overhead, NO seat selection, NO changes. Only viable for day trips with a small backpack.

## When Planning Any Trip, ALWAYS:

1. **Check nearby airports** (Strategy 1)
2. **Compare one-way vs round-trip** (Strategy 2)
3. **Compare split one-ways vs round-trip** (Strategy 3)
4. **Offer flexible dates** if not fixed (Strategy 4)
5. **For long-haul**: check positioning flights (Strategy 5)
6. **For 4+ night stays**: suggest hotel split (Strategy 7)
7. **Present total trip cost** using `calculate_trip_cost` or manual sum
8. **Include booking links** from the `booking_url` field in results

## What to Say to the User

Don't just dump search results. Say things like:
- "I found flights from €113 (Norwegian nonstop) but checking nearby airports saved €40 — Tampere has a Ryanair flight for €73."
- "One-way is €580 but round-trip is €420. You could book round-trip and skip the return to save €160."
- "Flying Wednesday instead of Friday saves €180 on this route."
- "Splitting your 6-night stay into 4 nights at Hotel A (€80/night weekday) + 2 nights at Hotel B (€65/night weekend) saves €90 total."
- "A positioning flight to Istanbul (€89) + Turkish Airlines to Tokyo (€340) = €429, vs €875 direct from Helsinki."

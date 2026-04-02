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

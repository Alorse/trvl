---
name: trvl
description: Search Google Flights and Hotels. Real-time prices, no API keys. Flights, cheapest dates, hotels, price comparison, explore destinations, price grids.
triggers:
  - flight
  - flights
  - hotel
  - hotels
  - travel
  - trip
  - airfare
  - booking
  - cheapest dates
  - when to fly
  - accommodation
  - where to stay
  - explore destinations
  - where to go
  - cheapest destinations
  - price grid
  - flexible dates
allowed-tools:
  - Bash
  - mcp__gateway__gateway_invoke
  - mcp__gateway__gateway_search_tools
---

# trvl — Google Flights + Hotels Search

You have access to real-time Google Flights and Google Hotels data via the `trvl` tool.

## BEFORE SEARCHING: Ask Clarifying Questions

Do NOT immediately search. First understand what the user needs. Ask about anything that's unclear or missing from this checklist:

**Essential (must know before searching):**
- Where from? Where to? (or "flexible/open to suggestions"?)
- When? Fixed dates or flexible? (±days? specific month? "whenever cheapest"?)
- How many travelers?

**Important (ask if not mentioned):**
- One-way or round-trip?
- Budget range? (helps filter and prioritize)
- Any schedule constraints? (must arrive by X, meeting on date Y, etc.)

**Nice to have (ask based on context):**
- Cabin class preference? (economy default, but ask for long-haul >6h)
- Hotel preferences? (stars, location, type — hotel vs apartment)
- Nonstop preference or OK with connections?
- Frequent flyer status? (Star Alliance Gold/oneworld/SkyTeam — affects airline preference and true cost via lounge access, extra bags, priority)
- Luggage needs? (critical for low-cost carrier comparison — Ryanair+bags can cost more than Finnair all-in)

**For multi-city / complex trips:**
- What cities need to be visited?
- Any fixed dates for specific cities? (conferences, events)
- Preferred order or optimize freely?
- How many nights per city?

**How to ask efficiently:**
- Bundle 2-3 questions, not a wall of 10
- Use the information already given — don't re-ask what's obvious
- If the user says "find me cheap flights to Barcelona next month" — you already know: destination=BCN, timeframe=next month, priority=cheap. Just ask: "Flying from Helsinki? How many travelers? Fixed dates or flexible within the month?"
- If enough info is given to start, start searching and ask follow-ups after showing initial results

**After initial results, offer refinements:**
- "Want me to check nearby airports too?"
- "Should I look at flexible dates? ±3 days could save €X"
- "Want me to search hotels there as well?"
- "Should I check positioning flights via a cheaper hub?"

## Via MCP Gateway (preferred)

```
gateway_invoke(server="trvl", tool="search_flights", arguments={...})
gateway_invoke(server="trvl", tool="search_hotels", arguments={...})
```

### search_flights
Search flights between two airports.
```json
{"origin": "HEL", "destination": "NRT", "departure_date": "2026-06-15"}
```
Optional: `return_date`, `cabin_class` (economy/premium_economy/business/first), `max_stops` (any/nonstop/one_stop/two_plus), `sort_by` (cheapest/duration/departure/arrival)

### search_dates
Find cheapest dates to fly across a range. Uses CalendarGraph API for fast single-request results.
```json
{"origin": "HEL", "destination": "NRT", "start_date": "2026-06-01", "end_date": "2026-06-30"}
```
Optional: `trip_duration` (int days), `is_round_trip` (bool)

### explore_destinations
Discover cheapest flight destinations from an airport. Great for "where should I go?" queries.
```json
{"origin": "HEL"}
```
Optional: `start_date`, `end_date`, `trip_type` (round-trip/one-way), `max_stops` (-1=any, 0=nonstop)

### search_price_grid
Get a 2D price matrix of departure x return date combinations.
```json
{"origin": "HEL", "destination": "NRT", "depart_from": "2026-07-01", "depart_to": "2026-07-07", "return_from": "2026-07-08", "return_to": "2026-07-14"}
```

### search_hotels
Search hotels by location.
```json
{"location": "Tokyo", "check_in": "2026-06-15", "check_out": "2026-06-18"}
```
Optional: `guests` (int), `stars` (1-5 minimum), `sort` (price/rating)

### hotel_prices
Compare booking provider prices for a specific hotel.
```json
{"hotel_id": "<from search_hotels results>", "check_in": "2026-06-15", "check_out": "2026-06-18"}
```

## Via CLI (fallback)

```bash
trvl flights HEL NRT 2026-06-15 --format json
trvl hotels "Tokyo" --checkin 2026-06-15 --checkout 2026-06-18 --format json
trvl dates HEL NRT --from 2026-06-01 --to 2026-06-30 --format json
trvl prices "<hotel_id>" --checkin 2026-06-15 --checkout 2026-06-18 --format json
trvl explore HEL --format json
trvl grid HEL NRT --depart-from 2026-07-01 --depart-to 2026-07-07 --return-from 2026-07-08 --return-to 2026-07-14 --format json
```

## Response Format

All tools return structured JSON with:
- `success` (bool), `count` (int)
- `flights[]` or `hotels[]` or `destinations[]` or `cells[]` with full details
- `suggestions[]` for follow-up searches
- `booking_url` on each result for direct Google links

## Tips

- Use IATA airport codes: HEL (Helsinki), NRT (Tokyo Narita), JFK (New York), LHR (London), CDG (Paris), BCN (Barcelona), BKK (Bangkok), SIN (Singapore), DXB (Dubai), LAX (Los Angeles)
- Prices reflect the user's IP geolocation currency
- For trip planning: search flights first, then hotels at the destination
- For budget planning: use search_dates to find the cheapest departure day
- For flexible destinations: use explore_destinations to find cheapest options
- For flexible dates on both legs: use search_price_grid for a 2D price matrix

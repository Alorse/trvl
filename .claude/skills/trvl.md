---
name: trvl
description: Search Google Flights and Hotels. Real-time prices, no API keys. Flights, cheapest dates, hotels, price comparison.
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
allowed-tools:
  - Bash
  - mcp__gateway__gateway_invoke
  - mcp__gateway__gateway_search_tools
---

# trvl — Google Flights + Hotels Search

You have access to real-time Google Flights and Google Hotels data via the `trvl` tool.

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
Find cheapest dates to fly across a range.
```json
{"origin": "HEL", "destination": "NRT", "start_date": "2026-06-01", "end_date": "2026-06-30"}
```
Optional: `trip_duration` (int days), `is_round_trip` (bool)

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
```

## Response Format

All tools return structured JSON with:
- `success` (bool), `count` (int)
- `flights[]` or `hotels[]` with full details
- `suggestions[]` for follow-up searches
- `booking_url` on each result for direct Google links

## Tips

- Use IATA airport codes: HEL (Helsinki), NRT (Tokyo Narita), JFK (New York), LHR (London), CDG (Paris), BCN (Barcelona), BKK (Bangkok), SIN (Singapore), DXB (Dubai), LAX (Los Angeles)
- Prices reflect the user's IP geolocation currency
- For trip planning: search flights first, then hotels at the destination
- For budget planning: use search_dates to find the cheapest departure day

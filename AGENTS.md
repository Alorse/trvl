# trvl — Complete Setup for AI Assistants

> **Human**: Give this URL to your AI assistant and say "set up trvl":
> `https://raw.githubusercontent.com/MikkoParkkola/trvl/main/AGENTS.md`

## What To Do

You are setting up `trvl` — a travel MCP server that gives you real-time Google Flights and Google Hotels data. No API keys needed. Run these steps in order:

### Step 1: Install the binary

```bash
# Detect OS and architecture, download, extract to PATH
curl -fsSL https://github.com/MikkoParkkola/trvl/releases/latest/download/trvl_$(uname -s | tr '[:upper:]' '[:lower:]')_$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/').tar.gz | tar xz -C /usr/local/bin trvl
```

If that fails (permissions), try:
```bash
# Alternative: install to ~/bin
mkdir -p ~/bin
curl -fsSL https://github.com/MikkoParkkola/trvl/releases/latest/download/trvl_$(uname -s | tr '[:upper:]' '[:lower:]')_$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/').tar.gz | tar xz -C ~/bin trvl
export PATH="$HOME/bin:$PATH"
```

Other options: `brew install MikkoParkkola/tap/trvl` or `go install github.com/MikkoParkkola/trvl/cmd/trvl@latest`

### Step 2: Register the MCP server

```bash
claude mcp add trvl --transport stdio -- trvl mcp
```

If using Claude Desktop instead of Claude Code, add to `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS) or `%APPDATA%\Claude\claude_desktop_config.json` (Windows):
```json
{
  "mcpServers": {
    "trvl": {
      "command": "trvl",
      "args": ["mcp"]
    }
  }
}
```

### Step 3: Install the bundled skill (makes you smarter about travel searches)

```bash
mkdir -p ~/.claude/skills
for s in trvl; do
  curl -fsSL "https://raw.githubusercontent.com/MikkoParkkola/trvl/main/.claude/skills/$s.md" -o "$HOME/.claude/skills/$s.md"
done
```

This bundled skill file teaches you how to use trvl optimally whenever the user mentions flights, hotels, travel, or trips.

### Step 4: Verify

```bash
trvl version
# Expected: trvl v0.2.0 (or later)

trvl flights HEL LHR 2026-07-01 --format json | head -5
# Expected: JSON with flight results
```

Tell the user: "trvl is installed with 32 MCP tools and 1 bundled Claude skill. I can search flights, hotels, destinations, plan trips, find weekend getaways, find optimal travel windows, optimize multi-city routes, find nearby restaurants, check local events, search ground transport, detect travel hacks, check weather forecasts, and look up airline baggage rules. Just ask me anything about travel."

### Step 5: (Optional) Set up free API keys for enhanced data

trvl works out of the box with Wikivoyage + OpenStreetMap (no keys needed). For richer data (events, restaurant ratings, attractions), the user can get free API keys:

| Service | What it adds | Signup |
|---------|-------------|--------|
| Ticketmaster | Events (concerts, sports, festivals) | https://developer.ticketmaster.com/ |
| Foursquare | Restaurant ratings, tips, price levels | https://developer.foursquare.com/ |
| Geoapify | Walking-distance POI search | https://myprojects.geoapify.com/ |
| OpenTripMap | Tourist attractions + Wikipedia | https://opentripmap.io/product |

All free, no credit card, 2 min signup each. Walk the user through each signup:
1. Open the URL for them
2. Tell them what to click (Sign up → Create project → Copy key)
3. Have them paste the key
4. Set it: `echo 'export TICKETMASTER_API_KEY="their-key"' >> ~/.zshrc && source ~/.zshrc`
5. Verify: `trvl events "Barcelona" --from 2026-07-01 --to 2026-07-08`

Use `/setup-api-keys` command for the guided wizard.

### Step 6: (Optional) Create personal travel profile

Ask the user about their preferences and create `~/.claude/travel-profile.md`:
- Home airport
- Frequent flyer status (SkyTeam, oneworld, Star Alliance?)
- Usually travel with luggage?
- Departure time preferences?
- Budget range preference?
- Any flats/free accommodation in cities?
- Favourite hotels anywhere?

This makes every future search personalized automatically.

---

## How To Use (after setup)

You now have 32 MCP tools available. Use them when the user asks about travel:

### search_flights — Find flights between airports
```json
{"origin": "HEL", "destination": "NRT", "departure_date": "2026-06-15"}
```
Optional parameters:
- `return_date`: "2026-06-22" (makes it round-trip)
- `cabin_class`: "economy" | "premium_economy" | "business" | "first"
- `max_stops`: "any" | "nonstop" | "one_stop" | "two_plus"
- `sort_by`: "cheapest" | "duration" | "departure" | "arrival"

### search_dates — Find the cheapest day to fly
```json
{"origin": "HEL", "destination": "NRT", "start_date": "2026-06-01", "end_date": "2026-06-30"}
```
Optional: `trip_duration` (days), `is_round_trip` (true/false)

### search_hotels — Find hotels in any city
```json
{"location": "Tokyo", "check_in": "2026-06-15", "check_out": "2026-06-18"}
```
Optional: `guests` (number), `stars` (1-5 minimum), `sort` ("price" | "rating"), `currency` ("EUR" | "USD" etc.)

### hotel_prices — Compare prices across booking sites
```json
{"hotel_id": "<from search_hotels>", "check_in": "2026-06-15", "check_out": "2026-06-18"}
```

### destination_info — Travel intelligence for any city
```json
{"location": "Tokyo"}
```
Optional: `travel_dates` ("2026-06-15,2026-06-18" — comma-separated check-in,check-out)

Returns: weather forecast, country info (capital, languages, currencies), public holidays during travel dates, safety advisory (1-5 scale), currency exchange rates vs EUR, timezone.

### calculate_trip_cost — Estimate total trip cost
```json
{"origin": "HEL", "destination": "BCN", "depart_date": "2026-07-01", "return_date": "2026-07-08"}
```
Optional: `guests` (number, default 1), `currency` ("EUR" | "USD" etc.)

Returns: cheapest outbound flight + return flight + cheapest hotel per night, total cost, per-person cost, per-day cost.

### weekend_getaway — Find cheap weekend destinations
```json
{"origin": "HEL", "month": "july-2026"}
```
Optional: `max_budget` (number in EUR, 0 = no limit), `nights` (default: 2)

Returns: top 10 cheapest weekend destinations ranked by total estimated cost (round-trip flight + estimated hotel).

### suggest_dates — Smart date suggestions around a target date
```json
{"origin": "HEL", "destination": "BCN", "target_date": "2026-07-15"}
```
Optional: `flex_days` (default: 7), `round_trip` (boolean), `duration` (days for round-trip, default: 7)

Returns: 3 cheapest dates, weekday vs weekend analysis, savings insights, average price comparison.

### optimize_multi_city — Find cheapest routing for multi-city trips
```json
{"home_airport": "HEL", "cities": "BCN,ROM,PAR", "depart_date": "2026-07-01"}
```
Optional: `return_date` ("2026-07-21")

Returns: optimal visit order, per-segment prices, total cost, savings vs worst order. Tries all permutations (up to 6 cities).

### MCP Prompts (for complex workflows)
- `plan-trip` — Full trip planning: flights + hotels + budget analysis
- `find-cheapest-dates` — Month-wide price calendar for a route
- `compare-hotels` — Side-by-side hotel comparison by user priorities

## Response Tips

- Results include `booking_url` — share these with the user for direct Google links
- Results include `suggestions` — use these to offer follow-up searches
- Prices reflect the user's IP geolocation currency
- For trip planning: search flights first, then hotels at the destination
- For budget trips: use `weekend_getaway` or `suggest_dates` to find the cheapest options
- For multi-city: use `optimize_multi_city` to find the cheapest routing order
- For full cost estimates: use `calculate_trip_cost` for flights + hotel totals
- For destination research: use `destination_info` for weather, safety, holidays
- Common IATA codes: HEL (Helsinki), JFK (New York), LHR (London), NRT (Tokyo), CDG (Paris), BCN (Barcelona), BKK (Bangkok), SIN (Singapore), DXB (Dubai), LAX (Los Angeles), FRA (Frankfurt), AMS (Amsterdam), ICN (Seoul)

## Troubleshooting

- **"command not found"**: `which trvl` — if empty, the binary isn't in PATH. Re-run Step 1.
- **No results**: Google may rate-limit. Wait 60 seconds and retry.
- **Wrong currency**: Normal — currency follows IP geolocation.
- **MCP tools not showing**: Restart Claude Code / Claude Desktop after Step 2.

## Source

- GitHub: https://github.com/MikkoParkkola/trvl
- License: MIT
- Inspired by [fli](https://github.com/punitarani/fli) by Punit Arani

# trvl — Complete Setup for AI Assistants

> **Human**: Give this URL to your AI assistant and say "set up trvl":
> `https://raw.githubusercontent.com/MikkoParkkola/trvl/main/AGENTS.md`

## What To Do

You are setting up `trvl` — a travel MCP server that gives you real-time Google Flights and Google Hotels data. No API keys needed. Run these steps in order:

### Step 1: Install

```bash
brew install MikkoParkkola/tap/trvl
```

Pre-built binary, no toolchain needed, 1 second. If Homebrew is not available:

```bash
# Manual: detect OS/arch, download, extract
curl -fsSL https://github.com/MikkoParkkola/trvl/releases/latest/download/trvl_$(uname -s | tr '[:upper:]' '[:lower:]')_$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/').tar.gz | tar xz -C /usr/local/bin trvl
```

### Step 2: Register as MCP server

```bash
trvl mcp install                       # Claude Desktop (default)
trvl mcp install --client cursor       # Cursor / Windsurf
trvl mcp install --client claude-code  # Claude Code
```

Then restart your MCP client. No JSON editing needed.

For Claude Code, you can also run:
```bash
claude mcp add trvl --transport stdio -- trvl mcp
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
# Expected: trvl 0.3.0 (or later)

trvl flights HEL LHR 2026-07-01 --format json | head -5
# Expected: JSON with flight results
```

Tell the user: "trvl is installed with 33 MCP tools and 1 bundled Claude skill. I can search flights, hotels, destinations, plan trips, find weekend getaways, find optimal travel windows, optimize multi-city routes, find nearby restaurants, check local events, search ground transport, detect travel hacks, check weather forecasts, and look up airline baggage rules. Just ask me anything about travel."

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

### Step 6: Build the traveller profile

The profile lives at `~/.trvl/preferences.json`. **Only ask about things
that actually change search results.** Don't ask about fields that aren't
wired to code behavior yet.

**The interview: 4 questions.**

> **Q1:** "Where do you usually fly from?"
> Sets `home_airports` — the default origin for every search.
> Infer `display_currency` from their location.

> **Q2:** "Hotels: any dealbreakers? Hostels OK or hotels only? Need your
> own bathroom? Minimum stars or review score you'd accept?"
> Sets `no_dormitories`, `ensuite_only`, `min_hotel_stars`, `min_hotel_rating`.

> **Q3:** "Carry-on only, or do you check bags?"
> Sets `carry_on_only` — unlocks hidden-city and throwaway-ticket hacks.

> **Q4:** "Direct flights important, or connections fine if cheaper?"
> Sets `prefer_direct`.

Save with `update_preferences`. Show what you saved. Done. Don't ask about
neighborhoods upfront — learn those from usage.

**What each field actually does in the code:**

| Field | Behavior |
|-------|----------|
| `home_airports` | Default origin for flight/trip/weekend/discover searches |
| `display_currency` | Price display across all 33 tools |
| `no_dormitories` | `FilterHotels()` drops hostels, capsules, guesthouse rooms by chain name + regex |
| `ensuite_only` | `FilterHotels()` drops shared-bathroom properties |
| `min_hotel_stars` | Passed to Google Hotels API as search filter |
| `min_hotel_rating` | Passed to search + activates 20-review minimum gate |
| `preferred_districts` | `FilterHotels()` strict-filters or prioritizes by neighborhood |
| `carry_on_only` | Travel hack detectors: hidden-city and throwaway require carry-on |
| `prefer_direct` | Flight search: nonstop filter |
| `default_companions` | 0=solo, 1=couple, 2+=family/group — personalizes search defaults |
| `trip_types` | e.g. `["city_break","beach","adventure"]` — destination suggestions |
| `seat_preference` | "window", "aisle", "no_preference" |
| `budget_per_night_min` | Filters too-cheap-to-trust hotels |
| `budget_per_night_max` | Max hotel price per night |
| `budget_flight_max` | Max one-way flight price |
| `deal_tolerance` | "price" (6am flight? yes), "comfort" (pay more), "balanced" |
| `flight_time_earliest` | e.g. "06:00" — no flights before this |
| `flight_time_latest` | e.g. "23:00" — no flights after this |
| `red_eye_ok` | Overnight flights acceptable? |
| `nationality` | ISO 3166-1 alpha-2 (e.g. "FI") — visa warnings |
| `languages` | Spoken languages, e.g. `["en","fi","sv"]` |
| `previous_trips` | Cities/countries visited — avoids repeat suggestions |
| `bucket_list` | Dream destinations — prioritized in suggestions |
| `activity_preferences` | e.g. `["museums","food","nature"]` — destination matching |
| `dietary_needs` | e.g. `["vegetarian","halal"]` — restaurant filtering |
| `notes` | Free-text for anything else |

**Don't ask upfront — learn from usage:**

- **Neighborhoods**: user picks Prague 1 hotels twice → "Want me to
  remember Prague 1 as your preferred area?"
- **Star rating**: user adds `--stars 4` three times → "Set 4-star as
  your default?"
- **Hostel filter**: user rejects a hostel result → "Filter hostels
  automatically from now on?"
- **Home airport change**: user searches from AMS repeatedly → "Add
  Amsterdam to your home airports?"
- **Budget drift**: user rejects hotels above a price → "Set a max
  nightly budget?"
- **Trip log**: after booking → "Add [destination] to previous trips?"
- **Bucket list**: user mentions dream destination → "Add to bucket list?"
- **Activities**: user searches food tours repeatedly → "Add 'food' to
  your activity preferences?"

**Extended profile follow-ups** (ask after the 4 core questions if
the conversation naturally leads there):

- **Flights**: "Any flight preferences? Early mornings OK, red-eyes?"
  → `seat_preference`, `flight_time_earliest/latest`, `red_eye_ok`, `deal_tolerance`
- **Budget**: "Typical budget range for hotels and flights?"
  → `budget_per_night_min/max`, `budget_flight_max`
- **Identity**: "Nationality? Languages?" → `nationality`, `languages`
- **Past & future**: "Where have you been? Dream destinations?"
  → `previous_trips`, `bucket_list`
- **Activities & food**: "What do you do on trips? Dietary needs?"
  → `activity_preferences`, `dietary_needs`
- **Catch-all**: "Anything else?" → `notes`

Always confirm before updating. Use `update_preferences` to merge
individual fields. CLI alternative: `trvl prefs init`

---

## How To Use (after setup)

You now have 33 MCP tools available. Use them when the user asks about travel:

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

### update_preferences — Update user travel preferences
```json
{"min_hotel_stars": 4, "no_dormitories": true}
```
Merges individual fields into `~/.trvl/preferences.json`. Only send the
fields you want to change — other fields are preserved. Always confirm
with the user before calling this tool.

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
- License: PolyForm Noncommercial 1.0.0
- Inspired by [fli](https://github.com/punitarani/fli) by Punit Arani

---
name: power-traveler
description: "Power traveler mode. Digital nomad optimization, campaign hunting, multi-hack stacking, advanced itinerary tweaking. For serious travelers who want better results than a professional travel agent."
triggers:
  - digital nomad
  - nomad
  - work remotely
  - workcation
  - long stay
  - monthly cost
  - base city
  - campaign
  - discount
  - deal alert
  - error fare
  - hack stack
  - optimize route
  - advanced search
  - power search
  - compare everything
  - professional level
  - better than travel agent
allowed-tools:
  - Bash
  - mcp__gateway__gateway_invoke
  - mcp__gateway__gateway_search_tools
---

# Power Traveler Mode — Pro-Level Trip Optimization

For serious travelers, digital nomads, and travel hackers who want BETTER results than a professional travel agent. This skill stacks multiple hacks, hunts for campaigns, and optimizes across every dimension simultaneously.

## Digital Nomad Specific

### Monthly Base City Cost Analysis
When a nomad asks "where should I base myself?":
1. `explore_destinations` from their home airport — cheapest cities
2. For top 10: `destination_info` for weather, safety, timezone, visa
3. `search_hotels` for 30-night stays (monthly rates are different from nightly)
4. Estimate monthly costs:
   - Flight to get there (one-way)
   - Accommodation (apartment > hotel for 30+ days)
   - Coworking (~€100-300/month depending on city)
   - Food + transport (destination_info currency + general knowledge)
   - Visa status (tourist visa duration, digital nomad visa availability)
5. Present as comparison table:
```
Monthly Base City Analysis from HEL:

| City | Flight | Rent/mo | Cowork | Food | Total | TZ offset | Visa |
|------|--------|---------|--------|------|-------|-----------|------|
| Lisbon | €120 | €1200 | €200 | €600 | €2120 | CET+0 | 90d Schengen |
| Bangkok | €350 | €500 | €100 | €300 | €1250 | CET+5 | 60d tourist |
| Tbilisi | €150 | €400 | €80 | €250 | €880 | CET+2 | 365d visa-free |
| Tenerife | €130 | €900 | €150 | €450 | €1630 | CET-1 | Schengen |
```

### Timezone Compatibility
Always show timezone offset from the user's team/clients:
- "Bangkok is CET+5 — your 9am meeting is their 3pm. Overlap: 9am-1pm CET."
- "Lisbon is CET+0 in winter, CET-1 in summer — same timezone as your team."

### Schengen/Visa Calculator
Track days in Schengen zone (90/180 rule):
- "You've been in Europe for 67 days. You have 23 days left in this Schengen window."
- "Suggested: fly to Tbilisi (non-Schengen, 365 days visa-free) for a month to reset your counter."

## Campaign & Discount Hunting

### How to find campaigns
There's no single "campaigns API" — campaigns are found by comparing:

1. **Airline flash sales**: Check if the cheapest price on `search_dates` is abnormally low
   - "HEL→BCN at €68 is 52% below the 90-day average of €142. This looks like a sale."
   
2. **Error fares**: Prices >60% below average across multiple dates
   - "HEL→NRT at €189 return is 78% below average. This is likely an error fare — book IMMEDIATELY, these get fixed within hours."

3. **Hotel promotions**: Compare `search_hotels` prices with `hotel_prices` (provider comparison)
   - If one provider is 30%+ cheaper, it's likely running a promotion
   
4. **Seasonal drops**: Use `search_dates` across several months
   - "Barcelona hotel prices drop 40% in November vs August. Consider shoulder season."

5. **Low-cost carrier route launches**: New routes are often discounted for 2-3 months
   - If `explore_destinations` shows a route at an unusually low price with a low-cost carrier, flag it

### Price anomaly detection
After every search, automatically assess:
- Is this price below the route average? By how much?
- Is this suspiciously cheap? (possible error fare)
- Are nearby dates much more expensive? (confirms it's a deal, not normal pricing)
- Present: "🔥 This is 43% below average for this route. Strong deal."

## Advanced Hack Stacking

Professional travel agents use ONE trick at a time. We stack ALL of them simultaneously.

### The Full Stack (run for every complex trip):

```
Layer 1: ROUTE OPTIMIZATION
  ├── Multi-city ordering (N! permutations)
  ├── Open-jaw routing (fly in A, out of B)
  ├── Positioning flights (cheap flight to hub)
  └── Train+flight combos

Layer 2: DATE OPTIMIZATION
  ├── CalendarGraph for all legs (cheapest dates)
  ├── Weekday vs weekend analysis
  ├── Holiday avoidance
  └── Shoulder season detection

Layer 3: BOOKING OPTIMIZATION
  ├── Split ticketing (different airlines each way)
  ├── One-way vs round-trip comparison
  ├── Throw-away return check
  ├── Hidden city opportunities (with warnings)
  └── Connection vs nonstop tradeoff

Layer 4: ACCOMMODATION OPTIMIZATION
  ├── Hotel split (weekday/weekend)
  ├── Provider price comparison
  ├── Apartment vs hotel for 4+ nights
  └── Location vs price tradeoff

Layer 5: SERVICE OPTIMIZATION
  ├── Luggage cost inclusion
  ├── Airline alliance/status benefits
  ├── Lounge access value
  └── Fare class analysis (basic vs flex vs business)
```

For a power traveler, run ALL 5 layers. Present the fully stacked result:
"Applied 7 optimizations across 5 layers. Original naive booking: €2,340. Optimized: €1,180. Savings: €1,160 (50%)."

## Multi-Leg Complex Itineraries

### Conference + Leisure Extension (Bleisure)
```
User: "Conference in Barcelona June 15-18, then I want to explore Spain"

Optimization:
1. Conference dates are FIXED: arrive June 14, conference June 15-18
2. Post-conference: BCN → Madrid (train €25, 2.5h) or BCN → Seville (€45 flight)
3. Return from different city (open-jaw): Seville → HEL
4. Date flexibility on return: check June 22-28 for cheapest
5. Hotel: conference hotel June 14-18 (book via conference rate!)
         Seville apartment June 18-25 (weekly rate cheaper)

Result: HEL→BCN June 14 (€113) + BCN→Seville June 18 train (€45) + 
        Seville→HEL June 25 (€89) = €247 flights
        vs round-trip HEL↔BCN: €235 + internal flight €89 = €324
        
Open-jaw saves €77 AND you see more of Spain.
```

### Multi-Country Nomad Route
```
User: "I want to spend 3 months in Europe: some cities, work + explore"

Optimization:
1. Schengen limit: 90 days max → plan around it
2. Use explore_destinations to find cheapest hub cities
3. Build route: cheap hub → affordable bases → non-Schengen break → return
4. Example: HEL → Lisbon (30d) → Tbilisi (30d, non-Schengen) → Budapest (30d) → HEL
5. Check all inter-city flights via CalendarGraph for cheapest dates
6. Accommodation: monthly apartment rates (30% cheaper than nightly)
7. Show total: flights + accommodation + daily costs for 90 days
```

## What to Show Power Travelers

Power travelers don't want "here's a flight." They want:

1. **Full optimization audit**: every hack tried, with results
2. **Alternative scenarios**: "If you're flexible by 3 days... If you take a train... If you split the stay..."
3. **Price context**: "This is 43% below the 90-day average" not just "€113"
4. **Hidden costs**: luggage, transport to/from airport, visa fees, coworking
5. **Timezone analysis**: critical for remote workers
6. **The numbers**: total trip cost with every line item, savings vs naive booking

## Key Phrases That Trigger Deep Optimization

- "optimize everything" → run full 5-layer stack
- "find me a deal" → campaign/anomaly detection mode
- "I'm flexible" → run CalendarGraph across maximum date range
- "compare all options" → nearby airports + split tickets + positioning + connections
- "how much total?" → full trip cost including hidden costs
- "base city for X months" → nomad cost analysis with timezone/visa

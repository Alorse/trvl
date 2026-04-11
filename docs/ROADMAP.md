# trvl Roadmap — Power Traveler Features

## Current: 17 providers, 14 with real prices

## DONE: Multi-modal routing (MVP)
`trvl route HEL DBV --arrive-by 2026-04-10`
Combine flights + trains + buses + ferries into Pareto-optimal itineraries.
26 European hub cities. Pareto filtering on price vs. duration.

## DONE: Ferries (Baltic + North Sea)
6 ferry providers live:
- Tallink/Silja — live booking SPA API (`booking.tallink.com/api/timetables`)
- Eckerö Line — live Magento AJAX API (`getdepartures`)
- Finnlines — live GraphQL API (AWS AppSync, schedules + prices + cabin catalog)
- DFDS — live date availability API (`travel-search-prod.dfds-pax-web.com`)
- Viking Line — reference schedule (FerryGateway Switch pending)
- Stena Line — reference schedule (FerryGateway Switch pending)

## P0: Distribusion API integration
Replace Viking Line and Stena Line reference schedules with live prices via
the Distribusion ferry distribution platform. Covers 50+ ferry operators.
Contact: distribusion.com/developers

## P1: Positioning flights
`trvl position HEL --radius 500km --to BCN`
Multi-origin price comparison across nearby airports.

## P1: Price prediction
`trvl predict HEL BCN --month june`
Historical price analysis from watch data.

## P1: Calendar-aware flex search
`trvl suggest HEL BCN --duration 4 --avoid 2026-04-15:2026-04-20`

## P2: Disruption rerouting
`trvl reroute --from FRA --to HEL --now`

## P2: Digital nomad cost optimization
`trvl nomad HEL --budget 2000 --duration 30`

## P2: Hidden city ticketing detection
## P2: Alliance/status mile optimization
## P2: Layover activity suggestions

# trvl Roadmap — Power Traveler Features

## Current: 11 providers, 10 with real prices

## P0: Multi-modal routing
`trvl route HEL DBV --arrive-by 2026-04-10`
Combine flights + trains + buses + ferries into optimal itineraries.
All 11 provider APIs already exist — need routing algorithm.

## P0: Ferries (Baltic + Mediterranean)
Tallink API confirmed working: `/api/timetables`, `/api/reservation/summary/v2`
Session GUID generated client-side, no auth needed.
Providers: Tallink/Silja, Viking Line, Eckerö, Stena Line, DFDS.

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

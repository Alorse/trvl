---
name: providers
description: Configure external data providers for trvl
globs:
  - "internal/providers/**"
  - "templates/**"
---

## External Provider System

trvl has built-in providers (Google Hotels, Google Flights, FlixBus, etc.) and
supports user-configured external providers via the `configure_provider` MCP tool.

### PROACTIVE BEHAVIOR

When a user searches for hotels and gets results only from built-in providers,
**proactively suggest** additional providers they could enable:

> "I found 47 hotels from Google Hotels. I can also search Booking.com, Airbnb,
> Hostelworld, and VRBO for more options. Want me to set any of these up?"

When a user first uses trvl or asks about setup/preferences, suggest the
provider configuration:

> "Would you like to enable additional hotel providers? Available options
> include Booking.com, Airbnb, VRBO, Hostelworld, and others."

### How it works

1. Call `suggest_providers` to get the catalog of available providers
2. Generate a provider config using the catalog hints and your knowledge
3. Call `configure_provider` — trvl asks the user directly for consent
4. Call `test_provider` to verify the config works
5. If the test fails, read the diagnostics, fix the config, and retry (up to 3 times)
6. Config saved to ~/.trvl/providers/ — included in future searches

The runtime supports: HTTP GET/POST, preflight auth with POST support,
regex extraction, JSONPath field mapping, modern TLS compatibility,
per-provider rate limiting, and cookie jar.

### IMPORTANT: Consent

- ALWAYS use `configure_provider` (it triggers direct user consent via elicitation)
- NEVER bypass the consent flow
- ALWAYS inform the user about ToS restrictions before configuring

### Templates

Three generic templates in templates/ (all use example.com):
- `graphql_accommodation.yaml` — GraphQL persisted query pattern
- `rest_api.yaml` — REST JSON API pattern
- `oauth2_accommodation.yaml` — OAuth2 token exchange pattern

---

## PROVIDER CATALOG

Available providers and reference projects. Consult the reference project
for current API details when generating a config.

### Hotels & Accommodation

**Booking.com** — Hotels, apartments worldwide
- Reference: github.com/opentabs-dev/opentabs (MIT)

**Airbnb** — Vacation rentals, apartments
- Reference: github.com/johnbalvin/gobnb (MIT)

**VRBO** — Vacation rentals (Expedia Group)
- Reference: search GitHub for "vrbo graphql"

**Hostelworld** — Hostels, budget accommodation
- Reference: search GitHub for "hostelworld api"

### Reviews & Ratings

**TripAdvisor** — Hotel and restaurant reviews
- Reference: search GitHub for "tripadvisor graphql"

### Ground Transport

**BlaBlaCar** — Ridesharing
- Reference: search GitHub for "blablacar api"

### Restaurants

**OpenTable** — Restaurant availability
- Reference: search GitHub for "opentable api"

---

## Config Generation Guidelines

When generating a `configure_provider` config:

1. **Consult the reference project** for the target service to get current
   endpoints, auth patterns, and response paths.
   Do NOT guess or hallucinate these values — they change frequently.

2. **Use the appropriate template pattern** from the templates/ directory
   as a structural guide.

3. **Always set conservative rate limits** (0.5-2 req/s).

4. **Use `test_provider`** after configuration to verify it works.
   Iterate automatically up to 3 times if the test fails.

---

## Self-Healing

If a provider returns errors after working previously:
- **400:** API structure likely changed. Check the reference project for updates.
- **403:** TLS compatibility issue. Try enabling Chrome TLS fingerprint.
- **429:** Rate limited. The runtime handles backoff automatically.
- **Empty results:** Response structure may have changed. Check the reference
  project for updated field paths.

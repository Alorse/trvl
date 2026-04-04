Output demo.gif

Set Shell "bash"
Set FontSize 12
Set Width 1400
Set Height 700
Set Theme "Catppuccin Mocha"
Set Padding 16
Set TypingSpeed 25ms
Set CursorBlink false

# ─────────────────────────────────────────────────────────
# trvl — Plan a long weekend in Prague, from Amsterdam.
# Pre-warm every visible command so results come from cache.
# ─────────────────────────────────────────────────────────

Hide
# Trip first — needs flights + hotels, give it fresh rate-limit budget
Type "trvl trip AMS PRG --depart 2026-06-15 --return 2026-06-18 --currency EUR 2>/dev/null"
Enter
Sleep 60s
# Non-Google APIs next (RSS feeds, no rate-limit competition)
Type "trvl deals --from Amsterdam --hours 72 2>/dev/null"
Enter
Sleep 15s
Type "trvl ground Prague Vienna 2026-06-18 --currency EUR 2>/dev/null"
Enter
Sleep 45s
Type "trvl destination Prague 2>/dev/null"
Enter
Sleep 15s
# Google APIs — spaced out after trip has its results cached
Type "trvl explore AMS --currency EUR 2>/dev/null"
Enter
Sleep 25s
Type "trvl dates AMS PRG --from 2026-06-10 --to 2026-06-20 --currency EUR 2>/dev/null"
Enter
Sleep 15s
Type "trvl grid AMS PRG --depart-from 2026-06-13 --depart-to 2026-06-16 --return-from 2026-06-17 --return-to 2026-06-20 --currency EUR 2>/dev/null"
Enter
Sleep 30s
Type "trvl flights AMS,EIN PRG 2026-06-15 --stops 1 --currency EUR 2>/dev/null"
Enter
Sleep 30s
Type "clear"
Enter
Sleep 500ms
Show

# ═══════════════════════════════════════════════════════════
# ACT 1: Discovery — "Where should I go?"
# ═══════════════════════════════════════════════════════════

Type "# Where's cheap from Amsterdam right now?"
Enter
Sleep 300ms
Type "trvl explore AMS --currency EUR"
Enter
Sleep 8s

Type ""
Enter
Sleep 1500ms

Type "# Any hot deals from Amsterdam?"
Enter
Sleep 300ms
Type "trvl deals --from Amsterdam --hours 72"
Enter
Sleep 8s

Type ""
Enter
Sleep 1500ms

# ═══════════════════════════════════════════════════════════
# ACT 2: Planning — "Prague! When's cheapest?"
# ═══════════════════════════════════════════════════════════

Type "# Cheapest dates to fly AMS -> Prague"
Enter
Sleep 300ms
Type "trvl dates AMS PRG --from 2026-06-10 --to 2026-06-20 --currency EUR"
Enter
Sleep 6s

Type ""
Enter
Sleep 1500ms

Type "# Round-trip price matrix: every departure x return combo"
Enter
Sleep 300ms
Type "trvl grid AMS PRG --depart-from 2026-06-13 --depart-to 2026-06-16 --return-from 2026-06-17 --return-to 2026-06-20 --currency EUR"
Enter
Sleep 8s

Type ""
Enter
Sleep 1500ms

# ═══════════════════════════════════════════════════════════
# ACT 3: Booking — "Plan the whole trip in one search"
# ═══════════════════════════════════════════════════════════

Type "# One search: flights + hotel + cost summary"
Enter
Sleep 300ms
Type "trvl trip AMS PRG --depart 2026-06-15 --return 2026-06-18 --currency EUR"
Enter
Sleep 12s

Type ""
Enter
Sleep 1500ms

Type "# More options? Search AMS *and* EIN airports at once"
Enter
Sleep 300ms
Type "trvl flights AMS,EIN PRG 2026-06-15 --stops 1 --currency EUR"
Enter
Sleep 8s

Type ""
Enter
Sleep 1500ms

# ═══════════════════════════════════════════════════════════
# ACT 4: Travel onward + monitor
# ═══════════════════════════════════════════════════════════

Type "# After Prague — train or bus to Vienna?"
Enter
Sleep 300ms
Type "trvl ground Prague Vienna 2026-06-18 --currency EUR"
Enter
Sleep 8s

Type ""
Enter
Sleep 1500ms

Type "# Track the route — alert me if price drops below 80"
Enter
Sleep 300ms
Type "trvl watch add AMS PRG --below 80 --currency EUR"
Enter
Sleep 2s

Type ""
Enter
Sleep 1000ms

Type "# What's Prague like? Weather, holidays, currency, safety"
Enter
Sleep 300ms
Type "trvl destination Prague"
Enter
Sleep 8s

Type ""
Enter
Sleep 1500ms
Type "# 16 MCP tools · 22 CLI commands · 6 ground providers · No API keys"
Enter
Sleep 400ms
Type "# github.com/MikkoParkkola/trvl"
Enter
Sleep 4s

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
# Explore first — lightweight, warms up currency detection cache
Type "trvl explore AMS --from 2026-06-12 --to 2026-06-14 --currency EUR 2>/dev/null"
Enter
Sleep 45s
# Trip — needs flights + hotels
Type "trvl trip AMS PRG --depart 2026-06-15 --return 2026-06-18 --currency EUR 2>/dev/null"
Enter
Sleep 60s
# Non-Google APIs (RSS feeds, ground transport, destination)
Type "trvl deals --from Amsterdam --hours 72 2>/dev/null"
Enter
Sleep 15s
Type "trvl ground Prague Vienna 2026-06-18 --currency EUR 2>/dev/null"
Enter
Sleep 45s
Type "trvl destination Prague 2>/dev/null"
Enter
Sleep 15s
# Remaining Google APIs — currency already cached from explore
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
# ACT 1: "Where should I go this weekend?"
# ═══════════════════════════════════════════════════════════

Type "# Where's cheap from Amsterdam this weekend?"
Enter
Sleep 300ms
Type "trvl explore AMS --from 2026-06-12 --to 2026-06-14 --currency EUR"
Enter
Sleep 10s

Type "clear"
Enter
Sleep 300ms

Type "# Prague looks good! Cheapest dates in June?"
Enter
Sleep 300ms
Type "trvl dates AMS PRG --from 2026-06-10 --to 2026-06-20 --currency EUR"
Enter
Sleep 8s

Type ""
Enter
Sleep 500ms

Type "# Round-trip price matrix: every departure x return combo"
Enter
Sleep 300ms
Type "trvl grid AMS PRG --depart-from 2026-06-13 --depart-to 2026-06-16 --return-from 2026-06-17 --return-to 2026-06-20 --currency EUR"
Enter
Sleep 10s

# ═══════════════════════════════════════════════════════════
# ACT 2: "Plan the whole trip — flights + hotel"
# ═══════════════════════════════════════════════════════════

Type "clear"
Enter
Sleep 300ms

Type "# One search: flights + hotel + total cost"
Enter
Sleep 300ms
Type "trvl trip AMS PRG --depart 2026-06-15 --return 2026-06-18 --currency EUR"
Enter
Sleep 12s

# ═══════════════════════════════════════════════════════════
# ACT 3: "Find better prices with multi-airport search"
# ═══════════════════════════════════════════════════════════

Type "clear"
Enter
Sleep 300ms

Type "# Search AMS *and* Eindhoven at once — find the cheapest"
Enter
Sleep 300ms
Type "trvl flights AMS,EIN PRG 2026-06-15 --stops 1 --currency EUR"
Enter
Sleep 8s

# ═══════════════════════════════════════════════════════════
# ACT 4: "After Prague — ground transport + monitoring"
# ═══════════════════════════════════════════════════════════

Type "clear"
Enter
Sleep 300ms

Type "# After Prague — bus or train to Vienna from EUR 9.90"
Enter
Sleep 300ms
Type "trvl ground Prague Vienna 2026-06-18 --currency EUR"
Enter
Sleep 8s

Type "clear"
Enter
Sleep 300ms

Type "# Track prices — alert me if flights drop below EUR 80"
Enter
Sleep 300ms
Type "trvl watch add AMS PRG --below 80 --currency EUR"
Enter
Sleep 3s

Type ""
Enter
Sleep 500ms

Type "# What's Prague like? Weather, holidays, currency"
Enter
Sleep 300ms
Type "trvl destination Prague"
Enter
Sleep 10s

Type ""
Enter
Sleep 1500ms
Type "# 16 MCP tools · 22 CLI commands · 6 ground providers · No API keys"
Enter
Sleep 400ms
Type "# github.com/MikkoParkkola/trvl"
Enter
Sleep 4s

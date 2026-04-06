Output demo.gif

Set Shell "bash"
Set FontSize 12
Set Width 1400
Set Height 700
Set Theme "Catppuccin Mocha"
Set Padding 16
Set TypingSpeed 50ms
Set CursorBlink false

# ─────────────────────────────────────────────────────────
# trvl — Multi-modal travel: flights, buses, ferries, routes
# Pre-warm every visible command so results come from cache.
# ─────────────────────────────────────────────────────────

Hide
Exec "trvl explore HEL --format table 2>/dev/null"
Sleep 45s
Exec "trvl route Helsinki Dubrovnik 2026-07-01 2>/dev/null"
Sleep 60s
Exec "trvl ground Helsinki Tallinn 2026-07-01 2>/dev/null"
Sleep 45s
Exec "trvl trip HEL BCN --depart 2026-07-01 --return 2026-07-08 2>/dev/null"
Sleep 60s
Type "clear"
Enter
Sleep 500ms
Show

# ═══════════════════════════════════════════════════════════
# ACT 1: Discovery — "Where's cheap from Helsinki?"
# ═══════════════════════════════════════════════════════════

Type "# Where's cheap from Helsinki right now?"
Enter
Sleep 300ms
Type "trvl explore HEL --format table 2>/dev/null"
Enter
Sleep 10s

# ═══════════════════════════════════════════════════════════
# ACT 2: Multi-Modal Route — THE HERO FEATURE
# ═══════════════════════════════════════════════════════════

Type "clear"
Enter
Sleep 300ms

Type "# Dubrovnik! How do I get there? Flights, buses, combos..."
Enter
Sleep 300ms
Type "trvl route Helsinki Dubrovnik 2026-07-01 2>/dev/null"
Enter
Sleep 15s

# ═══════════════════════════════════════════════════════════
# ACT 3: Ferry Search — live prices from 4 providers
# ═══════════════════════════════════════════════════════════

Type "clear"
Enter
Sleep 300ms

Type "# Helsinki → Tallinn: ferries, buses — live prices from 4 providers"
Enter
Sleep 300ms
Type "trvl ground Helsinki Tallinn 2026-07-01 2>/dev/null"
Enter
Sleep 10s

# ═══════════════════════════════════════════════════════════
# ACT 4: Trip Planning — flights + hotel in one search
# ═══════════════════════════════════════════════════════════

Type "clear"
Enter
Sleep 300ms

Type "# Full trip: flights + hotel + total cost"
Enter
Sleep 300ms
Type "trvl trip HEL BCN --depart 2026-07-01 --return 2026-07-08 2>/dev/null"
Enter
Sleep 12s

Type ""
Enter
Sleep 1500ms
Type "# 18 MCP tools · 22 CLI commands · 11 providers · No API keys"
Enter
Sleep 400ms
Type "# github.com/MikkoParkkola/trvl"
Enter
Sleep 4s

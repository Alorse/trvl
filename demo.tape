Output demo.gif

Set Shell "bash"
Set FontSize 12
Set Width 1400
Set Height 700
Set Theme "Catppuccin Mocha"
Set Padding 16
Set TypingSpeed 50ms
Set CursorBlink false

# ═══════════════════════════════════════════════════════════
# ACT 1: Discovery — "Where's cheap from Helsinki?"
# ═══════════════════════════════════════════════════════════

Type "# Where's cheap from Helsinki right now?"
Enter
Sleep 300ms
Type "trvl explore HEL --format table"
Enter
Sleep 30s

# ═══════════════════════════════════════════════════════════
# ACT 2: Flights — real-time prices
# ═══════════════════════════════════════════════════════════

Type "clear"
Enter
Sleep 300ms

Type "# Flights to Prague next month"
Enter
Sleep 300ms
Type "trvl flights HEL PRG 2026-05-15"
Enter
Sleep 20s

# ═══════════════════════════════════════════════════════════
# ACT 3: Ferries — Helsinki to Tallinn (future date, live prices)
# ═══════════════════════════════════════════════════════════

Type "clear"
Enter
Sleep 300ms

Type "# Helsinki to Tallinn: ferries + buses — live prices"
Enter
Sleep 300ms
Type "trvl ground Helsinki Tallinn 2026-05-15"
Enter
Sleep 30s

# ═══════════════════════════════════════════════════════════
# ACT 4: Trip cost — flights + hotel total
# ═══════════════════════════════════════════════════════════

Type "clear"
Enter
Sleep 300ms

Type "# Full trip cost: Helsinki → Paris, flights + hotel"
Enter
Sleep 300ms
Type "trvl trip-cost HEL CDG --depart 2026-06-01 --return 2026-06-05"
Enter
Sleep 30s

# ═══════════════════════════════════════════════════════════
# CLOSING
# ═══════════════════════════════════════════════════════════

Type ""
Enter
Sleep 500ms
Type "# 33 MCP tools · 33 CLI commands · 17 providers · No API keys"
Enter
Sleep 400ms
Type "# github.com/MikkoParkkola/trvl"
Enter
Sleep 4s

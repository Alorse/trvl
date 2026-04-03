Output demo.gif

Set Shell "bash"
Set FontSize 12
Set Width 1300
Set Height 650
Set Theme "Catppuccin Mocha"
Set Padding 12
Set TypingSpeed 30ms

# ═══════════════════════════════════════════════════
# Story 1: "I need a cheap weekend escape from Helsinki"
# ═══════════════════════════════════════════════════

Type "# ═══ Story 1: Weekend escape on a budget ═══"
Enter
Sleep 800ms

Type "# Where can I go for under €400 this July?"
Enter
Sleep 300ms
Type "./bin/trvl weekend HEL --month july-2026 --budget 400"
Enter
Sleep 15s

Type ""
Enter
Sleep 600ms

Type "# Dubrovnik looks great. What's it like?"
Enter
Sleep 300ms
Type "./bin/trvl destination Dubrovnik"
Enter
Sleep 8s

Type ""
Enter
Sleep 600ms

Type "# Which exact dates are cheapest?"
Enter
Sleep 300ms
Type "./bin/trvl dates HEL DBV --from 2026-07-01 --to 2026-07-14"
Enter
Sleep 15s

Type ""
Enter
Sleep 600ms

Type "# Show me ALL departure × return combos as a price grid"
Enter
Sleep 300ms
Type "./bin/trvl grid HEL DBV --depart-from 2026-07-04 --depart-to 2026-07-07 --return-from 2026-07-11 --return-to 2026-07-14"
Enter
Sleep 15s

Type ""
Enter
Sleep 600ms

Type "# Monitor BOTH flight and hotel prices — alerts on drops + flash sales"
Enter
Sleep 300ms
Type "./bin/trvl watch add HEL DBV --depart 2026-07-04 --return 2026-07-13 --below 200"
Enter
Sleep 3s
Type "./bin/trvl watch add --type hotel Dubrovnik --depart 2026-07-04 --return 2026-07-13 --below 80"
Enter
Sleep 3s
Type "./bin/trvl watch list"
Enter
Sleep 3s

Type ""
Enter
Sleep 800ms

# ═══════════════════════════════════════════════════
# Story 2: "Plan a multi-city Europe trip"
# ═══════════════════════════════════════════════════

Type "# ═══ Story 2: Multi-city Europe trip ═══"
Enter
Sleep 800ms

Type "# What's the cheapest order to visit 3 cities?"
Enter
Sleep 300ms
Type "./bin/trvl multi-city HEL --visit BCN,ROM,PAR --dates 2026-07-01,2026-07-21"
Enter
Sleep 20s

Type ""
Enter
Sleep 600ms

Type "# Hotels in Barcelona — sorted by rating, with amenities"
Enter
Sleep 300ms
Type "./bin/trvl hotels Barcelona --checkin 2026-07-01 --checkout 2026-07-05 --sort rating"
Enter
Sleep 15s

Type ""
Enter
Sleep 600ms

Type "# Total trip cost: flights + hotel?"
Enter
Sleep 300ms
Type "./bin/trvl trip-cost HEL BCN --depart 2026-07-01 --return 2026-07-08 --guests 2"
Enter
Sleep 15s

Type ""
Enter
Sleep 600ms

Type "# Train from Barcelona to Rome?"
Enter
Sleep 300ms
Type "./bin/trvl ground Barcelona Rome 2026-07-07"
Enter
Sleep 25s

Type ""
Enter
Sleep 800ms

# ═══════════════════════════════════════════════════
# Story 3: "Quick — flexible dates, best deal"
# ═══════════════════════════════════════════════════

Type "# ═══ Story 3: Flexible dates, best deal ═══"
Enter
Sleep 800ms

Type "# Where's cheap from Amsterdam right now?"
Enter
Sleep 300ms
Type "./bin/trvl explore AMS"
Enter
Sleep 25s

Type ""
Enter
Sleep 600ms

Type "# When should I fly to Prague? Check ±7 days"
Enter
Sleep 300ms
Type "./bin/trvl suggest AMS PRG --around 2026-04-15 --flex 7"
Enter
Sleep 15s

Type ""
Enter
Sleep 600ms

Type "# Show me the flights — with booking links"
Enter
Sleep 300ms
Type "./bin/trvl flights AMS PRG 2026-04-12 --format json | head -25"
Enter
Sleep 15s

Type ""
Enter
Sleep 400ms
Type "# 15 tools. 6 train/bus providers. One binary. No API keys."
Enter
Sleep 3s

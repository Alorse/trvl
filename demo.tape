Output demo.gif

Set Shell "bash"
Set FontSize 12
Set Width 1300
Set Height 650
Set Theme "Catppuccin Mocha"
Set Padding 12
Set TypingSpeed 30ms

# ─────────────────────────────────────────────────────
# trvl — What no other travel tool does
# ─────────────────────────────────────────────────────

Type "# 💰 When's the cheapest day? (date range price comparison)"
Enter
Sleep 400ms
Type "trvl dates AMS HEL --from 2026-06-10 --to 2026-06-20"
Enter
Sleep 20s

Type ""
Enter
Sleep 800ms

Type "# ✈️ Multi-airport: AMS or EIN → HEL or TLL (direct flights)"
Enter
Sleep 400ms
Type "trvl flights AMS,EIN HEL,TLL 2026-06-15 --stops nonstop"
Enter
Sleep 30s

Type ""
Enter
Sleep 800ms

Type "# 🏨 Hotels in Prague — sorted by rating, with amenities"
Enter
Sleep 400ms
Type "trvl hotels Prague --checkin 2026-06-15 --checkout 2026-06-18 --sort rating"
Enter
Sleep 20s

Type ""
Enter
Sleep 800ms

Type "# 🚂 Prague → Amsterdam by train? (6 providers in parallel)"
Enter
Sleep 400ms
Type "trvl ground Prague Amsterdam 2026-06-18"
Enter
Sleep 45s

Type ""
Enter
Sleep 800ms

Type "# 🔥 Travel deals from 4 RSS feeds (under 500)"
Enter
Sleep 400ms
Type "trvl deals --max-price 500"
Enter
Sleep 20s

Type ""
Enter
Sleep 800ms

Type "# 📉 Monitor prices — flights AND hotels"
Enter
Sleep 400ms
Type "trvl watch add HEL PRG --below 100"
Enter
Sleep 3s
Type "trvl watch add --type hotel Prague --depart 2026-06-15 --return 2026-06-18 --below 80"
Enter
Sleep 3s

Type ""
Enter
Sleep 400ms
Type "# 16 tools. 6 ground providers. One binary. No API keys."
Enter
Sleep 3s

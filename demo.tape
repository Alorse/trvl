Output demo.gif

Set Shell "bash"
Set FontSize 12
Set Width 1300
Set Height 650
Set Theme "Catppuccin Mocha"
Set Padding 12
Set TypingSpeed 30ms

# ─────────────────────────────────────────────────────────────
# trvl — Things no other travel tool does from the command line
# ─────────────────────────────────────────────────────────────

Type "# 💰 \"When's the cheapest day to fly to Helsinki this week?\""
Enter
Sleep 400ms
Type "./bin/trvl dates AMS HEL --from 2026-04-04 --to 2026-04-10"
Enter
Sleep 15s

Type ""
Enter
Sleep 600ms

Type "# 🌍 \"I have a free weekend. Where can I fly cheap from Helsinki?\""
Enter
Sleep 400ms
Type "./bin/trvl explore HEL"
Enter
Sleep 25s

Type ""
Enter
Sleep 600ms

Type "# 🚂 \"Can I take a train from Prague back to Amsterdam?\""
Enter
Sleep 400ms
Type "./bin/trvl ground Prague Amsterdam 2026-04-15"
Enter
Sleep 25s

Type ""
Enter
Sleep 600ms

Type "# 🏨 \"Best-rated hotels in Prague with pool and breakfast?\""
Enter
Sleep 400ms
Type "./bin/trvl hotels Prague --checkin 2026-04-12 --checkout 2026-04-15 --sort rating"
Enter
Sleep 15s

Type ""
Enter
Sleep 600ms

Type "# 📊 \"What's the total cost: flights + hotel for a week in Barcelona?\""
Enter
Sleep 400ms
Type "./bin/trvl trip-cost HEL BCN --depart 2026-07-01 --return 2026-07-08"
Enter
Sleep 15s

Type ""
Enter
Sleep 600ms

Type "# 📉 \"Alert me when HEL→PRG drops below €100\""
Enter
Sleep 400ms
Type "./bin/trvl watch add HEL PRG --depart 2026-04-10 --below 100"
Enter
Sleep 5s

Type ""
Enter
Sleep 400ms
Type "# One binary. No API keys. No monthly fees. Real prices."
Enter
Sleep 3s

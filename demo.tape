Output demo.gif

Set Shell "bash"
Set FontSize 14
Set Width 1200
Set Height 500
Set Theme "Catppuccin Mocha"
Set Padding 20
Set TypingSpeed 40ms
Set CursorBlink false

# ── ACT 1: The magic moment — budget in, trips out ──

Type "# I have EUR 500 and a free week in May. Surprise me."
Enter
Sleep 600ms

Type "trvl discover --from 2026-05-01 --until 2026-05-31 --budget 500"
Enter
Sleep 10s

# ── ACT 2: Quick follow-up — weekend getaway ──

Sleep 600ms
Type "# That Vienna deal looks good. What about a weekend trip?"
Enter
Sleep 500ms

Type "trvl weekend HEL --budget 400"
Enter
Sleep 10s

# ── CLOSING ──

Sleep 800ms
Type "# 34 MCP tools · 33 CLI commands · 18 providers · No API keys"
Enter
Sleep 400ms
Type "# brew install MikkoParkkola/tap/trvl"
Enter
Sleep 3s

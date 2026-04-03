Output demo.gif

Set Shell "bash"
Set FontSize 14
Set Width 900
Set Height 500
Set Theme "Catppuccin Mocha"

Type "# Search flights from Helsinki to Barcelona"
Enter
Sleep 500ms

Type "trvl flights HEL BCN 2026-07-01"
Enter
Sleep 8s

Type ""
Enter
Sleep 1s

Type "# Search hotels in Barcelona"
Enter
Sleep 500ms

Type "trvl hotels Barcelona --checkin 2026-07-01 --checkout 2026-07-08"
Enter
Sleep 8s

Type ""
Enter
Sleep 1s

Type "# Explore cheapest destinations from Helsinki"
Enter
Sleep 500ms

Type "trvl explore HEL --from 2026-07-01 --to 2026-07-14"
Enter
Sleep 10s

Sleep 2s

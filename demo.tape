Output demo.gif

Set Shell "bash"
Set FontSize 14
Set Width 1200
Set Height 600
Set Theme "Catppuccin Mocha"
Set Padding 20
Set TypingSpeed 30ms
Set CursorBlink false

# ── ACT 1: Natural language budget search ──

Type "# I have EUR 500 and a free week in June. Surprise me."
Enter
Sleep 600ms

Type "trvl discover --from 2026-06-01 --until 2026-06-30 --budget 500"
Enter
Sleep 400ms

Type@0ms "Top 5 value trips from HEL within 500 EUR (2026-06-01 to 2026-06-30)"
Enter
Type@0ms ""
Enter
Type@0ms "| # | Destination       | Dates                    | Nights | Flight  | Hotel                          | Total   | Slack  | Notes     |"
Enter
Type@0ms "+---+-------------------+--------------------------+--------+---------+--------------------------------+---------+--------+-----------+"
Enter
Type@0ms "| 1 | Vienna (VIE)      | 2026-06-09 -> 2026-06-12 | 3      | EUR 127 | EUR 138 (Hotel Capri)          | EUR 265 | EUR 235 | nonstop  |"
Enter
Type@0ms "| 2 | Krakow (KRK)      | 2026-06-16 -> 2026-06-19 | 3      | EUR 89  | EUR 102 (Hotel Stary)          | EUR 191 | EUR 309 | nonstop  |"
Enter
Type@0ms "| 3 | Dubrovnik (DBV)   | 2026-06-23 -> 2026-06-26 | 3      | EUR 167 | EUR 84 (Old Town Studios)      | EUR 251 | EUR 249 | 1 stop   |"
Enter
Type@0ms "| 4 | Lisbon (LIS)      | 2026-06-10 -> 2026-06-14 | 4      | EUR 189 | EUR 196 (Alma Lusa)            | EUR 385 | EUR 115 | 1 stop   |"
Enter
Type@0ms "| 5 | Milan (MXP)       | 2026-06-15 -> 2026-06-18 | 3      | EUR 109 | EUR 135 (Biocity Hotel)        | EUR 244 | EUR 256 | nonstop  |"
Enter

Sleep 5s

# ── ACT 2: Multi-city optimizer ──

Sleep 300ms
Type "# Prague, Krakow, Madrid — what's the cheapest order?"
Enter
Sleep 600ms

Type "trvl multi-city HEL --visit PRG,KRK,MAD --dates 2026-07-01,2026-07-21"
Enter
Sleep 400ms

Type@0ms "Multi-city from HEL: optimal order found (6 permutations checked)"
Enter
Type@0ms ""
Enter
Type@0ms "Route: HEL -> KRK -> PRG -> MAD -> HEL"
Enter
Type@0ms ""
Enter
Type@0ms "| From                  | To                    | Price   |"
Enter
Type@0ms "+-----------------------+-----------------------+---------+"
Enter
Type@0ms "| Helsinki (HEL)        | Krakow (KRK)          | EUR 89  |"
Enter
Type@0ms "| Krakow (KRK)          | Prague (PRG)           | EUR 34  |"
Enter
Type@0ms "| Prague (PRG)           | Madrid (MAD)           | EUR 112 |"
Enter
Type@0ms "| Madrid (MAD)           | Helsinki (HEL)         | EUR 148 |"
Enter
Type@0ms "|                       |                       |         |"
Enter
Type@0ms "| Total                 |                       | EUR 383 |"
Enter
Type@0ms "| Savings vs worst order |                       | EUR 174 |"
Enter

Sleep 5s

# ── ACT 3: Calendar-aware planning ──

Sleep 300ms
Type "# When should I go to Barcelona? Skip the 11-13th."
Enter
Sleep 600ms

Type 'trvl when --to BCN --from 2026-06-01 --until 2026-08-31 --busy 2026-07-11:2026-07-13'
Enter
Sleep 400ms

Type@0ms "Travel windows from HEL to BCN"
Enter
Type@0ms ""
Enter
Type@0ms "| # | Departure  | Return     | Nights | Flight  | Hotel   | Total   | Notes              |"
Enter
Type@0ms "+---+------------+------------+--------+---------+---------+---------+--------------------+"
Enter
Type@0ms "| 1 | 2026-06-16 | 2026-06-20 | 4      | EUR 139 | EUR 184 | EUR 323 | Hotel 1898         |"
Enter
Type@0ms "| 2 | 2026-06-23 | 2026-06-26 | 3      | EUR 127 | EUR 141 | EUR 268 | Casa Bonay         |"
Enter
Type@0ms "| 3 | 2026-08-04 | 2026-08-08 | 4      | EUR 158 | EUR 192 | EUR 350 | Hotel Brummell     |"
Enter
Type@0ms "| 4 | 2026-07-07 | 2026-07-10 | 3      | EUR 149 | EUR 135 | EUR 284 | Yurbban Passage    |"
Enter
Type@0ms "| 5 | 2026-08-18 | 2026-08-22 | 4      | EUR 144 | EUR 176 | EUR 320 | Sir Victor Hotel   |"
Enter

Sleep 5s

# ── ACT 4: Preference memory ──

Sleep 300ms
Type "# trvl remembers your style — neighborhoods, no hostels, real prices."
Enter
Sleep 600ms

Type "trvl prefs set preferred_districts Prague=Prague 1,Prague 5"
Enter
Sleep 300ms
Type@0ms 'Set preferred_districts["Prague"] = Prague 1, Prague 5'
Enter

Sleep 2s

# ── CLOSING ──

Sleep 400ms
Type "# 34 MCP tools · 33 CLI commands · 18 providers · No API keys"
Enter
Sleep 400ms
Type "# brew install MikkoParkkola/tap/trvl"
Enter
Sleep 3s

#!/usr/bin/env python3
"""
Browser-based train price scraper using Playwright.

Reads JSON from stdin:
  {"provider":"trainline","from":"London","to":"Paris","date":"2026-04-10","currency":"EUR"}

Writes JSON to stdout:
  {"routes":[{"price":39.00,"currency":"GBP","departure":"06:31","arrival":"09:47",
              "duration":196,"type":"train","provider":"eurostar","transfers":0}]}

On error:
  {"routes":[],"error":"reason"}
"""

import json
import sys
import re


def main():
    try:
        from playwright.sync_api import sync_playwright, TimeoutError as PWTimeoutError
    except ImportError:
        out([], "playwright not installed: pip install playwright && playwright install chromium")
        return

    raw = sys.stdin.read().strip()
    if not raw:
        out([], "no input on stdin")
        return

    try:
        inp = json.loads(raw)
    except json.JSONDecodeError as e:
        out([], f"invalid JSON input: {e}")
        return

    provider = inp.get("provider", "").lower()
    from_city = inp.get("from", "")
    to_city = inp.get("to", "")
    date = inp.get("date", "")
    currency = inp.get("currency", "EUR").upper()

    if not all([provider, from_city, to_city, date]):
        out([], "missing required fields: provider, from, to, date")
        return

    scrapers = {
        "trainline": scrape_trainline,
        "oebb": scrape_oebb,
        "sncf": scrape_sncf,
    }

    fn = scrapers.get(provider)
    if fn is None:
        out([], f"unsupported provider: {provider}")
        return

    try:
        with sync_playwright() as pw:
            browser = pw.chromium.launch(
                headless=True,
                args=[
                    "--no-sandbox",
                    "--disable-blink-features=AutomationControlled",
                    "--disable-dev-shm-usage",
                ],
            )
            context = browser.new_context(
                user_agent=(
                    "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) "
                    "AppleWebKit/537.36 (KHTML, like Gecko) "
                    "Chrome/133.0.0.0 Safari/537.36"
                ),
                locale="en-GB",
                viewport={"width": 1280, "height": 800},
            )
            # Mask webdriver flag to reduce bot detection.
            context.add_init_script(
                "Object.defineProperty(navigator, 'webdriver', {get: () => undefined})"
            )
            page = context.new_page()
            routes = fn(page, from_city, to_city, date, currency)
            browser.close()
            out(routes)
    except Exception as e:
        out([], f"{provider} scraper error: {e}")


# ---------------------------------------------------------------------------
# Trainline
# ---------------------------------------------------------------------------

# Station ID map matching trainline.go
TRAINLINE_STATIONS = {
    "london": "8267",
    "paris": "4916",
    "amsterdam": "8657",
    "brussels": "5893",
    "berlin": "7527",
    "munich": "7480",
    "frankfurt": "7604",
    "hamburg": "7626",
    "cologne": "21178",
    "vienna": "22644",
    "zurich": "6401",
    "milan": "8490",
    "rome": "8544",
    "barcelona": "6617",
    "madrid": "6663",
    "prague": "17587",
    "warsaw": "10491",
    "budapest": "18819",
    "copenhagen": "17515",
    "stockholm": "38711",
    "rotterdam": "23616",
    "lille": "4652",
    "lyon": "4718",
    "marseille": "4790",
    "nice": "4836",
    "strasbourg": "153",
    "toulouse": "5306",
    "venice": "8574",
    "florence": "8434",
    "salzburg": "6994",
    "innsbruck": "10461",
    "geneva": "5335",
    "basel": "5877",
    "antwerp": "5929",
}


def scrape_trainline(page, from_city, to_city, date, currency):
    from_id = TRAINLINE_STATIONS.get(from_city.lower())
    to_id = TRAINLINE_STATIONS.get(to_city.lower())
    if not from_id or not to_id:
        raise ValueError(f"no Trainline station ID for {from_city!r} or {to_city!r}")

    url = (
        "https://www.thetrainline.com/book/results"
        f"?journeySearchType=single"
        f"&origin=urn%3Atrainline%3Ageneric%3Aloc%3A{from_id}"
        f"&destination=urn%3Atrainline%3Ageneric%3Aloc%3A{to_id}"
        f"&outwardDate={date}T06%3A00%3A00"
        f"&outwardDateType=departAfter"
        f"&passengers%5B%5D=1996-01-01"
        f"&lang=en"
    )

    page.goto(url, timeout=30000, wait_until="domcontentloaded")
    _dismiss_cookies(page)

    # Wait for journey result cards — try multiple selectors.
    result_selectors = [
        "[data-test='search-result-card']",
        "[data-testid='journey-leg']",
        "[data-test='journey-card']",
        ".journey-result",
        "[class*='JourneyResult']",
        "[class*='journeyResult']",
        "[class*='SearchResult']",
    ]
    loaded_sel = None
    for sel in result_selectors:
        try:
            page.wait_for_selector(sel, timeout=20000)
            loaded_sel = sel
            break
        except Exception:
            continue

    if loaded_sel is None:
        # Dump page title to help debugging.
        raise RuntimeError(f"no result cards found on Trainline page (title: {page.title()!r})")

    routes = []
    cards = page.query_selector_all(loaded_sel)

    for card in cards[:10]:
        try:
            route = _parse_trainline_card(card, from_city, to_city, date, currency)
            if route:
                routes.append(route)
        except Exception:
            continue

    return routes


def _parse_trainline_card(card, from_city, to_city, date, currency):
    """Extract price/time data from a single Trainline journey card."""
    text = card.inner_text()
    if not text:
        return None

    # Extract price — look for £ / € / $ followed by digits.
    price = 0.0
    price_cur = currency
    price_m = re.search(r"([£€\$])\s*([\d,]+(?:\.\d{2})?)", text)
    if price_m:
        sym = price_m.group(1)
        price = float(price_m.group(2).replace(",", ""))
        price_cur = {"£": "GBP", "€": "EUR", "$": "USD"}.get(sym, currency)

    if price <= 0:
        return None

    # Extract times — HH:MM pattern.
    times = re.findall(r"\b(\d{1,2}:\d{2})\b", text)
    departure = times[0] if len(times) >= 1 else ""
    arrival = times[1] if len(times) >= 2 else ""

    # Derive departure/arrival ISO strings.
    dep_iso = f"{date}T{departure}:00" if departure else date
    arr_iso = f"{date}T{arrival}:00" if arrival else date

    # Duration — look for "Xh Ym" or "Xh" or similar.
    duration = 0
    dur_m = re.search(r"(\d+)\s*h(?:rs?)?\s*(?:(\d+)\s*m(?:in)?s?)?", text, re.IGNORECASE)
    if dur_m:
        duration = int(dur_m.group(1)) * 60 + int(dur_m.group(2) or 0)

    # Transfers — look for "direct" or "N change(s)".
    transfers = 0
    if re.search(r"\bdirect\b", text, re.IGNORECASE):
        transfers = 0
    else:
        chg_m = re.search(r"(\d+)\s+change", text, re.IGNORECASE)
        if chg_m:
            transfers = int(chg_m.group(1))

    # Provider — try to find carrier name.
    provider = "trainline"
    for carrier in ("Eurostar", "Thalys", "TGV", "Intercity", "ICE", "Ouigo", "SNCF"):
        if carrier.lower() in text.lower():
            provider = carrier.lower()
            break

    return {
        "price": price,
        "currency": price_cur,
        "departure": dep_iso,
        "arrival": arr_iso,
        "duration": duration,
        "type": "train",
        "provider": provider,
        "transfers": transfers,
        "booking_url": (
            f"https://www.thetrainline.com/book/trains/"
            f"{from_city.lower().replace(' ','-')}/"
            f"{to_city.lower().replace(' ','-')}/"
            f"{date}"
        ),
    }


# ---------------------------------------------------------------------------
# ÖBB
# ---------------------------------------------------------------------------

# ÖBB shop station ExtIDs (UIC/EVA) for browser URL construction.
OEBB_SHOP_STATIONS = {
    # Austria
    "vienna": "1190100",
    "wien": "1190100",
    "salzburg": "8100002",
    "innsbruck": "8100108",
    "graz": "8100173",
    "linz": "8100013",
    # Germany
    "munich": "8000261",
    "münchen": "8000261",
    "berlin": "8011160",
    "frankfurt": "8000105",
    "hamburg": "8002549",
    # Switzerland
    "zurich": "8503000",
    "zürich": "8503000",
    "geneva": "8501008",
    "basel": "8500010",
    # Italy
    "venice": "8300137",
    "milan": "8300046",
    "rome": "8300003",
    # Hungary
    "budapest": "5500017",
    # Czech Republic
    "prague": "5400014",
    "praha": "5400014",
    # Slovakia
    "bratislava": "5600002",
    # Slovenia
    "ljubljana": "7900001",
    # Croatia
    "zagreb": "7800001",
    # Poland
    "warsaw": "5100028",
    "krakow": "5100066",
}


def scrape_oebb(page, from_city, to_city, date, currency):
    from_id = OEBB_SHOP_STATIONS.get(from_city.lower())
    to_id = OEBB_SHOP_STATIONS.get(to_city.lower())
    if not from_id or not to_id:
        raise ValueError(f"no ÖBB station ID for {from_city!r} or {to_city!r}")

    url = (
        "https://shop.oebbtickets.at/en/ticket"
        f"?stationOrigExtId={from_id}"
        f"&stationDestExtId={to_id}"
        f"&outwardDate={date}"
        f"&passengers=ADULT"
    )

    page.goto(url, timeout=30000, wait_until="domcontentloaded")
    _dismiss_cookies(page)

    # Wait for connection/result list.
    result_selectors = [
        "[class*='connection']",
        "[class*='Connection']",
        "[class*='journey']",
        "[class*='Journey']",
        ".result-item",
        "[data-testid*='journey']",
        "[data-testid*='connection']",
    ]
    loaded_sel = None
    for sel in result_selectors:
        try:
            page.wait_for_selector(sel, timeout=20000)
            loaded_sel = sel
            break
        except Exception:
            continue

    if loaded_sel is None:
        raise RuntimeError(f"no connection cards found on ÖBB page (title: {page.title()!r})")

    routes = []
    cards = page.query_selector_all(loaded_sel)

    for card in cards[:10]:
        try:
            route = _parse_oebb_card(card, from_city, to_city, date, currency, from_id, to_id)
            if route:
                routes.append(route)
        except Exception:
            continue

    return routes


def _parse_oebb_card(card, from_city, to_city, date, currency, from_id, to_id):
    text = card.inner_text()
    if not text:
        return None

    # Extract price — EUR with comma or dot separator.
    price = 0.0
    price_cur = "EUR"
    price_m = re.search(r"([€])\s*([\d,]+(?:[.,]\d{2})?)|(\d+[.,]\d{2})\s*€", text)
    if price_m:
        raw = (price_m.group(2) or price_m.group(3) or "").replace(",", ".")
        try:
            price = float(raw)
        except ValueError:
            price = 0.0

    if price <= 0:
        return None

    times = re.findall(r"\b(\d{1,2}:\d{2})\b", text)
    departure = times[0] if len(times) >= 1 else ""
    arrival = times[1] if len(times) >= 2 else ""

    dep_iso = f"{date}T{departure}:00" if departure else date
    arr_iso = f"{date}T{arrival}:00" if arrival else date

    duration = 0
    dur_m = re.search(r"(\d+)\s*h(?:rs?)?\s*(?:(\d+)\s*m(?:in)?s?)?", text, re.IGNORECASE)
    if dur_m:
        duration = int(dur_m.group(1)) * 60 + int(dur_m.group(2) or 0)

    transfers = 0
    chg_m = re.search(r"(\d+)\s+(?:change|transfer|Umstieg)", text, re.IGNORECASE)
    if chg_m:
        transfers = int(chg_m.group(1))

    return {
        "price": price,
        "currency": price_cur,
        "departure": dep_iso,
        "arrival": arr_iso,
        "duration": duration,
        "type": "train",
        "provider": "oebb",
        "transfers": transfers,
        "booking_url": (
            f"https://tickets.oebb.at/en/ticket"
            f"?stationOrigExtId={from_id}"
            f"&stationDestExtId={to_id}"
            f"&outwardDate={date}"
        ),
    }


# ---------------------------------------------------------------------------
# SNCF
# ---------------------------------------------------------------------------

SNCF_STATION_CODES = {
    "paris": "FRPAR",
    "paris gare de lyon": "FRPLY",
    "paris nord": "FRPNO",
    "paris montparnasse": "FRPMO",
    "paris est": "FRPST",
    "lyon": "FRLYS",
    "marseille": "FRMRS",
    "bordeaux": "FRBOJ",
    "toulouse": "FRTLS",
    "nice": "FRNIC",
    "strasbourg": "FRSBG",
    "lille": "FRLIL",
    "nantes": "FRNTE",
    "montpellier": "FRMPL",
    "rennes": "FRRNS",
    "avignon": "FRAVT",
    "dijon": "FRDIJ",
    "brussels": "BEBMI",
    "geneva": "CHGVA",
    "zurich": "CHZRH",
    "barcelona": "ESBCN",
    "milan": "ITMIL",
    "frankfurt": "DEFRA",
    "london": "GBSPX",
}


def scrape_sncf(page, from_city, to_city, date, currency):
    from_code = SNCF_STATION_CODES.get(from_city.lower())
    to_code = SNCF_STATION_CODES.get(to_city.lower())
    if not from_code or not to_code:
        raise ValueError(f"no SNCF station code for {from_city!r} or {to_city!r}")

    url = f"https://www.sncf-connect.com/en-en/results/train/{from_code}/{to_code}/{date}"

    page.goto(url, timeout=30000, wait_until="domcontentloaded")
    _dismiss_cookies(page)

    result_selectors = [
        "[class*='journey']",
        "[class*='Journey']",
        "[class*='result']",
        "[class*='Result']",
        "[data-testid*='journey']",
        "[data-testid*='result']",
    ]
    loaded_sel = None
    for sel in result_selectors:
        try:
            page.wait_for_selector(sel, timeout=20000)
            loaded_sel = sel
            break
        except Exception:
            continue

    if loaded_sel is None:
        raise RuntimeError(f"no result cards found on SNCF page (title: {page.title()!r})")

    routes = []
    cards = page.query_selector_all(loaded_sel)

    for card in cards[:10]:
        try:
            route = _parse_sncf_card(card, from_city, to_city, date, currency, from_code, to_code)
            if route:
                routes.append(route)
        except Exception:
            continue

    return routes


def _parse_sncf_card(card, from_city, to_city, date, currency, from_code, to_code):
    text = card.inner_text()
    if not text:
        return None

    # SNCF prices in EUR with French locale (e.g. "29,00 €" or "€29.00").
    price = 0.0
    price_m = re.search(
        r"(\d+)[,.](\d{2})\s*€|€\s*(\d+)[,.](\d{2})", text
    )
    if price_m:
        if price_m.group(1):
            price = float(f"{price_m.group(1)}.{price_m.group(2)}")
        else:
            price = float(f"{price_m.group(3)}.{price_m.group(4)}")

    if price <= 0:
        return None

    times = re.findall(r"\b(\d{1,2}[hH]\d{2}|\d{1,2}:\d{2})\b", text)
    # Normalise "14h30" -> "14:30"
    times = [t.replace("h", ":").replace("H", ":") for t in times]
    departure = times[0] if len(times) >= 1 else ""
    arrival = times[1] if len(times) >= 2 else ""

    dep_iso = f"{date}T{departure}:00" if departure else date
    arr_iso = f"{date}T{arrival}:00" if arrival else date

    duration = 0
    dur_m = re.search(r"(\d+)\s*h(?:rs?)?\s*(?:(\d+)\s*m(?:in)?s?)?", text, re.IGNORECASE)
    if dur_m:
        duration = int(dur_m.group(1)) * 60 + int(dur_m.group(2) or 0)

    transfers = 0
    if re.search(r"\bdirect\b|\bsans changement\b", text, re.IGNORECASE):
        transfers = 0
    else:
        chg_m = re.search(r"(\d+)\s+(?:change|correspondance)", text, re.IGNORECASE)
        if chg_m:
            transfers = int(chg_m.group(1))

    return {
        "price": price,
        "currency": "EUR",
        "departure": dep_iso,
        "arrival": arr_iso,
        "duration": duration,
        "type": "train",
        "provider": "sncf",
        "transfers": transfers,
        "booking_url": (
            f"https://www.sncf-connect.com/en-en/result/train"
            f"/{from_code}/{to_code}/{date}"
        ),
    }


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _dismiss_cookies(page):
    """Accept cookie banners — try common button patterns."""
    selectors = [
        "button[id*='accept']",
        "button[id*='cookie']",
        "button[class*='accept']",
        "button[class*='cookie']",
        "[data-testid*='cookie'] button",
        "#onetrust-accept-btn-handler",
        ".cookie-accept",
        "button:has-text('Accept all')",
        "button:has-text('Accept cookies')",
        "button:has-text('I agree')",
        "button:has-text('Agree')",
        "button:has-text('OK')",
    ]
    for sel in selectors:
        try:
            btn = page.query_selector(sel)
            if btn and btn.is_visible():
                btn.click()
                page.wait_for_timeout(500)
                return
        except Exception:
            continue


def out(routes, error=None):
    payload = {"routes": routes}
    if error:
        payload["error"] = error
    print(json.dumps(payload), flush=True)


if __name__ == "__main__":
    main()

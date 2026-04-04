package ground

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/MikkoParkkola/trvl/internal/models"
	"golang.org/x/time/rate"
)

// oebbEndpoint is the ÖBB HAFAS mgate endpoint used by the fahrplan.oebb.at web app.
const oebbEndpoint = "https://fahrplan.oebb.at/bin/mgate.exe"

// oebbLimiter: conservative 5 req/min.
var oebbLimiter = rate.NewLimiter(rate.Every(12*time.Second), 1)

// oebbClient is a shared HTTP client for ÖBB API calls.
var oebbClient = &http.Client{
	Timeout: 30 * time.Second,
}

// oebbStation holds an ÖBB/HAFAS station entry.
type oebbStation struct {
	// extId is the ÖBB/HAFAS external ID (EVA/UIC number).
	ExtID string
	Name  string
	City  string
}

// oebbStations maps lowercase city name to ÖBB station metadata.
// ExtIDs are EVA numbers verified against fahrplan.oebb.at.
var oebbStations = map[string]oebbStation{
	// Austria — home network
	"vienna":    {ExtID: "1190100", Name: "Wien Hbf", City: "Vienna"},
	"wien":      {ExtID: "1190100", Name: "Wien Hbf", City: "Vienna"},
	"salzburg":  {ExtID: "8100002", Name: "Salzburg Hbf", City: "Salzburg"},
	"innsbruck": {ExtID: "8100108", Name: "Innsbruck Hbf", City: "Innsbruck"},
	"graz":      {ExtID: "8100173", Name: "Graz Hbf", City: "Graz"},
	"linz":      {ExtID: "8100013", Name: "Linz Hbf", City: "Linz"},
	"klagenfurt": {ExtID: "8100085", Name: "Klagenfurt Hbf", City: "Klagenfurt"},
	"villach":   {ExtID: "8100071", Name: "Villach Hbf", City: "Villach"},
	"bregenz":   {ExtID: "8100356", Name: "Bregenz", City: "Bregenz"},
	"feldkirch": {ExtID: "8100358", Name: "Feldkirch", City: "Feldkirch"},

	// Germany (served by ÖBB Railjet/Nightjet)
	"munich":    {ExtID: "8000261", Name: "München Hbf", City: "Munich"},
	"münchen":   {ExtID: "8000261", Name: "München Hbf", City: "Munich"},
	"berlin":    {ExtID: "8011160", Name: "Berlin Hbf", City: "Berlin"},
	"frankfurt": {ExtID: "8000105", Name: "Frankfurt(Main)Hbf", City: "Frankfurt"},
	"hamburg":   {ExtID: "8002549", Name: "Hamburg Hbf", City: "Hamburg"},
	"stuttgart": {ExtID: "8000096", Name: "Stuttgart Hbf", City: "Stuttgart"},

	// Switzerland
	"zurich": {ExtID: "8503000", Name: "Zürich HB", City: "Zurich"},
	"zürich": {ExtID: "8503000", Name: "Zürich HB", City: "Zurich"},
	"geneva": {ExtID: "8501008", Name: "Genève", City: "Geneva"},
	"basel":  {ExtID: "8500010", Name: "Basel SBB", City: "Basel"},
	"bern":   {ExtID: "8507000", Name: "Bern", City: "Bern"},

	// Italy (served by ÖBB/Trenitalia Railjet)
	"venice":  {ExtID: "8300137", Name: "Venezia Santa Lucia", City: "Venice"},
	"verona":  {ExtID: "8300066", Name: "Verona P.N.", City: "Verona"},
	"milan":   {ExtID: "8300046", Name: "Milano Centrale", City: "Milan"},
	"rome":    {ExtID: "8300003", Name: "Roma Termini", City: "Rome"},
	"bologna": {ExtID: "8300027", Name: "Bologna Centrale", City: "Bologna"},

	// Hungary
	"budapest": {ExtID: "5500017", Name: "Budapest-Keleti", City: "Budapest"},

	// Czech Republic
	"prague": {ExtID: "5400014", Name: "Praha hl.n.", City: "Prague"},
	"praha":  {ExtID: "5400014", Name: "Praha hl.n.", City: "Prague"},

	// Slovakia
	"bratislava": {ExtID: "5600002", Name: "Bratislava hl.st.", City: "Bratislava"},

	// Slovenia
	"ljubljana": {ExtID: "7900001", Name: "Ljubljana", City: "Ljubljana"},

	// Croatia
	"zagreb": {ExtID: "7800001", Name: "Zagreb Gl. kol.", City: "Zagreb"},

	// Poland
	"warsaw":  {ExtID: "5100028", Name: "Warszawa Centralna", City: "Warsaw"},
	"krakow":  {ExtID: "5100066", Name: "Kraków Główny", City: "Krakow"},
	"kraków":  {ExtID: "5100066", Name: "Kraków Główny", City: "Krakow"},
}

// LookupOebbStation resolves a city name to an ÖBB station (case-insensitive).
func LookupOebbStation(city string) (oebbStation, bool) {
	s, ok := oebbStations[strings.ToLower(strings.TrimSpace(city))]
	return s, ok
}

// HasOebbStation returns true if the city has a known ÖBB station.
func HasOebbStation(city string) bool {
	_, ok := LookupOebbStation(city)
	return ok
}

// HasOebbRoute returns true if both cities are in the ÖBB network.
// ÖBB focuses on Austria and neighbouring countries (DE, CH, IT, HU, CZ, SK, SI, HR).
func HasOebbRoute(from, to string) bool {
	return HasOebbStation(from) && HasOebbStation(to)
}

// oebbTripSearchRequest builds the HAFAS mgate JSON envelope for a trip search.
func oebbTripSearchRequest(fromExtID, toExtID, dateStr, timeStr string) map[string]any {
	return map[string]any{
		"auth": map[string]any{
			"aid": "OWDL4fE4ixNiPBBm",
			"type": "AID",
		},
		"client": map[string]any{
			"id":   "OEBB",
			"name": "OEBB",
			"os":   "Windows NT 10.0",
			"type": "WEB",
			"ua":   "Mozilla/5.0",
			"v":    100,
		},
		"ext":      "OEBB.1",
		"formatted": false,
		"lang":     "en",
		"svcReqL": []map[string]any{
			{
				"cfg": map[string]any{"polyEnc": "GPA"},
				"meth": "TripSearch",
				"req": map[string]any{
					"arrLocL": []map[string]any{
						{"extId": toExtID, "type": "S"},
					},
					"depLocL": []map[string]any{
						{"extId": fromExtID, "type": "S"},
					},
					"extChgTime": -1,
					"getPasslist": false,
					"getPolyline": false,
					"jnyFltrL": []map[string]any{
						{"mode": "BIT", "type": "PROD", "value": "1111111111111111"},
					},
					"numF": 5,
					"outDate": dateStr,
					"outTime": timeStr,
					"outFrwd": true,
					// trfReq omitted — causes "empty svcResL" error on ÖBB HAFAS.
					// Fares need a separate query or different HAFAS method.
				},
			},
		},
		"ver": "1.45",
	}
}

// oebbMgateResponse is the top-level mgate response envelope.
type oebbMgateResponse struct {
	SvcResL []oebbSvcRes `json:"svcResL"`
}

type oebbSvcRes struct {
	Meth string          `json:"meth"`
	Res  oebbTripRes     `json:"res"`
	Err  string          `json:"err,omitempty"`
}

type oebbTripRes struct {
	Common    oebbCommon  `json:"common"`
	OutConL   []oebbCon   `json:"outConL"`
}

type oebbCommon struct {
	LocL    []oebbLoc    `json:"locL"`
	OpL     []oebbOp     `json:"opL"`
	ProdL   []oebbProd   `json:"prodL"`
}

type oebbLoc struct {
	Name  string `json:"name"`
	ExtID string `json:"extId,omitempty"`
}

type oebbOp struct {
	Name string `json:"name"`
}

type oebbProd struct {
	Name  string `json:"name"`
	OpIdx int    `json:"oprX,omitempty"`
}

type oebbCon struct {
	Dep     oebbConStop  `json:"dep"`
	Arr     oebbConStop  `json:"arr"`
	SecL    []oebbSec    `json:"secL"`
	TrfRes  *oebbTrfRes  `json:"trfRes,omitempty"`
	CHG     int          `json:"chg"` // number of changes
	Dur     string       `json:"dur"` // "HHMMSS" format (e.g. "041300" = 4h 13m)
	Date    string       `json:"date"` // connection date "YYYYMMDD" (e.g. "20260410")
}

type oebbConStop struct {
	// dTimeS/aTimeS = scheduled time (HHMMSS string), no date — use con.Date
	DTimeS  string `json:"dTimeS,omitempty"`
	ATimeS  string `json:"aTimeS,omitempty"`
	LocX    int    `json:"locX"`
}

type oebbSec struct {
	Type    string      `json:"type"`     // "JNY", "WALK"
	Dep     oebbConStop `json:"dep"`
	Arr     oebbConStop `json:"arr"`
	JnyL    *oebbJny    `json:"jny,omitempty"`
}

type oebbJny struct {
	ProdX int `json:"prodX"`
}

type oebbTrfRes struct {
	FareSetL []oebbFareSet `json:"fareSetL"`
}

type oebbFareSet struct {
	Desc  string      `json:"desc"`
	FareL []oebbFare  `json:"fareL"`
}

type oebbFare struct {
	Name  string `json:"name"`
	Price int    `json:"prc"` // cents — HAFAS uses "prc" not "price"
	Cur   string `json:"cur"`
}

// SearchOebb searches ÖBB (Austrian Federal Railways) for train journeys between two cities.
func SearchOebb(ctx context.Context, from, to, date, currency string) ([]models.GroundRoute, error) {
	fromStation, ok := LookupOebbStation(from)
	if !ok {
		return nil, fmt.Errorf("no ÖBB station for %q", from)
	}
	toStation, ok := LookupOebbStation(to)
	if !ok {
		return nil, fmt.Errorf("no ÖBB station for %q", to)
	}

	if currency == "" {
		currency = "EUR"
	}

	// ÖBB HAFAS date format: YYYYMMDD, time: HHMMSS
	dt, err := time.Parse("2006-01-02", date)
	if err != nil {
		return nil, fmt.Errorf("invalid date %q: %w", date, err)
	}
	dateStr := dt.Format("20060102")
	timeStr := "060000" // 06:00:00

	reqBody := oebbTripSearchRequest(fromStation.ExtID, toStation.ExtID, dateStr, timeStr)
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("oebb marshal: %w", err)
	}

	if err := oebbLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("oebb rate limiter: %w", err)
	}

	reqURL := oebbEndpoint + "?rnd=" + fmt.Sprintf("%d", time.Now().UnixMilli())
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://fahrplan.oebb.at/")
	req.Header.Set("Origin", "https://fahrplan.oebb.at")

	slog.Debug("oebb search", "from", fromStation.City, "to", toStation.City, "date", date)

	resp, err := oebbClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("oebb search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("oebb: HTTP %d: %s", resp.StatusCode, respBody)
	}

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("oebb read: %w", err)
	}
	slog.Debug("oebb raw response", "status", resp.StatusCode, "body_len", len(respBody))

	var mgateResp oebbMgateResponse
	if err := json.Unmarshal(respBody, &mgateResp); err != nil {
		return nil, fmt.Errorf("oebb decode: %w", err)
	}

	if len(mgateResp.SvcResL) == 0 {
		return nil, fmt.Errorf("oebb: empty svcResL")
	}
	svc := mgateResp.SvcResL[0]
	if svc.Err != "" && svc.Err != "OK" {
		return nil, fmt.Errorf("oebb api error: %s", svc.Err)
	}

	routes := parseOebbConnections(svc.Res, fromStation, toStation, date, currency)
	slog.Debug("oebb results", "outConL", len(svc.Res.OutConL), "parsed", len(routes))

	// ÖBB HAFAS often omits fares (trfReq causes API errors). If no priced
	// routes came back, try the browser scraper against shop.oebbtickets.at
	// which renders real prices.
	hasPrices := false
	for _, r := range routes {
		if r.Price > 0 {
			hasPrices = true
			break
		}
	}
	if !hasPrices {
		slog.Debug("oebb no prices from HAFAS — trying browser scraper")
		if bRoutes, bErr := BrowserScrapeRoutes(ctx, "oebb", from, to, date, currency); bErr == nil && len(bRoutes) > 0 {
			// Merge: browser routes take precedence for pricing; keep HAFAS for schedule detail.
			return bRoutes, nil
		} else if bErr != nil {
			slog.Debug("oebb browser scraper failed", "err", bErr)
		}
	}

	return routes, nil
}

// parseOebbConnections converts HAFAS connections to GroundRoute models.
func parseOebbConnections(res oebbTripRes, fromStation, toStation oebbStation, searchDate, currency string) []models.GroundRoute {
	var routes []models.GroundRoute

	for _, con := range res.OutConL {
		// Parse departure and arrival times.
		// HAFAS puts the date on the connection (con.Date = "YYYYMMDD"),
		// not on the individual stop. Arrival may be next-day when time wraps past 2359.
		depTime := oebbParseDateTime(con.Date, con.Dep.DTimeS)
		arrTime := oebbParseDateTime(con.Date, con.Arr.ATimeS)

		// Duration from "dHHMMSS" field, fallback to computed.
		duration := oebbParseDuration(con.Dur)
		if duration == 0 {
			duration = computeDBDuration(depTime, arrTime)
		}

		// Extract price from tariff result.
		price := 0.0
		priceCur := strings.ToUpper(currency)
		if con.TrfRes != nil {
			for _, fs := range con.TrfRes.FareSetL {
				for _, fare := range fs.FareL {
					if fare.Price > 0 {
						p := float64(fare.Price) / 100.0
						if price == 0 || p < price {
							price = p
							if fare.Cur != "" {
								priceCur = strings.ToUpper(fare.Cur)
							}
						}
					}
				}
			}
		}

		// Count JNY (journey) sections to determine transfers.
		jnySections := 0
		for _, sec := range con.SecL {
			if sec.Type == "JNY" {
				jnySections++
			}
		}
		transfers := jnySections - 1
		if transfers < 0 {
			transfers = 0
		}

		// Build legs.
		var legs []models.GroundLeg
		for _, sec := range con.SecL {
			if sec.Type != "JNY" {
				continue
			}
			legDep := oebbParseDateTime(con.Date, sec.Dep.DTimeS)
			legArr := oebbParseDateTime(con.Date, sec.Arr.ATimeS)

			legProvider := ""
			if sec.JnyL != nil && sec.JnyL.ProdX >= 0 && sec.JnyL.ProdX < len(res.Common.ProdL) {
				legProvider = res.Common.ProdL[sec.JnyL.ProdX].Name
			}

			depName := fromStation.City
			if sec.Dep.LocX >= 0 && sec.Dep.LocX < len(res.Common.LocL) {
				depName = res.Common.LocL[sec.Dep.LocX].Name
			}
			arrName := toStation.City
			if sec.Arr.LocX >= 0 && sec.Arr.LocX < len(res.Common.LocL) {
				arrName = res.Common.LocL[sec.Arr.LocX].Name
			}

			legs = append(legs, models.GroundLeg{
				Type:     "train",
				Provider: legProvider,
				Departure: models.GroundStop{
					City: depName,
					Time: legDep,
				},
				Arrival: models.GroundStop{
					City: arrName,
					Time: legArr,
				},
				Duration: computeDBDuration(legDep, legArr),
			})
		}

		routes = append(routes, models.GroundRoute{
			Provider: "oebb",
			Type:     "train",
			Price:    price,
			Currency: priceCur,
			Duration: duration,
			Departure: models.GroundStop{
				City:    fromStation.City,
				Station: fromStation.Name,
				Time:    depTime,
			},
			Arrival: models.GroundStop{
				City:    toStation.City,
				Station: toStation.Name,
				Time:    arrTime,
			},
			Transfers:  transfers,
			Legs:       legs,
			BookingURL: buildOebbBookingURL(fromStation, toStation, searchDate),
		})
	}

	return routes
}

// oebbParseDateTime converts HAFAS date (YYYYMMDD) + time (HHMMSS) to ISO 8601.
// HAFAS puts the date on the connection, not the stop. Stops only carry HHMMSS.
// Time may be ≥240000 when the journey crosses midnight (e.g. "250000" = 01:00 next day).
func oebbParseDateTime(dateS, timeS string) string {
	if dateS == "" || timeS == "" {
		return ""
	}
	// Pad time to 6 digits in case leading zeros were dropped.
	for len(timeS) < 6 {
		timeS = "0" + timeS
	}

	// Handle day-overflow: HAFAS encodes next-day arrivals as hour ≥ 24.
	extraDays := 0
	hh := 0
	if len(timeS) >= 2 {
		if _, err := fmt.Sscanf(timeS[:2], "%d", &hh); err == nil && hh >= 24 {
			extraDays = hh / 24
			hh = hh % 24
			timeS = fmt.Sprintf("%02d%s", hh, timeS[2:])
		}
	}

	t, err := time.Parse("20060102150405", dateS+timeS)
	if err != nil {
		return ""
	}
	if extraDays > 0 {
		t = t.Add(time.Duration(extraDays) * 24 * time.Hour)
	}
	return t.Format("2006-01-02T15:04:05")
}

// oebbParseDuration parses the HAFAS duration string to minutes.
// HAFAS returns "HHMMSS" (6 chars, e.g. "041300" = 4h 13m 0s = 253 min)
// or occasionally "DHHMMSS" (7 chars with a leading day digit).
func oebbParseDuration(dur string) int {
	switch len(dur) {
	case 6:
		// "HHMMSS"
		hh, mm := 0, 0
		if _, err := fmt.Sscanf(dur[0:2], "%d", &hh); err != nil {
			return 0
		}
		if _, err := fmt.Sscanf(dur[2:4], "%d", &mm); err != nil {
			return 0
		}
		return hh*60 + mm
	case 7:
		// "DHHMMSS" where D is days digit
		days := int(dur[0] - '0')
		hh, mm := 0, 0
		if _, err := fmt.Sscanf(dur[1:3], "%d", &hh); err != nil {
			return 0
		}
		if _, err := fmt.Sscanf(dur[3:5], "%d", &mm); err != nil {
			return 0
		}
		return days*24*60 + hh*60 + mm
	default:
		return 0
	}
}

// buildOebbBookingURL constructs a fahrplan.oebb.at booking URL.
func buildOebbBookingURL(from, to oebbStation, date string) string {
	return fmt.Sprintf("https://tickets.oebb.at/en/ticket?stationOrigExtId=%s&stationDestExtId=%s&outwardDate=%s",
		url.QueryEscape(from.ExtID),
		url.QueryEscape(to.ExtID),
		url.QueryEscape(date),
	)
}


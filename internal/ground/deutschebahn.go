package ground

import (
	"bytes"
	"context"
	"crypto/rand"
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

const dbJourneysEndpoint = "https://app.services-bahn.de/mob/angebote/fahrplan"

// generateCorrelationID creates a DB-compatible correlation ID (uuid_uuid format).
func generateCorrelationID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x_%x-%x-%x-%x-%x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16],
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// dbLimiter enforces a conservative rate limit: 30 req/min (half of the actual 60/min limit).
var dbLimiter = rate.NewLimiter(rate.Every(2*time.Second), 1)

// dbClient is a dedicated HTTP client for Deutsche Bahn API calls.
var dbClient = &http.Client{
	Timeout: 30 * time.Second,
}

// DBStation holds metadata for a Deutsche Bahn station.
type DBStation struct {
	EVA     string // EVA number (IBNR)
	Name    string
	City    string
	Country string
}

// dbStations maps lowercase city name to station info.
// EVA numbers sourced from Deutsche Bahn's public station data.
var dbStations = map[string]DBStation{
	// Germany
	"berlin":      {EVA: "8011160", Name: "Berlin Hbf", City: "Berlin", Country: "DE"},
	"munich":      {EVA: "8000261", Name: "München Hbf", City: "Munich", Country: "DE"},
	"münchen":     {EVA: "8000261", Name: "München Hbf", City: "Munich", Country: "DE"},
	"frankfurt":   {EVA: "8000105", Name: "Frankfurt(Main)Hbf", City: "Frankfurt", Country: "DE"},
	"hamburg":     {EVA: "8002549", Name: "Hamburg Hbf", City: "Hamburg", Country: "DE"},
	"cologne":     {EVA: "8000207", Name: "Köln Hbf", City: "Cologne", Country: "DE"},
	"köln":        {EVA: "8000207", Name: "Köln Hbf", City: "Cologne", Country: "DE"},
	"düsseldorf":  {EVA: "8000085", Name: "Düsseldorf Hbf", City: "Düsseldorf", Country: "DE"},
	"dusseldorf":  {EVA: "8000085", Name: "Düsseldorf Hbf", City: "Düsseldorf", Country: "DE"},
	"stuttgart":   {EVA: "8000096", Name: "Stuttgart Hbf", City: "Stuttgart", Country: "DE"},
	"nuremberg":   {EVA: "8000284", Name: "Nürnberg Hbf", City: "Nuremberg", Country: "DE"},
	"nürnberg":    {EVA: "8000284", Name: "Nürnberg Hbf", City: "Nuremberg", Country: "DE"},
	"hannover":    {EVA: "8000152", Name: "Hannover Hbf", City: "Hannover", Country: "DE"},
	"hanover":     {EVA: "8000152", Name: "Hannover Hbf", City: "Hannover", Country: "DE"},
	"leipzig":     {EVA: "8010205", Name: "Leipzig Hbf", City: "Leipzig", Country: "DE"},
	"dresden":     {EVA: "8010085", Name: "Dresden Hbf", City: "Dresden", Country: "DE"},
	"bremen":      {EVA: "8000050", Name: "Bremen Hbf", City: "Bremen", Country: "DE"},
	"freiburg":    {EVA: "8000107", Name: "Freiburg(Breisgau) Hbf", City: "Freiburg", Country: "DE"},
	"karlsruhe":   {EVA: "8000191", Name: "Karlsruhe Hbf", City: "Karlsruhe", Country: "DE"},
	"mannheim":    {EVA: "8000244", Name: "Mannheim Hbf", City: "Mannheim", Country: "DE"},
	"augsburg":    {EVA: "8000013", Name: "Augsburg Hbf", City: "Augsburg", Country: "DE"},
	"dortmund":    {EVA: "8000080", Name: "Dortmund Hbf", City: "Dortmund", Country: "DE"},
	"essen":       {EVA: "8000098", Name: "Essen Hbf", City: "Essen", Country: "DE"},
	"aachen":      {EVA: "8000001", Name: "Aachen Hbf", City: "Aachen", Country: "DE"},

	// Austria
	"vienna": {EVA: "8101003", Name: "Wien Hbf", City: "Vienna", Country: "AT"},
	"wien":   {EVA: "8101003", Name: "Wien Hbf", City: "Vienna", Country: "AT"},
	"salzburg": {EVA: "8100002", Name: "Salzburg Hbf", City: "Salzburg", Country: "AT"},
	"innsbruck": {EVA: "8100108", Name: "Innsbruck Hbf", City: "Innsbruck", Country: "AT"},

	// Switzerland
	"zurich": {EVA: "8503000", Name: "Zürich HB", City: "Zurich", Country: "CH"},
	"zürich": {EVA: "8503000", Name: "Zürich HB", City: "Zurich", Country: "CH"},
	"basel":  {EVA: "8500010", Name: "Basel SBB", City: "Basel", Country: "CH"},
	"bern":   {EVA: "8507000", Name: "Bern", City: "Bern", Country: "CH"},

	// Netherlands
	"amsterdam": {EVA: "8400058", Name: "Amsterdam Centraal", City: "Amsterdam", Country: "NL"},
	"rotterdam":  {EVA: "8400530", Name: "Rotterdam Centraal", City: "Rotterdam", Country: "NL"},

	// Belgium
	"brussels": {EVA: "8814001", Name: "Bruxelles-Midi", City: "Brussels", Country: "BE"},

	// Czech Republic
	"prague": {EVA: "5400014", Name: "Praha hl.n.", City: "Prague", Country: "CZ"},
	"praha":  {EVA: "5400014", Name: "Praha hl.n.", City: "Prague", Country: "CZ"},

	// Poland
	"warsaw": {EVA: "5100028", Name: "Warszawa Centralna", City: "Warsaw", Country: "PL"},

	// Hungary
	"budapest": {EVA: "5500017", Name: "Budapest-Keleti", City: "Budapest", Country: "HU"},

	// Denmark
	"copenhagen": {EVA: "8600626", Name: "København H", City: "Copenhagen", Country: "DK"},

	// France
	"paris":      {EVA: "8727100", Name: "Paris Gare du Nord", City: "Paris", Country: "FR"},
	"strasbourg": {EVA: "8700011", Name: "Strasbourg", City: "Strasbourg", Country: "FR"},

	// Italy
	"milan": {EVA: "8300046", Name: "Milano Centrale", City: "Milan", Country: "IT"},

	// Luxembourg
	"luxembourg": {EVA: "8200100", Name: "Luxembourg", City: "Luxembourg", Country: "LU"},
}

// LookupDBStation resolves a city name to a DB station (case-insensitive).
func LookupDBStation(city string) (DBStation, bool) {
	s, ok := dbStations[strings.ToLower(strings.TrimSpace(city))]
	return s, ok
}

// HasDBStation returns true if the city has a known DB station.
func HasDBStation(city string) bool {
	_, ok := LookupDBStation(city)
	return ok
}

// HasDBRoute returns true if at least one of the two cities has a DB station.
// DB covers most European cities so we search if either end is in the network.
func HasDBRoute(from, to string) bool {
	return HasDBStation(from) && HasDBStation(to)
}

// dbJourneysRequest builds the JSON request body for the DB Vendo journeys API.
// The structure matches the DB Navigator app's "angebote/fahrplan" endpoint.
func dbJourneysRequest(fromEVA, toEVA string, when time.Time) map[string]any {
	fromLid := fmt.Sprintf("A=1@L=%s@", fromEVA)
	toLid := fmt.Sprintf("A=1@L=%s@", toEVA)

	return map[string]any{
		"autonomeReservierung": false,
		"einstiegsTypList":     []string{"STANDARD"},
		"fahrverguenstigungen": map[string]any{
			"deutschlandTicketVorhanden":       false,
			"nurDeutschlandTicketVerbindungen": false,
		},
		"klasse": "KLASSE_2",
		"reisendenProfil": map[string]any{
			"reisende": []map[string]any{
				{
					"ermaessigungen": []string{"KEINE_ERMAESSIGUNG KLASSENLOS"},
					"reisendenTyp":   "ERWACHSENER",
				},
			},
		},
		"reservierungsKontingenteVorhanden": false,
		"reiseHin": map[string]any{
			"wunsch": map[string]any{
				"abgangsLocationId":          fromLid,
				"zielLocationId":             toLid,
				"verkehrsmittel":             []string{"ALL"},
				"alternativeHalteBerechnung": true,
				"zeitWunsch": map[string]any{
					"reiseDatum":  when.Format("2006-01-02T15:04:05"),
					"zeitPunktArt": "ABFAHRT",
				},
			},
		},
	}
}

// dbJourneysResponse represents the top-level API response for journey search.
type dbJourneysResponse struct {
	Verbindungen    []dbVerbindung `json:"verbindungen"`
	FehlerNachricht *dbError       `json:"fehlerNachricht,omitempty"`
}

type dbError struct {
	Code         string `json:"code"`
	Ueberschrift string `json:"ueberschrift"`
	Text         string `json:"text"`
}

type dbVerbindung struct {
	Verbindung              *dbVerbindungInner `json:"verbindung,omitempty"`
	Angebote                *dbAngebote        `json:"angebote,omitempty"`
	VerbindungsAbschnitte   []dbAbschnitt      `json:"verbindungsAbschnitte,omitempty"`
	AngebotsPreis           *dbPreis           `json:"angebotsPreis,omitempty"`
	AbPreis                 *dbPreis           `json:"abPreis,omitempty"`
}

type dbVerbindungInner struct {
	VerbindungsAbschnitte []dbAbschnitt `json:"verbindungsAbschnitte"`
	ReiseDauer            int           `json:"reiseDauer"`           // seconds
	UmstiegeAnzahl        int           `json:"umstiegeAnzahl"`
}

type dbAngebote struct {
	Preise *dbPreise `json:"preise,omitempty"`
}

type dbPreise struct {
	IstTeilpreis bool     `json:"istTeilpreis"`
	Gesamt       *dbGesamt `json:"gesamt,omitempty"`
}

type dbGesamt struct {
	Klasse string  `json:"klasse"`
	Ab     *dbPreis `json:"ab,omitempty"`
}

type dbPreis struct {
	Betrag   float64 `json:"betrag"`
	Waehrung string  `json:"waehrung"`
}

type dbAbschnitt struct {
	AbgangsDatum        string          `json:"abgangsDatum"`
	AnkunftsDatum       string          `json:"ankunftsDatum"`
	AbfahrtsZeitpunkt   string          `json:"abfahrtsZeitpunkt,omitempty"`
	AnkunftsZeitpunkt   string          `json:"ankunftsZeitpunkt,omitempty"`
	AbgangsOrt          *dbOrt          `json:"abgangsOrt,omitempty"`
	AnkunftsOrt         *dbOrt          `json:"ankunftsOrt,omitempty"`
	AnkunftsOrtObj      *dbOrt          `json:"ankunftsOrtObj,omitempty"`
	Typ                 string          `json:"typ"`           // "FAHRZEUG", "FUSSWEG", "TRANSFER", "WALK"
	Langtext            string          `json:"langtext"`
	Mitteltext          string          `json:"mitteltext"`
	Kurztext            string          `json:"kurztext"`
	ProduktGattung      string          `json:"produktGattung"`
	Verkehrsmittel      *dbVerkehrsmittel `json:"verkehrsmittel,omitempty"`
	Halte               []dbHalt        `json:"halte"`
}

type dbVerkehrsmittel struct {
	Name     string `json:"name"`
	LangText string `json:"langText"`
	KurzText string `json:"kurzText"`
}

type dbOrt struct {
	Name  string `json:"name"`
	EvaNr string `json:"evaNr"`
}

type dbHalt struct {
	AbgangsDatum  string `json:"abgangsDatum"`
	AnkunftsDatum string `json:"ankunftsDatum"`
	Ort           *dbOrt `json:"ort,omitempty"`
	Gleis         string `json:"gleis"`
}

// SearchDeutscheBahn searches Deutsche Bahn for train journeys between two cities.
func SearchDeutscheBahn(ctx context.Context, from, to, date, currency string) ([]models.GroundRoute, error) {
	fromStation, ok := LookupDBStation(from)
	if !ok {
		return nil, fmt.Errorf("no DB station for %q", from)
	}
	toStation, ok := LookupDBStation(to)
	if !ok {
		return nil, fmt.Errorf("no DB station for %q", to)
	}

	if currency == "" {
		currency = "EUR"
	}

	// Parse the date and set departure to morning (06:00 local time).
	when, err := time.Parse("2006-01-02", date)
	if err != nil {
		return nil, fmt.Errorf("invalid date %q: %w", date, err)
	}
	when = when.Add(6 * time.Hour) // 06:00

	reqBody := dbJourneysRequest(fromStation.EVA, toStation.EVA, when)
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("db marshal request: %w", err)
	}

	// Wait for rate limiter.
	if err := dbLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("db rate limiter: %w", err)
	}

	contentType := "application/x.db.vendo.mob.verbindungssuche.v9+json"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, dbJourneysEndpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", contentType)
	req.Header.Set("Accept-Language", "en")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Linux; Android 14) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Mobile Safari/537.36")
	req.Header.Set("X-Correlation-ID", generateCorrelationID())

	slog.Debug("db search", "from", fromStation.City, "to", toStation.City, "date", date)

	resp, err := dbClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("db search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("db search: HTTP %d: %s", resp.StatusCode, respBody)
	}

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("db read: %w", err)
	}
	slog.Debug("db response", "status", resp.StatusCode, "body_len", len(respBody))

	var dbResp dbJourneysResponse
	if err := json.Unmarshal(respBody, &dbResp); err != nil {
		return nil, fmt.Errorf("db decode: %w", err)
	}

	slog.Debug("db parsed", "verbindungen", len(dbResp.Verbindungen), "body_len", len(respBody))

	if dbResp.FehlerNachricht != nil && dbResp.FehlerNachricht.Code != "" {
		return nil, fmt.Errorf("db api error: %s: %s", dbResp.FehlerNachricht.Code, dbResp.FehlerNachricht.Ueberschrift)
	}

	routes := parseDBVerbindungen(dbResp.Verbindungen, fromStation, toStation, currency)
	slog.Debug("db routes", "count", len(routes))
	return routes, nil
}

// parseDBVerbindungen converts raw DB API connections into GroundRoute models.
func parseDBVerbindungen(verbindungen []dbVerbindung, fromStation, toStation DBStation, currency string) []models.GroundRoute {
	var routes []models.GroundRoute
	for _, v := range verbindungen {
		// The response may nest the journey under .verbindung or directly.
		abschnitte := v.VerbindungsAbschnitte
		if v.Verbindung != nil && len(v.Verbindung.VerbindungsAbschnitte) > 0 {
			abschnitte = v.Verbindung.VerbindungsAbschnitte
		}
		if len(abschnitte) == 0 {
			continue
		}

		// Extract price.
		price := 0.0
		priceCurrency := strings.ToUpper(currency)
		if v.AngebotsPreis != nil && v.AngebotsPreis.Betrag > 0 {
			price = v.AngebotsPreis.Betrag
			if v.AngebotsPreis.Waehrung != "" {
				priceCurrency = v.AngebotsPreis.Waehrung
			}
		} else if v.AbPreis != nil && v.AbPreis.Betrag > 0 {
			price = v.AbPreis.Betrag
			if v.AbPreis.Waehrung != "" {
				priceCurrency = v.AbPreis.Waehrung
			}
		}

		// Compute departure/arrival from first/last leg.
		first := abschnitte[0]
		last := abschnitte[len(abschnitte)-1]

		depTime := firstNonEmpty(first.AbfahrtsZeitpunkt, first.AbgangsDatum)
		arrTime := firstNonEmpty(last.AnkunftsZeitpunkt, last.AnkunftsDatum)

		// Compute duration in minutes.
		duration := computeDBDuration(depTime, arrTime)

		// Count non-walking transfers.
		transfers := 0
		for _, a := range abschnitte {
			if a.Typ != "WALK" && a.Typ != "FUSSWEG" && a.Typ != "TRANSFER" {
				transfers++
			}
		}
		if transfers > 0 {
			transfers-- // first leg is not a transfer
		}

		// Parse legs.
		var legs []models.GroundLeg
		for _, a := range abschnitte {
			if a.Typ == "WALK" || a.Typ == "FUSSWEG" || a.Typ == "TRANSFER" {
				continue
			}
			legDep := firstNonEmpty(a.AbfahrtsZeitpunkt, a.AbgangsDatum)
			legArr := firstNonEmpty(a.AnkunftsZeitpunkt, a.AnkunftsDatum)
			legName := ""
			if a.Verkehrsmittel != nil {
				legName = firstNonEmpty(a.Verkehrsmittel.Name, a.Verkehrsmittel.LangText, a.Verkehrsmittel.KurzText)
			}

			depCity := fromStation.City
			arrCity := toStation.City
			depStation := ""
			arrStation := ""

			if len(a.Halte) > 0 {
				if a.Halte[0].Ort != nil {
					depStation = a.Halte[0].Ort.Name
				}
				if a.Halte[len(a.Halte)-1].Ort != nil {
					arrStation = a.Halte[len(a.Halte)-1].Ort.Name
				}
			}
			if a.AbgangsOrt != nil && a.AbgangsOrt.Name != "" {
				depStation = a.AbgangsOrt.Name
			}
			if a.AnkunftsOrtObj != nil && a.AnkunftsOrtObj.Name != "" {
				arrStation = a.AnkunftsOrtObj.Name
			}

			legs = append(legs, models.GroundLeg{
				Type:     "train",
				Provider: legName,
				Departure: models.GroundStop{
					City:    depCity,
					Station: depStation,
					Time:    legDep,
				},
				Arrival: models.GroundStop{
					City:    arrCity,
					Station: arrStation,
					Time:    legArr,
				},
				Duration: computeDBDuration(legDep, legArr),
			})
		}

		depStation := fromStation.Name
		arrStation := toStation.Name
		if len(abschnitte) > 0 && len(first.Halte) > 0 && first.Halte[0].Ort != nil {
			depStation = first.Halte[0].Ort.Name
		}
		if len(abschnitte) > 0 && len(last.Halte) > 0 && last.Halte[len(last.Halte)-1].Ort != nil {
			arrStation = last.Halte[len(last.Halte)-1].Ort.Name
		}

		route := models.GroundRoute{
			Provider: "db",
			Type:     "train",
			Price:    price,
			Currency: priceCurrency,
			Duration: duration,
			Departure: models.GroundStop{
				City:    fromStation.City,
				Station: depStation,
				Time:    depTime,
			},
			Arrival: models.GroundStop{
				City:    toStation.City,
				Station: arrStation,
				Time:    arrTime,
			},
			Transfers:  transfers,
			Legs:       legs,
			BookingURL: buildDBBookingURL(fromStation.EVA, toStation.EVA, depTime),
		}

		routes = append(routes, route)
	}
	return routes
}

// computeDBDuration computes the duration in minutes between two ISO 8601 time strings.
func computeDBDuration(dep, arr string) int {
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05-07:00",
		"2006-01-02T15:04:05",
		"2006-01-02T15:04:05Z",
	}
	var depTime, arrTime time.Time
	var err error
	for _, layout := range layouts {
		depTime, err = time.Parse(layout, dep)
		if err == nil {
			break
		}
	}
	if err != nil {
		return 0
	}
	for _, layout := range layouts {
		arrTime, err = time.Parse(layout, arr)
		if err == nil {
			break
		}
	}
	if err != nil {
		return 0
	}
	d := arrTime.Sub(depTime)
	if d < 0 {
		return 0
	}
	return int(d.Minutes())
}

// buildDBBookingURL constructs a bahn.de booking URL.
func buildDBBookingURL(fromEVA, toEVA, depTime string) string {
	// Extract date from the departure time.
	date := depTime
	if len(depTime) >= 10 {
		date = depTime[:10]
	}
	timeOfDay := "06:00:00"
	if len(depTime) >= 19 {
		timeOfDay = depTime[11:19]
	}

	return fmt.Sprintf("https://www.bahn.de/buchung/fahrplan/suche#hin=%s&rueck=%s&von=%s&nach=%s&hinpidalias=&rueckpidalias=&zeit=%s&zeitart=ABFAHRT&klasse=2&reisende=%s&spidalias=&zpidalias=",
		url.QueryEscape(date),
		url.QueryEscape(date),
		url.QueryEscape(fromEVA),
		url.QueryEscape(toEVA),
		url.QueryEscape(timeOfDay),
		url.QueryEscape("1"),
	)
}

// firstNonEmpty returns the first non-empty string from the arguments.
func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}

package ground

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/MikkoParkkola/trvl/internal/models"
)

// RenfeStation holds metadata for a Renfe station.
type RenfeStation struct {
	Code    string // IATA-style 3-letter code used by renfe.com
	Numeric string // Numeric station ID used by venta.renfe.com
	Name    string
	City    string
	Country string
}

// renfeStations maps lowercase city name to Renfe station metadata.
var renfeStations = map[string]RenfeStation{
	// Spain
	"madrid":          {Code: "MAD", Numeric: "60000", Name: "Madrid Puerta de Atocha", City: "Madrid", Country: "ES"},
	"barcelona":       {Code: "BCN", Numeric: "71801", Name: "Barcelona Sants", City: "Barcelona", Country: "ES"},
	"seville":         {Code: "SVQ", Numeric: "51300", Name: "Sevilla Santa Justa", City: "Seville", Country: "ES"},
	"sevilla":         {Code: "SVQ", Numeric: "51300", Name: "Sevilla Santa Justa", City: "Seville", Country: "ES"},
	"valencia":        {Code: "VLC", Numeric: "65000", Name: "Valencia Joaquin Sorolla", City: "Valencia", Country: "ES"},
	"malaga":          {Code: "AGP", Numeric: "61400", Name: "Malaga Maria Zambrano", City: "Malaga", Country: "ES"},
	"bilbao":          {Code: "BIO", Numeric: "70200", Name: "Bilbao Abando", City: "Bilbao", Country: "ES"},
	"zaragoza":        {Code: "ZAZ", Numeric: "65100", Name: "Zaragoza Delicias", City: "Zaragoza", Country: "ES"},
	"cordoba":         {Code: "XWA", Numeric: "51600", Name: "Córdoba", City: "Cordoba", Country: "ES"},
	"alicante":        {Code: "ALC", Numeric: "65200", Name: "Alicante", City: "Alicante", Country: "ES"},
	"granada":         {Code: "GRX", Numeric: "61500", Name: "Granada", City: "Granada", Country: "ES"},
	"pamplona":        {Code: "PNA", Numeric: "70600", Name: "Pamplona Irunlarrea", City: "Pamplona", Country: "ES"},
	"san sebastian":   {Code: "EAS", Numeric: "70100", Name: "San Sebastián - Donostia", City: "San Sebastian", Country: "ES"},
	"donostia":        {Code: "EAS", Numeric: "70100", Name: "San Sebastián - Donostia", City: "San Sebastian", Country: "ES"},
	"valladolid":      {Code: "VLL", Numeric: "62200", Name: "Valladolid Campo Grande", City: "Valladolid", Country: "ES"},
	"murcia":          {Code: "MJV", Numeric: "65300", Name: "Murcia del Carmen", City: "Murcia", Country: "ES"},
	"gijon":           {Code: "GIJ", Numeric: "20101", Name: "Gijón Cercanías", City: "Gijón", Country: "ES"},
	"salamanca":       {Code: "SLM", Numeric: "63200", Name: "Salamanca", City: "Salamanca", Country: "ES"},
	"toledo":          {Code: "TOJ", Numeric: "60901", Name: "Toledo", City: "Toledo", Country: "ES"},
	"cadiz":           {Code: "XRY", Numeric: "51100", Name: "Cádiz", City: "Cadiz", Country: "ES"},
	"tarragona":       {Code: "TGN", Numeric: "71500", Name: "Tarragona", City: "Tarragona", Country: "ES"},
	"santiago de compostela": {Code: "SCQ", Numeric: "36205", Name: "Santiago de Compostela", City: "Santiago de Compostela", Country: "ES"},
	// International (SNCF high-speed connections via Renfe-SNCF)
	"paris":           {Code: "PAR", Numeric: "", Name: "Paris Gare de Lyon", City: "Paris", Country: "FR"},
	"marseille":       {Code: "MRS", Numeric: "", Name: "Marseille Saint-Charles", City: "Marseille", Country: "FR"},
	"lyon":            {Code: "LYS", Numeric: "", Name: "Lyon Part-Dieu", City: "Lyon", Country: "FR"},
}

// LookupRenfeStation resolves a city name to a Renfe station (case-insensitive).
func LookupRenfeStation(city string) (RenfeStation, bool) {
	s, ok := renfeStations[strings.ToLower(strings.TrimSpace(city))]
	return s, ok
}

// HasRenfeRoute returns true if both cities have Renfe stations and at least
// one is a Spanish domestic station (Renfe primarily serves Spain).
func HasRenfeRoute(from, to string) bool {
	fromStation, fromOK := LookupRenfeStation(from)
	toStation, toOK := LookupRenfeStation(to)
	if !fromOK || !toOK {
		return false
	}
	// Require at least one Spanish station.
	return fromStation.Country == "ES" || toStation.Country == "ES"
}

// SearchRenfe searches Renfe for train fares between two cities using the
// Playwright browser scraper (page-context API approach).
//
// Renfe's booking site (venta.renfe.com and renfe.com) uses SPA architecture
// with anti-bot protection, so we navigate via a real browser and call the
// internal APIs from JavaScript context.
func SearchRenfe(ctx context.Context, from, to, date, currency string) ([]models.GroundRoute, error) {
	fromStation, ok := LookupRenfeStation(from)
	if !ok {
		return nil, fmt.Errorf("no Renfe station for %q", from)
	}
	toStation, ok := LookupRenfeStation(to)
	if !ok {
		return nil, fmt.Errorf("no Renfe station for %q", to)
	}

	if currency == "" {
		currency = "EUR"
	}

	slog.Debug("renfe search", "from", fromStation.City, "to", toStation.City, "date", date)

	routes, err := BrowserScrapeRoutes(ctx, "renfe", from, to, date, currency)
	if err != nil {
		return nil, fmt.Errorf("renfe browser scraper: %w", err)
	}
	return routes, nil
}

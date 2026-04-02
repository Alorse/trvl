// Package destinations provides travel intelligence: weather, country info,
// holidays, safety advisories, and currency exchange rates for any city.
//
// All data comes from free public APIs (no API keys required).
package destinations

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const nominatimURL = "https://nominatim.openstreetmap.org/search"

// GeoResult holds the geocoding result with country code for downstream APIs.
type GeoResult struct {
	Lat         float64
	Lon         float64
	DisplayName string
	CountryCode string // ISO 3166-1 alpha-2 (uppercase)
}

// nominatimResult represents a single result from the Nominatim API.
type nominatimResult struct {
	Lat         string            `json:"lat"`
	Lon         string            `json:"lon"`
	DisplayName string            `json:"display_name"`
	Address     map[string]string `json:"address"`
}

// geoCache caches geocoding results.
var geoCache = struct {
	sync.RWMutex
	entries map[string]GeoResult
}{entries: make(map[string]GeoResult)}

// Geocode resolves a location name to coordinates and a country code.
func Geocode(ctx context.Context, query string) (GeoResult, error) {
	geoCache.RLock()
	if entry, ok := geoCache.entries[query]; ok {
		geoCache.RUnlock()
		return entry, nil
	}
	geoCache.RUnlock()

	result, err := nominatimLookup(ctx, query)
	if err != nil {
		return GeoResult{}, err
	}

	geoCache.Lock()
	geoCache.entries[query] = result
	geoCache.Unlock()

	return result, nil
}

func nominatimLookup(ctx context.Context, query string) (GeoResult, error) {
	u, _ := url.Parse(nominatimAPIURL)
	q := u.Query()
	q.Set("q", query)
	q.Set("format", "json")
	q.Set("limit", "1")
	q.Set("addressdetails", "1")
	u.RawQuery = q.Encode()

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return GeoResult{}, fmt.Errorf("create nominatim request: %w", err)
	}
	req.Header.Set("User-Agent", "trvl/1.0 (destination intelligence)")

	resp, err := client.Do(req)
	if err != nil {
		return GeoResult{}, fmt.Errorf("nominatim request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return GeoResult{}, fmt.Errorf("read nominatim response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return GeoResult{}, fmt.Errorf("nominatim returned status %d", resp.StatusCode)
	}

	var results []nominatimResult
	if err := json.Unmarshal(body, &results); err != nil {
		return GeoResult{}, fmt.Errorf("parse nominatim response: %w", err)
	}

	if len(results) == 0 {
		return GeoResult{}, fmt.Errorf("no geocoding results for %q", query)
	}

	r := results[0]
	var lat, lon float64
	if _, err := fmt.Sscanf(r.Lat, "%f", &lat); err != nil {
		return GeoResult{}, fmt.Errorf("parse latitude %q: %w", r.Lat, err)
	}
	if _, err := fmt.Sscanf(r.Lon, "%f", &lon); err != nil {
		return GeoResult{}, fmt.Errorf("parse longitude %q: %w", r.Lon, err)
	}

	cc := ""
	if code, ok := r.Address["country_code"]; ok {
		cc = strings.ToUpper(code)
	}

	return GeoResult{
		Lat:         lat,
		Lon:         lon,
		DisplayName: r.DisplayName,
		CountryCode: cc,
	}, nil
}

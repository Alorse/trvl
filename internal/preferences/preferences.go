// Package preferences manages user preferences stored in ~/.trvl/preferences.json.
// Preferences are optional — if the file is missing, Default() is returned and
// no existing behaviour changes.
package preferences

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MikkoParkkola/trvl/internal/models"
)

// Preferences holds all personal travel preferences for the user.
type Preferences struct {
	// Identity
	HomeAirports []string `json:"home_airports"` // e.g. ["HEL", "AMS"]
	HomeCities   []string `json:"home_cities"`   // e.g. ["Helsinki", "Amsterdam"]

	// Travel style
	CarryOnOnly  bool `json:"carry_on_only"` // affects route hacks
	PreferDirect bool `json:"prefer_direct"` // flight stops preference

	// Accommodation
	NoDormitories  bool    `json:"no_dormitories"`   // exclude hostels with shared rooms
	EnSuiteOnly    bool    `json:"ensuite_only"`     // require own bathroom
	FastWifiNeeded bool    `json:"fast_wifi_needed"` // co-working capable
	MinHotelStars  int     `json:"min_hotel_stars"`  // 0 = any
	MinHotelRating float64 `json:"min_hotel_rating"` // e.g. 4.0

	// Preferred districts/neighborhoods per city.
	// e.g. {"Prague": ["Prague 1", "Prague 2"], "Helsinki": ["Kallio", "Punavuori"]}
	PreferredDistricts map[string][]string `json:"preferred_districts,omitempty"`

	// Currency & locale
	DisplayCurrency string `json:"display_currency"` // "EUR"
	Locale          string `json:"locale"`           // "en-FI"

	// Loyalty programmes
	LoyaltyAirlines []string `json:"loyalty_airlines,omitempty"` // IATA codes, e.g. ["KL", "AY"]
	LoyaltyHotels   []string `json:"loyalty_hotels,omitempty"`   // e.g. ["Marriott Bonvoy", "IHG"]

	// Family members for booking on behalf of
	FamilyMembers []FamilyMember `json:"family_members,omitempty"`
}

// FamilyMember represents a person the user may book travel for.
type FamilyMember struct {
	Name         string `json:"name"`
	Relationship string `json:"relationship"` // "father", "spouse", etc.
	Notes        string `json:"notes"`        // free-form preferences
}

// defaultPath returns the canonical preferences file path (~/.trvl/preferences.json).
func defaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".trvl", "preferences.json"), nil
}

// Default returns sensible zero-value preferences.
// These are used when no preferences file exists.
func Default() *Preferences {
	return &Preferences{
		DisplayCurrency: "EUR",
		Locale:          "en",
	}
}

// Load reads preferences from ~/.trvl/preferences.json.
// If the file does not exist, Default() is returned with no error.
func Load() (*Preferences, error) {
	path, err := defaultPath()
	if err != nil {
		return Default(), nil
	}
	return LoadFrom(path)
}

// LoadFrom reads preferences from an explicit file path.
// If the file does not exist, Default() is returned with no error.
func LoadFrom(path string) (*Preferences, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Default(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read preferences: %w", err)
	}
	if len(data) == 0 {
		return Default(), nil
	}

	p := Default()
	if err := json.Unmarshal(data, p); err != nil {
		return nil, fmt.Errorf("parse preferences: %w", err)
	}
	return p, nil
}

// Save writes preferences to ~/.trvl/preferences.json atomically.
func Save(p *Preferences) error {
	path, err := defaultPath()
	if err != nil {
		return err
	}
	return SaveTo(path, p)
}

// SaveTo writes preferences to an explicit file path atomically.
func SaveTo(path string, p *Preferences) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create preferences dir: %w", err)
	}

	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("encode preferences: %w", err)
	}

	// Atomic write: write to temp file, rename.
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}

	cleanup = false
	return nil
}

// HomeAirport returns the first configured home airport, or "" if none.
func (p *Preferences) HomeAirport() string {
	if len(p.HomeAirports) > 0 {
		return p.HomeAirports[0]
	}
	return ""
}

// DistrictsFor returns the preferred districts for the given city (case-insensitive).
func (p *Preferences) DistrictsFor(city string) []string {
	if p.PreferredDistricts == nil {
		return nil
	}
	// Exact match first.
	if d, ok := p.PreferredDistricts[city]; ok {
		return d
	}
	// Case-insensitive fallback.
	cityLower := lowerStr(city)
	for k, v := range p.PreferredDistricts {
		if lowerStr(k) == cityLower {
			return v
		}
	}
	return nil
}

func lowerStr(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

// FilterHotels applies preference-based post-filters to a hotel list.
//
// Applied filters:
//   - NoDormitories: removes properties whose name or description suggests shared rooms
//   - EnSuiteOnly: removes properties that appear to lack a private bathroom
//   - Preferred districts: deprioritises (moves to end) hotels not in preferred districts
//     for the given city when preferences exist for that city
//
// The function always returns a valid (possibly empty) slice and never mutates
// the input. It is a no-op when p is nil.
func FilterHotels(hotels []models.HotelResult, city string, p *Preferences) []models.HotelResult {
	if p == nil {
		return hotels
	}

	out := make([]models.HotelResult, 0, len(hotels))
	for _, h := range hotels {
		if p.NoDormitories && isDormitory(h) {
			continue
		}
		if p.EnSuiteOnly && lacksPrivateBathroom(h) {
			continue
		}
		out = append(out, h)
	}

	// Preferred districts: float matching hotels to the front.
	districts := p.DistrictsFor(city)
	if len(districts) > 0 {
		out = prioritiseByDistrict(out, districts)
	}

	return out
}

// dormKeywords are substrings that indicate shared-room accommodation.
// Includes generic terms + known hostel chains that don't contain "hostel"
// in their brand name (St Christopher's Inn, Generator, MEININGER, etc.).
var dormKeywords = []string{
	// Generic terms
	"hostel", "dorm", "dormitory", "capsule", "pod hotel", "bunk",
	"youth hostel", "backpacker",
	// Known hostel/hybrid chains (lowercase substring match)
	"st christopher", // St Christopher's Inn
	"generator ",     // Generator Hostels (trailing space to avoid "generator hotel" false positives)
	"meininger",      // MEININGER Hotels (hybrid hostel/hotel)
	"wombat",         // Wombats Hostels
	"clink",          // Clink Hostels
	"safestay",       // Safestay
	"yha ",           // YHA hostels
	"nomad cave",     // Nomad Cave (budget shared-room)
	"nomad city",     // Nomad City (budget shared-room)
	"a&o",            // A&O Hotels and Hostels
	"rygerfjord",     // Rygerfjord (Stockholm hostel boat)
	"citybox",        // Citybox (Nordic budget self-service, shared kitchens)
	"travelodge",     // Travelodge (debatable — UK budget chain, but avg < 4★ experience)
}

// isDormitory returns true when the hotel name or amenities suggest shared sleeping.
func isDormitory(h models.HotelResult) bool {
	combined := strings.ToLower(h.Name + " " + h.Address)
	for _, kw := range dormKeywords {
		if strings.Contains(combined, kw) {
			return true
		}
	}
	// Also check amenity strings.
	for _, a := range h.Amenities {
		aLow := strings.ToLower(a)
		if strings.Contains(aLow, "shared room") || strings.Contains(aLow, "dorm") {
			return true
		}
	}
	return false
}

// bathroomKeywords indicate that a hotel explicitly shares bathrooms.
var sharedBathroomKeywords = []string{
	"shared bathroom", "shared bath", "shared facilities",
	"communal bathroom", "common bathroom",
}

// lacksPrivateBathroom returns true when the hotel signals it does NOT have
// a private bathroom. If there is no evidence either way, returns false (keep
// the hotel — don't falsely exclude).
func lacksPrivateBathroom(h models.HotelResult) bool {
	combined := strings.ToLower(h.Name + " " + h.Address)
	for _, a := range h.Amenities {
		combined += " " + strings.ToLower(a)
	}

	for _, kw := range sharedBathroomKeywords {
		if strings.Contains(combined, kw) {
			return true
		}
	}
	return false
}

// prioritiseByDistrict reorders hotels so those whose address contains one of
// the preferred district strings appear first. Order within each group is
// preserved.
func prioritiseByDistrict(hotels []models.HotelResult, districts []string) []models.HotelResult {
	var preferred, rest []models.HotelResult
	for _, h := range hotels {
		addrLow := strings.ToLower(h.Address)
		matched := false
		for _, d := range districts {
			if strings.Contains(addrLow, strings.ToLower(d)) {
				matched = true
				break
			}
		}
		if matched {
			preferred = append(preferred, h)
		} else {
			rest = append(rest, h)
		}
	}
	return append(preferred, rest...)
}

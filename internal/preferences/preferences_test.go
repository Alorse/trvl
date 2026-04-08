package preferences

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/MikkoParkkola/trvl/internal/models"
)

func TestDefault(t *testing.T) {
	p := Default()
	if p == nil {
		t.Fatal("Default() returned nil")
	}
	if p.DisplayCurrency == "" {
		t.Error("Default() DisplayCurrency should not be empty")
	}
}

func TestLoadFrom_MissingFile(t *testing.T) {
	p, err := LoadFrom("/tmp/trvl-nonexistent-prefs-xyzzy.json")
	if err != nil {
		t.Fatalf("LoadFrom missing file: unexpected error: %v", err)
	}
	if p == nil {
		t.Fatal("LoadFrom missing file returned nil preferences")
	}
	// Should return defaults.
	d := Default()
	if p.DisplayCurrency != d.DisplayCurrency {
		t.Errorf("got currency %q, want %q", p.DisplayCurrency, d.DisplayCurrency)
	}
}

func TestLoadFrom_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "preferences.json")
	if err := os.WriteFile(path, []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom empty file: unexpected error: %v", err)
	}
	if p == nil {
		t.Fatal("LoadFrom empty file returned nil")
	}
}

func TestSaveTo_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "preferences.json")

	original := &Preferences{
		HomeAirports:    []string{"HEL", "AMS"},
		HomeCities:      []string{"Helsinki", "Amsterdam"},
		CarryOnOnly:     true,
		PreferDirect:    true,
		NoDormitories:   true,
		EnSuiteOnly:     false,
		MinHotelStars:   3,
		MinHotelRating:  4.0,
		DisplayCurrency: "EUR",
		Locale:          "en-FI",
		PreferredDistricts: map[string][]string{
			"Prague": {"Prague 1", "Prague 2"},
		},
		FamilyMembers: []FamilyMember{
			{Name: "Dad", Relationship: "father", Notes: "prefers sea view"},
		},
	}

	if err := SaveTo(path, original); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}

	loaded, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}

	if loaded.DisplayCurrency != "EUR" {
		t.Errorf("DisplayCurrency: got %q, want EUR", loaded.DisplayCurrency)
	}
	if !loaded.CarryOnOnly {
		t.Error("CarryOnOnly should be true")
	}
	if loaded.MinHotelRating != 4.0 {
		t.Errorf("MinHotelRating: got %v, want 4.0", loaded.MinHotelRating)
	}
	if len(loaded.HomeAirports) != 2 || loaded.HomeAirports[0] != "HEL" {
		t.Errorf("HomeAirports: got %v", loaded.HomeAirports)
	}
	if len(loaded.FamilyMembers) != 1 || loaded.FamilyMembers[0].Name != "Dad" {
		t.Errorf("FamilyMembers: got %v", loaded.FamilyMembers)
	}
}

func TestSaveTo_CreatesDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deep", "preferences.json")

	p := Default()
	p.DisplayCurrency = "GBP"

	if err := SaveTo(path, p); err != nil {
		t.Fatalf("SaveTo nested dir: %v", err)
	}

	loaded, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if loaded.DisplayCurrency != "GBP" {
		t.Errorf("DisplayCurrency: got %q, want GBP", loaded.DisplayCurrency)
	}
}

func TestSaveTo_WritesValidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "preferences.json")

	p := Default()
	p.HomeAirports = []string{"JFK"}
	if err := SaveTo(path, p); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Errorf("saved file is not valid JSON: %v", err)
	}
}

func TestHomeAirport(t *testing.T) {
	p := Default()
	if p.HomeAirport() != "" {
		t.Errorf("HomeAirport() on empty prefs should return empty, got %q", p.HomeAirport())
	}

	p.HomeAirports = []string{"HEL", "TKU"}
	if got := p.HomeAirport(); got != "HEL" {
		t.Errorf("HomeAirport() = %q, want HEL", got)
	}
}

func TestDistrictsFor(t *testing.T) {
	p := Default()
	p.PreferredDistricts = map[string][]string{
		"Prague":   {"Prague 1", "Prague 2"},
		"Helsinki": {"Kallio", "Punavuori"},
	}

	got := p.DistrictsFor("Prague")
	if len(got) != 2 || got[0] != "Prague 1" {
		t.Errorf("DistrictsFor Prague: got %v", got)
	}

	// Case-insensitive
	got = p.DistrictsFor("helsinki")
	if len(got) != 2 || got[0] != "Kallio" {
		t.Errorf("DistrictsFor helsinki (lower): got %v", got)
	}

	// Missing city
	got = p.DistrictsFor("Tokyo")
	if got != nil {
		t.Errorf("DistrictsFor unknown city: expected nil, got %v", got)
	}
}

func TestFilterHotels_NilPrefs(t *testing.T) {
	hotels := []models.HotelResult{{Name: "Hilton"}}
	got := FilterHotels(hotels, "Helsinki", nil)
	if len(got) != 1 {
		t.Errorf("FilterHotels with nil prefs: expected 1 hotel, got %d", len(got))
	}
}

func TestFilterHotels_NoDormitories(t *testing.T) {
	hotels := []models.HotelResult{
		{Name: "Hilton Helsinki"},
		{Name: "Helsinki Hostel Dorm"},
		{Name: "City Backpackers Hostel"},
	}
	p := Default()
	p.NoDormitories = true

	got := FilterHotels(hotels, "Helsinki", p)
	if len(got) != 1 {
		t.Errorf("FilterHotels NoDormitories: expected 1 hotel, got %d: %v", len(got), got)
	}
	if got[0].Name != "Hilton Helsinki" {
		t.Errorf("FilterHotels NoDormitories: kept wrong hotel %q", got[0].Name)
	}
}

func TestFilterHotels_EnSuiteOnly_Excludes(t *testing.T) {
	hotels := []models.HotelResult{
		{Name: "Grand Hotel", Amenities: []string{"free wifi", "pool"}},
		{Name: "Budget Inn", Amenities: []string{"shared bathroom", "free wifi"}},
	}
	p := Default()
	p.EnSuiteOnly = true

	got := FilterHotels(hotels, "Prague", p)
	if len(got) != 1 || got[0].Name != "Grand Hotel" {
		t.Errorf("FilterHotels EnSuiteOnly: expected Grand Hotel only, got %v", got)
	}
}

func TestFilterHotels_PreferredDistricts_Reorders(t *testing.T) {
	hotels := []models.HotelResult{
		{Name: "Far Hotel", Address: "Suburbs, Prague 8"},
		{Name: "Central Hotel", Address: "Old Town Square, Prague 1"},
	}
	p := Default()
	p.PreferredDistricts = map[string][]string{
		"Prague": {"Prague 1", "Prague 2"},
	}

	got := FilterHotels(hotels, "Prague", p)
	if len(got) != 2 {
		t.Fatalf("FilterHotels districts: expected 2 hotels, got %d", len(got))
	}
	if got[0].Name != "Central Hotel" {
		t.Errorf("FilterHotels districts: expected Central Hotel first, got %q", got[0].Name)
	}
}

func TestFilterHotels_NoPrefs_PassesAll(t *testing.T) {
	hotels := []models.HotelResult{
		{Name: "Hotel A"},
		{Name: "Hostel B"},
	}
	got := FilterHotels(hotels, "Helsinki", Default())
	if len(got) != 2 {
		t.Errorf("FilterHotels with defaults: expected 2 hotels, got %d", len(got))
	}
}

func TestLoadFrom_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "preferences.json")
	if err := os.WriteFile(path, []byte("{not valid json"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadFrom(path)
	if err == nil {
		t.Error("LoadFrom invalid JSON should return error")
	}
}

func TestSaveTo_FilePermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "preferences.json")

	if err := SaveTo(path, Default()); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	assertCrossPlatformPrivateFile(t, path, info)
}

func assertCrossPlatformPrivateFile(t *testing.T, path string, info os.FileInfo) {
	t.Helper()

	if !info.Mode().IsRegular() {
		t.Fatalf("%s is not a regular file: %v", path, info.Mode())
	}

	perm := info.Mode().Perm()
	if runtime.GOOS == "windows" {
		if perm&0o111 != 0 {
			t.Errorf("%s should not be executable on Windows, got %o", path, perm)
		}
		return
	}

	if perm != 0o600 {
		t.Errorf("%s permissions: got %o, want 600", path, perm)
	}
}

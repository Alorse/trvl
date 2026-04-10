package mcp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MikkoParkkola/trvl/internal/preferences"
)

// setupTempPrefs creates a temp dir with a preferences file containing the
// given prefs and returns the file path. Caller should defer os.RemoveAll on
// the parent dir.
func setupTempPrefs(t *testing.T, p *preferences.Preferences) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "preferences.json")
	if err := preferences.SaveTo(path, p); err != nil {
		t.Fatalf("setup prefs: %v", err)
	}
	return path
}

func TestUpdatePreferences_PartialUpdate_PreservesOtherFields(t *testing.T) {
	initial := &preferences.Preferences{
		HomeAirports:    []string{"HEL"},
		HomeCities:      []string{"Helsinki"},
		CarryOnOnly:     true,
		DisplayCurrency: "EUR",
		Locale:          "en-FI",
		MinHotelRating:  3.5,
	}
	path := setupTempPrefs(t, initial)

	// Update only min_hotel_stars.
	args := map[string]any{
		"min_hotel_stars": float64(4),
	}
	content, structured, err := handleUpdatePreferencesWithPath(args, path, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(content) == 0 {
		t.Fatal("expected content blocks")
	}

	result, ok := structured.(*preferences.Preferences)
	if !ok {
		t.Fatalf("structured result is %T, want *preferences.Preferences", structured)
	}

	// Updated field.
	if result.MinHotelStars != 4 {
		t.Errorf("MinHotelStars = %d, want 4", result.MinHotelStars)
	}

	// Preserved fields.
	if len(result.HomeAirports) != 1 || result.HomeAirports[0] != "HEL" {
		t.Errorf("HomeAirports = %v, want [HEL]", result.HomeAirports)
	}
	if !result.CarryOnOnly {
		t.Error("CarryOnOnly should be preserved as true")
	}
	if result.DisplayCurrency != "EUR" {
		t.Errorf("DisplayCurrency = %q, want EUR", result.DisplayCurrency)
	}
	if result.MinHotelRating != 3.5 {
		t.Errorf("MinHotelRating = %f, want 3.5", result.MinHotelRating)
	}

	// Verify file was actually written.
	reloaded, err := preferences.LoadFrom(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.MinHotelStars != 4 {
		t.Errorf("reloaded MinHotelStars = %d, want 4", reloaded.MinHotelStars)
	}
	if !reloaded.CarryOnOnly {
		t.Error("reloaded CarryOnOnly should be true")
	}
}

func TestUpdatePreferences_AddFamilyMember(t *testing.T) {
	initial := preferences.Default()
	path := setupTempPrefs(t, initial)

	args := map[string]any{
		"family_members": []any{
			map[string]any{
				"name":         "Liisa",
				"relationship": "spouse",
				"notes":        "vegetarian, window seat",
			},
			map[string]any{
				"name":         "Eero",
				"relationship": "son",
				"notes":        "age 8",
			},
		},
	}

	_, structured, err := handleUpdatePreferencesWithPath(args, path, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := structured.(*preferences.Preferences)
	if len(result.FamilyMembers) != 2 {
		t.Fatalf("FamilyMembers len = %d, want 2", len(result.FamilyMembers))
	}
	if result.FamilyMembers[0].Name != "Liisa" {
		t.Errorf("first member name = %q, want Liisa", result.FamilyMembers[0].Name)
	}
	if result.FamilyMembers[1].Relationship != "son" {
		t.Errorf("second member relationship = %q, want son", result.FamilyMembers[1].Relationship)
	}
}

func TestUpdatePreferences_FamilyMembersFromJSON(t *testing.T) {
	initial := preferences.Default()
	path := setupTempPrefs(t, initial)

	args := map[string]any{
		"family_members": `[{"name":"Liisa","relationship":"spouse","notes":""}]`,
	}

	_, structured, err := handleUpdatePreferencesWithPath(args, path, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := structured.(*preferences.Preferences)
	if len(result.FamilyMembers) != 1 {
		t.Fatalf("FamilyMembers len = %d, want 1", len(result.FamilyMembers))
	}
	if result.FamilyMembers[0].Name != "Liisa" {
		t.Errorf("name = %q, want Liisa", result.FamilyMembers[0].Name)
	}
}

func TestUpdatePreferences_PreferredDistricts_Merge(t *testing.T) {
	initial := &preferences.Preferences{
		DisplayCurrency: "EUR",
		Locale:          "en",
		PreferredDistricts: map[string][]string{
			"Helsinki": {"Kallio", "Punavuori"},
			"Prague":   {"Prague 1"},
		},
	}
	path := setupTempPrefs(t, initial)

	// Add Amsterdam, replace Prague.
	args := map[string]any{
		"preferred_districts": map[string]any{
			"Amsterdam": []any{"Jordaan", "De Pijp"},
			"Prague":    []any{"Prague 1", "Prague 2", "Vinohrady"},
		},
	}

	_, structured, err := handleUpdatePreferencesWithPath(args, path, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := structured.(*preferences.Preferences)

	// Helsinki preserved.
	if ds := result.PreferredDistricts["Helsinki"]; len(ds) != 2 {
		t.Errorf("Helsinki districts = %v, want [Kallio Punavuori]", ds)
	}
	// Amsterdam added.
	if ds := result.PreferredDistricts["Amsterdam"]; len(ds) != 2 {
		t.Errorf("Amsterdam districts = %v, want [Jordaan De Pijp]", ds)
	}
	// Prague replaced.
	if ds := result.PreferredDistricts["Prague"]; len(ds) != 3 {
		t.Errorf("Prague districts = %v, want 3 entries", ds)
	}
}

func TestUpdatePreferences_PreferredDistricts_FromJSON(t *testing.T) {
	initial := preferences.Default()
	path := setupTempPrefs(t, initial)

	args := map[string]any{
		"preferred_districts": `{"Berlin":["Mitte","Kreuzberg"]}`,
	}

	_, structured, err := handleUpdatePreferencesWithPath(args, path, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := structured.(*preferences.Preferences)
	if ds := result.PreferredDistricts["Berlin"]; len(ds) != 2 {
		t.Errorf("Berlin districts = %v, want [Mitte Kreuzberg]", ds)
	}
}

func TestUpdatePreferences_InvalidFieldsIgnored(t *testing.T) {
	initial := &preferences.Preferences{
		DisplayCurrency: "EUR",
		Locale:          "en",
		MinHotelStars:   3,
	}
	path := setupTempPrefs(t, initial)

	args := map[string]any{
		"nonexistent_field":  "should be ignored",
		"another_bad_field":  42,
		"min_hotel_stars":    float64(4), // valid field mixed in
	}

	_, structured, err := handleUpdatePreferencesWithPath(args, path, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := structured.(*preferences.Preferences)
	if result.MinHotelStars != 4 {
		t.Errorf("MinHotelStars = %d, want 4", result.MinHotelStars)
	}
	// Confirm no panic or error from unknown fields.
}

func TestUpdatePreferences_BooleanFields(t *testing.T) {
	initial := &preferences.Preferences{
		DisplayCurrency: "EUR",
		Locale:          "en",
		CarryOnOnly:     false,
		PreferDirect:    true,
		NoDormitories:   false,
	}
	path := setupTempPrefs(t, initial)

	args := map[string]any{
		"carry_on_only":   true,
		"no_dormitories":  true,
		// prefer_direct NOT included — should stay true.
	}

	_, structured, err := handleUpdatePreferencesWithPath(args, path, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := structured.(*preferences.Preferences)
	if !result.CarryOnOnly {
		t.Error("CarryOnOnly should be true")
	}
	if !result.NoDormitories {
		t.Error("NoDormitories should be true")
	}
	if !result.PreferDirect {
		t.Error("PreferDirect should be preserved as true")
	}
}

func TestUpdatePreferences_StringArrayFromJSON(t *testing.T) {
	initial := preferences.Default()
	path := setupTempPrefs(t, initial)

	args := map[string]any{
		"home_airports":    `["HEL","AMS"]`,
		"loyalty_airlines": `["AY","KL"]`,
	}

	_, structured, err := handleUpdatePreferencesWithPath(args, path, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := structured.(*preferences.Preferences)
	if len(result.HomeAirports) != 2 || result.HomeAirports[0] != "HEL" || result.HomeAirports[1] != "AMS" {
		t.Errorf("HomeAirports = %v, want [HEL AMS]", result.HomeAirports)
	}
	if len(result.LoyaltyAirlines) != 2 || result.LoyaltyAirlines[0] != "AY" {
		t.Errorf("LoyaltyAirlines = %v, want [AY KL]", result.LoyaltyAirlines)
	}
}

func TestUpdatePreferences_EmptyArgs_NoChange(t *testing.T) {
	initial := &preferences.Preferences{
		HomeAirports:    []string{"HEL"},
		DisplayCurrency: "EUR",
		Locale:          "en-FI",
		MinHotelStars:   4,
	}
	path := setupTempPrefs(t, initial)

	// Empty args — nothing should change.
	args := map[string]any{}

	_, structured, err := handleUpdatePreferencesWithPath(args, path, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := structured.(*preferences.Preferences)
	if result.MinHotelStars != 4 {
		t.Errorf("MinHotelStars = %d, want 4", result.MinHotelStars)
	}
	if len(result.HomeAirports) != 1 {
		t.Errorf("HomeAirports = %v, want [HEL]", result.HomeAirports)
	}
}

func TestUpdatePreferences_NoExistingFile_CreatesNew(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "preferences.json")

	args := map[string]any{
		"home_airports":    []any{"AMS"},
		"display_currency": "USD",
		"min_hotel_stars":  float64(3),
	}

	_, structured, err := handleUpdatePreferencesWithPath(args, path, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := structured.(*preferences.Preferences)
	if len(result.HomeAirports) != 1 || result.HomeAirports[0] != "AMS" {
		t.Errorf("HomeAirports = %v, want [AMS]", result.HomeAirports)
	}
	if result.DisplayCurrency != "USD" {
		t.Errorf("DisplayCurrency = %q, want USD", result.DisplayCurrency)
	}

	// File should exist on disk.
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("preferences file was not created")
	}
}

func TestUpdatePreferencesTool_Annotations(t *testing.T) {
	tool := updatePreferencesTool()

	if tool.Annotations == nil {
		t.Fatal("Annotations is nil")
	}
	if tool.Annotations.ReadOnlyHint {
		t.Error("ReadOnlyHint should be false for a write tool")
	}
	if tool.Annotations.DestructiveHint {
		t.Error("DestructiveHint should be false (merge, not replace)")
	}
	if !tool.Annotations.IdempotentHint {
		t.Error("IdempotentHint should be true")
	}
	if tool.Name != "update_preferences" {
		t.Errorf("Name = %q, want update_preferences", tool.Name)
	}
}

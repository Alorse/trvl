package profile

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profile.json")

	p := &TravelProfile{
		TotalTrips:   5,
		TotalFlights: 10,
		BudgetTier:   "mid-range",
		Bookings: []Booking{
			{Type: "flight", Provider: "KLM", From: "HEL", To: "AMS", Price: 189},
		},
	}

	if err := SaveTo(path, p); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}

	loaded, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}

	if loaded.TotalTrips != 5 {
		t.Errorf("TotalTrips = %d, want 5", loaded.TotalTrips)
	}
	if loaded.TotalFlights != 10 {
		t.Errorf("TotalFlights = %d, want 10", loaded.TotalFlights)
	}
	if loaded.BudgetTier != "mid-range" {
		t.Errorf("BudgetTier = %q, want mid-range", loaded.BudgetTier)
	}
	if len(loaded.Bookings) != 1 {
		t.Fatalf("Bookings len = %d, want 1", len(loaded.Bookings))
	}
	if loaded.Bookings[0].Provider != "KLM" {
		t.Errorf("Bookings[0].Provider = %q, want KLM", loaded.Bookings[0].Provider)
	}
}

func TestLoadNonExistent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.json")
	p, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom nonexistent should not error: %v", err)
	}
	if p == nil {
		t.Fatal("should return empty profile, not nil")
	}
	if len(p.Bookings) != 0 {
		t.Error("empty profile should have no bookings")
	}
}

func TestLoadEmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profile.json")
	if err := os.WriteFile(path, []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}

	p, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom empty: %v", err)
	}
	if p == nil {
		t.Fatal("should return empty profile, not nil")
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profile.json")
	if err := os.WriteFile(path, []byte("{invalid"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadFrom(path)
	if err == nil {
		t.Fatal("should error on invalid JSON")
	}
}

func TestAddBookingTo(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profile.json")

	// Add first booking.
	b1 := Booking{
		Type: "flight", Provider: "KLM", From: "HEL", To: "AMS",
		Price: 189, Currency: "EUR", TravelDate: "2026-03-15",
	}
	if err := AddBookingTo(path, b1); err != nil {
		t.Fatalf("AddBookingTo: %v", err)
	}

	// Load and verify.
	p, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if len(p.Bookings) != 1 {
		t.Fatalf("Bookings len = %d, want 1", len(p.Bookings))
	}
	if p.TotalFlights != 1 {
		t.Errorf("TotalFlights = %d, want 1", p.TotalFlights)
	}

	// Add second booking.
	b2 := Booking{
		Type: "hotel", Provider: "Marriott", Price: 450, Nights: 3,
		Currency: "EUR", TravelDate: "2026-03-15",
	}
	if err := AddBookingTo(path, b2); err != nil {
		t.Fatalf("AddBookingTo: %v", err)
	}

	// Load and verify rebuild.
	p2, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if len(p2.Bookings) != 2 {
		t.Fatalf("Bookings len = %d, want 2", len(p2.Bookings))
	}
	if p2.TotalFlights != 1 {
		t.Errorf("TotalFlights = %d, want 1", p2.TotalFlights)
	}
	if p2.TotalHotelNights != 3 {
		t.Errorf("TotalHotelNights = %d, want 3", p2.TotalHotelNights)
	}
}

func TestSaveToCreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "deep", "nested", "path")
	path := filepath.Join(dir, "profile.json")

	p := &TravelProfile{TotalTrips: 1}
	if err := SaveTo(path, p); err != nil {
		t.Fatalf("SaveTo with nested dir: %v", err)
	}

	loaded, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if loaded.TotalTrips != 1 {
		t.Errorf("TotalTrips = %d, want 1", loaded.TotalTrips)
	}
}

func TestSaveToFilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix file permissions not applicable on Windows")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "profile.json")

	p := &TravelProfile{TotalTrips: 1}
	if err := SaveTo(path, p); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	perm := info.Mode().Perm()
	if perm != 0o600 {
		t.Errorf("file permissions = %o, want 0600", perm)
	}
}

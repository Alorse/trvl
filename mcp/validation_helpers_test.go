package mcp

import (
	"strings"
	"testing"
)

func TestValidateOriginDest_Valid(t *testing.T) {
	t.Parallel()
	args := map[string]any{"origin": "hel", "destination": "nrt"}
	origin, dest, err := validateOriginDest(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if origin != "HEL" {
		t.Errorf("expected HEL, got %s", origin)
	}
	if dest != "NRT" {
		t.Errorf("expected NRT, got %s", dest)
	}
}

func TestValidateOriginDest_MissingOrigin(t *testing.T) {
	t.Parallel()
	args := map[string]any{"destination": "NRT"}
	_, _, err := validateOriginDest(args)
	if err == nil {
		t.Fatal("expected error for missing origin")
	}
	if !strings.Contains(err.Error(), "required") {
		t.Errorf("error should mention required: %v", err)
	}
}

func TestValidateOriginDest_MissingDest(t *testing.T) {
	t.Parallel()
	args := map[string]any{"origin": "HEL"}
	_, _, err := validateOriginDest(args)
	if err == nil {
		t.Fatal("expected error for missing destination")
	}
}

func TestValidateOriginDest_InvalidOrigin(t *testing.T) {
	t.Parallel()
	args := map[string]any{"origin": "XX", "destination": "NRT"}
	_, _, err := validateOriginDest(args)
	if err == nil {
		t.Fatal("expected error for invalid origin")
	}
	if !strings.Contains(err.Error(), "invalid origin") {
		t.Errorf("error should mention invalid origin: %v", err)
	}
}

func TestValidateOriginDest_InvalidDest(t *testing.T) {
	t.Parallel()
	args := map[string]any{"origin": "HEL", "destination": "1234"}
	_, _, err := validateOriginDest(args)
	if err == nil {
		t.Fatal("expected error for invalid destination")
	}
	if !strings.Contains(err.Error(), "invalid destination") {
		t.Errorf("error should mention invalid destination: %v", err)
	}
}

func TestValidateDate_Valid(t *testing.T) {
	t.Parallel()
	args := map[string]any{"departure_date": "2026-05-01"}
	d, err := validateDate(args, "departure_date")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d != "2026-05-01" {
		t.Errorf("expected 2026-05-01, got %s", d)
	}
}

func TestValidateDate_Missing(t *testing.T) {
	t.Parallel()
	args := map[string]any{}
	_, err := validateDate(args, "departure_date")
	if err == nil {
		t.Fatal("expected error for missing date")
	}
	if !strings.Contains(err.Error(), "departure_date is required") {
		t.Errorf("error should mention field name: %v", err)
	}
}

func TestValidateDate_Invalid(t *testing.T) {
	t.Parallel()
	args := map[string]any{"departure_date": "not-a-date"}
	_, err := validateDate(args, "departure_date")
	if err == nil {
		t.Fatal("expected error for invalid date")
	}
}

func TestValidateDate_NilArgs(t *testing.T) {
	t.Parallel()
	_, err := validateDate(nil, "date")
	if err == nil {
		t.Fatal("expected error for nil args")
	}
}

package main

import (
	"testing"

	"github.com/MikkoParkkola/trvl/internal/models"
)

func TestParseProviderListAcceptsSerpApi(t *testing.T) {
	out, err := parseProviderList("google_serpapi")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(out) != 1 || out[0] != "google_serpapi" {
		t.Fatalf("expected [google_serpapi], got %v", out)
	}
}

func TestFilterSearchProvidersIncludesSerpApi(t *testing.T) {
	out := filterSearchProviders([]string{"afklm", "google_serpapi"})
	if len(out) != 1 || out[0] != "google_serpapi" {
		t.Fatalf("expected [google_serpapi], got %v", out)
	}
}

func TestFlightProviderLabelSerpApi(t *testing.T) {
	got := flightProviderLabel(models.FlightResult{Provider: "google_serpapi"})
	if got != "Google (SerpApi)" {
		t.Fatalf("expected 'Google (SerpApi)', got %q", got)
	}
}

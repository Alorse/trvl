package main

import (
	"reflect"
	"testing"
)

func TestParseProviderList(t *testing.T) {
	cases := []struct {
		in      string
		want    []string
		wantErr bool
	}{
		{"", nil, false},
		{"duffel", []string{"duffel"}, false},
		{"google,kiwi", []string{"google", "kiwi"}, false},
		{" Google , DUFFEL ", []string{"google", "duffel"}, false},
		{"afklm", []string{"afklm"}, false},
		{"google,bogus", nil, true},
		{"x", nil, true},
	}
	for _, c := range cases {
		got, err := parseProviderList(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("parseProviderList(%q) expected error, got %v", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseProviderList(%q) unexpected error: %v", c.in, err)
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("parseProviderList(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestFilterSearchProviders(t *testing.T) {
	// afklm is excluded (it has its own routing); google/kiwi/duffel pass through.
	got := filterSearchProviders([]string{"afklm", "duffel", "google"})
	want := []string{"duffel", "google"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("filterSearchProviders = %v, want %v", got, want)
	}
	if filterSearchProviders([]string{"afklm"}) != nil {
		t.Errorf("afklm-only should yield nil search providers (default behavior)")
	}
}

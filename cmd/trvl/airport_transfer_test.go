package main

import (
	"testing"

	"github.com/MikkoParkkola/trvl/internal/models"
)

func TestHasAirportTransferProvider(t *testing.T) {
	routes := []models.GroundRoute{
		{Provider: "flixbus"},
		{Provider: "RegioJet"},
	}

	tests := []struct {
		provider string
		want     bool
	}{
		{"flixbus", true},
		{"FlixBus", true},
		{"regiojet", true},
		{"eurostar", false},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			got := hasAirportTransferProvider(routes, tt.provider)
			if got != tt.want {
				t.Errorf("hasAirportTransferProvider(%q) = %v, want %v", tt.provider, got, tt.want)
			}
		})
	}
}

func TestHasAirportTransferProvider_Empty(t *testing.T) {
	got := hasAirportTransferProvider(nil, "flixbus")
	if got {
		t.Error("should return false for nil routes")
	}
}

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var readmeToolMarkers = []string{
	"search_flights",
	"search_dates",
	"search_hotels",
	"hotel_prices",
	"hotel_reviews",
	"hotel_rooms",
	"destination_info",
	"calculate_trip_cost",
	"weekend_getaway",
	"suggest_dates",
	"optimize_multi_city",
	"nearby_places",
	"travel_guide",
	"local_events",
	"search_ground",
	"search_airport_transfers",
	"search_restaurants",
	"search_deals",
	"plan_trip",
	"search_route",
	"get_preferences",
	"detect_travel_hacks",
	"detect_accommodation_hacks",
	"search_natural",
	"list_trips",
	"get_trip",
	"create_trip",
	"add_trip_leg",
	"mark_trip_booked",
	"get_weather",
	"get_baggage_rules",
}

func TestPublicDocsAdvertiseCurrentCounts(t *testing.T) {
	t.Parallel()

	checks := []struct {
		path      string
		required  []string
		forbidden []string
	}{
		{
			path: filepath.Join("..", "..", "README.md"),
			required: []string{
				"31 travel tools for your AI assistant",
				"standalone CLI with 31 commands",
				"31 travel tools available",
				"Full v2025-11-25 — 31 tools",
				"31 commands (+ 6 watch subcommands)",
				"Full JSON Schema validation for all 31 tool responses",
			},
			forbidden: []string{
				"29 travel tools for your AI assistant",
				"standalone CLI with 29 commands",
				"29 travel tools available",
				"Full v2025-11-25 — 29 tools",
				"29 commands (+ 6 watch subcommands)",
				"Full JSON Schema validation for all 29 tool responses",
			},
		},
		{
			path: filepath.Join("..", "..", "AGENTS.md"),
			required: []string{
				"installed with 31 MCP tools and 5 skills",
				"You now have 31 MCP tools available.",
			},
			forbidden: []string{
				"installed with 22 MCP tools and 5 skills",
				"You now have 22 MCP tools available.",
			},
		},
		{
			path: filepath.Join("..", "..", "demo.tape"),
			required: []string{
				"# 31 MCP tools · 31 CLI commands · 17 providers · No API keys",
			},
			forbidden: []string{
				"# 29 MCP tools · 29 CLI commands · 17 providers · No API keys",
			},
		},
	}

	for _, check := range checks {
		check := check
		t.Run(filepath.Base(check.path), func(t *testing.T) {
			t.Parallel()

			data, err := os.ReadFile(check.path)
			if err != nil {
				t.Fatalf("ReadFile(%q): %v", check.path, err)
			}
			text := string(data)

			for _, needle := range check.required {
				if !strings.Contains(text, needle) {
					t.Errorf("%s missing required text %q", check.path, needle)
				}
			}
			for _, needle := range check.forbidden {
				if strings.Contains(text, needle) {
					t.Errorf("%s still contains stale text %q", check.path, needle)
				}
			}

			if filepath.Base(check.path) == "README.md" {
				for _, tool := range readmeToolMarkers {
					marker := fmt.Sprintf("**%s**", tool)
					if count := strings.Count(text, marker); count != 1 {
						t.Errorf("%s should mention %s exactly once in the MCP tool table, got %d", check.path, marker, count)
					}
				}
			}
		})
	}
}

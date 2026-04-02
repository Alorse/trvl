//go:build proof

package explore

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/MikkoParkkola/trvl/internal/batchexec"
)

// TestExploreDestinations is a proof test for the GetExploreDestinations endpoint.
//
// PASS: returns rich destination data (city, airport, price, airline) from HEL.
func TestExploreDestinations(t *testing.T) {
	c := batchexec.NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Search cheapest destinations from HEL, departing ~2 months out, round trip 7 days
	depDate := time.Now().AddDate(0, 2, 0).Format("2006-01-02")
	retDate := time.Now().AddDate(0, 2, 7).Format("2006-01-02")

	encoded := EncodeExplorePayload("HEL", ExploreOptions{
		DepartureDate: depDate,
		ReturnDate:    retDate,
		Adults:        1,
	})

	t.Logf("Encoded explore payload length: %d chars", len(encoded))

	status, body, err := c.PostExplore(ctx, encoded)
	if err != nil {
		t.Fatalf("KILL: explore request failed: %v", err)
	}

	t.Logf("Status: %d", status)
	t.Logf("Raw response length: %d bytes", len(body))

	if status == 403 {
		t.Fatalf("KILL: Google returned 403 -- endpoint blocked")
	}
	if status == 400 {
		t.Logf("=== RAW RESPONSE (first 2000) ===")
		t.Logf("%s", truncate(body, 2000))
		t.Fatalf("400 Bad Request -- payload format wrong")
	}
	if status != 200 {
		t.Logf("=== RAW RESPONSE (first 2000) ===")
		t.Logf("%s", truncate(body, 2000))
		t.Fatalf("unexpected status %d", status)
	}

	// Parse destinations
	destinations, err := ParseExploreResponse(body)
	if err != nil {
		t.Logf("=== RAW RESPONSE (first 5000) ===")
		t.Logf("%s", truncate(body, 5000))
		t.Fatalf("KILL: could not parse explore response: %v", err)
	}

	t.Logf("Found %d destinations", len(destinations))

	if len(destinations) == 0 {
		t.Fatalf("KILL: no destinations found in response")
	}

	// Print first 10 destinations
	for i, d := range destinations {
		if i >= 10 {
			t.Logf("... and %d more destinations", len(destinations)-10)
			break
		}
		t.Logf("  [%d] city=%s airport=%s price=%.0f airline=%s stops=%d",
			i, d.CityID, d.AirportCode, d.Price, d.AirlineName, d.Stops)
	}

	// Verify at least one destination has all required fields
	found := false
	for _, d := range destinations {
		if d.AirportCode != "" && d.Price > 0 && d.CityID != "" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("KILL: no destination has all required fields (airport, price, city)")
	}

	t.Log("PASS: GetExploreDestinations endpoint is working")
}

// TestExploreOneWay tests one-way explore search.
func TestExploreOneWay(t *testing.T) {
	c := batchexec.NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	depDate := time.Now().AddDate(0, 2, 0).Format("2006-01-02")

	encoded := EncodeExplorePayload("HEL", ExploreOptions{
		DepartureDate: depDate,
		Adults:        1,
	})

	status, body, err := c.PostExplore(ctx, encoded)
	if err != nil {
		t.Fatalf("explore one-way request failed: %v", err)
	}

	t.Logf("One-way: status=%d len=%d", status, len(body))

	if status == 200 {
		destinations, err := ParseExploreResponse(body)
		if err == nil && len(destinations) > 0 {
			t.Logf("One-way explore: %d destinations found", len(destinations))
			for i, d := range destinations[:min(5, len(destinations))] {
				t.Logf("  [%d] %s (%s) = %.0f", i, d.CityID, d.AirportCode, d.Price)
			}
		}
	}
}

// TestExploreWithCoordinates tests geographic bounding box filtering.
func TestExploreWithCoordinates(t *testing.T) {
	c := batchexec.NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	depDate := time.Now().AddDate(0, 2, 0).Format("2006-01-02")
	retDate := time.Now().AddDate(0, 2, 7).Format("2006-01-02")

	// Bounding box for southern Europe
	encoded := EncodeExplorePayload("HEL", ExploreOptions{
		DepartureDate: depDate,
		ReturnDate:    retDate,
		Adults:        1,
		NorthLat:      45.0,
		SouthLat:      35.0,
		EastLng:       30.0,
		WestLng:       -10.0,
	})

	status, body, err := c.PostExplore(ctx, encoded)
	if err != nil {
		t.Fatalf("explore with coords request failed: %v", err)
	}

	t.Logf("With coords: status=%d len=%d", status, len(body))

	if status == 200 {
		destinations, err := ParseExploreResponse(body)
		if err == nil {
			t.Logf("Filtered explore: %d destinations", len(destinations))
			for i, d := range destinations[:min(5, len(destinations))] {
				t.Logf("  [%d] %s (%s) = %.0f", i, d.CityID, d.AirportCode, d.Price)
			}
		}
	}
}

// TestExploreRawResponse prints the full raw response for analysis.
func TestExploreRawResponse(t *testing.T) {
	c := batchexec.NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	depDate := time.Now().AddDate(0, 2, 0).Format("2006-01-02")
	retDate := time.Now().AddDate(0, 2, 7).Format("2006-01-02")

	encoded := EncodeExplorePayload("HEL", ExploreOptions{
		DepartureDate: depDate,
		ReturnDate:    retDate,
		Adults:        1,
	})

	status, body, err := c.PostExplore(ctx, encoded)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	if status != 200 {
		t.Fatalf("status %d", status)
	}

	// Decode and pretty-print the batch entries
	entries, err := batchexec.DecodeBatchResponse(body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	for i, entry := range entries {
		pretty, _ := json.MarshalIndent(entry, "", "  ")
		t.Logf("=== ENTRY %d (first 3000) ===", i)
		t.Logf("%s", truncate(pretty, 3000))
	}
}

func truncate(b []byte, maxLen int) string {
	if len(b) <= maxLen {
		return string(b)
	}
	return string(b[:maxLen]) + fmt.Sprintf("... [truncated, %d total]", len(b))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

//go:build proof

package hotels

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/MikkoParkkola/trvl/internal/batchexec"
)

// TestReviewsBatchExecute tries the ocp93e rpcid for hotel reviews.
//
// This is a discovery test to understand the response format.
func TestReviewsBatchExecute(t *testing.T) {
	c := batchexec.NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// Try known hotel IDs from Helsinki.
	hotelIDs := []string{
		"/g/11b6d4_v_4",               // Hotel Kamp Helsinki
		"ChIJy7MSZP0LkkYRZw2dDekQP78", // Helsinki hotel
	}

	for _, hotelID := range hotelIDs {
		t.Logf("--- Trying ocp93e for hotel ID: %s ---", hotelID)

		// Try several payload formats for reviews.
		payloads := []struct {
			name string
			args string
		}{
			{
				name: "minimal",
				args: fmt.Sprintf(`[%q]`, hotelID),
			},
			{
				name: "with null padding",
				args: fmt.Sprintf(`[null,%q]`, hotelID),
			},
			{
				name: "with sort and limit",
				args: fmt.Sprintf(`[null,%q,null,null,null,10,1]`, hotelID),
			},
			{
				name: "hotel id first with params",
				args: fmt.Sprintf(`[%q,null,null,null,null,10]`, hotelID),
			},
		}

		for _, p := range payloads {
			encoded := batchexec.EncodeBatchExecute("ocp93e", p.args)
			status, body, err := c.BatchExecute(ctx, encoded)
			if err != nil {
				t.Logf("[ocp93e/%s/%s] error: %v", hotelID, p.name, err)
				continue
			}

			hasData := len(body) > 200
			t.Logf("[ocp93e/%s/%s] status=%d len=%d hasData=%v",
				hotelID, p.name, status, len(body), hasData)

			if status == 200 && hasData {
				t.Logf("=== PROMISING: ocp93e/%s/%s (first 5000) ===", hotelID, p.name)
				t.Logf("%s", truncateBytes(body, 5000))

				entries, err := batchexec.DecodeBatchResponse(body)
				if err == nil {
					t.Logf("Decoded %d entries", len(entries))
					for i, entry := range entries {
						pretty, _ := json.MarshalIndent(entry, "", "  ")
						t.Logf("Entry %d (first 3000): %s", i, truncateBytes(pretty, 3000))
					}
				}
				return
			}
		}
	}

	t.Log("ocp93e did not return useful data — trying page scraping fallback")
}

// TestReviewsPageScrape fetches the hotel entity page and looks for review data
// in AF_initDataCallback blocks.
func TestReviewsPageScrape(t *testing.T) {
	c := batchexec.NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// First, do a hotel search to get a real hotel ID.
	t.Log("=== Step 1: Search hotels to get a real hotel ID ===")
	searchResult, err := SearchHotels(ctx, "Helsinki", HotelSearchOptions{
		CheckIn:  "2026-06-15",
		CheckOut: "2026-06-18",
		Guests:   2,
	})
	if err != nil {
		t.Fatalf("hotel search failed: %v", err)
	}

	var hotelID string
	for _, h := range searchResult.Hotels {
		if h.HotelID != "" {
			hotelID = h.HotelID
			t.Logf("Using hotel: %s (ID: %s, rating: %.1f, reviews: %d)",
				h.Name, h.HotelID, h.Rating, h.ReviewCount)
			break
		}
	}
	if hotelID == "" {
		t.Fatal("no hotel with ID found in search results")
	}

	// Step 2: Fetch the hotel entity page.
	t.Log("=== Step 2: Fetch hotel entity page ===")
	entityURL := fmt.Sprintf("https://www.google.com/travel/hotels/entity/%s?hl=en-US&gl=us", hotelID)
	t.Logf("Fetching: %s", entityURL)

	status, body, err := c.Get(ctx, entityURL)
	if err != nil {
		t.Fatalf("entity page request failed: %v", err)
	}

	t.Logf("Status: %d, Body length: %d", status, len(body))

	if status != 200 {
		t.Logf("Response (first 2000): %s", truncateBytes(body, 2000))
		t.Fatalf("entity page returned status %d", status)
	}

	// Step 3: Extract AF_initDataCallback blocks.
	t.Log("=== Step 3: Parse AF_initDataCallback blocks ===")
	callbacks := extractCallbacks(string(body))
	t.Logf("Found %d AF_initDataCallback blocks", len(callbacks))

	for i, cb := range callbacks {
		data, _ := json.Marshal(cb)
		t.Logf("Callback %d: %d bytes", i, len(data))
		// Print first portion of each callback to find review data.
		t.Logf("Callback %d (first 3000): %s", i, truncateBytes(data, 3000))
	}
}

// TestReviewsEndToEnd searches for a hotel and fetches its reviews.
func TestReviewsEndToEnd(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Search hotels first.
	searchResult, err := SearchHotels(ctx, "Helsinki", HotelSearchOptions{
		CheckIn:  "2026-06-15",
		CheckOut: "2026-06-18",
		Guests:   2,
	})
	if err != nil {
		t.Fatalf("hotel search failed: %v", err)
	}

	var hotelID, hotelName string
	for _, h := range searchResult.Hotels {
		if h.HotelID != "" && h.ReviewCount > 0 {
			hotelID = h.HotelID
			hotelName = h.Name
			break
		}
	}
	if hotelID == "" {
		t.Fatal("no hotel with reviews found")
	}

	t.Logf("Fetching reviews for: %s (%s)", hotelName, hotelID)

	result, err := GetHotelReviews(ctx, hotelID, ReviewOptions{Limit: 5})
	if err != nil {
		t.Fatalf("GetHotelReviews failed: %v", err)
	}

	t.Logf("Success: %v", result.Success)
	t.Logf("Hotel: %s", result.Name)
	t.Logf("Average rating: %.1f", result.Summary.AverageRating)
	t.Logf("Total reviews: %d", result.Summary.TotalReviews)
	t.Logf("Reviews returned: %d", result.Count)

	for i, r := range result.Reviews {
		t.Logf("Review %d: %.0f/5 by %s (%s) — %s",
			i+1, r.Rating, r.Author, r.Date, truncateString(r.Text, 100))
	}

	pretty, _ := json.MarshalIndent(result, "", "  ")
	t.Logf("Full result:\n%s", string(pretty))
}

func truncateBytes(b []byte, maxLen int) string {
	if len(b) <= maxLen {
		return string(b)
	}
	return string(b[:maxLen]) + fmt.Sprintf("... [truncated, %d total]", len(b))
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

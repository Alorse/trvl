//go:build proof

package flights

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/MikkoParkkola/trvl/internal/batchexec"
)

// encodeCalendarGraphPayload builds the f.req body for GetCalendarGraph.
//
// Uses the [[],<settings>] prefix format (proven to get 200 from the endpoint)
// with gflights-style calendar settings.
func encodeCalendarGraphPayload(src, dst, rangeStart, rangeEnd string, tripLengthDays int) string {
	tripType := 2 // one-way
	if tripLengthDays > 0 {
		tripType = 1 // round-trip
	}

	serSrc := fmt.Sprintf(`[\"%s\",0]`, src)
	serDst := fmt.Sprintf(`[\"%s\",0]`, dst)

	// Calendar settings with segments (gflights getCalendarRawData format)
	rawData := fmt.Sprintf(`null,null,[null,null,%d,null,[],%d,[1,0,0,0],null,null,null,null,null,null,`,
		tripType, 1) // class=1 economy

	// Outbound segment
	rawData += fmt.Sprintf(`[[[[%s]],[[%s]],null,0,null,null,\"%s\",null,null,null,null,null,null,null,3]`,
		serSrc, serDst, rangeStart)

	// Return segment (for round-trip)
	if tripLengthDays > 0 {
		rawData += fmt.Sprintf(`,[[[%s]],[[%s]],null,0,null,null,\"%s\",null,null,null,null,null,null,null,1]`,
			serDst, serSrc, rangeEnd)
	}

	rawData += `]]`

	// Wrap with explore-style [[],] prefix which gets 200 from the endpoint
	prefix := `[null,"[[],`

	var suffix string
	if tripLengthDays > 0 {
		suffix = fmt.Sprintf(`],null,null,null,1,null,null,null,null,null,[]],[\"%s\",\"%s\"],null,[%d,%d]]"]`,
			rangeStart, rangeEnd, tripLengthDays, tripLengthDays)
	} else {
		suffix = fmt.Sprintf(`],null,null,null,1],[\"%s\",\"%s\"]]"]`,
			rangeStart, rangeEnd)
	}

	return url.QueryEscape(prefix + rawData + suffix)
}

// TestCalendarGraph is a proof test for the GetCalendarGraph endpoint.
//
// FINDING: The endpoint accepts our request (200) but returns [3] error code
// in the response body, meaning the query parameters are not in the format
// the endpoint expects. The gflights library requires a full Session with
// cookies and city name resolution (serialiseFlightLocations -> abbrCities)
// which we cannot replicate without browser cookies.
//
// STATUS: NEEDS INVESTIGATION -- endpoint exists and accepts requests but
// the exact payload format for airport codes (vs city abbreviations) is not
// yet determined.
func TestCalendarGraph(t *testing.T) {
	c := batchexec.NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	rangeStart := "2026-06-01"
	rangeEnd := "2026-06-30"

	// One-way test
	encoded := encodeCalendarGraphPayload("HEL", "NRT", rangeStart, rangeEnd, 0)

	t.Logf("Encoded calendar graph payload length: %d chars", len(encoded))

	status, body, err := c.PostCalendarGraph(ctx, encoded)
	if err != nil {
		t.Fatalf("KILL: calendar graph request failed: %v", err)
	}

	t.Logf("Status: %d, Body length: %d", status, len(body))
	t.Logf("Raw response: %s", string(body))

	if status == 403 {
		t.Fatalf("KILL: Google returned 403 -- endpoint blocked")
	}

	if status == 200 {
		t.Log("Endpoint accepts requests (200)")

		// Check for [3] error code
		if len(body) < 200 {
			t.Log("FINDING: Response is small -- likely [3] error (query format not right)")
			t.Log("The GetCalendarGraph endpoint likely requires city abbreviation codes")
			t.Log("(obtained via Session.abbrCities) rather than raw IATA airport codes.")
			t.Log("STATUS: Endpoint exists, needs city code resolution to produce data.")
		} else {
			t.Log("Got substantive response -- parsing...")
			entries, err := batchexec.DecodeBatchResponse(body)
			if err != nil {
				t.Logf("DecodeBatchResponse failed: %v", err)
			} else {
				t.Logf("Got %d entries", len(entries))
				for i, entry := range entries {
					pretty, _ := json.MarshalIndent(entry, "", "  ")
					t.Logf("Entry %d: %s", i, truncateStr(pretty, 3000))
				}
			}
		}
		return
	}

	if status == 400 {
		t.Log("FINDING: 400 Bad Request -- payload format rejected")
		t.Log("The GetCalendarGraph endpoint requires [null,] prefix (not [[],])")
		t.Log("in the inner payload, but this format gets rejected without cookies.")
	}
}

// TestCalendarGraphRoundTrip tests the round-trip variant.
func TestCalendarGraphRoundTrip(t *testing.T) {
	c := batchexec.NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	rangeStart := "2026-06-01"
	rangeEnd := "2026-06-30"
	tripLength := 7

	encoded := encodeCalendarGraphPayload("HEL", "NRT", rangeStart, rangeEnd, tripLength)
	status, body, err := c.PostCalendarGraph(ctx, encoded)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	t.Logf("Round-trip: status=%d len=%d", status, len(body))
	t.Logf("Raw: %s", string(body))
}

func truncateStr(b []byte, maxLen int) string {
	if len(b) <= maxLen {
		return string(b)
	}
	return string(b[:maxLen]) + fmt.Sprintf("... [truncated, %d total]", len(b))
}

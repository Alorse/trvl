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

// encodeProofCalendarGridPayload builds the f.req body for GetCalendarGrid
// using raw IATA codes (no city code resolution) for proof testing.
//
// Uses [null,] prefix and wraps segments in an array (gflights format).
func encodeProofCalendarGridPayload(src, dst, depStart, depEnd, retStart, retEnd string) string {
	serSrc := fmt.Sprintf(`[\"%s\",0]`, src)
	serDst := fmt.Sprintf(`[\"%s\",0]`, dst)

	// Always round trip for grid. rawData opens settings + segments, unclosed.
	rawData := fmt.Sprintf(`[null,null,%d,null,[],%d,[1,0,0,0],null,null,null,null,null,null,[`,
		1, 1) // tripType=1, class=1

	// Outbound segment
	rawData += fmt.Sprintf(`[[[%s]],[[%s]],null,0,null,null,\"%s\",null,null,null,null,null,null,null,3]`,
		serSrc, serDst, depStart)

	// Return segment
	rawData += fmt.Sprintf(`,[[[%s]],[[%s]],null,0,null,null,\"%s\",null,null,null,null,null,null,null,1]`,
		serDst, serSrc, retStart)

	// rawData left unclosed -- suffix handles closing brackets.

	prefix := `[null,"[null,`
	suffix := fmt.Sprintf(`],null,null,null,1],[\"%s\",\"%s\"],[\"%s\",\"%s\"]]"]`,
		depStart, depEnd, retStart, retEnd)

	return url.QueryEscape(prefix + rawData + suffix)
}

// TestCalendarGrid is a proof test for the GetCalendarGrid endpoint.
//
// FINDING: Same as CalendarGraph -- the endpoint requires city abbreviation
// codes from a Session, not raw IATA airport codes. Returns [3] error with
// our format.
//
// STATUS: NEEDS INVESTIGATION -- endpoint exists, needs city code resolution.
func TestCalendarGrid(t *testing.T) {
	c := batchexec.NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	depStart := "2026-07-01"
	depEnd := "2026-07-07"
	retStart := "2026-07-08"
	retEnd := "2026-07-14"

	encoded := encodeProofCalendarGridPayload("HEL", "BCN", depStart, depEnd, retStart, retEnd)

	t.Logf("Encoded calendar grid payload length: %d chars", len(encoded))

	status, body, err := c.PostCalendarGrid(ctx, encoded)
	if err != nil {
		t.Fatalf("KILL: calendar grid request failed: %v", err)
	}

	t.Logf("Status: %d, Body length: %d", status, len(body))
	t.Logf("Raw response: %s", string(body))

	if status == 403 {
		t.Fatalf("KILL: Google returned 403 -- endpoint blocked")
	}

	if status == 200 {
		t.Log("Endpoint accepts requests (200)")

		if len(body) < 200 {
			t.Log("FINDING: Small response -- likely [3] error")
			t.Log("Same issue as CalendarGraph: needs city abbreviation codes.")
		} else {
			t.Log("Got substantive response -- parsing...")
			entries, err := batchexec.DecodeBatchResponse(body)
			if err != nil {
				t.Logf("DecodeBatchResponse failed: %v", err)
			} else {
				t.Logf("Got %d entries", len(entries))
				for i, entry := range entries {
					pretty, _ := json.MarshalIndent(entry, "", "  ")
					t.Logf("Entry %d: %s", i, gridTruncate(pretty, 3000))
				}
			}
		}
	}

	if status == 400 {
		t.Log("FINDING: 400 Bad Request -- payload format needs [[],] prefix")
	}
}

func gridTruncate(b []byte, maxLen int) string {
	if len(b) <= maxLen {
		return string(b)
	}
	return string(b[:maxLen]) + fmt.Sprintf("... [truncated, %d total]", len(b))
}

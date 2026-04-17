package hotels

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestRoomsDebugSearchData(t *testing.T) {
	if os.Getenv("TRVL_TEST_LIVE_PROBES") == "" {
		t.Skip("set TRVL_TEST_LIVE_PROBES=1 to run live probe tests")
	}

	checkIn := "2026-08-01"
	checkOut := "2026-08-04"
	currency := "EUR"
	ctx := context.Background()
	client := DefaultClient()

	searchURL := fmt.Sprintf(
		"https://www.google.com/travel/hotels/Paris?q=Paris+hotels&dates=%s,%s&hl=en&currency=%s",
		checkIn, checkOut, currency,
	)

	status, body, err := client.Get(ctx, searchURL)
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}
	t.Logf("Search page: status=%d, length=%d", status, len(body))

	bodyStr := string(body)
	callbacks := extractCallbacks(bodyStr)
	t.Logf("Callbacks: %d", len(callbacks))

	// Find the largest callback (hotel data)
	var hotelData any
	maxSize := 0
	for i, cb := range callbacks {
		data, _ := json.Marshal(cb)
		t.Logf("  Callback %d: %d bytes", i, len(data))
		if len(data) > maxSize {
			maxSize = len(data)
			hotelData = cb
		}
	}

	if hotelData == nil {
		t.Fatal("no hotel data callback found")
	}

	// Navigate to data[0][0][0][1] -- the hotel list
	hotelList := navArr(hotelData, 0, 0, 0, 1)
	if hotelList == nil {
		t.Fatal("couldn't navigate to hotel list")
	}

	arr, ok := hotelList.([]any)
	if !ok {
		t.Fatal("hotel list is not array")
	}
	t.Logf("Hotel list has %d entries", len(arr))

	// Find the first actual hotel (not metadata)
	hotelsDumped := 0
	for idx, entry := range arr {
		if hotelsDumped >= 1 {
			break
		}
		entryArr, ok := entry.([]any)
		if !ok || len(entryArr) < 2 {
			continue
		}
		mapVal, ok := entryArr[1].(map[string]any)
		if !ok {
			continue
		}

		for key, val := range mapVal {
			// Skip known non-hotel keys
			if key == "300000000" || key == "416343588" || key == "410579159" || key == "429411180" {
				continue
			}

			hotelArr, ok := val.([]any)
			if !ok || len(hotelArr) == 0 {
				continue
			}
			hotelEntry, ok := hotelArr[0].([]any)
			if !ok || len(hotelEntry) < 3 {
				continue
			}

			name := ""
			if len(hotelEntry) > 1 {
				if s, ok := hotelEntry[1].(string); ok {
					name = s
				}
			}
			if name == "" {
				continue
			}

			hotelsDumped++
			t.Logf("\n=== Hotel %d: %s (key=%s, %d fields) ===", idx, name, key, len(hotelEntry))

			// Dump field [6] (price block) in full detail
			if len(hotelEntry) > 6 && hotelEntry[6] != nil {
				t.Log("  --- Price block [6] full dump ---")
				dumpFull(t, hotelEntry[6], "  [6]", 0, 6)
			}
			break
		}
	}
}

func dumpFull(t *testing.T, v any, path string, depth, maxDepth int) {
	if depth >= maxDepth {
		data, _ := json.Marshal(v)
		preview := string(data)
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}
		t.Logf("%s: %s", path, preview)
		return
	}
	switch val := v.(type) {
	case []any:
		t.Logf("%s: array[%d]", path, len(val))
		for i, item := range val {
			dumpFull(t, item, fmt.Sprintf("%s[%d]", path, i), depth+1, maxDepth)
		}
	case map[string]any:
		t.Logf("%s: map[%d]", path, len(val))
		for k, mv := range val {
			dumpFull(t, mv, fmt.Sprintf("%s.%s", path, k), depth+1, maxDepth)
		}
	case string:
		if len(val) > 100 {
			t.Logf("%s: %q...", path, val[:100])
		} else {
			t.Logf("%s: %q", path, val)
		}
	case float64:
		t.Logf("%s: %v", path, val)
	case nil:
		t.Logf("%s: null", path)
	default:
		t.Logf("%s: %T", path, val)
	}
}

func navArr(v any, indices ...int) any {
	for _, idx := range indices {
		arr, ok := v.([]any)
		if !ok || idx >= len(arr) {
			return nil
		}
		v = arr[idx]
	}
	return v
}

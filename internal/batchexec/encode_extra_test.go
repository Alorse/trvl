package batchexec

import (
	"encoding/json"
	"net/url"
	"testing"
)

func TestEncodeBatchExecute_VariousRPCIDs(t *testing.T) {
	tests := []struct {
		name    string
		rpcid   string
		args    string
	}{
		{"hotel search", "AtySUc", `["Helsinki"]`},
		{"hotel price", "yY52ce", `[null,[2026,6,15],[2026,6,18]]`},
		{"empty args", "TestRPC", `[]`},
		{"nested args", "NestedRPC", `[null,[1,2,3],["a","b"]]`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EncodeBatchExecute(tt.rpcid, tt.args)
			if result == "" {
				t.Fatal("encoded result is empty")
			}

			decoded, err := url.QueryUnescape(result)
			if err != nil {
				t.Fatalf("unescape: %v", err)
			}

			var outer []any
			if err := json.Unmarshal([]byte(decoded), &outer); err != nil {
				t.Fatalf("unmarshal outer: %v", err)
			}

			// Verify structure: [[[rpcid, args, null, "generic"]]]
			mid := outer[0].([]any)
			inner := mid[0].([]any)
			if inner[0] != tt.rpcid {
				t.Errorf("rpcid = %v, want %v", inner[0], tt.rpcid)
			}
			if inner[1] != tt.args {
				t.Errorf("args = %v, want %v", inner[1], tt.args)
			}
			if inner[3] != "generic" {
				t.Errorf("inner[3] = %v, want generic", inner[3])
			}
		})
	}
}

func TestEncodeFlightFilters_VariousInputs(t *testing.T) {
	tests := []struct {
		name     string
		filters  any
		wantErr  bool
	}{
		{"simple string", "hello", false},
		{"nested array", []any{1, "test", nil}, false},
		{"map", map[string]any{"key": "val"}, false},
		{"nil", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := EncodeFlightFilters(tt.filters)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result == "" {
				t.Error("result is empty")
			}

			// Verify URL encoding.
			decoded, err := url.QueryUnescape(result)
			if err != nil {
				t.Fatalf("unescape: %v", err)
			}

			// Should be valid JSON: [null, "<json-string>"]
			var wrapper []any
			if err := json.Unmarshal([]byte(decoded), &wrapper); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if len(wrapper) != 2 {
				t.Fatalf("expected 2 elements, got %d", len(wrapper))
			}
			if wrapper[0] != nil {
				t.Errorf("wrapper[0] = %v, want nil", wrapper[0])
			}
		})
	}
}

func TestBuildFlightFilters_MultiplePassengers(t *testing.T) {
	filters := BuildFlightFilters("JFK", "LAX", "2026-12-25", 3)

	data, err := json.Marshal(filters)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var arr []any
	if err := json.Unmarshal(data, &arr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	settings := arr[1].([]any)
	pax := settings[6].([]any)
	if v, ok := pax[0].(float64); !ok || int(v) != 3 {
		t.Errorf("adults = %v, want 3", pax[0])
	}
}

func TestBuildFlightFilters_DifferentAirports(t *testing.T) {
	tests := []struct {
		dep, arr, date string
	}{
		{"HEL", "NRT", "2026-06-15"},
		{"JFK", "LAX", "2026-12-25"},
		{"LHR", "CDG", "2026-01-01"},
	}

	for _, tt := range tests {
		t.Run(tt.dep+"->"+tt.arr, func(t *testing.T) {
			filters := BuildFlightFilters(tt.dep, tt.arr, tt.date, 1)
			data, err := json.Marshal(filters)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			var arr []any
			if err := json.Unmarshal(data, &arr); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			settings := arr[1].([]any)
			segments := settings[13].([]any)
			seg := segments[0].([]any)

			// Verify date in segment
			if seg[6] != tt.date {
				t.Errorf("date = %v, want %v", seg[6], tt.date)
			}
		})
	}
}

func TestBuildHotelSearchPayload_VariousLocations(t *testing.T) {
	tests := []struct {
		location string
		adults   int
	}{
		{"Helsinki", 2},
		{"Tokyo, Japan", 1},
		{"New York City", 4},
	}

	for _, tt := range tests {
		t.Run(tt.location, func(t *testing.T) {
			result := BuildHotelSearchPayload(tt.location, [3]int{2026, 6, 15}, [3]int{2026, 6, 18}, tt.adults)
			if result == "" {
				t.Fatal("empty result")
			}

			decoded, err := url.QueryUnescape(result)
			if err != nil {
				t.Fatalf("unescape: %v", err)
			}

			var outer []any
			if err := json.Unmarshal([]byte(decoded), &outer); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			mid := outer[0].([]any)
			inner := mid[0].([]any)
			if inner[0] != "AtySUc" {
				t.Errorf("rpcid = %v, want AtySUc", inner[0])
			}
		})
	}
}

func TestEncodeFlightFilters_MarshalError(t *testing.T) {
	// A channel cannot be marshalled to JSON.
	_, err := EncodeFlightFilters(make(chan int))
	if err == nil {
		t.Error("expected error for unmarshalable input")
	}
}

func TestBuildHotelPricePayload_Currencies(t *testing.T) {
	tests := []struct {
		hotelID  string
		currency string
	}{
		{"/g/11test", "USD"},
		{"/g/11other", "EUR"},
		{"ChIJ123", "GBP"},
	}

	for _, tt := range tests {
		t.Run(tt.currency, func(t *testing.T) {
			result := BuildHotelPricePayload(tt.hotelID, [3]int{2026, 6, 15}, [3]int{2026, 6, 18}, tt.currency)
			if result == "" {
				t.Fatal("empty result")
			}

			decoded, err := url.QueryUnescape(result)
			if err != nil {
				t.Fatalf("unescape: %v", err)
			}

			var outer []any
			if err := json.Unmarshal([]byte(decoded), &outer); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			mid := outer[0].([]any)
			inner := mid[0].([]any)
			if inner[0] != "yY52ce" {
				t.Errorf("rpcid = %v, want yY52ce", inner[0])
			}
		})
	}
}

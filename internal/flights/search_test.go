package flights

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/MikkoParkkola/trvl/internal/models"
)

// loadGoldenResponse loads and parses the golden test response file.
// The file simulates the inner JSON structure after DecodeFlightResponse
// has stripped the anti-XSSI prefix and parsed the outer envelope.
func loadGoldenResponse(t *testing.T) []any {
	t.Helper()
	data, err := os.ReadFile("testdata/flight_response.json")
	if err != nil {
		t.Fatalf("read golden file: %v", err)
	}

	// The golden file represents the outer response array.
	// DecodeFlightResponse would extract outer[0][2] as the inner JSON.
	// Our golden file is structured as: [ [null, null, flights_data] ]
	// So we parse the whole thing, then extract [0][2] to match what
	// ExtractFlightData expects.
	var outer []any
	if err := json.Unmarshal(data, &outer); err != nil {
		t.Fatalf("unmarshal golden file: %v", err)
	}

	first, ok := outer[0].([]any)
	if !ok || len(first) < 3 {
		t.Fatalf("golden file outer[0] invalid")
	}

	return first[2].([]any)
}

func TestParseFlights_GoldenFile(t *testing.T) {
	rawFlights := loadGoldenResponse(t)

	// rawFlights is the array of flight entries at the [2] position.
	flights := parseFlights(rawFlights)

	if len(flights) != 3 {
		t.Fatalf("expected 3 flights, got %d", len(flights))
	}

	// Flight 1: Direct HEL->NRT on Finnair
	f1 := flights[0]
	if f1.Price != 523 {
		t.Errorf("flight 1 price: got %v, want 523", f1.Price)
	}
	if f1.Currency != "EUR" {
		t.Errorf("flight 1 currency: got %q, want EUR", f1.Currency)
	}
	if f1.Duration != 780 {
		t.Errorf("flight 1 duration: got %d, want 780", f1.Duration)
	}
	if f1.Stops != 0 {
		t.Errorf("flight 1 stops: got %d, want 0", f1.Stops)
	}
	if len(f1.Legs) != 1 {
		t.Fatalf("flight 1 legs: got %d, want 1", len(f1.Legs))
	}

	leg := f1.Legs[0]
	if leg.DepartureAirport.Code != "HEL" {
		t.Errorf("leg dep airport: got %q, want HEL", leg.DepartureAirport.Code)
	}
	if leg.ArrivalAirport.Code != "NRT" {
		t.Errorf("leg arr airport: got %q, want NRT", leg.ArrivalAirport.Code)
	}
	if leg.Airline != "Finnair" {
		t.Errorf("leg airline: got %q, want Finnair", leg.Airline)
	}
	if leg.AirlineCode != "AY" {
		t.Errorf("leg airline code: got %q, want AY", leg.AirlineCode)
	}
	if leg.FlightNumber != "AY 79" {
		t.Errorf("leg flight number: got %q, want AY 79", leg.FlightNumber)
	}
	if leg.DepartureTime != "2026-06-15T10:30" {
		t.Errorf("leg dep time: got %q, want 2026-06-15T10:30", leg.DepartureTime)
	}
	if leg.ArrivalTime != "2026-06-16T07:15" {
		t.Errorf("leg arr time: got %q, want 2026-06-16T07:15", leg.ArrivalTime)
	}
	if leg.Duration != 780 {
		t.Errorf("leg duration: got %d, want 780", leg.Duration)
	}

	// Flight 2: 1-stop HEL->FRA->NRT on Lufthansa
	f2 := flights[1]
	if f2.Price != 487 {
		t.Errorf("flight 2 price: got %v, want 487", f2.Price)
	}
	if f2.Stops != 1 {
		t.Errorf("flight 2 stops: got %d, want 1", f2.Stops)
	}
	if len(f2.Legs) != 2 {
		t.Fatalf("flight 2 legs: got %d, want 2", len(f2.Legs))
	}
	if f2.Legs[0].ArrivalAirport.Code != "FRA" {
		t.Errorf("flight 2 leg 0 arr: got %q, want FRA", f2.Legs[0].ArrivalAirport.Code)
	}
	if f2.Legs[1].DepartureAirport.Code != "FRA" {
		t.Errorf("flight 2 leg 1 dep: got %q, want FRA", f2.Legs[1].DepartureAirport.Code)
	}
	if f2.Duration != 1470 {
		t.Errorf("flight 2 duration: got %d, want 1470", f2.Duration)
	}

	// Flight 3: Direct on JAL
	f3 := flights[2]
	if f3.Price != 612 {
		t.Errorf("flight 3 price: got %v, want 612", f3.Price)
	}
	if f3.Legs[0].Airline != "Japan Airlines" {
		t.Errorf("flight 3 airline: got %q, want Japan Airlines", f3.Legs[0].Airline)
	}
}

func TestParseFlights_EmptyInput(t *testing.T) {
	flights := parseFlights(nil)
	if flights != nil {
		t.Errorf("expected nil for nil input, got %d flights", len(flights))
	}

	flights = parseFlights([]any{})
	if flights != nil {
		t.Errorf("expected nil for empty input, got %d flights", len(flights))
	}
}

func TestParseFlights_MalformedEntries(t *testing.T) {
	// Should skip entries that aren't arrays or are too short
	malformed := []any{
		"not an array",
		42,
		nil,
		[]any{}, // too short (< 2 elements)
		[]any{"only one element"},
	}

	flights := parseFlights(malformed)
	if len(flights) != 0 {
		t.Errorf("expected 0 flights from malformed input, got %d", len(flights))
	}
}

func TestFormatTime(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want string
	}{
		{"valid", []any{float64(2026), float64(6), float64(15), float64(10), float64(30)}, "2026-06-15T10:30"},
		{"midnight", []any{float64(2026), float64(1), float64(1), float64(0), float64(0)}, "2026-01-01T00:00"},
		{"nil", nil, ""},
		{"short array", []any{float64(2026), float64(6)}, ""},
		{"not array", "2026-06-15", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatTime(tt.in)
			if got != tt.want {
				t.Errorf("formatTime(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestToString(t *testing.T) {
	tests := []struct {
		in   any
		want string
	}{
		{nil, ""},
		{"hello", "hello"},
		{float64(42), "42"},
		{float64(3.14), "3.14"},
	}
	for _, tt := range tests {
		got := toString(tt.in)
		if got != tt.want {
			t.Errorf("toString(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestToInt(t *testing.T) {
	tests := []struct {
		in   any
		want int
	}{
		{nil, 0},
		{float64(42), 42},
		{float64(0), 0},
		{"not a number", 0},
	}
	for _, tt := range tests {
		got := toInt(tt.in)
		if got != tt.want {
			t.Errorf("toInt(%v) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestBuildFilters_OneWay(t *testing.T) {
	opts := SearchOptions{Adults: 1, CabinClass: models.Economy}
	filters := buildFilters("HEL", "NRT", "2026-06-15", opts)

	// Verify it produces a marshalable structure
	data, err := json.Marshal(filters)
	if err != nil {
		t.Fatalf("marshal filters: %v", err)
	}

	// Parse back and verify structure
	var arr []any
	if err := json.Unmarshal(data, &arr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(arr) != 6 {
		t.Fatalf("expected 6 top-level elements, got %d", len(arr))
	}

	// arr[1] is settings
	settings, ok := arr[1].([]any)
	if !ok {
		t.Fatalf("arr[1] not array")
	}

	// Trip type should be 2 (one-way)
	if tripType, ok := settings[2].(float64); !ok || int(tripType) != 2 {
		t.Errorf("trip type: got %v, want 2", settings[2])
	}

	// Cabin class should be 1 (economy)
	if cabin, ok := settings[5].(float64); !ok || int(cabin) != 1 {
		t.Errorf("cabin class: got %v, want 1", settings[5])
	}
}

func TestBuildFilters_RoundTrip(t *testing.T) {
	opts := SearchOptions{
		Adults:     2,
		CabinClass: models.Business,
		ReturnDate: "2026-06-20",
		SortBy:     models.SortCheapest,
	}
	filters := buildFilters("HEL", "NRT", "2026-06-15", opts)

	data, err := json.Marshal(filters)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var arr []any
	if err := json.Unmarshal(data, &arr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	settings := arr[1].([]any)

	// Trip type should be 1 (round-trip)
	if tripType, ok := settings[2].(float64); !ok || int(tripType) != 1 {
		t.Errorf("trip type: got %v, want 1 (round-trip)", settings[2])
	}

	// Cabin class should be 3 (business)
	if cabin, ok := settings[5].(float64); !ok || int(cabin) != 3 {
		t.Errorf("cabin class: got %v, want 3 (business)", settings[5])
	}

	// Should have 2 segments
	segments := settings[13].([]any)
	if len(segments) != 2 {
		t.Errorf("segments: got %d, want 2", len(segments))
	}

	// Sort by should be 2 (cheapest)
	if sortBy, ok := arr[2].(float64); !ok || int(sortBy) != 2 {
		t.Errorf("sort by: got %v, want 2 (cheapest)", arr[2])
	}
}

func TestSearchFlights_MissingParams(t *testing.T) {
	_, err := SearchFlights(t.Context(), "", "NRT", "2026-06-15", SearchOptions{})
	if err == nil {
		t.Error("expected error for missing origin")
	}

	_, err = SearchFlights(t.Context(), "HEL", "", "2026-06-15", SearchOptions{})
	if err == nil {
		t.Error("expected error for missing destination")
	}

	_, err = SearchFlights(t.Context(), "HEL", "NRT", "", SearchOptions{})
	if err == nil {
		t.Error("expected error for missing date")
	}
}

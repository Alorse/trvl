package batchexec

import (
	"encoding/json"
	"testing"
)

func TestStripAntiXSSI(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "standard prefix",
			in:   ")]}'\n[1,2,3]",
			want: "[1,2,3]",
		},
		{
			name: "prefix with spaces",
			in:   "  )]}' \n [1,2,3]",
			want: "[1,2,3]",
		},
		{
			name: "no prefix",
			in:   "[1,2,3]",
			want: "[1,2,3]",
		},
		{
			name: "empty after strip",
			in:   ")]}'",
			want: "",
		},
		{
			name: "empty input",
			in:   "",
			want: "",
		},
		{
			name: "only whitespace",
			in:   "   \n\t  ",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(StripAntiXSSI([]byte(tt.in)))
			if got != tt.want {
				t.Errorf("StripAntiXSSI(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestDecodeFlightResponse(t *testing.T) {
	// Build a minimal valid flight response.
	// Structure: outer[0][2] contains a JSON string with flight data.
	innerData := []any{"flight_data_here", nil, []any{"leg1"}}
	innerJSON, _ := json.Marshal(innerData)

	outer := []any{
		[]any{nil, nil, string(innerJSON)},
	}
	outerJSON, _ := json.Marshal(outer)

	// Prepend anti-XSSI prefix.
	body := append([]byte(")]}'\n"), outerJSON...)

	result, err := DecodeFlightResponse(body)
	if err != nil {
		t.Fatalf("DecodeFlightResponse error: %v", err)
	}

	// Result should be the parsed inner data.
	arr, ok := result.([]any)
	if !ok {
		t.Fatalf("result not array, got %T", result)
	}
	if len(arr) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(arr))
	}
	if arr[0] != "flight_data_here" {
		t.Errorf("arr[0] = %v, want flight_data_here", arr[0])
	}
}

func TestDecodeFlightResponse_AlreadyParsed(t *testing.T) {
	// When outer[0][2] is already a parsed structure (not a JSON string).
	inner := []any{"data"}
	outer := []any{
		[]any{nil, nil, inner},
	}
	outerJSON, _ := json.Marshal(outer)

	body := append([]byte(")]}'\n"), outerJSON...)

	result, err := DecodeFlightResponse(body)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	arr, ok := result.([]any)
	if !ok {
		t.Fatalf("result not array, got %T", result)
	}
	if len(arr) != 1 || arr[0] != "data" {
		t.Errorf("result = %v, want [data]", arr)
	}
}

func TestDecodeFlightResponse_Empty(t *testing.T) {
	_, err := DecodeFlightResponse([]byte(")]}'\n"))
	if err != ErrEmptyResponse {
		t.Errorf("expected ErrEmptyResponse, got %v", err)
	}

	_, err = DecodeFlightResponse([]byte(""))
	if err != ErrEmptyResponse {
		t.Errorf("expected ErrEmptyResponse for empty input, got %v", err)
	}
}

func TestDecodeFlightResponse_MalformedJSON(t *testing.T) {
	_, err := DecodeFlightResponse([]byte(")]}'\n{not valid json}"))
	if err == nil {
		t.Error("expected error for malformed JSON")
	}
}

func TestDecodeBatchResponse_DirectJSON(t *testing.T) {
	// When the response is a direct JSON array.
	data := []any{
		[]any{"wrb.fr", "AtySUc", `["data"]`, nil},
	}
	body, _ := json.Marshal(data)
	body = append([]byte(")]}'\n"), body...)

	results, err := DecodeBatchResponse(body)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected non-empty results")
	}
}

func TestDecodeBatchResponse_LengthPrefixed(t *testing.T) {
	// Length-prefixed format: \n<length>\n<json>\n
	entry := `[["wrb.fr","AtySUc","[\"hello\"]",null]]`
	body := []byte(")]}'\n")
	body = append(body, []byte("\n42\n")...)
	body = append(body, []byte(entry)...)
	body = append(body, '\n')

	results, err := DecodeBatchResponse(body)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected non-empty results")
	}
}

func TestDecodeBatchResponse_Empty(t *testing.T) {
	_, err := DecodeBatchResponse([]byte(")]}'\n"))
	if err != ErrEmptyResponse {
		t.Errorf("expected ErrEmptyResponse, got %v", err)
	}
}

func TestExtractFlightData(t *testing.T) {
	// ExtractFlightData expects inner[2] and/or inner[3] to contain flight buckets.
	// Each bucket has bucket[0] = array of flight entries.
	flight1 := []any{"flight1_data"}
	flight2 := []any{"flight2_data"}
	flight3 := []any{"flight3_data"}

	inner := []any{
		nil,                            // [0]
		nil,                            // [1]
		[]any{[]any{flight1, flight2}}, // [2]: bucket with 2 flights
		[]any{[]any{flight3}},          // [3]: bucket with 1 flight
	}

	flights, err := ExtractFlightData(inner)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	if len(flights) != 3 {
		t.Fatalf("expected 3 flights, got %d", len(flights))
	}
}

func TestExtractFlightData_OnlyIndex2(t *testing.T) {
	inner := []any{
		nil,
		nil,
		[]any{[]any{[]any{"f1"}, []any{"f2"}}},
	}

	flights, err := ExtractFlightData(inner)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(flights) != 2 {
		t.Fatalf("expected 2 flights, got %d", len(flights))
	}
}

// TestExtractFlightData_NoData covers a well-formed response that simply
// contains no flights. That is an answer, not a failure: ask for a non-stop
// BER-FUK and Google has none to give.
//
// This used to assert an error, and the error was expensive. It read like a
// parsing fault, was worded in terms of array indices, and was identical to
// what an anti-bot page produced — so an over-constrained search looked like a
// broken request encoder, and a filtering caller could not tell "nothing
// matched" from "the call failed".
func TestExtractFlightData_NoData(t *testing.T) {
	inner := []any{nil, nil}

	flights, err := ExtractFlightData(inner)
	if err != nil {
		t.Errorf("an empty result is not an error: %v", err)
	}
	if len(flights) != 0 {
		t.Errorf("expected no flights, got %d", len(flights))
	}
}

func TestExtractFlightData_NotArray(t *testing.T) {
	_, err := ExtractFlightData("not an array")
	if err == nil {
		t.Error("expected error for non-array input")
	}
}

// TestExtractFlightDataEmptyIsAnAnswerNotAnError pins the distinction that a
// downstream price cache depends on, using the shape that actually caused the
// bug: an over-constrained search.
//
// Google answers "one stop, BER to FUK" with a well-formed page containing no
// itineraries, because none exist — the cheapest real option has two stops.
// Returning an error there is wrong twice over. It says the call failed when it
// succeeded, and it collapses into the same message an anti-bot page produces,
// so nobody downstream can tell an impossible filter from a rate limit.
//
// The anti-bot case is already separated upstream: IsBlockedFlightResponse
// treats anything that does not decode into a []any as blocked, so a
// well-formed empty array only ever reaches here as a genuine empty result.
func TestExtractFlightDataEmptyIsAnAnswerNotAnError(t *testing.T) {
	cases := []struct {
		name  string
		inner any
	}{
		{"buckets absent entirely", []any{nil, nil}},
		{"buckets present but empty", []any{nil, nil, []any{[]any{}}, []any{[]any{}}}},
		{"array too short to hold buckets", []any{nil}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			flights, err := ExtractFlightData(tc.inner)
			if err != nil {
				t.Errorf("a filter matching nothing is not a failure: %v", err)
			}
			if len(flights) != 0 {
				t.Errorf("expected no flights, got %d", len(flights))
			}
		})
	}

	// A payload that is not an array never came from a real flight response,
	// and that stays an error — it is the one case here that means something
	// went wrong rather than something was absent.
	if _, err := ExtractFlightData("not an array"); err == nil {
		t.Error("a non-array payload is a genuine fault and must still error")
	}
}

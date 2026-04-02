package batchexec

import (
	"encoding/json"
	"testing"
)

func TestDecodeFlightResponse_ShortFirstArray(t *testing.T) {
	// outer[0] has only 2 elements (< 3 required).
	outer := []any{[]any{nil, nil}}
	outerJSON, _ := json.Marshal(outer)
	body := append([]byte(")]}'\n"), outerJSON...)

	_, err := DecodeFlightResponse(body)
	if err == nil {
		t.Error("expected error for short first array")
	}
}

func TestDecodeFlightResponse_NotArray(t *testing.T) {
	// outer[0] is not an array.
	outer := []any{"not an array"}
	outerJSON, _ := json.Marshal(outer)
	body := append([]byte(")]}'\n"), outerJSON...)

	_, err := DecodeFlightResponse(body)
	if err == nil {
		t.Error("expected error for non-array outer[0]")
	}
}

func TestDecodeFlightResponse_EmptyOuter(t *testing.T) {
	outer := []any{}
	outerJSON, _ := json.Marshal(outer)
	body := append([]byte(")]}'\n"), outerJSON...)

	_, err := DecodeFlightResponse(body)
	if err == nil {
		t.Error("expected error for empty outer array")
	}
}

func TestDecodeFlightResponse_InvalidInnerJSON(t *testing.T) {
	// outer[0][2] is a string that is NOT valid JSON.
	outer := []any{[]any{nil, nil, "not valid json {"}}
	outerJSON, _ := json.Marshal(outer)
	body := append([]byte(")]}'\n"), outerJSON...)

	_, err := DecodeFlightResponse(body)
	if err == nil {
		t.Error("expected error for invalid inner JSON string")
	}
}

func TestDecodeBatchResponse_MultipleLengthPrefixed(t *testing.T) {
	// Two length-prefixed entries.
	entry1 := `[["wrb.fr","rpc1","data1"]]`
	entry2 := `[["wrb.fr","rpc2","data2"]]`

	body := []byte(")]}'\n")
	body = append(body, []byte("\n42\n")...)
	body = append(body, []byte(entry1)...)
	body = append(body, []byte("\n42\n")...)
	body = append(body, []byte(entry2)...)

	results, err := DecodeBatchResponse(body)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

func TestDecodeBatchResponse_MalformedChunk(t *testing.T) {
	// Length-prefixed but with malformed JSON.
	body := []byte(")]}'\n")
	body = append(body, []byte("\n10\n")...)
	body = append(body, []byte("not json here but then\n")...)
	body = append(body, []byte("\n30\n")...)
	body = append(body, []byte(`[["wrb.fr","rpc","data"]]`)...)
	body = append(body, '\n')

	results, err := DecodeBatchResponse(body)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(results) == 0 {
		t.Error("expected at least 1 result despite malformed first chunk")
	}
}

func TestDecodeBatchResponse_SingleChunkNoNewline(t *testing.T) {
	// A single chunk without any newlines after the anti-XSSI prefix.
	body := []byte(")]}'\n[1,2,3]")
	results, err := DecodeBatchResponse(body)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 elements in direct parse, got %d", len(results))
	}
}

func TestExtractFlightData_EmptyBuckets(t *testing.T) {
	// Buckets at [2] and [3] exist but have empty inner arrays.
	inner := []any{
		nil,
		nil,
		[]any{[]any{}}, // bucket with empty items
		[]any{[]any{}}, // bucket with empty items
	}

	_, err := ExtractFlightData(inner)
	if err == nil {
		t.Error("expected error for empty flight buckets")
	}
}

func TestExtractFlightData_OnlyIndex3(t *testing.T) {
	// Only index 3 has data.
	inner := []any{
		nil,
		nil,
		nil, // index 2 is nil
		[]any{[]any{[]any{"flight_at_3"}}},
	}

	flights, err := ExtractFlightData(inner)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(flights) != 1 {
		t.Fatalf("expected 1 flight, got %d", len(flights))
	}
}

func TestExtractFlightData_BucketNotArray(t *testing.T) {
	// Index [2] exists but is not an array.
	inner := []any{
		nil,
		nil,
		"not an array",
	}

	_, err := ExtractFlightData(inner)
	if err == nil {
		t.Error("expected error when bucket is not array")
	}
}

func TestStripAntiXSSI_WithDoublePrefix(t *testing.T) {
	// Edge case: only the first prefix is stripped.
	input := ")]}')]}'\ndata"
	got := string(StripAntiXSSI([]byte(input)))
	// After trimming and stripping first prefix, remainder has second prefix.
	if got != ")]}'\ndata" {
		t.Errorf("got %q, want %q", got, ")]}'\ndata")
	}
}

func TestStripAntiXSSI_WhitespaceOnly(t *testing.T) {
	got := string(StripAntiXSSI([]byte("   \t\n   ")))
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestDecodeBatchResponse_NoArrayStart(t *testing.T) {
	// Length-prefixed but no '[' found.
	body := []byte(")]}'\n\n10\nno array here")
	_, err := DecodeBatchResponse(body)
	if err == nil {
		t.Error("expected error when no parseable entries found")
	}
}

func TestDecodeBatchResponse_LengthThenNoNewline(t *testing.T) {
	// A number line followed by EOF (no more data after the length).
	body := []byte(")]}'\n42")
	// This is treated as a single remaining chunk with text "42".
	// json.Unmarshal of "42" yields a float64, which is valid.
	results, err := DecodeBatchResponse(body)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestDecodeBatchResponse_LengthOnlyNewline(t *testing.T) {
	// Length line then newline then nothing — may parse "42" as a direct chunk.
	body := []byte(")]}'\n42\n")
	// "42" by itself is valid JSON (a number), so it gets parsed.
	results, err := DecodeBatchResponse(body)
	if err != nil {
		t.Logf("got error (acceptable): %v", err)
	} else if len(results) == 0 {
		t.Error("expected at least some results or an error")
	}
}

func TestDecodeBatchResponse_WhitespaceEntries(t *testing.T) {
	// Multiple length-prefixed entries with whitespace.
	body := []byte(")]}'\n  \n  42\n  [1,2,3]  \n  \n  42\n  [4,5,6]  \n")
	results, err := DecodeBatchResponse(body)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(results) < 1 {
		t.Error("expected at least 1 result")
	}
}

func TestDecodeBatchResponse_TrailingWhitespace(t *testing.T) {
	body := []byte(")]}'\n[1,2,3]   \n   ")
	results, err := DecodeBatchResponse(body)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("expected 3, got %d", len(results))
	}
}

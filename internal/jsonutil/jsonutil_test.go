package jsonutil

import (
	"encoding/json"
	"testing"
)

func TestNavigateArray(t *testing.T) {
	data := []any{[]any{[]any{"deep value"}}}
	if got := NavigateArray(data, 0, 0, 0); got != "deep value" {
		t.Fatalf("NavigateArray(...) = %v, want %q", got, "deep value")
	}
	if got := NavigateArray(data, 0, 9); got != nil {
		t.Fatalf("NavigateArray out-of-bounds = %v, want nil", got)
	}
	if got := NavigateArray("not an array", 0); got != nil {
		t.Fatalf("NavigateArray non-array = %v, want nil", got)
	}
}

func TestStringValue(t *testing.T) {
	if got := StringValue("hello"); got != "hello" {
		t.Fatalf("StringValue(\"hello\") = %q", got)
	}
	if got := StringValue(42); got != "" {
		t.Fatalf("StringValue(non-string) = %q, want empty", got)
	}
}

func TestToFloat(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  float64
		ok    bool
	}{
		{"float64", 3.14, 3.14, true},
		{"int", 42, 42, true},
		{"json number", json.Number("99.9"), 99.9, true},
		{"string", "not a number", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ToFloat(tt.input)
			if ok != tt.ok || (ok && got != tt.want) {
				t.Fatalf("ToFloat(%v) = (%v, %v), want (%v, %v)", tt.input, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestToInt(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  int
	}{
		{"nil", nil, 0},
		{"float64", float64(42), 42},
		{"int", 7, 7},
		{"json number", json.Number("12"), 12},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToInt(tt.input); got != tt.want {
				t.Fatalf("ToInt(%v) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

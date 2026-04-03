package models

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestFormatJSON(t *testing.T) {
	v := map[string]interface{}{
		"name":  "test",
		"price": 99.5,
	}

	var buf bytes.Buffer
	if err := FormatJSON(&buf, v); err != nil {
		t.Fatalf("FormatJSON error: %v", err)
	}

	// Must be valid JSON.
	var parsed map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	// Must be indented (contains newlines beyond just the trailing one).
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) < 3 {
		t.Errorf("expected pretty-printed output with multiple lines, got %d", len(lines))
	}

	// Verify content roundtrips.
	if parsed["name"] != "test" {
		t.Errorf("expected name=test, got %v", parsed["name"])
	}
}

func TestFormatJSON_HTMLNotEscaped(t *testing.T) {
	v := map[string]string{"url": "https://example.com?a=1&b=2"}

	var buf bytes.Buffer
	if err := FormatJSON(&buf, v); err != nil {
		t.Fatalf("FormatJSON error: %v", err)
	}

	if strings.Contains(buf.String(), `\u0026`) {
		t.Error("HTML entities should not be escaped in JSON output")
	}
}

func TestFormatTable_Basic(t *testing.T) {
	var buf bytes.Buffer
	headers := []string{"Name", "Price", "Rating"}
	rows := [][]string{
		{"Hotel A", "150.00", "4.5"},
		{"Hotel Brilliant", "89.99", "4.8"},
	}

	FormatTable(&buf, headers, rows)
	output := buf.String()

	// Must contain all header values.
	for _, h := range headers {
		if !strings.Contains(output, h) {
			t.Errorf("output missing header %q", h)
		}
	}

	// Must contain all cell values.
	for _, row := range rows {
		for _, cell := range row {
			if !strings.Contains(output, cell) {
				t.Errorf("output missing cell %q", cell)
			}
		}
	}

	// Must contain separator line.
	if !strings.Contains(output, "---") {
		t.Error("output missing separator line")
	}
}

func TestFormatTable_VariableWidths(t *testing.T) {
	var buf bytes.Buffer
	headers := []string{"X", "LongHeader"}
	rows := [][]string{
		{"short", "a"},
		{"a-much-longer-value", "b"},
	}

	FormatTable(&buf, headers, rows)
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")

	// All rows (header, separator, 2 data) should exist.
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines, got %d:\n%s", len(lines), buf.String())
	}

	// All pipe-delimited lines should have the same length (aligned columns).
	headerLen := len(lines[0])
	for i, line := range lines {
		if len(line) != headerLen {
			t.Errorf("line %d length %d != header length %d\nline: %q", i, len(line), headerLen, line)
		}
	}
}

func TestFormatTable_Empty(t *testing.T) {
	var buf bytes.Buffer
	FormatTable(&buf, nil, nil)
	if buf.Len() != 0 {
		t.Errorf("expected empty output for nil headers, got %q", buf.String())
	}
}

func TestFormatTable_NoRows(t *testing.T) {
	var buf bytes.Buffer
	FormatTable(&buf, []string{"A", "B"}, nil)
	output := buf.String()

	// Should have header + separator = 2 lines.
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines (header + separator), got %d", len(lines))
	}
}

func TestFormatTable_MismatchedColumns(t *testing.T) {
	var buf bytes.Buffer
	headers := []string{"A", "B", "C"}
	rows := [][]string{
		{"1"}, // fewer cells than headers
	}

	// Should not panic.
	FormatTable(&buf, headers, rows)
	if buf.Len() == 0 {
		t.Error("expected output even with mismatched column count")
	}
}

func TestColorHelpers_Enabled(t *testing.T) {
	origColor := UseColor
	UseColor = true
	defer func() { UseColor = origColor }()

	if got := Green("ok"); got != "\033[32mok\033[0m" {
		t.Errorf("Green = %q, want ANSI green", got)
	}
	if got := Red("err"); got != "\033[31merr\033[0m" {
		t.Errorf("Red = %q, want ANSI red", got)
	}
	if got := Yellow("warn"); got != "\033[33mwarn\033[0m" {
		t.Errorf("Yellow = %q, want ANSI yellow", got)
	}
}

func TestColorHelpers_Disabled(t *testing.T) {
	origColor := UseColor
	UseColor = false
	defer func() { UseColor = origColor }()

	if got := Green("ok"); got != "ok" {
		t.Errorf("Green(disabled) = %q, want plain text", got)
	}
	if got := Red("err"); got != "err" {
		t.Errorf("Red(disabled) = %q, want plain text", got)
	}
	if got := Yellow("warn"); got != "warn" {
		t.Errorf("Yellow(disabled) = %q, want plain text", got)
	}
}

package models

import (
	"bytes"
	"strings"
	"testing"
)

func TestBold(t *testing.T) {
	UseColor = false
	if got := Bold("test"); got != "test" {
		t.Errorf("Bold with color off = %q, want 'test'", got)
	}
	UseColor = true
	if got := Bold("test"); !strings.Contains(got, "test") {
		t.Errorf("Bold with color on should contain 'test', got %q", got)
	}
	if got := Bold("test"); !strings.Contains(got, "\033[1m") {
		t.Errorf("Bold with color on should contain ANSI bold, got %q", got)
	}
}

func TestDim(t *testing.T) {
	UseColor = false
	if got := Dim("test"); got != "test" {
		t.Errorf("Dim with color off = %q", got)
	}
	UseColor = true
	if got := Dim("test"); !strings.Contains(got, "\033[2m") {
		t.Errorf("Dim with color on should contain ANSI dim, got %q", got)
	}
}

func TestCyan(t *testing.T) {
	UseColor = false
	if got := Cyan("test"); got != "test" {
		t.Errorf("Cyan with color off = %q", got)
	}
	UseColor = true
	if got := Cyan("test"); !strings.Contains(got, "\033[36m") {
		t.Errorf("Cyan with color on should contain ANSI cyan, got %q", got)
	}
}

func TestBanner(t *testing.T) {
	var buf bytes.Buffer
	Banner(&buf, "✈️", "Flights · round_trip", "Found 73 flights")
	got := buf.String()
	if !strings.Contains(got, "╭") {
		t.Error("Banner should contain top-left corner")
	}
	if !strings.Contains(got, "╰") {
		t.Error("Banner should contain bottom-left corner")
	}
	if !strings.Contains(got, "Flights") {
		t.Error("Banner should contain title")
	}
	if !strings.Contains(got, "73 flights") {
		t.Error("Banner should contain subtitle")
	}
}

func TestBanner_NoSubtitle(t *testing.T) {
	var buf bytes.Buffer
	Banner(&buf, "🔍", "Search")
	got := buf.String()
	if !strings.Contains(got, "Search") {
		t.Error("Banner should contain title")
	}
}

func TestBanner_MultipleSubtitles(t *testing.T) {
	var buf bytes.Buffer
	Banner(&buf, "🔥", "Deals", "Line 1", "Line 2", "")
	got := buf.String()
	if !strings.Contains(got, "Line 1") {
		t.Error("Banner should contain first subtitle")
	}
	if !strings.Contains(got, "Line 2") {
		t.Error("Banner should contain second subtitle")
	}
}

func TestSummary(t *testing.T) {
	var buf bytes.Buffer
	UseColor = false
	Summary(&buf, "Showing 10 of 50 results")
	got := buf.String()
	if !strings.Contains(got, "Showing 10 of 50 results") {
		t.Errorf("Summary should contain text, got %q", got)
	}
}

func TestBookingHint(t *testing.T) {
	var buf bytes.Buffer
	UseColor = false
	BookingHint(&buf)
	got := buf.String()
	if !strings.Contains(got, "booking") {
		t.Errorf("BookingHint should mention booking, got %q", got)
	}
}

func TestDisplayWidth_ASCII(t *testing.T) {
	if got := displayWidth("hello"); got != 5 {
		t.Errorf("displayWidth('hello') = %d, want 5", got)
	}
}

func TestDisplayWidth_Emoji(t *testing.T) {
	// Emoji should be 2 cells wide.
	w := displayWidth("✈️")
	if w < 2 {
		t.Errorf("displayWidth(airplane emoji) = %d, want >= 2", w)
	}
}

func TestDisplayWidth_CJK(t *testing.T) {
	// CJK characters should be 2 cells wide.
	w := displayWidth("中文")
	if w != 4 {
		t.Errorf("displayWidth('中文') = %d, want 4", w)
	}
}

func TestDisplayWidth_Empty(t *testing.T) {
	if got := displayWidth(""); got != 0 {
		t.Errorf("displayWidth('') = %d, want 0", got)
	}
}

func TestStripANSI(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "no ansi", input: "hello", want: "hello"},
		{name: "green", input: "\033[32mhello\033[0m", want: "hello"},
		{name: "bold", input: "\033[1mtest\033[0m", want: "test"},
		{name: "multiple", input: "\033[31mred\033[0m and \033[32mgreen\033[0m", want: "red and green"},
		{name: "empty", input: "", want: ""},
		{name: "partial escape", input: "\033[", want: ""},
		{name: "mixed", input: "pre\033[36mcyan\033[0mpost", want: "precyanpost"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripANSI(tt.input)
			if got != tt.want {
				t.Errorf("stripANSI(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

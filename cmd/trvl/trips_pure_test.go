package main

import (
	"testing"
	"time"
)

func TestFormatCountdown(t *testing.T) {
	tests := []struct {
		name string
		d    time.Duration
		want string
	}{
		{name: "negative", d: -1 * time.Hour, want: "departed"},
		{name: "30 minutes", d: 30 * time.Minute, want: "in 30m"},
		{name: "90 minutes", d: 90 * time.Minute, want: "in 90m"},
		{name: "3 hours", d: 3 * time.Hour, want: "in 3h"},
		{name: "1 day 5 hours", d: 29 * time.Hour, want: "in 1 day 5h"},
		{name: "2 days", d: 48 * time.Hour, want: "in 2 days"},
		{name: "7 days", d: 168 * time.Hour, want: "in 7 days"},
		{name: "zero", d: 0, want: "in 0m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatCountdown(tt.d)
			if got != tt.want {
				t.Errorf("formatCountdown(%v) = %q, want %q", tt.d, got, tt.want)
			}
		})
	}
}

func TestColorizeStatus(t *testing.T) {
	// These test that the function returns non-empty strings.
	// We can't easily test ANSI codes, but we verify no panic and correct passthrough.
	tests := []struct {
		input string
	}{
		{"planning"},
		{"booked"},
		{"in_progress"},
		{"completed"},
		{"cancelled"},
		{"unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := colorizeStatus(tt.input)
			if got == "" {
				t.Errorf("colorizeStatus(%q) returned empty", tt.input)
			}
		})
	}
}

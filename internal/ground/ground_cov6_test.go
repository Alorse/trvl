package ground

import (
	"testing"
)

// ---------------------------------------------------------------------------
// eurostarRouteDuration — covers the switch in eurostar.go:469
// ---------------------------------------------------------------------------

func TestEurostarRouteDuration(t *testing.T) {
	tests := []struct {
		from, to string
		want     int
	}{
		{"London", "Paris", 135},
		{"Paris", "London", 135},
		{"London", "Brussels", 120},
		{"Brussels", "London", 120},
		{"London", "Amsterdam", 195},
		{"Amsterdam", "London", 195},
		{"London", "Rotterdam", 180},
		{"Rotterdam", "London", 180},
		{"London", "Cologne", 240},
		{"Cologne", "London", 240},
		{"Paris", "Amsterdam", 135}, // default
		{"Unknown", "Unknown", 135}, // default
	}

	for _, tt := range tests {
		got := eurostarRouteDuration(tt.from, tt.to)
		if got != tt.want {
			t.Errorf("eurostarRouteDuration(%q, %q) = %d, want %d", tt.from, tt.to, got, tt.want)
		}
	}
}

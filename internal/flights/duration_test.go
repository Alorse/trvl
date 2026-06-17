package flights

import "testing"

func TestParseISO8601Duration(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"PT2H10M", 130},
		{"PT12H35M", 755},
		{"P1DT4H55M", 24*60 + 4*60 + 55}, // 1775
		{"PT45M", 45},
		{"PT3H", 180},
		{"P2D", 2 * 24 * 60},
		{"PT0S", 0},
		{"", 0},
		{"garbage", 0},
	}
	for _, c := range cases {
		if got := parseISO8601Duration(c.in); got != c.want {
			t.Errorf("parseISO8601Duration(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

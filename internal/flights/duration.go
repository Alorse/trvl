package flights

import (
	"strconv"
	"strings"
)

// parseISO8601Duration converts an ISO-8601 duration (e.g. "P1DT4H55M",
// "PT12H35M") into total minutes. Seconds are floored away. Malformed or empty
// input returns 0. Duffel reports flight/segment durations in this format,
// whereas models.FlightResult uses integer minutes.
func parseISO8601Duration(s string) int {
	if !strings.HasPrefix(s, "P") {
		return 0
	}
	s = s[1:] // drop leading 'P'

	datePart, timePart := s, ""
	if i := strings.Index(s, "T"); i >= 0 {
		datePart, timePart = s[:i], s[i+1:]
	}

	total := 0
	// Date part: only days are meaningful for flight durations.
	if strings.HasSuffix(datePart, "D") {
		if d, err := strconv.Atoi(strings.TrimSuffix(datePart, "D")); err == nil {
			total += d * 24 * 60
		}
	}

	// Time part: accumulate digits, flush on H/M/S designator.
	num := ""
	for _, r := range timePart {
		if r >= '0' && r <= '9' {
			num += string(r)
			continue
		}
		n, _ := strconv.Atoi(num)
		num = ""
		switch r {
		case 'H':
			total += n * 60
		case 'M':
			total += n
		case 'S':
			// floored away at minute granularity
		}
	}
	return total
}

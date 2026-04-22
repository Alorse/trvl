package mcp

import (
	"fmt"
	"strings"

	"github.com/MikkoParkkola/trvl/internal/models"
)

// validateOriginDest extracts and validates origin/destination from tool
// arguments. Accepts either IATA codes ("HEL") or city names ("Helsinki").
// City names are resolved to their primary airport code.
func validateOriginDest(args map[string]any) (origin, dest string, err error) {
	origin = strings.TrimSpace(argString(args, "origin"))
	dest = strings.TrimSpace(argString(args, "destination"))
	if origin == "" || dest == "" {
		return "", "", fmt.Errorf("origin and destination are required")
	}

	origin = resolveMCPLocation(origin)
	dest = resolveMCPLocation(dest)

	if err := models.ValidateIATA(origin); err != nil {
		return "", "", fmt.Errorf("invalid origin %q: %w", origin, err)
	}
	if err := models.ValidateIATA(dest); err != nil {
		return "", "", fmt.Errorf("invalid destination %q: %w", dest, err)
	}
	return origin, dest, nil
}

// validateDate extracts and validates a date argument (YYYY-MM-DD).
func validateDate(args map[string]any, key string) (string, error) {
	d := argString(args, key)
	if d == "" {
		return "", fmt.Errorf("%s is required", key)
	}
	if err := models.ValidateDate(d); err != nil {
		return "", err
	}
	return d, nil
}

// resolveMCPLocation converts a city name to an IATA code for MCP tool use.
// If already an IATA code, returns it uppercased. If a city name resolves to
// airports, returns the first airport alphabetically — note this may not be
// the most-trafficked airport for a given city (e.g. London → LGW, not LHR).
// Unknown inputs are uppercased and passed through for ValidateIATA to reject.
func resolveMCPLocation(s string) string {
	upper := strings.ToUpper(s)
	if models.IsIATACode(upper) {
		return upper
	}
	airports := models.ResolveCityToAirports(s)
	if len(airports) > 0 {
		return airports[0]
	}
	return upper
}

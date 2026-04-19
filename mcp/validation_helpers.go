package mcp

import (
	"fmt"
	"strings"

	"github.com/MikkoParkkola/trvl/internal/models"
)

// validateOriginDest extracts and validates origin/destination IATA codes from
// tool arguments. Both are upper-cased automatically.
func validateOriginDest(args map[string]any) (origin, dest string, err error) {
	origin = strings.ToUpper(argString(args, "origin"))
	dest = strings.ToUpper(argString(args, "destination"))
	if origin == "" || dest == "" {
		return "", "", fmt.Errorf("origin and destination are required")
	}
	if err := models.ValidateIATA(origin); err != nil {
		return "", "", fmt.Errorf("invalid origin: %w", err)
	}
	if err := models.ValidateIATA(dest); err != nil {
		return "", "", fmt.Errorf("invalid destination: %w", err)
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

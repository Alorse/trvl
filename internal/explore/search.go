// Package explore implements Google Flights explore destination search
// via the GetExploreDestinations endpoint.
package explore

import (
	"context"
	"fmt"

	"github.com/MikkoParkkola/trvl/internal/batchexec"
	"github.com/MikkoParkkola/trvl/internal/models"
)

// SearchExplore searches for cheapest flight destinations from the given origin.
//
// Unlike a normal flight search which requires a destination, explore searches
// return a list of destinations with their cheapest prices, optionally filtered
// by geographic coordinates.
func SearchExplore(ctx context.Context, client *batchexec.Client, origin string, opts ExploreOptions) (*models.ExploreResult, error) {
	if origin == "" {
		return nil, fmt.Errorf("origin airport is required")
	}

	encoded := EncodeExplorePayload(origin, opts)

	status, body, err := client.PostExplore(ctx, encoded)
	if err != nil {
		return nil, fmt.Errorf("explore request: %w", err)
	}

	if status == 403 {
		return nil, batchexec.ErrBlocked
	}
	if status != 200 {
		return nil, fmt.Errorf("unexpected status %d", status)
	}

	destinations, err := ParseExploreResponse(body)
	if err != nil {
		return nil, fmt.Errorf("parse explore response: %w", err)
	}

	return &models.ExploreResult{
		Success:      true,
		Count:        len(destinations),
		Destinations: destinations,
	}, nil
}

// ExploreOptions configures an explore destination search.
type ExploreOptions struct {
	DepartureDate string  // YYYY-MM-DD (required)
	ReturnDate    string  // YYYY-MM-DD (empty = one-way)
	Adults        int     // Number of adult passengers (default: 1)
	NorthLat      float64 // Geographic bounding box (all zero = worldwide)
	SouthLat      float64
	EastLng       float64
	WestLng       float64
}

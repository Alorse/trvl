package hotels

import (
	"context"

	"github.com/MikkoParkkola/trvl/internal/models"
)

// DefaultProvider wraps the package-level SearchHotels function, implementing
// models.HotelSearcher. It uses the shared default client for connection reuse.
type DefaultProvider struct{}

// SearchHotels delegates to the package-level SearchHotels, converting
// models.HotelSearchOptions to the package's HotelSearchOptions.
func (p *DefaultProvider) SearchHotels(ctx context.Context, location string, opts models.HotelSearchOptions) (*models.HotelSearchResult, error) {
	return SearchHotels(ctx, location, HotelSearchOptions{
		CheckIn:         opts.CheckIn,
		CheckOut:        opts.CheckOut,
		Guests:          opts.Guests,
		Stars:           opts.Stars,
		Sort:            opts.Sort,
		Currency:        opts.Currency,
		MinPrice:        opts.MinPrice,
		MaxPrice:        opts.MaxPrice,
		MinRating:       opts.MinRating,
		MaxDistanceKm:   opts.MaxDistanceKm,
		Amenities:       opts.Amenities,
		CenterLat:       opts.CenterLat,
		CenterLon:       opts.CenterLon,
		EnrichAmenities: opts.EnrichAmenities,
		EnrichLimit:     opts.EnrichLimit,
	})
}

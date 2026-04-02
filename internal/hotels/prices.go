package hotels

import (
	"context"
	"fmt"

	"github.com/MikkoParkkola/trvl/internal/batchexec"
	"github.com/MikkoParkkola/trvl/internal/models"
)

// GetHotelPrices looks up booking provider prices for a specific hotel.
//
// The hotelID should be a Google place ID (e.g. "/g/11b6d4_v_4" or
// "ChIJ..."). These IDs are returned in hotel search results.
//
// Dates should be in YYYY-MM-DD format.
func GetHotelPrices(ctx context.Context, hotelID string, checkIn, checkOut string, currency string) (*models.HotelPriceResult, error) {
	if hotelID == "" {
		return nil, fmt.Errorf("hotel ID is required")
	}
	if checkIn == "" || checkOut == "" {
		return nil, fmt.Errorf("check-in and check-out dates are required")
	}
	if currency == "" {
		currency = "USD"
	}

	checkInArr, err := parseDateArray(checkIn)
	if err != nil {
		return nil, fmt.Errorf("parse check-in date: %w", err)
	}
	checkOutArr, err := parseDateArray(checkOut)
	if err != nil {
		return nil, fmt.Errorf("parse check-out date: %w", err)
	}

	client := batchexec.NewClient()
	encoded := batchexec.BuildHotelPricePayload(hotelID, checkInArr, checkOutArr, currency)

	status, body, err := client.BatchExecute(ctx, encoded)
	if err != nil {
		return nil, fmt.Errorf("hotel price request: %w", err)
	}

	if status == 403 {
		return nil, batchexec.ErrBlocked
	}
	if status != 200 {
		return nil, fmt.Errorf("hotel price lookup returned status %d", status)
	}
	if len(body) < 50 {
		return nil, fmt.Errorf("hotel price lookup returned empty response")
	}

	entries, err := batchexec.DecodeBatchResponse(body)
	if err != nil {
		return nil, fmt.Errorf("decode hotel price response: %w", err)
	}

	providers, err := ParseHotelPriceResponse(entries)
	if err != nil {
		return nil, fmt.Errorf("parse hotel prices: %w", err)
	}

	// Set currency on providers that don't have one.
	for i := range providers {
		if providers[i].Currency == "" {
			providers[i].Currency = currency
		}
	}

	return &models.HotelPriceResult{
		Success:   true,
		HotelID:   hotelID,
		CheckIn:   checkIn,
		CheckOut:  checkOut,
		Providers: providers,
	}, nil
}

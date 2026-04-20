package destinations

import (
	"context"
	"sync"
	"time"

	"github.com/MikkoParkkola/trvl/internal/models"
)

// GetDestinationInfo fetches travel intelligence for a location by querying
// 5 free APIs in parallel: weather, country, holidays, safety, and currency.
//
// Graceful degradation: if any single source fails, the others still return
// data. Only the geocoding step is fatal (we need lat/lon and country code).
func GetDestinationInfo(ctx context.Context, location string, travelDates models.DateRange) (*models.DestinationInfo, error) {
	// Step 1: Geocode (required -- everything else depends on this).
	geo, err := Geocode(ctx, location)
	if err != nil {
		return nil, err
	}

	info := &models.DestinationInfo{
		Location: geo.DisplayName,
	}

	// Determine travel year for holiday lookup.
	year := time.Now().Year()
	if travelDates.CheckIn != "" {
		if t, err := models.ParseDate(travelDates.CheckIn); err == nil {
			year = t.Year()
		}
	}

	// Determine the primary currency code (populated after country fetch).
	var primaryCurrency string

	// Step 2: Fetch all 5 sources in parallel.
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Weather
	wg.Add(1)
	go func() {
		defer wg.Done()
		weather, tz, fetchErr := FetchWeather(ctx, geo.Lat, geo.Lon)
		if fetchErr != nil {
			return
		}
		mu.Lock()
		info.Weather = weather
		if tz != "" {
			info.Timezone = tz
		}
		mu.Unlock()
	}()

	// Country
	wg.Add(1)
	go func() {
		defer wg.Done()
		country, fetchErr := FetchCountry(ctx, geo.CountryCode)
		if fetchErr != nil {
			return
		}
		mu.Lock()
		info.Country = country
		if len(country.Currencies) > 0 {
			primaryCurrency = country.Currencies[0]
		}
		mu.Unlock()
	}()

	// Holidays
	wg.Add(1)
	go func() {
		defer wg.Done()
		holidays, fetchErr := FetchHolidays(ctx, geo.CountryCode, year, travelDates.CheckIn, travelDates.CheckOut)
		if fetchErr != nil {
			return
		}
		mu.Lock()
		info.Holidays = holidays
		mu.Unlock()
	}()

	// Safety
	wg.Add(1)
	go func() {
		defer wg.Done()
		safety, fetchErr := FetchSafety(ctx, geo.CountryCode)
		if fetchErr != nil {
			return
		}
		mu.Lock()
		info.Safety = safety
		mu.Unlock()
	}()

	wg.Wait()

	// Step 3: Fetch currency (needs primary currency from country info).
	// Done after the parallel batch since it depends on country data.
	if primaryCurrency != "" {
		currency, fetchErr := FetchCurrency(ctx, primaryCurrency)
		if fetchErr == nil {
			info.Currency = currency
		}
	}

	return info, nil
}

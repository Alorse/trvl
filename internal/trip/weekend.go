package trip

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/MikkoParkkola/trvl/internal/explore"
	"github.com/MikkoParkkola/trvl/internal/flights"
	"github.com/MikkoParkkola/trvl/internal/hotels"
	"github.com/MikkoParkkola/trvl/internal/models"
	"github.com/MikkoParkkola/trvl/internal/preferences"
)

// WeekendOptions configures a weekend getaway search.
type WeekendOptions struct {
	Month     string  // Month to search in, e.g. "july-2026" or "2026-07"
	MaxBudget float64 // Maximum total budget (flight + hotel)
	Nights    int     // Number of nights (default: 2)
}

// WeekendDestination is a single destination in the weekend getaway results.
type WeekendDestination struct {
	Destination string  `json:"destination"`
	AirportCode string  `json:"airport_code"`
	FlightPrice float64 `json:"flight_price"`
	HotelPrice  float64 `json:"hotel_price"`
	HotelName   string  `json:"hotel_name,omitempty"`
	Total       float64 `json:"total"`
	Currency    string  `json:"currency"`
	Stops       int     `json:"stops"`
	AirlineName string  `json:"airline_name,omitempty"`
}

// WeekendResult is the top-level response for a weekend getaway search.
type WeekendResult struct {
	Success      bool                 `json:"success"`
	Origin       string               `json:"origin"`
	Month        string               `json:"month"`
	Nights       int                  `json:"nights"`
	Count        int                  `json:"count"`
	Destinations []WeekendDestination `json:"destinations"`
	Error        string               `json:"error,omitempty"`
}

// defaults fills in zero-value fields.
func (o *WeekendOptions) defaults() {
	if o.Nights <= 0 {
		o.Nights = 2
	}
}

// parseMonth parses month strings like "july-2026", "2026-07", "jul-2026".
// Returns the first Friday of the month as departure date and the following
// Sunday as return date.
func parseMonth(month string) (departDate, returnDate string, displayMonth string, err error) {
	formats := []string{
		"January-2006", "january-2006",
		"Jan-2006", "jan-2006",
		"2006-01",
	}

	var t time.Time
	for _, f := range formats {
		t, err = time.Parse(f, month)
		if err == nil {
			break
		}
	}
	if err != nil {
		return "", "", "", fmt.Errorf("invalid month %q: use format like july-2026 or 2026-07", month)
	}

	displayMonth = t.Format("January 2006")

	// Find first Friday of the month.
	d := t
	for d.Weekday() != time.Friday {
		d = d.AddDate(0, 0, 1)
	}
	departDate = d.Format("2006-01-02")
	returnDate = d.AddDate(0, 0, 2).Format("2006-01-02") // Sunday

	return departDate, returnDate, displayMonth, nil
}

// FindWeekendGetaways searches for cheap weekend getaway destinations from an origin.
//
// It uses the explore API to find the cheapest destinations, then searches
// real hotel prices for each with user preferences applied. Results are
// ranked by total cost (flight round-trip + cheapest qualifying hotel).
func FindWeekendGetaways(ctx context.Context, origin string, opts WeekendOptions) (*WeekendResult, error) {
	opts.defaults()

	if origin == "" {
		return nil, fmt.Errorf("origin airport is required")
	}

	departDate, returnDate, displayMonth, err := parseMonth(opts.Month)
	if err != nil {
		return nil, err
	}

	client := flights.DefaultClient()

	exploreOpts := explore.ExploreOptions{
		DepartureDate: departDate,
		ReturnDate:    returnDate,
		Adults:        1,
	}

	exploreResult, err := explore.SearchExplore(ctx, client, origin, exploreOpts)
	if err != nil {
		return nil, fmt.Errorf("explore destinations: %w", err)
	}

	// Detect the actual API currency.
	apiCurrency := ""
	if len(exploreResult.Destinations) > 0 {
		apiCurrency = flights.DetectSourceCurrency(ctx, origin, exploreResult.Destinations[0].AirportCode)
	}

	// Sort by flight price, take top 10 for hotel search.
	dests := exploreResult.Destinations
	sort.Slice(dests, func(i, j int) bool {
		return dests[i].Price < dests[j].Price
	})
	if len(dests) > 10 {
		dests = dests[:10]
	}

	// Load user preferences for hotel filtering.
	prefs, _ := preferences.Load()

	// Search real hotel prices with bounded concurrency.
	type hotelResult struct {
		perNight float64
		total    float64
		name     string
	}
	var mu sync.Mutex
	hotelPrices := make(map[int]*hotelResult)

	sem := make(chan struct{}, 5)
	var wg sync.WaitGroup

	for i, d := range dests {
		wg.Add(1)
		go func(idx int, dest models.ExploreDestination) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			cityName := dest.CityName
			if cityName == "" {
				cityName = models.LookupAirportName(dest.AirportCode)
			}

			hotelOpts := hotels.HotelSearchOptions{
				CheckIn:  departDate,
				CheckOut: returnDate,
				Guests:   1,
				Sort:     "cheapest",
			}
			if prefs != nil {
				if prefs.MinHotelStars > 0 {
					hotelOpts.Stars = prefs.MinHotelStars
				}
				if prefs.MinHotelRating > 0 {
					hotelOpts.MinRating = prefs.MinHotelRating
				}
				if prefs.BudgetPerNightMax > 0 {
					hotelOpts.MaxPrice = prefs.BudgetPerNightMax
				}
			}

			hr, searchErr := hotels.SearchHotels(ctx, cityName, hotelOpts)
			if searchErr != nil || hr == nil || !hr.Success || len(hr.Hotels) == 0 {
				return
			}

			filtered := hr.Hotels
			if prefs != nil {
				filtered = preferences.FilterHotels(filtered, cityName, prefs)
			}
			if len(filtered) == 0 {
				return
			}

			cheapest := filtered[0]
			for _, h := range filtered[1:] {
				if h.Price > 0 && h.Price < cheapest.Price {
					cheapest = h
				}
			}
			if cheapest.Price <= 0 {
				return
			}

			mu.Lock()
			hotelPrices[idx] = &hotelResult{
				perNight: cheapest.Price,
				total:    cheapest.Price * float64(opts.Nights),
				name:     cheapest.Name,
			}
			mu.Unlock()
		}(i, d)
	}
	wg.Wait()

	// Build results with real hotel prices.
	var results []WeekendDestination
	for i, d := range dests {
		hp := hotelPrices[i]
		if hp == nil {
			continue // skip destinations where hotel search failed
		}

		total := d.Price + hp.total
		if opts.MaxBudget > 0 && total > opts.MaxBudget {
			continue
		}

		cityName := d.CityName
		if cityName == "" {
			cityName = models.LookupAirportName(d.AirportCode)
		}

		results = append(results, WeekendDestination{
			Destination: cityName,
			AirportCode: d.AirportCode,
			FlightPrice: d.Price,
			HotelPrice:  hp.total,
			HotelName:   hp.name,
			Total:       total,
			Currency:    apiCurrency,
			Stops:       d.Stops,
			AirlineName: d.AirlineName,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Total < results[j].Total
	})

	return &WeekendResult{
		Success:      true,
		Origin:       origin,
		Month:        displayMonth,
		Nights:       opts.Nights,
		Count:        len(results),
		Destinations: results,
	}, nil
}

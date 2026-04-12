package hotels

import (
	"context"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/MikkoParkkola/trvl/internal/batchexec"
	"github.com/MikkoParkkola/trvl/internal/models"
)

var (
	defaultClient     *batchexec.Client
	defaultClientOnce sync.Once
)

// DefaultClient returns a shared batchexec.Client for the hotels package.
// The client is created once and reused across all requests, enabling
// connection reuse and shared rate limiting.
func DefaultClient() *batchexec.Client {
	defaultClientOnce.Do(func() {
		defaultClient = batchexec.NewClient()
		// Hotel searches make many sequential page requests across multiple
		// sort orders. Google Travel rate-limits at ~2 req/s; the default
		// 10 req/s triggers persistent 429 blocks.
		defaultClient.SetRateLimit(2)
	})
	return defaultClient
}

// HotelSearchOptions configures a hotel search.
type HotelSearchOptions struct {
	CheckIn  string // YYYY-MM-DD
	CheckOut string // YYYY-MM-DD
	Guests   int
	Stars    int    // 0 = any, 2-5 filter
	Sort     string // "cheapest", "rating", "distance", "stars"
	Currency string // default "USD"

	// Post-fetch filters.
	MinPrice      float64  // minimum price per night (0 = no filter)
	MaxPrice      float64  // maximum price per night (0 = no filter)
	MinRating     float64  // minimum guest rating, e.g. 4.0 (0 = no filter)
	MaxDistanceKm float64  // max km from city center (0 = no filter)
	Amenities     []string // required amenities, all must match (nil = no filter)
	CenterLat     float64  // city center latitude (resolved automatically if 0)
	CenterLon     float64  // city center longitude (resolved automatically if 0)

	// Enrichment options.
	EnrichAmenities bool // fetch detail pages for top hotels to get full amenity lists
	EnrichLimit     int  // max hotels to enrich (default: 5, max: 10)

	// MaxPages overrides the default pagination depth (maxPages).
	// Compound commands (trip-cost, weekend, multi-city) set this to 1
	// because they only need the cheapest result, not 75 hotels.
	// 0 means use the default (maxPages).
	MaxPages int

	// FreeCancellation filters for hotels offering free cancellation when true.
	FreeCancellation bool

	// PropertyType restricts results to a specific property category.
	// Accepted values: "hotel", "apartment", "hostel", "resort", "bnb", "villa".
	// Empty string means no filter.
	PropertyType string

	// Brand filters results to hotels whose name contains the brand string
	// (case-insensitive). Applied as a client-side post-filter since Google
	// Hotels does not expose a server-side brand/chain parameter.
	// Examples: "hilton", "marriott", "ibis", "hyatt".
	Brand string

	// EcoCertified filters for hotels with sustainability certifications
	// (Google's "Eco-certified" badge). Applied server-side via the &ecof=1
	// URL parameter. When true, all returned hotels are marked EcoCertified.
	EcoCertified bool
}

// SearchHotels searches for hotels in the given location.
//
// The location can be a city name, address, or any text that Google Travel
// accepts as a destination query. We fetch the Google Travel Hotels page
// directly and parse the embedded JSON data from AF_initDataCallback blocks.
func SearchHotels(ctx context.Context, location string, opts HotelSearchOptions) (*models.HotelSearchResult, error) {
	return SearchHotelsWithClient(ctx, DefaultClient(), location, opts)
}

// hotelCityAliases maps common English city names to the form that Google
// Hotels actually resolves correctly. Without this, "Prague" returns zero
// results while "Praha" works fine.
var hotelCityAliases = map[string]string{
	"prague":     "Praha",
	"munich":     "München",
	"vienna":     "Wien",
	"cologne":    "Köln",
	"copenhagen": "København",
	"warsaw":     "Warszawa",
	"bucharest":  "București",
	"gothenburg": "Göteborg",
	"nuremberg":  "Nürnberg",
}

// normalizeHotelCity replaces known English city names with the form Google
// Hotels expects. Passthrough for unknown cities.
func normalizeHotelCity(location string) string {
	if mapped, ok := hotelCityAliases[strings.ToLower(strings.TrimSpace(location))]; ok {
		return mapped
	}
	return location
}

// maxPages is the maximum number of paginated requests per sort order.
// Each page returns ~20-26 hotels; 3 sort orders x 3 pages = up to ~180 unique.
// Kept at 3 per sort to limit total requests (9 max) and avoid 429 rate limits.
const maxPages = 3

// pageSize is the offset step between paginated requests. Google Travel
// Hotels returns ~20 results per page and uses a "start" query parameter
// for offset-based pagination.
const pageSize = 20

// googleSortOrders are the Google Hotels &sort= parameter values used to
// diversify results. The primary sort (empty string = Google's default
// relevance) is always fetched first. Additional sort orders pull in hotels
// that rank differently, significantly increasing unique coverage.
//
// Known values: 3=highest rated, 4=most reviewed, 8=price low-to-high.
var googleSortOrders = []string{"", "3", "8"}

// SearchHotelsWithClient is like SearchHotels but reuses the provided client.
func SearchHotelsWithClient(ctx context.Context, client *batchexec.Client, location string, opts HotelSearchOptions) (*models.HotelSearchResult, error) {
	location = normalizeHotelCity(location)
	if opts.CheckIn == "" || opts.CheckOut == "" {
		return nil, fmt.Errorf("check-in and check-out dates are required")
	}
	if opts.Guests <= 0 {
		opts.Guests = 2
	}
	if opts.Currency == "" {
		opts.Currency = "USD" // Google's default when no currency specified
	}

	// Validate dates.
	_, err := parseDateArray(opts.CheckIn)
	if err != nil {
		return nil, fmt.Errorf("parse check-in date: %w", err)
	}
	_, err = parseDateArray(opts.CheckOut)
	if err != nil {
		return nil, fmt.Errorf("parse check-out date: %w", err)
	}

	pageLimit := maxPages
	if opts.MaxPages > 0 && opts.MaxPages < maxPages {
		pageLimit = opts.MaxPages
	}

	// Determine which sort orders to use. When MaxPages is 1 (compound
	// commands that only need the cheapest result), skip sort diversity.
	sortOrders := googleSortOrders
	if pageLimit <= 1 {
		sortOrders = []string{""}
	}

	var totalAvailable int
	// Accumulate raw results per-page; MergeHotelResults deduplicates at the end.
	var rawBatches [][]models.HotelResult

	for sortIdx, googleSort := range sortOrders {
		// Brief cooldown between sort orders to avoid Google 429 rate limits.
		// Skip for the first sort order (no prior requests to cool down from).
		if sortIdx > 0 {
			select {
			case <-time.After(500 * time.Millisecond):
			case <-ctx.Done():
				break
			}
		}

		// Fetch first page for this sort order (with metadata on the primary sort).
		firstPage, err := fetchHotelPageFull(ctx, client, location, opts, 0, googleSort)
		if err != nil {
			if sortIdx == 0 {
				// Primary sort failed — fatal.
				return nil, err
			}
			// Secondary sort failed — non-fatal, keep what we have.
			break
		}

		if sortIdx == 0 {
			totalAvailable = firstPage.TotalAvailable
		}

		tagged := tagHotelSource(firstPage.Hotels, "google_hotels")
		rawBatches = append(rawBatches, tagged)

		// Paginate within this sort order.
		for page := 1; page < pageLimit; page++ {
			pageHotels, err := fetchHotelPage(ctx, client, location, opts, page*pageSize, googleSort)
			if err != nil {
				// Non-fatal: keep what we have from previous pages.
				break
			}
			if len(pageHotels) == 0 {
				// End of results for this sort order.
				break
			}
			rawBatches = append(rawBatches, tagHotelSource(pageHotels, "google_hotels"))
		}
	}

	// Deduplicate across all pages and sort orders using name-normalisation +
	// geo-proximity. MergeHotelResults preserves all provider price sources and
	// keeps the lowest price as the primary. This replaces the previous naive
	// strings.ToLower(name) seen-map approach.
	hotels := models.MergeHotelResults(rawBatches...)

	// Resolve city center for distance filter/sort if needed.
	if opts.MaxDistanceKm > 0 || strings.EqualFold(opts.Sort, "distance") {
		if opts.CenterLat == 0 && opts.CenterLon == 0 {
			lat, lon, err := ResolveLocation(ctx, location)
			if err == nil {
				opts.CenterLat = lat
				opts.CenterLon = lon
			}
		}
	}

	// Apply post-filters.
	hotels = filterHotels(hotels, opts)

	// Sort results.
	sortHotels(hotels, opts.Sort, opts.CenterLat, opts.CenterLon)

	// Enrich top hotels with full amenity data from detail pages.
	if opts.EnrichAmenities {
		hotels = enrichHotelAmenities(ctx, hotels, opts.EnrichLimit)
	}

	// When the eco-certified filter is active, all returned hotels have
	// Google's sustainability certification — mark them accordingly.
	if opts.EcoCertified {
		for i := range hotels {
			hotels[i].EcoCertified = true
		}
	}

	// Add booking URLs to each hotel.
	for i := range hotels {
		hotels[i].BookingURL = buildHotelBookingURL(location, opts.CheckIn, opts.CheckOut)
	}

	return &models.HotelSearchResult{
		Success:        true,
		Count:          len(hotels),
		TotalAvailable: totalAvailable,
		Hotels:         hotels,
	}, nil
}

// fetchHotelPage fetches a single page of hotel results at the given offset.
// offset=0 is the first page, offset=20 is the second, etc.
// googleSort is the Google Hotels &sort= parameter value ("" for default).
func fetchHotelPage(ctx context.Context, client *batchexec.Client, location string, opts HotelSearchOptions, offset int, googleSort string) ([]models.HotelResult, error) {
	pr, err := fetchHotelPageFull(ctx, client, location, opts, offset, googleSort)
	if err != nil {
		return nil, err
	}
	return pr.Hotels, nil
}

// fetchHotelPageFull fetches a single page and returns the full parseResult
// including metadata like total available count.
// googleSort is the Google Hotels &sort= parameter value ("" for default).
func fetchHotelPageFull(ctx context.Context, client *batchexec.Client, location string, opts HotelSearchOptions, offset int, googleSort string) (parseResult, error) {
	travelURL := buildTravelURL(location, opts)
	if googleSort != "" {
		travelURL += "&sort=" + googleSort
	}
	if offset > 0 {
		travelURL += fmt.Sprintf("&start=%d", offset)
	}

	status, body, err := client.Get(ctx, travelURL)
	if err != nil {
		return parseResult{}, fmt.Errorf("hotel search request: %w", err)
	}

	if status == 403 {
		return parseResult{}, batchexec.ErrBlocked
	}
	if status != 200 {
		return parseResult{}, fmt.Errorf("hotel search returned status %d", status)
	}
	if len(body) < 1000 {
		return parseResult{}, fmt.Errorf("hotel search returned empty response")
	}

	pr := parseHotelsFromPageFull(string(body), opts.Currency)
	if len(pr.Hotels) == 0 {
		return parseResult{}, fmt.Errorf("parse hotel results: no hotels found in response payload")
	}

	return pr, nil
}

// buildHotelBookingURL constructs a Google Hotels deep link for a location and dates.
func buildHotelBookingURL(location, checkIn, checkOut string) string {
	encoded := url.PathEscape(location)
	return fmt.Sprintf("https://www.google.com/travel/hotels/%s?q=%s+hotels&dates=%s,%s",
		encoded, url.QueryEscape(location), checkIn, checkOut)
}

// buildTravelURL constructs the Google Travel Hotels search URL.
//
// Format: https://www.google.com/travel/hotels/{location}?q={location}&dates={checkin},{checkout}&adults={n}&hl=en-US&currency={cur}
func buildTravelURL(location string, opts HotelSearchOptions) string {
	encoded := url.PathEscape(location)
	query := url.Values{}
	query.Set("q", location)
	query.Set("dates", opts.CheckIn+","+opts.CheckOut)
	query.Set("adults", fmt.Sprintf("%d", opts.Guests))
	query.Set("hl", "en")
	query.Set("currency", opts.Currency)

	// Server-side filters — let Google do the heavy lifting.
	// Client-side filterHotels() remains as a safety net.
	if opts.MinPrice > 0 {
		query.Set("min_price", fmt.Sprintf("%.0f", opts.MinPrice))
	}
	if opts.MaxPrice > 0 {
		query.Set("max_price", fmt.Sprintf("%.0f", opts.MaxPrice))
	}
	if opts.Stars > 0 {
		query.Set("class", fmt.Sprintf("%d", opts.Stars))
	}
	if opts.MinRating > 0 {
		// Google uses rating=8 for 4.0+, rating=6 for 3.0+, etc.
		// The scale is rating * 2.
		query.Set("rating", fmt.Sprintf("%.0f", opts.MinRating*2))
	}
	if opts.MaxDistanceKm > 0 {
		// Google uses meters for the lrad (location radius) parameter.
		query.Set("lrad", fmt.Sprintf("%.0f", opts.MaxDistanceKm*1000))
	}
	if opts.FreeCancellation {
		query.Set("fc", "1")
	}
	if ptype := propertyTypeCode(opts.PropertyType); ptype != "" {
		query.Set("ptype", ptype)
	}
	if opts.EcoCertified {
		query.Set("ecof", "1")
	}

	return fmt.Sprintf("https://www.google.com/travel/hotels/%s?%s", encoded, query.Encode())
}

// filterHotels applies all post-fetch filters to hotel results.
func filterHotels(hotels []models.HotelResult, opts HotelSearchOptions) []models.HotelResult {
	filtered := hotels[:0]
	for _, h := range hotels {
		// Stars filter: h.Stars==0 means Google didn't annotate this hotel
		// with star data (~92% of hotels). Pass those through rather than
		// treating "unknown" as "zero stars".
		if opts.Stars > 0 && h.Stars > 0 && h.Stars < opts.Stars {
			continue
		}
		if opts.MinPrice > 0 && h.Price > 0 && h.Price < opts.MinPrice {
			continue
		}
		if opts.MaxPrice > 0 && h.Price > 0 && h.Price > opts.MaxPrice {
			continue
		}
		// Rating filter: when MinRating is set, require rating data AND that
		// it meets the minimum. Unrated properties (h.Rating == 0) are
		// suspicious — usually new listings, private rooms, or apartments
		// without enough reviews to establish quality.
		if opts.MinRating > 0 && h.Rating < opts.MinRating {
			continue
		}
		if opts.MaxDistanceKm > 0 && h.Lat != 0 && h.Lon != 0 && opts.CenterLat != 0 {
			dist := Haversine(opts.CenterLat, opts.CenterLon, h.Lat, h.Lon)
			if dist > opts.MaxDistanceKm {
				continue
			}
		}
		if len(opts.Amenities) > 0 && !hasAllAmenities(h.Amenities, opts.Amenities) {
			continue
		}
		if opts.Brand != "" && !strings.Contains(strings.ToLower(h.Name), strings.ToLower(opts.Brand)) {
			continue
		}
		filtered = append(filtered, h)
	}
	return filtered
}

// filterByStars removes hotels below the requested star rating.
// Hotels with Stars==0 (no star data from Google) are kept, since "unknown"
// should not be treated as "zero stars".
func filterByStars(hotels []models.HotelResult, minStars int) []models.HotelResult {
	return filterHotels(hotels, HotelSearchOptions{Stars: minStars})
}

// hasAllAmenities returns true if the hotel's amenities contain every
// requested amenity (case-insensitive substring match).
func hasAllAmenities(have, want []string) bool {
	set := make(map[string]bool, len(have))
	for _, a := range have {
		set[strings.ToLower(a)] = true
	}
	for _, req := range want {
		if !set[strings.ToLower(strings.TrimSpace(req))] {
			return false
		}
	}
	return true
}

// sortHotels sorts hotel results in-place by the given criteria.
func sortHotels(hotels []models.HotelResult, sortBy string, centerLat, centerLon float64) {
	switch strings.ToLower(sortBy) {
	case "cheapest", "price", "":
		// Sort by price ascending. Hotels with price=0 go to the end.
		sort.Slice(hotels, func(i, j int) bool {
			return lessPrice(hotels[i], hotels[j])
		})
	case "rating":
		// Sort by rating descending.
		sort.Slice(hotels, func(i, j int) bool {
			return hotels[i].Rating > hotels[j].Rating
		})
	case "stars":
		// Sort by star rating descending.
		sort.Slice(hotels, func(i, j int) bool {
			return hotels[i].Stars > hotels[j].Stars
		})
	case "distance":
		// Sort by distance from city center ascending.
		if centerLat != 0 || centerLon != 0 {
			sort.Slice(hotels, func(i, j int) bool {
				di := Haversine(centerLat, centerLon, hotels[i].Lat, hotels[i].Lon)
				dj := Haversine(centerLat, centerLon, hotels[j].Lat, hotels[j].Lon)
				return di < dj
			})
		}
	}
}

// Haversine returns the great-circle distance in kilometers between two
// points specified in decimal degrees.
func Haversine(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadiusKm = 6371.0
	dLat := degreesToRadians(lat2 - lat1)
	dLon := degreesToRadians(lon2 - lon1)

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(degreesToRadians(lat1))*math.Cos(degreesToRadians(lat2))*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadiusKm * c
}

func degreesToRadians(deg float64) float64 {
	return deg * math.Pi / 180
}

func lessPrice(a, b models.HotelResult) bool {
	if a.Price == 0 {
		return false
	}
	if b.Price == 0 {
		return true
	}
	return a.Price < b.Price
}

// parseDateArray converts "YYYY-MM-DD" to [year, month, day].
func parseDateArray(s string) ([3]int, error) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return [3]int{}, fmt.Errorf("invalid date %q: expected YYYY-MM-DD", s)
	}
	return [3]int{t.Year(), int(t.Month()), t.Day()}, nil
}

// SearchHotelByName searches for a specific hotel by name and returns its details.
// Unlike SearchHotels (which searches by area), this uses Google's entity
// resolution to find a specific property via name matching within search results.
//
// Strategy: Google Hotels returns listings when searching by city/area, not hotel
// names. We extract a location context from the query (text after comma, or last
// word(s)), search that area, then fuzzy-match the hotel name in results. If that
// fails we fall back to searching the full query as the location.
func SearchHotelByName(ctx context.Context, query string, checkIn, checkOut string) (*models.HotelResult, error) {
	if query == "" {
		return nil, fmt.Errorf("hotel name query is required")
	}
	if checkIn == "" || checkOut == "" {
		return nil, fmt.Errorf("check-in and check-out dates are required")
	}

	opts := HotelSearchOptions{
		CheckIn:  checkIn,
		CheckOut: checkOut,
		Guests:   2,
		Currency: "USD",
	}

	// Build search location candidates: prefer context after comma, then last word.
	candidates := buildLocationCandidates(query)

	var lastErr error
	for _, loc := range candidates {
		result, err := SearchHotels(ctx, loc, opts)
		if err != nil {
			lastErr = err
			continue
		}
		if len(result.Hotels) == 0 {
			continue
		}

		match := findBestNameMatch(result.Hotels, query)
		if match != nil {
			return match, nil
		}

		// Area search succeeded but no name match — return first result with a note.
		first := result.Hotels[0]
		return &first, nil
	}

	if lastErr != nil {
		return nil, fmt.Errorf("hotel name search: %w", lastErr)
	}
	return nil, fmt.Errorf("no hotels found for %q", query)
}

// buildLocationCandidates generates location search strings from a hotel name query.
// E.g. "Beverly Hills Heights, Tenerife" -> ["Tenerife", "Beverly Hills Heights Tenerife"]
func buildLocationCandidates(query string) []string {
	var candidates []string

	// If comma-separated, use the part after the last comma as primary location.
	if idx := strings.LastIndex(query, ","); idx >= 0 {
		after := strings.TrimSpace(query[idx+1:])
		before := strings.TrimSpace(query[:idx])
		if after != "" {
			candidates = append(candidates, after)
		}
		// Also try "before after" as the full query.
		if before != "" && after != "" {
			candidates = append(candidates, before+" "+after)
		}
	}

	// Try the full query as location (works when it contains a city).
	candidates = append(candidates, query)

	return candidates
}

// findBestNameMatch searches hotels for the best fuzzy match to the query.
func findBestNameMatch(hotels []models.HotelResult, query string) *models.HotelResult {
	queryLower := strings.ToLower(query)
	queryWords := strings.Fields(queryLower)

	var best *models.HotelResult
	bestScore := 0

	for i := range hotels {
		h := &hotels[i]
		nameLower := strings.ToLower(h.Name)

		score := 0
		// Exact contains match scores highest.
		if strings.Contains(nameLower, queryLower) {
			score = 100
		} else {
			// Count how many query words (≥3 chars) appear in the hotel name.
			for _, w := range queryWords {
				if len(w) >= 3 && strings.Contains(nameLower, w) {
					score += 10
				}
			}
		}

		if score > bestScore {
			bestScore = score
			best = h
		}
	}

	if bestScore == 0 {
		return nil
	}
	return best
}

// enrichHotelAmenities fetches detail pages for the top N hotels to get full
// amenity lists. Runs up to 3 concurrent fetches. Hotels without a HotelID
// are skipped. Failures are silently ignored (search results still have
// partial amenities from the search page).
func enrichHotelAmenities(ctx context.Context, hotels []models.HotelResult, limit int) []models.HotelResult {
	if limit <= 0 {
		limit = 5
	}
	if limit > 10 {
		limit = 10
	}

	// Collect indices of hotels eligible for enrichment.
	var indices []int
	for i := range hotels {
		if hotels[i].HotelID != "" && len(indices) < limit {
			indices = append(indices, i)
		}
	}
	if len(indices) == 0 {
		return hotels
	}

	// Fetch detail pages in parallel with concurrency limit of 3.
	const concurrency = 3
	type result struct {
		index     int
		amenities []string
	}

	results := make(chan result, len(indices))
	sem := make(chan struct{}, concurrency)

	var wg sync.WaitGroup
	for _, idx := range indices {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			amenities, err := FetchHotelAmenities(ctx, hotels[i].HotelID)
			if err != nil || len(amenities) == 0 {
				return
			}
			results <- result{index: i, amenities: amenities}
		}(idx)
	}

	// Close results channel when all goroutines complete.
	go func() {
		wg.Wait()
		close(results)
	}()

	// Apply enriched amenities back to hotels.
	for r := range results {
		hotels[r.index].Amenities = mergeAmenities(hotels[r.index].Amenities, r.amenities)
	}

	return hotels
}

// propertyTypeCode converts a human-readable property type string to the
// Google Hotels &ptype= parameter value. Returns "" if the type is unknown
// or empty (meaning: no filter applied).
//
// Known Google Hotels ptype values (reverse-engineered):
//
//	2 = hotel, 3 = apartment, 4 = hostel, 5 = resort, 7 = bnb, 8 = villa
func propertyTypeCode(t string) string {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "hotel":
		return "2"
	case "apartment":
		return "3"
	case "hostel":
		return "4"
	case "resort":
		return "5"
	case "bnb", "bed_and_breakfast", "bed and breakfast":
		return "7"
	case "villa":
		return "8"
	default:
		return ""
	}
}

// mergeAmenities combines two amenity lists, deduplicating by lowercase name.
// The first list's items take priority in ordering.
// tagHotelSource stamps each hotel with a PriceSource for the given provider
// so that MergeHotelResults can track per-provider prices. Hotels that already
// carry Sources (e.g. from a previous enrichment pass) are left unchanged.
func tagHotelSource(hotels []models.HotelResult, provider string) []models.HotelResult {
	tagged := make([]models.HotelResult, len(hotels))
	copy(tagged, hotels)
	for i := range tagged {
		if len(tagged[i].Sources) == 0 {
			tagged[i].Sources = []models.PriceSource{{
				Provider:   provider,
				Price:      tagged[i].Price,
				Currency:   tagged[i].Currency,
				BookingURL: tagged[i].BookingURL,
			}}
		}
	}
	return tagged
}

func mergeAmenities(existing, additional []string) []string {
	seen := make(map[string]bool, len(existing)+len(additional))
	var merged []string

	for _, a := range existing {
		lower := strings.ToLower(a)
		if !seen[lower] {
			seen[lower] = true
			merged = append(merged, a)
		}
	}
	for _, a := range additional {
		lower := strings.ToLower(a)
		if !seen[lower] {
			seen[lower] = true
			merged = append(merged, a)
		}
	}

	return merged
}

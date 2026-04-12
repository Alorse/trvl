package hotels

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/MikkoParkkola/trvl/internal/models"
	"golang.org/x/time/rate"
)

// airbnbLimiter enforces a conservative 1 req/s rate limit.
// Airbnb will block aggressive scrapers. We err on the side of caution.
var airbnbLimiter = rate.NewLimiter(rate.Every(time.Second), 1)

// airbnbClient is a shared HTTP client for Airbnb HTML scraping.
// 30-second timeout covers slow international responses.
var airbnbClient = &http.Client{
	Timeout: 30 * time.Second,
}

// airbnbUA is a realistic Chrome User-Agent to avoid bot detection.
const airbnbUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

// SearchAirbnb searches Airbnb for accommodations in the given location.
//
// Airbnb has no public API; we scrape the search HTML and extract the embedded
// JSON from the <script id="data-deferred-state-0"> tag. The HTML structure
// WILL change without notice — every path access is bounds-checked and the
// function returns empty results (not an error) when the format has changed.
func SearchAirbnb(ctx context.Context, location string, opts HotelSearchOptions) ([]models.HotelResult, error) {
	if opts.CheckIn == "" || opts.CheckOut == "" {
		return nil, fmt.Errorf("airbnb search: check-in and check-out dates are required")
	}

	// Enforce rate limit before making the request.
	if err := airbnbLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("airbnb rate limiter: %w", err)
	}

	searchURL := buildAirbnbURL(location, opts)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("airbnb build request: %w", err)
	}

	req.Header.Set("User-Agent", airbnbUA)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := airbnbClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("airbnb request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		return nil, fmt.Errorf("airbnb rate limited (429)")
	}
	if resp.StatusCode == 403 {
		return nil, fmt.Errorf("airbnb blocked (403)")
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("airbnb returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024)) // 10 MB cap
	if err != nil {
		return nil, fmt.Errorf("airbnb read body: %w", err)
	}

	return parseAirbnbHTML(string(body), opts)
}

// buildAirbnbURL constructs the Airbnb search URL for the given location and options.
func buildAirbnbURL(location string, opts HotelSearchOptions) string {
	encoded := url.PathEscape(location)
	q := url.Values{}

	if opts.CheckIn != "" {
		q.Set("checkin", opts.CheckIn)
	}
	if opts.CheckOut != "" {
		q.Set("checkout", opts.CheckOut)
	}
	if opts.Guests > 0 {
		q.Set("adults", fmt.Sprintf("%d", opts.Guests))
	}
	if opts.MinPrice > 0 {
		q.Set("price_min", fmt.Sprintf("%.0f", opts.MinPrice))
	}
	if opts.MaxPrice > 0 {
		q.Set("price_max", fmt.Sprintf("%.0f", opts.MaxPrice))
	}

	return fmt.Sprintf("https://www.airbnb.com/s/%s/homes?%s", encoded, q.Encode())
}

// parseAirbnbHTML extracts hotel results from Airbnb's search page HTML.
//
// The page embeds JSON in <script id="data-deferred-state-0"> tags.
// The JSON path to listings is:
//
//	niobeClientData[0][1].data.presentation.staysSearch.results.searchResults
//
// On any format change the function logs a warning and returns empty results.
func parseAirbnbHTML(html string, opts HotelSearchOptions) ([]models.HotelResult, error) {
	jsonBlob := extractAirbnbScriptJSON(html)
	if jsonBlob == "" {
		slog.Warn("airbnb: data-deferred-state-0 script tag not found — page format may have changed")
		return nil, nil
	}

	var root any
	if err := json.Unmarshal([]byte(jsonBlob), &root); err != nil {
		slog.Warn("airbnb: failed to parse embedded JSON", "error", err)
		return nil, nil
	}

	listings := extractAirbnbListings(root)
	if listings == nil {
		slog.Warn("airbnb: listing array not found at expected JSON path — format may have changed")
		return nil, nil
	}

	nights := airbnbNights(opts.CheckIn, opts.CheckOut)

	var results []models.HotelResult
	for _, item := range listings {
		hotel, ok := mapAirbnbListing(item, nights)
		if !ok {
			continue
		}
		results = append(results, hotel)
	}

	return results, nil
}

// extractAirbnbScriptJSON locates the <script> tag whose id attribute equals
// "data-deferred-state-0" and returns its text content. Returns "" when not found.
// Uses plain string search — no full HTML parser dependency.
//
// The search is done by finding the id attribute value in the document and
// walking back to the '<' that opens the enclosing script tag. This handles
// any ordering of attributes within the tag.
func extractAirbnbScriptJSON(html string) string {
	const idAttr = `id="data-deferred-state-0"`
	const closeTag = `</script>`

	// Find the id attribute anywhere in the document.
	idPos := strings.Index(html, idAttr)
	if idPos < 0 {
		return ""
	}

	// Walk backwards to find the '<' that opens this tag.
	tagStart := strings.LastIndex(html[:idPos], "<")
	if tagStart < 0 {
		return ""
	}

	// Verify we found a <script tag (not some other element).
	tagName := html[tagStart:]
	if !strings.HasPrefix(tagName, "<script") {
		return ""
	}

	// Find the closing '>' of the opening tag.
	openTagEnd := strings.Index(tagName, ">")
	if openTagEnd < 0 {
		return ""
	}
	contentStart := tagStart + openTagEnd + 1

	closeStart := strings.Index(html[contentStart:], closeTag)
	if closeStart < 0 {
		return ""
	}

	content := strings.TrimSpace(html[contentStart : contentStart+closeStart])
	return content
}

// extractAirbnbListings navigates the parsed JSON to the array of listing objects.
// Returns nil when any step in the path is missing or the wrong type.
//
// Path: niobeClientData[0][1].data.presentation.staysSearch.results.searchResults
func extractAirbnbListings(root any) []any {
	// niobeClientData
	rootMap, ok := root.(map[string]any)
	if !ok {
		return nil
	}
	niobeRaw, ok := rootMap["niobeClientData"]
	if !ok {
		return nil
	}

	// [0]
	niobeArr, ok := niobeRaw.([]any)
	if !ok || len(niobeArr) < 1 {
		return nil
	}
	elem0, ok := niobeArr[0].([]any)
	if !ok || len(elem0) < 2 {
		return nil
	}

	// [1]
	elem1, ok := elem0[1].(map[string]any)
	if !ok {
		return nil
	}

	// .data
	dataRaw, ok := elem1["data"]
	if !ok {
		return nil
	}
	dataMap, ok := dataRaw.(map[string]any)
	if !ok {
		return nil
	}

	// .presentation
	presRaw, ok := dataMap["presentation"]
	if !ok {
		return nil
	}
	presMap, ok := presRaw.(map[string]any)
	if !ok {
		return nil
	}

	// .staysSearch
	staysRaw, ok := presMap["staysSearch"]
	if !ok {
		return nil
	}
	staysMap, ok := staysRaw.(map[string]any)
	if !ok {
		return nil
	}

	// .results
	resultsRaw, ok := staysMap["results"]
	if !ok {
		return nil
	}
	resultsMap, ok := resultsRaw.(map[string]any)
	if !ok {
		return nil
	}

	// .searchResults
	srRaw, ok := resultsMap["searchResults"]
	if !ok {
		return nil
	}
	searchResults, ok := srRaw.([]any)
	if !ok {
		return nil
	}

	return searchResults
}

// mapAirbnbListing converts a single Airbnb listing JSON object to a HotelResult.
// Returns (result, false) when essential fields (id, name) are missing.
func mapAirbnbListing(item any, nights int) (models.HotelResult, bool) {
	listing, ok := item.(map[string]any)
	if !ok {
		return models.HotelResult{}, false
	}

	// listing.listing — the core listing object
	listingData, ok := listing["listing"].(map[string]any)
	if !ok {
		return models.HotelResult{}, false
	}

	id, _ := listingData["id"].(string)
	if id == "" {
		// Try numeric id
		if idNum, ok := listingData["id"].(json.Number); ok {
			id = idNum.String()
		}
	}
	if id == "" {
		return models.HotelResult{}, false
	}

	name, _ := listingData["name"].(string)
	if name == "" {
		return models.HotelResult{}, false
	}

	var rating float64
	var reviewCount int

	// avgRatingLocalized is a string like "4.85"; fall back to avgRating float.
	if rStr, ok := listingData["avgRatingLocalized"].(string); ok && rStr != "" {
		// May be "4.85 (123)" or just "4.85"
		fields := strings.Fields(rStr)
		if len(fields) > 0 {
			var v float64
			if _, err := fmt.Sscanf(fields[0], "%f", &v); err == nil {
				rating = v
			}
		}
	}
	if rating == 0 {
		if r, ok := listingData["avgRating"].(float64); ok {
			rating = r
		}
	}
	if rc, ok := listingData["reviewsCount"].(float64); ok {
		reviewCount = int(rc)
	}

	// Property type — map Airbnb's roomTypeCategory to our property type string.
	propertyType := ""
	if rtc, ok := listingData["roomTypeCategory"].(string); ok {
		propertyType = mapAirbnbPropertyType(rtc)
	}

	// Price: navigate listing.pricingQuote.structuredStayDisplayPrice or
	// pricingQuote.price.total.amount. Defensive multi-path extraction.
	price, currency := extractAirbnbPrice(listing, nights)

	// Superhost badge — check listing.contextualPictures or badges in listing.
	isSuperhost := false
	if badges, ok := listingData["badges"].([]any); ok {
		for _, b := range badges {
			if s, ok := b.(string); ok && strings.EqualFold(s, "superhost") {
				isSuperhost = true
				break
			}
		}
	}

	// Coordinates (may not be present in search results).
	var lat, lon float64
	if coord, ok := listingData["coordinate"].(map[string]any); ok {
		lat, _ = coord["latitude"].(float64)
		lon, _ = coord["longitude"].(float64)
	}

	// City from contextual information.
	address := ""
	if city, ok := listingData["city"].(string); ok && city != "" {
		address = city
		if country, ok := listingData["countryCode"].(string); ok && country != "" {
			address = city + ", " + country
		}
	}

	var amenities []string
	if isSuperhost {
		amenities = append(amenities, "superhost")
	}

	bookingURL := fmt.Sprintf("https://www.airbnb.com/rooms/%s", id)

	result := models.HotelResult{
		Name:         name,
		HotelID:      "airbnb_" + id,
		Rating:       rating,
		ReviewCount:  reviewCount,
		Stars:        0, // Airbnb does not have star ratings
		Price:        price,
		Currency:     currency,
		Address:      address,
		Lat:          lat,
		Lon:          lon,
		Amenities:    amenities,
		BookingURL:   bookingURL,
		Sources: []models.PriceSource{{
			Provider:   "airbnb",
			Price:      price,
			Currency:   currency,
			BookingURL: bookingURL,
		}},
	}

	// Store property type in amenities as a tag when available.
	if propertyType != "" {
		result.Amenities = append(result.Amenities, "type:"+propertyType)
	}

	return result, true
}

// extractAirbnbPrice attempts multiple JSON paths to find the total price.
// Returns (price per night, currency). On failure returns (0, "").
//
// Tried paths (in order):
//  1. listing.pricingQuote.structuredStayDisplayPrice.primaryLine.price (string with currency)
//  2. listing.pricingQuote.price.total.amount + .currency
//  3. listing.pricingQuote.priceString (fallback string parse)
func extractAirbnbPrice(listing map[string]any, nights int) (float64, string) {
	pricingQuote, ok := listing["pricingQuote"].(map[string]any)
	if !ok {
		return 0, ""
	}

	// Path 1: structuredStayDisplayPrice.primaryLine.price
	if ssdp, ok := pricingQuote["structuredStayDisplayPrice"].(map[string]any); ok {
		if primary, ok := ssdp["primaryLine"].(map[string]any); ok {
			if priceStr, ok := primary["price"].(string); ok && priceStr != "" {
				price, currency := parsePriceString(priceStr)
				if price > 0 {
					return normalizeToPerNight(price, nights), currency
				}
			}
		}
	}

	// Path 2: price.total.amount + price.total.currency
	if priceBlock, ok := pricingQuote["price"].(map[string]any); ok {
		if total, ok := priceBlock["total"].(map[string]any); ok {
			amount, hasAmount := total["amount"].(float64)
			cur, _ := total["currency"].(string)
			if hasAmount && amount > 0 {
				return normalizeToPerNight(amount, nights), cur
			}
		}
	}

	// Path 3: priceString — string like "€120 / night"
	if priceStr, ok := pricingQuote["priceString"].(string); ok && priceStr != "" {
		price, currency := parsePriceString(priceStr)
		if price > 0 {
			return normalizeToPerNight(price, nights), currency
		}
	}

	return 0, ""
}

// normalizeToPerNight converts a total trip price to per-night price.
// If nights <= 0, returns the total price as-is (can't divide).
func normalizeToPerNight(total float64, nights int) float64 {
	if nights <= 0 {
		return total
	}
	return total / float64(nights)
}

// mapAirbnbPropertyType converts Airbnb's roomTypeCategory to trvl's property type.
func mapAirbnbPropertyType(rtc string) string {
	switch strings.ToLower(rtc) {
	case "entire_home", "entire_home_apt", "entire_place":
		return "apartment"
	case "private_room":
		return "hostel"
	case "shared_room":
		return "hostel"
	case "hotel_room":
		return "hotel"
	default:
		return ""
	}
}

// airbnbNights returns the number of nights between check-in and check-out.
// Returns 0 when either date is empty or cannot be parsed.
func airbnbNights(checkIn, checkOut string) int {
	if checkIn == "" || checkOut == "" {
		return 0
	}
	in, err := time.Parse("2006-01-02", checkIn)
	if err != nil {
		return 0
	}
	out, err := time.Parse("2006-01-02", checkOut)
	if err != nil {
		return 0
	}
	n := int(out.Sub(in).Hours() / 24)
	if n < 0 {
		return 0
	}
	return n
}

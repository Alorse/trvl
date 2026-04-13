package hotels

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/MikkoParkkola/trvl/internal/models"
)

var (
	errBookingUnavailable = errors.New("booking provider unavailable")

	bookingEnabled             = !strings.HasSuffix(filepath.Base(os.Args[0]), ".test")
	bookingAPIBaseURL          = "https://metasearch-connect-api.booking.com"
	bookingHTTPClient          = &http.Client{Timeout: 30 * time.Second}
	bookingResolveLocationFunc = ResolveLocation
	searchBookingHotelsFunc    = SearchBookingHotels

	bookingTokenMu     sync.Mutex
	bookingTokenValue  string
	bookingTokenExpiry time.Time
)

const (
	bookingDefaultSearchRadiusKm = 25.0
	bookingDefaultRows           = 100
	bookingTokenRefreshLeeway    = time.Minute
)

type bookingAuthResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   string `json:"expires_in"`
}

type bookingSearchRequest struct {
	CheckIn  string             `json:"checkin"`
	CheckOut string             `json:"checkout"`
	Booker   bookingBookerInput `json:"booker"`
	Guests   bookingGuestsInput `json:"guests"`
	Currency string             `json:"currency,omitempty"`
	Rows     int                `json:"rows,omitempty"`
	Coords   bookingCoordinates `json:"coordinates"`
}

type bookingBookerInput struct {
	Country  string `json:"country"`
	Platform string `json:"platform"`
}

type bookingGuestsInput struct {
	NumberOfAdults int `json:"number_of_adults"`
	NumberOfRooms  int `json:"number_of_rooms"`
}

type bookingCoordinates struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Radius    float64 `json:"radius"`
}

type bookingSearchResponse struct {
	Data []bookingSearchHotel `json:"data"`
}

type bookingSearchHotel struct {
	ID          int                `json:"id"`
	Currency    string             `json:"currency"`
	URL         string             `json:"url"`
	DeepLinkURL string             `json:"deep_link_url"`
	Price       bookingPriceFields `json:"price"`
}

type bookingPriceFields struct {
	Base  float64 `json:"base"`
	Book  float64 `json:"book"`
	Total float64 `json:"total"`
}

type bookingDetailsRequest struct {
	Accommodations []int    `json:"accommodations"`
	Languages      []string `json:"languages,omitempty"`
	Rows           int      `json:"rows,omitempty"`
}

type bookingDetailsResponse struct {
	Data []bookingDetailsHotel `json:"data"`
}

type bookingDetailsHotel struct {
	ID          int                     `json:"id"`
	Name        bookingTranslatedString `json:"name"`
	Currency    string                  `json:"currency"`
	URL         string                  `json:"url"`
	DeepLinkURL string                  `json:"deep_link_url"`
	Location    bookingDetailsLocation  `json:"location"`
	Rating      bookingDetailsRating    `json:"rating"`
}

type bookingTranslatedString struct {
	Translations map[string]string `json:"translations"`
}

type bookingLazyTranslatedString struct {
	Fallback string `json:"fallback"`
}

type bookingDetailsLocation struct {
	Address     bookingLazyTranslatedString `json:"address"`
	Coordinates bookingCoordinatesDTO       `json:"coordinates"`
}

type bookingCoordinatesDTO struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type bookingDetailsRating struct {
	NumberOfReviews int     `json:"number_of_reviews"`
	ReviewScore     float64 `json:"review_score"`
	Stars           float64 `json:"stars"`
}

// HasBookingKey reports whether Booking.com MetaSearch Connect should be used.
func HasBookingKey() bool {
	return strings.TrimSpace(os.Getenv("BOOKING_API_KEY")) != ""
}

// SearchBookingHotels searches Booking.com MetaSearch Connect for hotels near
// the resolved location center and maps the results into the shared HotelResult
// model.
func SearchBookingHotels(ctx context.Context, location string, opts HotelSearchOptions) ([]models.HotelResult, error) {
	if !bookingEnabled {
		return nil, errBookingUnavailable
	}
	if !HasBookingKey() {
		return nil, errBookingUnavailable
	}
	if opts.CheckIn == "" || opts.CheckOut == "" {
		return nil, fmt.Errorf("booking search: check-in and check-out dates are required")
	}

	lat, lon, err := resolveBookingSearchCenter(ctx, location, opts)
	if err != nil {
		return nil, fmt.Errorf("booking resolve location: %w", err)
	}

	token, err := bookingAccessToken(ctx)
	if err != nil {
		return nil, err
	}

	guests := opts.Guests
	if guests <= 0 {
		guests = 2
	}

	searchReq := bookingSearchRequest{
		CheckIn:  opts.CheckIn,
		CheckOut: opts.CheckOut,
		Booker: bookingBookerInput{
			Country:  bookingBookerCountry(),
			Platform: "DESKTOP",
		},
		Guests: bookingGuestsInput{
			NumberOfAdults: guests,
			NumberOfRooms:  1,
		},
		Rows: bookingDefaultRows,
		Coords: bookingCoordinates{
			Latitude:  lat,
			Longitude: lon,
			Radius:    bookingSearchRadiusKm(opts),
		},
	}
	if opts.Currency != "" {
		searchReq.Currency = strings.ToUpper(opts.Currency)
	}

	var searchResp bookingSearchResponse
	if err := bookingPostJSON(ctx, token, "/demand-api-v3-compatible/accommodations/search", searchReq, &searchResp); err != nil {
		return nil, fmt.Errorf("booking search request: %w", err)
	}
	if len(searchResp.Data) == 0 {
		return nil, nil
	}

	ids := make([]int, 0, len(searchResp.Data))
	seen := make(map[int]struct{}, len(searchResp.Data))
	for _, item := range searchResp.Data {
		if item.ID == 0 {
			continue
		}
		if _, ok := seen[item.ID]; ok {
			continue
		}
		seen[item.ID] = struct{}{}
		ids = append(ids, item.ID)
	}
	if len(ids) == 0 {
		return nil, nil
	}

	detailsReq := bookingDetailsRequest{
		Accommodations: ids,
		Languages:      []string{"en"},
		Rows:           len(ids),
	}
	var detailsResp bookingDetailsResponse
	if err := bookingPostJSON(ctx, token, "/demand-api-v3-compatible/accommodations/details", detailsReq, &detailsResp); err != nil {
		return nil, fmt.Errorf("booking details request: %w", err)
	}

	return mapBookingHotels(searchResp.Data, detailsResp.Data, searchReq.Currency), nil
}

// bookingSearchEligibleOptions guards Booking integration behind the subset of
// filters whose semantics are preserved after mapping into HotelResult.
func bookingSearchEligibleOptions(opts HotelSearchOptions) bool {
	return !opts.FreeCancellation && !opts.EcoCertified && strings.TrimSpace(opts.PropertyType) == ""
}

func resolveBookingSearchCenter(ctx context.Context, location string, opts HotelSearchOptions) (float64, float64, error) {
	if opts.CenterLat != 0 || opts.CenterLon != 0 {
		return opts.CenterLat, opts.CenterLon, nil
	}
	return bookingResolveLocationFunc(ctx, location)
}

func bookingSearchRadiusKm(opts HotelSearchOptions) float64 {
	if opts.MaxDistanceKm > 0 {
		return math.Max(1, opts.MaxDistanceKm)
	}
	return bookingDefaultSearchRadiusKm
}

func bookingAccessToken(ctx context.Context) (string, error) {
	if !HasBookingKey() {
		return "", errBookingUnavailable
	}

	bookingTokenMu.Lock()
	defer bookingTokenMu.Unlock()

	if bookingTokenValue != "" && time.Now().Add(bookingTokenRefreshLeeway).Before(bookingTokenExpiry) {
		return bookingTokenValue, nil
	}

	form := url.Values{}
	form.Set("api_key", strings.TrimSpace(os.Getenv("BOOKING_API_KEY")))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, bookingAPIBaseURL+"/auth", strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("booking auth request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := bookingHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("booking auth request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("booking auth read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("booking auth returned status %d: %s", resp.StatusCode, summarizeBookingBody(body))
	}

	var parsed bookingAuthResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("decode booking auth response: %w", err)
	}
	if parsed.AccessToken == "" {
		return "", errors.New("booking auth response missing access_token")
	}

	expiresIn := time.Hour
	if parsed.ExpiresIn != "" {
		seconds, err := strconv.Atoi(parsed.ExpiresIn)
		if err != nil {
			return "", fmt.Errorf("parse booking auth expires_in %q: %w", parsed.ExpiresIn, err)
		}
		if seconds > 0 {
			expiresIn = time.Duration(seconds) * time.Second
		}
	}

	bookingTokenValue = parsed.AccessToken
	bookingTokenExpiry = time.Now().Add(expiresIn)
	return parsed.AccessToken, nil
}

func bookingPostJSON(ctx context.Context, token, path string, payload, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal %s payload: %w", path, err)
	}

	respBody, status, err := bookingPostJSONRaw(ctx, token, path, body)
	if err != nil {
		return err
	}
	if status == http.StatusUnauthorized {
		bookingResetTokenCache()
		refreshedToken, err := bookingAccessToken(ctx)
		if err != nil {
			return err
		}
		respBody, status, err = bookingPostJSONRaw(ctx, refreshedToken, path, body)
		if err != nil {
			return err
		}
	}
	if status != http.StatusOK {
		return fmt.Errorf("%s returned status %d: %s", path, status, summarizeBookingBody(respBody))
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("decode %s response: %w", path, err)
	}
	return nil
}

func bookingPostJSONRaw(ctx context.Context, token, path string, body []byte) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, bookingAPIBaseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, 0, fmt.Errorf("build %s request: %w", path, err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := bookingHTTPClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("booking request %s: %w", path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, 0, fmt.Errorf("read booking response %s: %w", path, err)
	}
	return respBody, resp.StatusCode, nil
}

func mapBookingHotels(search []bookingSearchHotel, details []bookingDetailsHotel, defaultCurrency string) []models.HotelResult {
	detailsByID := make(map[int]bookingDetailsHotel, len(details))
	for _, item := range details {
		detailsByID[item.ID] = item
	}

	results := make([]models.HotelResult, 0, len(search))
	for _, item := range search {
		detail, ok := detailsByID[item.ID]
		if !ok {
			continue
		}
		name := bookingTranslatedValue(detail.Name)
		if name == "" {
			continue
		}

		price := bookingPriceValue(item.Price)
		currency := strings.ToUpper(firstNonEmpty(item.Currency, detail.Currency, defaultCurrency))
		bookingURL := sanitizeBookingURL(firstNonEmpty(detail.URL, item.URL, detail.DeepLinkURL, item.DeepLinkURL))

		results = append(results, models.HotelResult{
			Name:        name,
			Rating:      detail.Rating.ReviewScore / 2,
			ReviewCount: detail.Rating.NumberOfReviews,
			Stars:       int(math.Round(detail.Rating.Stars)),
			Price:       price,
			Currency:    currency,
			Address:     detail.Location.Address.Fallback,
			Lat:         detail.Location.Coordinates.Latitude,
			Lon:         detail.Location.Coordinates.Longitude,
			BookingURL:  bookingURL,
			Sources: []models.PriceSource{{
				Provider:   "booking",
				Price:      price,
				Currency:   currency,
				BookingURL: bookingURL,
			}},
		})
	}
	return results
}

func bookingPriceValue(price bookingPriceFields) float64 {
	switch {
	case price.Total > 0:
		return price.Total
	case price.Book > 0:
		return price.Book
	default:
		return price.Base
	}
}

func bookingTranslatedValue(value bookingTranslatedString) string {
	if len(value.Translations) == 0 {
		return ""
	}
	for _, preferred := range []string{"en", "en-us", "en-gb"} {
		if text := strings.TrimSpace(value.Translations[preferred]); text != "" {
			return text
		}
	}
	keys := make([]string, 0, len(value.Translations))
	for key := range value.Translations {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if text := strings.TrimSpace(value.Translations[key]); text != "" {
			return text
		}
	}
	return ""
}

func bookingBookerCountry() string {
	if country := strings.ToLower(strings.TrimSpace(os.Getenv("TRVL_BOOKER_COUNTRY"))); isISO2CountryCode(country) {
		return country
	}
	for _, key := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if country := localeCountryCode(os.Getenv(key)); country != "" {
			return country
		}
	}
	return "us"
}

func localeCountryCode(locale string) string {
	parts := strings.FieldsFunc(strings.TrimSpace(locale), func(r rune) bool {
		return r == '_' || r == '-' || r == '.' || r == '@'
	})
	for i := len(parts) - 1; i >= 0; i-- {
		candidate := strings.ToLower(strings.TrimSpace(parts[i]))
		if isISO2CountryCode(candidate) {
			return candidate
		}
	}
	return ""
}

func isISO2CountryCode(value string) bool {
	if len(value) != 2 {
		return false
	}
	for _, r := range value {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	return true
}

func summarizeBookingBody(body []byte) string {
	text := strings.TrimSpace(string(body))
	if text == "" {
		return "empty response body"
	}
	if len(text) > 200 {
		return text[:200] + "..."
	}
	return text
}

func bookingResetTokenCache() {
	bookingTokenMu.Lock()
	defer bookingTokenMu.Unlock()
	bookingTokenValue = ""
	bookingTokenExpiry = time.Time{}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

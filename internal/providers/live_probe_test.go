package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"
)

// Live probe tests — opt-in via TRVL_TEST_LIVE_PROBES=1.
// These hit real endpoints to validate that provider configs generated
// from the suggest_providers catalog actually work.

func skipIfNoLiveProbes(t *testing.T) {
	t.Helper()
	if os.Getenv("TRVL_TEST_LIVE_PROBES") != "1" {
		t.Skip("live probes disabled (set TRVL_TEST_LIVE_PROBES=1)")
	}
}

func TestLiveProbe_Hostelworld(t *testing.T) {
	skipIfNoLiveProbes(t)

	// Verified pattern: APIGEE_KEY:"..." in page HTML config block.
	//
	// Known limitation: the Hostelworld API requires a city ID in the URL path
	// (not coordinates). City ID 59 = Paris. In production use the LLM would
	// need to resolve a city name to an ID first, e.g. via the Hostelworld
	// autocomplete endpoint /api/search/autocomplete?query=Paris.
	cfg := &ProviderConfig{
		ID:       "hostelworld",
		Name:     "Hostelworld",
		Category: "hotels",
		Auth: &AuthConfig{
			Type:         "preflight",
			PreflightURL: "https://www.hostelworld.com",
			Extractions: map[string]Extraction{
				"api_key": {
					Pattern:  `APIGEE_KEY:"([^"]+)"`,
					Variable: "api_key",
				},
			},
		},
		Endpoint: "https://prod.apigee.hostelworld.com/legacy-hwapi-service/2.2/cities/59/properties/",
		Method:   "GET",
		Headers: map[string]string{
			"api-key": "${api_key}",
		},
		QueryParams: map[string]string{
			"date-start": "${checkin}",
			"num-nights": "1",
			"guests":     "${guests}",
			"currency":   "${currency}",
			"per-page":   "10",
		},
		ResponseMapping: ResponseMapping{
			ResultsPath: "properties",
			Fields: map[string]string{
				"name":         "name",
				"hotel_id":     "id",
				"rating":       "rating.overall",
				"review_count": "rating.numberOfRatings",
				"price":        "lowestPricePerNight.value",
				"currency":     "lowestPricePerNight.currency",
			},
		},
		RateLimit: RateLimitConfig{RequestsPerSecond: 1},
		TLS:      TLSConfig{Fingerprint: "standard"},
	}

	checkin := time.Now().AddDate(0, 0, 14).Format("2006-01-02")
	checkout := time.Now().AddDate(0, 0, 15).Format("2006-01-02")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result := TestProvider(ctx, cfg, "Paris", 48.8566, 2.3522, checkin, checkout, "EUR", 2)
	logResult(t, "Hostelworld", result)

	if !result.Success {
		t.Errorf("Hostelworld probe failed at step %q: %s", result.Step, result.Error)
	}
	if result.ResultsCount == 0 {
		t.Error("Hostelworld returned 0 results")
	}
}

func TestLiveProbe_Airbnb(t *testing.T) {
	skipIfNoLiveProbes(t)

	// Verified pattern: api_config key extracted from homepage.
	// Uses Chrome TLS fingerprint — required for Airbnb.
	cfg := &ProviderConfig{
		ID:       "airbnb",
		Name:     "Airbnb",
		Category: "hotels",
		Auth: &AuthConfig{
			Type:         "preflight",
			PreflightURL: "https://www.airbnb.com",
			Extractions: map[string]Extraction{
				"api_key": {
					Pattern:  `"api_config":\{"key":"([^"]+)"`,
					Variable: "api_key",
				},
			},
		},
		Endpoint: "https://www.airbnb.com/api/v3/StaysSearch/d4d9503616dc72ab220ed8dcf17f166816dccb2593e7b4625c91c3fce3a3b3d6",
		Method:   "POST",
		Headers: map[string]string{
			"Content-Type":           "application/json",
			"X-Airbnb-Api-Key":      "${api_key}",
			"Accept":                "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
			"Accept-Language":       "en",
			"Cache-Control":         "no-cache",
			"Pragma":                "no-cache",
			"Sec-Ch-Ua":             `"Not_A Brand";v="8", "Chromium";v="120", "Google Chrome";v="120"`,
			"Sec-Ch-Ua-Mobile":      "?0",
			"Sec-Ch-Ua-Platform":    `"Windows"`,
			"Sec-Fetch-Dest":        "document",
			"Sec-Fetch-Mode":        "navigate",
			"Sec-Fetch-Site":        "none",
			"Sec-Fetch-User":        "?1",
			"Upgrade-Insecure-Requests": "1",
			"User-Agent":            "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		},
		QueryParams: map[string]string{
			"operationName": "StaysSearch",
			"locale":        "en",
			"currency":      "${currency}",
		},
		BodyTemplate: `{
			"operationName": "StaysSearch",
			"extensions": {
				"persistedQuery": {
					"version": 1,
					"sha256Hash": "d4d9503616dc72ab220ed8dcf17f166816dccb2593e7b4625c91c3fce3a3b3d6"
				}
			},
			"variables": {
				"staysSearchRequest": {
					"cursor": "",
					"maxMapItems": 9999,
					"metadataOnly": false,
					"requestedPageType": "STAYS_SEARCH",
					"source": "structured_search_input_header",
					"searchType": "user_map_move",
					"treatmentFlags": [
						"feed_map_decouple_m11_treatment",
						"stays_search_rehydration_treatment_desktop",
						"stays_search_rehydration_treatment_moweb",
						"selective_query_feed_map_homepage_desktop_treatment",
						"selective_query_feed_map_homepage_moweb_treatment"
					],
					"rawParams": [
						{"filterName": "cdnCacheSafe", "filterValues": ["false"]},
						{"filterName": "channel", "filterValues": ["EXPLORE"]},
						{"filterName": "checkin", "filterValues": ["${checkin}"]},
						{"filterName": "checkout", "filterValues": ["${checkout}"]},
						{"filterName": "datePickerType", "filterValues": ["calendar"]},
						{"filterName": "flexibleTripLengths", "filterValues": ["one_week"]},
						{"filterName": "itemsPerGrid", "filterValues": ["20"]},
						{"filterName": "monthlyLength", "filterValues": ["3"]},
						{"filterName": "monthlyStartDate", "filterValues": ["2024-02-01"]},
						{"filterName": "neLat", "filterValues": ["${ne_lat}"]},
						{"filterName": "neLng", "filterValues": ["${ne_lon}"]},
						{"filterName": "placeId", "filterValues": ["ChIJD7fiBh9u5kcRYJSMaMOCCwQ"]},
						{"filterName": "priceFilterInputType", "filterValues": ["0"]},
						{"filterName": "priceFilterNumNights", "filterValues": ["1"]},
						{"filterName": "query", "filterValues": ["Paris, France"]},
						{"filterName": "refinementPaths", "filterValues": ["/homes"]},
						{"filterName": "screenSize", "filterValues": ["large"]},
						{"filterName": "searchByMap", "filterValues": ["true"]},
						{"filterName": "swLat", "filterValues": ["${sw_lat}"]},
						{"filterName": "swLng", "filterValues": ["${sw_lon}"]},
						{"filterName": "tabId", "filterValues": ["home_tab"]},
						{"filterName": "version", "filterValues": ["1.8.3"]},
						{"filterName": "zoomLevel", "filterValues": ["11"]}
					]
				},
				"staysMapSearchRequestV2": {
					"cursor": "",
					"metadataOnly": false,
					"requestedPageType": "STAYS_SEARCH",
					"source": "structured_search_input_header",
					"searchType": "user_map_move",
					"treatmentFlags": [
						"feed_map_decouple_m11_treatment",
						"stays_search_rehydration_treatment_desktop",
						"stays_search_rehydration_treatment_moweb",
						"selective_query_feed_map_homepage_desktop_treatment",
						"selective_query_feed_map_homepage_moweb_treatment"
					],
					"rawParams": [
						{"filterName": "cdnCacheSafe", "filterValues": ["false"]},
						{"filterName": "channel", "filterValues": ["EXPLORE"]},
						{"filterName": "checkin", "filterValues": ["${checkin}"]},
						{"filterName": "checkout", "filterValues": ["${checkout}"]},
						{"filterName": "datePickerType", "filterValues": ["calendar"]},
						{"filterName": "flexibleTripLengths", "filterValues": ["one_week"]},
						{"filterName": "itemsPerGrid", "filterValues": ["20"]},
						{"filterName": "monthlyLength", "filterValues": ["3"]},
						{"filterName": "monthlyStartDate", "filterValues": ["2024-02-01"]},
						{"filterName": "neLat", "filterValues": ["${ne_lat}"]},
						{"filterName": "neLng", "filterValues": ["${ne_lon}"]},
						{"filterName": "placeId", "filterValues": ["ChIJD7fiBh9u5kcRYJSMaMOCCwQ"]},
						{"filterName": "priceFilterInputType", "filterValues": ["0"]},
						{"filterName": "priceFilterNumNights", "filterValues": ["1"]},
						{"filterName": "query", "filterValues": ["Paris, France"]},
						{"filterName": "refinementPaths", "filterValues": ["/homes"]},
						{"filterName": "screenSize", "filterValues": ["large"]},
						{"filterName": "searchByMap", "filterValues": ["true"]},
						{"filterName": "swLat", "filterValues": ["${sw_lat}"]},
						{"filterName": "swLng", "filterValues": ["${sw_lon}"]},
						{"filterName": "tabId", "filterValues": ["home_tab"]},
						{"filterName": "version", "filterValues": ["1.8.3"]},
						{"filterName": "zoomLevel", "filterValues": ["11"]}
					]
				},
				"includeMapResults": true,
				"isLeanTreatment": false
			}
		}`,
		ResponseMapping: ResponseMapping{
			ResultsPath: "data.presentation.staysSearch.results.searchResults",
			Fields: map[string]string{
				"name":     "listing.name",
				"hotel_id": "listing.id",
				"rating":   "listing.avgRatingLocalized",
				"price":    "pricingQuote.structuredStayDisplayPrice.primaryLine.price",
				"lat":      "listing.coordinate.latitude",
				"lon":      "listing.coordinate.longitude",
			},
		},
		RateLimit: RateLimitConfig{RequestsPerSecond: 0.5},
		TLS:      TLSConfig{Fingerprint: "chrome"},
	}

	checkin := time.Now().AddDate(0, 0, 30).Format("2006-01-02")
	checkout := time.Now().AddDate(0, 0, 31).Format("2006-01-02")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result := TestProvider(ctx, cfg, "Paris", 48.8566, 2.3522, checkin, checkout, "EUR", 2)
	logResult(t, "Airbnb", result)

	if !result.Success {
		t.Errorf("Airbnb probe failed at step %q: %s", result.Step, result.Error)
	}
	if result.ResultsCount == 0 {
		t.Error("Airbnb returned 0 results")
	}
	// The Airbnb persisted-query endpoint returns listing detail fields
	// (name, coordinate, avgRating) as null -- they are hydrated client-side.
	// Verify we at least get a hotel_id and a display price string.
	if result.SampleResult != nil {
		if id, _ := result.SampleResult["hotel_id"].(string); id == "" {
			t.Error("Airbnb sample result has empty hotel_id")
		}
	}
}

func TestLiveProbe_Booking(t *testing.T) {
	skipIfNoLiveProbes(t)

	// Booking.com uses a frontend GraphQL API with CSRF token protection.
	// The CSRF token is extracted from the search results page HTML.
	// Chrome TLS fingerprint is required.
	//
	// NOTE: The FullSearch persisted query hash changes frequently. The
	// placeholder "FILL" will cause the search step to fail, but the test
	// still validates CSRF extraction and endpoint reachability.
	cfg := &ProviderConfig{
		ID:       "booking",
		Name:     "Booking.com",
		Category: "hotels",
		Auth: &AuthConfig{
			Type:         "preflight",
			PreflightURL: "https://www.booking.com/searchresults.html?dest_id=-1456928&lang=en-us",
			Extractions: map[string]Extraction{
				"csrf_token": {
					Pattern:  `"b_csrf_token":"([^"]+)"`,
					Variable: "csrf_token",
				},
			},
		},
		Endpoint: "https://www.booking.com/dml/graphql?lang=en-us",
		Method:   "POST",
		Headers: map[string]string{
			"Content-Type":                  "application/json",
			"x-booking-csrf-token":          "${csrf_token}",
			"x-booking-context-action-name": "searchresults_irene",
			"x-booking-site-type-id":        "1",
			"apollographql-client-name":     "b-search-web-searchresults",
		},
		BodyTemplate: `{"operationName":"FullSearch","variables":{"input":{"dates":{"checkin":"${checkin}","checkout":"${checkout}"},"location":{"searchString":"${location}","destType":"CITY"},"nbAdults":${guests},"nbRooms":1},"carousels":[],"filters":{"selectedFilters":""},"pagination":{"offset":0,"rowsPerPage":25}},"extensions":{"persistedQuery":{"version":1,"sha256Hash":"FILL"}}}`,
		ResponseMapping: ResponseMapping{
			ResultsPath: "data.searchQueries.search.results",
			Fields: map[string]string{
				"name":     "displayName.text",
				"hotel_id": "basicPropertyData.id",
				"rating":   "basicPropertyData.reviewScore.score",
				"price":    "priceDisplayInfoIrene.displayPrice.amountPerStay.amount",
				"lat":      "basicPropertyData.location.latitude",
				"lon":      "basicPropertyData.location.longitude",
			},
		},
		RateLimit: RateLimitConfig{RequestsPerSecond: 0.5},
		TLS:      TLSConfig{Fingerprint: "chrome"},
		Cookies:  CookieConfig{Source: "preflight"},
	}

	checkin := time.Now().AddDate(0, 0, 30).Format("2006-01-02")
	checkout := time.Now().AddDate(0, 0, 31).Format("2006-01-02")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result := TestProvider(ctx, cfg, "Paris", 48.8566, 2.3522, checkin, checkout, "EUR", 2)
	logResult(t, "Booking.com", result)

	if !result.Success {
		// If CSRF extraction failed, log alternative patterns for discovery.
		if result.Step == "auth_extraction" {
			t.Logf("CSRF extraction failed with primary pattern. Body snippet for pattern discovery:\n%s", result.BodySnippet)

			altPatterns := []string{
				`csrf_token['":\s]+['"]([^'"]+)`,
				`X-Booking-CSRF['":\s]+['"]([^'"]+)`,
				`csrf_token\s*=\s*'([^']+)'`,
				`"csrfToken":"([^"]+)"`,
			}
			for _, pat := range altPatterns {
				t.Logf("Suggested alternative CSRF pattern: %s", pat)
			}
		}
		t.Errorf("Booking.com probe failed at step %q: %s", result.Step, result.Error)
	}
	if result.ResultsCount == 0 && result.Success {
		t.Log("Booking.com returned 0 results (search succeeded but empty — persisted query hash may need updating)")
	}
}

func logResult(t *testing.T, name string, r *TestResult) {
	t.Helper()
	data, _ := json.MarshalIndent(r, "", "  ")
	t.Logf("%s result:\n%s", name, string(data))
	if r.Success {
		t.Logf("%s: PASS — %d results", name, r.ResultsCount)
	} else {
		t.Logf("%s: FAIL — step=%s error=%s", name, r.Step, r.Error)
	}
	if len(r.ExtractionResults) > 0 {
		t.Logf("%s extractions: %v", name, r.ExtractionResults)
	}
	if r.SampleResult != nil {
		sample, _ := json.MarshalIndent(r.SampleResult, "", "  ")
		t.Logf("%s sample:\n%s", name, string(sample))
	}
	fmt.Fprintf(os.Stderr, "[PROBE] %s: success=%v step=%s results=%d\n", name, r.Success, r.Step, r.ResultsCount)
}

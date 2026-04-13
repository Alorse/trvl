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
		Endpoint: "https://prod.apigee.hostelworld.com/legacy-hwapi-service/2.2/cities/46/properties/",
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
			"Content-Type":     "application/json",
			"X-Airbnb-Api-Key": "${api_key}",
			"Accept-Language":  "en",
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
					"requestedPageType": "STAYS_SEARCH",
					"source": "structured_search_input_header",
					"searchType": "user_map_move",
					"rawParams": [
						{"filterName": "checkin", "filterValues": ["${checkin}"]},
						{"filterName": "checkout", "filterValues": ["${checkout}"]},
						{"filterName": "neLat", "filterValues": ["${ne_lat}"]},
						{"filterName": "neLng", "filterValues": ["${ne_lon}"]},
						{"filterName": "swLat", "filterValues": ["${sw_lat}"]},
						{"filterName": "swLng", "filterValues": ["${sw_lon}"]},
						{"filterName": "searchByMap", "filterValues": ["true"]},
						{"filterName": "itemsPerGrid", "filterValues": ["20"]}
					]
				},
				"staysMapSearchRequestV2": {
					"cursor": "",
					"requestedPageType": "STAYS_SEARCH",
					"source": "structured_search_input_header",
					"searchType": "user_map_move",
					"rawParams": [
						{"filterName": "checkin", "filterValues": ["${checkin}"]},
						{"filterName": "checkout", "filterValues": ["${checkout}"]},
						{"filterName": "neLat", "filterValues": ["${ne_lat}"]},
						{"filterName": "neLng", "filterValues": ["${ne_lon}"]},
						{"filterName": "swLat", "filterValues": ["${sw_lat}"]},
						{"filterName": "swLng", "filterValues": ["${sw_lon}"]},
						{"filterName": "searchByMap", "filterValues": ["true"]},
						{"filterName": "itemsPerGrid", "filterValues": ["20"]}
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
				"rating":   "listing.avgRating",
				"price":    "listing.pricingQuote.price.total.amount",
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

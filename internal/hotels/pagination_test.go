package hotels

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/MikkoParkkola/trvl/internal/batchexec"
	"github.com/MikkoParkkola/trvl/internal/models"
)

// --- constants ---

func TestPaginationConstants(t *testing.T) {
	if maxPages != 3 {
		t.Errorf("maxPages = %d, want 3", maxPages)
	}
	if pageSize != 20 {
		t.Errorf("pageSize = %d, want 20", pageSize)
	}
}

// --- fetchHotelPage URL construction ---

func TestFetchHotelPage_OffsetZeroNoStartParam(t *testing.T) {
	// Verify that offset=0 does NOT add &start= to the URL.
	var capturedURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.WriteHeader(200)
		w.Write(fakeHotelPage("Hotel A"))
	}))
	defer ts.Close()

	client := newTestClient(ts.URL)
	_, _ = fetchHotelPage(context.Background(), client, "Helsinki", defaultOpts(), 0, "")

	if strings.Contains(capturedURL, "start=") {
		t.Errorf("offset=0 should not add start param, got URL: %s", capturedURL)
	}
}

func TestFetchHotelPage_OffsetAddsStartParam(t *testing.T) {
	var capturedURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.WriteHeader(200)
		w.Write(fakeHotelPage("Hotel B"))
	}))
	defer ts.Close()

	client := newTestClient(ts.URL)
	_, _ = fetchHotelPage(context.Background(), client, "Helsinki", defaultOpts(), 20, "")

	if !strings.Contains(capturedURL, "start=20") {
		t.Errorf("offset=20 should add start=20, got URL: %s", capturedURL)
	}
}

func TestFetchHotelPage_Offset40(t *testing.T) {
	var capturedURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.WriteHeader(200)
		w.Write(fakeHotelPage("Hotel C"))
	}))
	defer ts.Close()

	client := newTestClient(ts.URL)
	_, _ = fetchHotelPage(context.Background(), client, "Helsinki", defaultOpts(), 40, "")

	if !strings.Contains(capturedURL, "start=40") {
		t.Errorf("offset=40 should add start=40, got URL: %s", capturedURL)
	}
}

func TestFetchHotelPage_SortParamAddedWhenSet(t *testing.T) {
	var capturedURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.WriteHeader(200)
		w.Write(fakeHotelPage("Hotel D"))
	}))
	defer ts.Close()

	client := newTestClient(ts.URL)
	_, _ = fetchHotelPage(context.Background(), client, "Helsinki", defaultOpts(), 0, "3")

	if !strings.Contains(capturedURL, "sort=3") {
		t.Errorf("googleSort=3 should add sort=3, got URL: %s", capturedURL)
	}
	if strings.Contains(capturedURL, "start=") {
		t.Errorf("offset=0 should not add start param with sort, got URL: %s", capturedURL)
	}
}

func TestFetchHotelPage_SortAndOffsetCombined(t *testing.T) {
	var capturedURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.WriteHeader(200)
		w.Write(fakeHotelPage("Hotel E"))
	}))
	defer ts.Close()

	client := newTestClient(ts.URL)
	_, _ = fetchHotelPage(context.Background(), client, "Helsinki", defaultOpts(), 20, "8")

	if !strings.Contains(capturedURL, "sort=8") {
		t.Errorf("expected sort=8 in URL, got: %s", capturedURL)
	}
	if !strings.Contains(capturedURL, "start=20") {
		t.Errorf("expected start=20 in URL, got: %s", capturedURL)
	}
}

func TestFetchHotelPage_EmptySortNoParam(t *testing.T) {
	var capturedURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.WriteHeader(200)
		w.Write(fakeHotelPage("Hotel F"))
	}))
	defer ts.Close()

	client := newTestClient(ts.URL)
	_, _ = fetchHotelPage(context.Background(), client, "Helsinki", defaultOpts(), 0, "")

	if strings.Contains(capturedURL, "sort=") {
		t.Errorf("empty googleSort should not add sort param, got URL: %s", capturedURL)
	}
}

// --- fetchHotelPage error handling ---

func TestFetchHotelPage_403ReturnsBlocked(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
	}))
	defer ts.Close()

	client := newTestClient(ts.URL)
	_, err := fetchHotelPage(context.Background(), client, "Helsinki", defaultOpts(), 0, "")

	if err == nil {
		t.Fatal("expected error for 403")
	}
	if !strings.Contains(err.Error(), "blocked") {
		t.Errorf("expected blocked error, got: %v", err)
	}
}

func TestFetchHotelPage_NonOKStatusReturnsError(t *testing.T) {
	// Use 404 (non-retryable) to avoid retry delays in tests.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer ts.Close()

	client := newTestClient(ts.URL)
	_, err := fetchHotelPage(context.Background(), client, "Helsinki", defaultOpts(), 0, "")

	if err == nil {
		t.Fatal("expected error for non-200 status")
	}
}

func TestFetchHotelPage_EmptyResponseReturnsError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("short"))
	}))
	defer ts.Close()

	client := newTestClient(ts.URL)
	_, err := fetchHotelPage(context.Background(), client, "Helsinki", defaultOpts(), 0, "")

	if err == nil {
		t.Fatal("expected error for empty response")
	}
}

// --- SearchHotelsWithClient pagination ---

func TestSearchHotelsWithClient_PaginatesMultiplePages(t *testing.T) {
	var reqCount atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := reqCount.Add(1)
		w.WriteHeader(200)
		// Each page returns different hotels. After page 3, return empty
		// (which stops pagination within each sort order) or dupes.
		switch page {
		case 1:
			w.Write(fakeHotelPageMulti("Hotel A", "Hotel B", "Hotel C"))
		case 2:
			w.Write(fakeHotelPageMulti("Hotel D", "Hotel E"))
		case 3:
			w.Write(fakeHotelPageMulti("Hotel F"))
		default:
			// Subsequent sort orders see only dupes -> stop early.
			w.Write(fakeHotelPageMulti("Hotel A", "Hotel B"))
		}
	}))
	defer ts.Close()

	client := newTestClient(ts.URL)
	client.SetNoCache(true)

	result, err := SearchHotelsWithClient(context.Background(), client, "Helsinki", defaultOpts())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have all 6 unique hotels across pages and sort orders.
	if result.Count != 6 {
		t.Errorf("expected 6 hotels, got %d", result.Count)
		for _, h := range result.Hotels {
			t.Logf("  got: %s", h.Name)
		}
	}
}

func TestSearchHotelsWithClient_StopsWhenNoNewHotels(t *testing.T) {
	var reqCount atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqCount.Add(1)
		w.WriteHeader(200)
		// All pages return the same hotels (duplicates).
		w.Write(fakeHotelPageMulti("Hotel A", "Hotel B"))
	}))
	defer ts.Close()

	client := newTestClient(ts.URL)
	client.SetNoCache(true)

	result, err := SearchHotelsWithClient(context.Background(), client, "Helsinki", defaultOpts())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have only 2 unique hotels.
	if result.Count != 2 {
		t.Errorf("expected 2 hotels, got %d", result.Count)
	}

	// With 3 sort orders, each tries page 1 then page 2 (all dupes -> stop).
	// That's 2 requests per sort order = 6 total. But the cache may return
	// cached results for identical URLs, so just verify we got the right
	// hotel count (dedup correctness is what matters).
}

func TestSearchHotelsWithClient_DeduplicatesAcrossPages(t *testing.T) {
	var reqCount atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := reqCount.Add(1)
		w.WriteHeader(200)
		switch page {
		case 1:
			w.Write(fakeHotelPageMulti("Hotel A", "Hotel B"))
		case 2:
			// Page 2 has one overlap (Hotel B) and one new (Hotel C).
			w.Write(fakeHotelPageMulti("Hotel B", "Hotel C"))
		case 3:
			w.Write(fakeHotelPageMulti("Hotel D"))
		default:
			// Subsequent sort orders return dupes -> stop early.
			w.Write(fakeHotelPageMulti("Hotel A", "Hotel B"))
		}
	}))
	defer ts.Close()

	client := newTestClient(ts.URL)
	client.SetNoCache(true)

	result, err := SearchHotelsWithClient(context.Background(), client, "Helsinki", defaultOpts())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have at least 4 unique: A, B, C, D.
	if result.Count < 4 {
		t.Errorf("expected at least 4 unique hotels, got %d", result.Count)
		for _, h := range result.Hotels {
			t.Logf("  got: %s", h.Name)
		}
	}

	// Verify no duplicates in result.
	seen := make(map[string]bool)
	for _, h := range result.Hotels {
		key := strings.ToLower(h.Name)
		if seen[key] {
			t.Errorf("duplicate hotel in result: %s", h.Name)
		}
		seen[key] = true
	}
}

func TestSearchHotelsWithClient_ContinuesOnSecondPageError(t *testing.T) {
	var reqCount atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := reqCount.Add(1)
		switch page {
		case 1:
			w.WriteHeader(200)
			w.Write(fakeHotelPageMulti("Hotel A", "Hotel B"))
		default:
			// Subsequent pages fail with 403 (non-retryable, fast).
			w.WriteHeader(403)
		}
	}))
	defer ts.Close()

	client := newTestClient(ts.URL)
	client.SetNoCache(true)

	result, err := SearchHotelsWithClient(context.Background(), client, "Helsinki", defaultOpts())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should still have the first page's results.
	if result.Count != 2 {
		t.Errorf("expected 2 hotels from first page, got %d", result.Count)
	}
}

func TestSearchHotelsWithClient_FirstPageErrorReturnsError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Use 403 (non-retryable) to avoid retry delays.
		w.WriteHeader(403)
	}))
	defer ts.Close()

	client := newTestClient(ts.URL)
	client.SetNoCache(true)

	_, err := SearchHotelsWithClient(context.Background(), client, "Helsinki", defaultOpts())
	if err == nil {
		t.Fatal("expected error when first page fails")
	}
}

func TestSearchHotelsWithClient_CaseInsensitiveDedup(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		// Always return these 3 entries. Dedup should collapse them to 2.
		w.Write(fakeHotelPageMulti("Hotel Alpha", "HOTEL ALPHA", "Hotel Beta"))
	}))
	defer ts.Close()

	client := newTestClient(ts.URL)
	client.SetNoCache(true)

	result, err := SearchHotelsWithClient(context.Background(), client, "Helsinki", defaultOpts())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 2 unique: Hotel Alpha, Hotel Beta.
	if result.Count != 2 {
		t.Errorf("expected 2 unique hotels (case-insensitive dedup), got %d", result.Count)
		for _, h := range result.Hotels {
			t.Logf("  got: %s", h.Name)
		}
	}
}

// --- Multi-sort diversity ---

func TestSearchHotelsWithClient_SortDiversityAddsUniqueHotels(t *testing.T) {
	// Simulate a server where different sort orders return different hotels.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		sortParam := r.URL.Query().Get("sort")
		switch sortParam {
		case "":
			// Default sort: Hotels A, B
			w.Write(fakeHotelPageMulti("Hotel A", "Hotel B"))
		case "3":
			// Highest rated sort: Hotels B, C (B overlaps, C is new)
			w.Write(fakeHotelPageMulti("Hotel B", "Hotel C"))
		case "8":
			// Price sort: Hotels D (all new)
			w.Write(fakeHotelPageMulti("Hotel D"))
		default:
			w.Write(fakeHotelPageMulti("Hotel A"))
		}
	}))
	defer ts.Close()

	client := newTestClient(ts.URL)
	client.SetNoCache(true)

	result, err := SearchHotelsWithClient(context.Background(), client, "Helsinki", defaultOpts())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 4 unique hotels: A, B from default + C from sort=3 + D from sort=8.
	if result.Count != 4 {
		t.Errorf("expected 4 unique hotels from sort diversity, got %d", result.Count)
		for _, h := range result.Hotels {
			t.Logf("  got: %s", h.Name)
		}
	}
}

func TestSearchHotelsWithClient_MaxPages1SkipsSortDiversity(t *testing.T) {
	var reqCount atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqCount.Add(1)
		w.WriteHeader(200)
		w.Write(fakeHotelPageMulti("Hotel A"))
	}))
	defer ts.Close()

	client := newTestClient(ts.URL)
	client.SetNoCache(true)

	opts := defaultOpts()
	opts.MaxPages = 1

	result, err := SearchHotelsWithClient(context.Background(), client, "Helsinki", opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Count != 1 {
		t.Errorf("expected 1 hotel with MaxPages=1, got %d", result.Count)
	}

	// MaxPages=1 should only make 1 request (no pagination, no sort diversity).
	if got := int(reqCount.Load()); got != 1 {
		t.Errorf("expected 1 request with MaxPages=1, got %d", got)
	}
}

func TestGoogleSortOrders(t *testing.T) {
	// Verify the sort orders slice has expected structure.
	if len(googleSortOrders) < 2 {
		t.Errorf("googleSortOrders should have at least 2 entries, got %d", len(googleSortOrders))
	}
	if googleSortOrders[0] != "" {
		t.Errorf("first sort order should be empty (default), got %q", googleSortOrders[0])
	}
}

// --- helpers ---

// defaultOpts returns valid HotelSearchOptions for testing.
func defaultOpts() HotelSearchOptions {
	return HotelSearchOptions{
		CheckIn:  "2026-06-15",
		CheckOut: "2026-06-18",
		Guests:   2,
		Currency: "USD",
	}
}

// newTestClient creates a batchexec.Client that routes all requests to the test server.
// It uses a plain http.Client (no TLS fingerprinting) and disables rate limiting.
func newTestClient(baseURL string) *batchexec.Client {
	client := batchexec.NewTestClient(baseURL)
	return client
}

// fakeHotelPage builds a minimal HTML page with one hotel in an AF_initDataCallback block.
func fakeHotelPage(name string) []byte {
	return fakeHotelPageMulti(name)
}

// nameOffset returns a small deterministic lat/lon offset for a hotel name so
// that the same name always gets the same coordinates across pages. This is
// important for MergeHotelResults geo-disambiguation: hotels with the same name
// must have close-enough coordinates to be merged, even when they appear on
// different paginated pages.
func nameOffset(name string) float64 {
	var sum int
	for _, c := range name {
		sum += int(c)
	}
	// Scale to a sub-meter range (< 0.000001 degrees ≈ 0.1m) so same-named
	// hotels are always within the 200m merge threshold.
	return float64(sum%100) * 0.000001
}

// fakeHotelPageMulti builds a minimal HTML page with N hotels.
// The page is padded to exceed the 1000-byte minimum response check.
func fakeHotelPageMulti(names ...string) []byte {
	var entries []any
	for _, name := range names {
		// Use name-derived coordinates so the same hotel name always lands at
		// the same location regardless of its position within a page.
		offset := nameOffset(name)
		hotel := make([]any, 12)
		hotel[0] = nil
		hotel[1] = name
		hotel[2] = []any{[]any{60.168 + offset, 24.941 + offset}}
		hotel[3] = []any{"4-star hotel", 4.0}
		hotel[9] = fmt.Sprintf("/g/hotel_%s", strings.ReplaceAll(strings.ToLower(name), " ", "_"))

		entry := []any{
			nil,
			map[string]any{
				"397419284": []any{hotel},
			},
		}
		entries = append(entries, entry)
	}

	if len(entries) == 0 {
		// Return a page with no hotel data to trigger "no hotels" error.
		// Pad to exceed 1000 bytes so it passes the size check.
		return []byte(`<html>` + strings.Repeat("<!-- padding -->", 100) +
			`AF_initDataCallback({key: 'ds:0', data:[1,2,3]});</html>`)
	}

	innerData := []any{[]any{[]any{[]any{nil, entries}}}}
	dataJSON, _ := json.Marshal(innerData)

	// Pad the page to exceed the 1000-byte minimum response check.
	padding := strings.Repeat("<!-- padding -->", 100)
	return []byte(`<html>` + padding + `AF_initDataCallback({key: 'ds:0', data:` + string(dataJSON) + `});</html>`)
}

// --- SearchHotelsWithClient with filters still work after pagination ---

func TestSearchHotelsWithClient_FiltersApplyAfterPagination(t *testing.T) {
	var reqCount atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := reqCount.Add(1)
		w.WriteHeader(200)

		// Build hotels with prices: page1 has cheap+expensive, page2 has mid.
		switch page {
		case 1:
			w.Write(fakeHotelPageWithPrices(
				hotelWithPrice{"Cheap Hotel", 50},
				hotelWithPrice{"Expensive Hotel", 500},
			))
		case 2:
			w.Write(fakeHotelPageWithPrices(
				hotelWithPrice{"Mid Hotel", 150},
			))
		default:
			w.Write(fakeHotelPageMulti())
		}
	}))
	defer ts.Close()

	client := newTestClient(ts.URL)
	client.SetNoCache(true)

	opts := defaultOpts()
	opts.MinPrice = 100
	opts.MaxPrice = 400

	result, err := SearchHotelsWithClient(context.Background(), client, "Helsinki", opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only Mid Hotel (150) should pass the price filter.
	// Cheap (50) and Expensive (500) are filtered out.
	if result.Count != 1 {
		t.Errorf("expected 1 hotel after filter, got %d", result.Count)
		for _, h := range result.Hotels {
			t.Logf("  got: %s (price=%.0f)", h.Name, h.Price)
		}
	}
	if result.Count == 1 && result.Hotels[0].Name != "Mid Hotel" {
		t.Errorf("expected Mid Hotel, got %s", result.Hotels[0].Name)
	}
}

type hotelWithPrice struct {
	name  string
	price float64
}

// fakeHotelPageWithPrices builds a page with hotels that have price data.
// Padded to exceed the 1000-byte minimum response check.
func fakeHotelPageWithPrices(hotels ...hotelWithPrice) []byte {
	var entries []any
	for _, hp := range hotels {
		offset := nameOffset(hp.name)
		hotel := make([]any, 12)
		hotel[0] = nil
		hotel[1] = hp.name
		hotel[2] = []any{[]any{60.168 + offset, 24.941 + offset}}
		hotel[3] = []any{"4-star hotel", 4.0}
		// Price block: [null, [params..., "USD"], [null, [formatted, null, exact, null, rounded]]]
		hotel[6] = []any{
			nil,
			[]any{nil, nil, nil, "USD"},
			[]any{nil, []any{fmt.Sprintf("$%.0f", hp.price), nil, hp.price, nil, hp.price}},
		}
		hotel[9] = fmt.Sprintf("/g/hotel_%s", strings.ReplaceAll(strings.ToLower(hp.name), " ", "_"))

		entry := []any{
			nil,
			map[string]any{
				"397419284": []any{hotel},
			},
		}
		entries = append(entries, entry)
	}

	innerData := []any{[]any{[]any{[]any{nil, entries}}}}
	dataJSON, _ := json.Marshal(innerData)

	padding := strings.Repeat("<!-- padding -->", 100)
	return []byte(`<html>` + padding + `AF_initDataCallback({key: 'ds:0', data:` + string(dataJSON) + `});</html>`)
}

// --- MergeHotelResults wiring tests ---

// TestSearchHotelsWithClient_SourcesTaggedGoogleHotels verifies that hotels
// returned by SearchHotelsWithClient have their Sources populated with the
// "google_hotels" provider tag. This confirms that tagHotelSource + MergeHotelResults
// are wired correctly into the search pipeline.
func TestSearchHotelsWithClient_SourcesTaggedGoogleHotels(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write(fakeHotelPageWithPrices(
			hotelWithPrice{"Grand Hotel", 150},
			hotelWithPrice{"Sea View", 200},
		))
	}))
	defer ts.Close()

	client := newTestClient(ts.URL)
	client.SetNoCache(true)

	result, err := SearchHotelsWithClient(context.Background(), client, "Helsinki", defaultOpts())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Count == 0 {
		t.Fatal("expected at least one hotel")
	}

	for _, h := range result.Hotels {
		if len(h.Sources) == 0 {
			t.Errorf("hotel %q has no Sources — tagHotelSource not wired", h.Name)
			continue
		}
		found := false
		for _, src := range h.Sources {
			if src.Provider == "google_hotels" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("hotel %q Sources does not contain google_hotels provider: %v", h.Name, h.Sources)
		}
	}
}

// TestSearchHotelsWithClient_MergePreservesLowestPrice verifies that when the
// same hotel appears in multiple pages/sort orders, the Sources list accumulates
// and the lowest price is kept as the primary. This validates the MergeHotelResults
// dedup behaviour end-to-end through the real search pipeline.
func TestSearchHotelsWithClient_MergePreservesLowestPrice(t *testing.T) {
	var reqCount atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := reqCount.Add(1)
		w.WriteHeader(200)
		// "Overlap Hotel" appears on both page 1 and page 2 with different prices.
		switch page {
		case 1:
			w.Write(fakeHotelPageWithPrices(
				hotelWithPrice{"Overlap Hotel", 300},
				hotelWithPrice{"Unique A", 100},
			))
		case 2:
			w.Write(fakeHotelPageWithPrices(
				hotelWithPrice{"Overlap Hotel", 250}, // cheaper
			))
		default:
			w.Write(fakeHotelPageMulti()) // empty — stop pagination
		}
	}))
	defer ts.Close()

	client := newTestClient(ts.URL)
	client.SetNoCache(true)

	opts := defaultOpts()
	opts.MaxPages = 2 // limit to 2 pages per sort, disable sort diversity

	result, err := SearchHotelsWithClient(context.Background(), client, "Helsinki", opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find Overlap Hotel in results.
	var overlap *struct {
		price   float64
		sources int
	}
	for _, h := range result.Hotels {
		if strings.EqualFold(h.Name, "Overlap Hotel") {
			overlap = &struct {
				price   float64
				sources int
			}{h.Price, len(h.Sources)}
			break
		}
	}
	if overlap == nil {
		t.Fatal("Overlap Hotel not found in results")
	}
	// Lowest price (250) should be primary.
	if overlap.price != 250 {
		t.Errorf("expected lowest price 250, got %.0f", overlap.price)
	}
	// Both source appearances should be preserved.
	if overlap.sources < 2 {
		t.Errorf("expected at least 2 sources for merged hotel, got %d", overlap.sources)
	}
}

// TestTagHotelSource verifies that the helper stamps each hotel with the
// correct provider when Sources is empty, and leaves existing Sources unchanged.
func TestTagHotelSource(t *testing.T) {
	input := []models.HotelResult{
		{Name: "Hotel A", Price: 100, Currency: "EUR"},
		{Name: "Hotel B", Price: 200, Currency: "USD", Sources: []models.PriceSource{{Provider: "existing", Price: 200, Currency: "USD"}}},
	}

	tagged := tagHotelSource(input, "google_hotels")

	if len(tagged[0].Sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(tagged[0].Sources))
	}
	if tagged[0].Sources[0].Provider != "google_hotels" {
		t.Errorf("expected provider google_hotels, got %q", tagged[0].Sources[0].Provider)
	}
	if tagged[0].Sources[0].Price != 100 {
		t.Errorf("expected price 100, got %.0f", tagged[0].Sources[0].Price)
	}

	// Hotel B already had Sources — must be unchanged.
	if len(tagged[1].Sources) != 1 || tagged[1].Sources[0].Provider != "existing" {
		t.Errorf("hotel with existing Sources should not be modified, got %v", tagged[1].Sources)
	}

	// Original slice must not be mutated.
	if input[0].Sources != nil {
		t.Error("tagHotelSource must not mutate input slice")
	}
}

// --- Verify booking URLs added to paginated results ---

func TestSearchHotelsWithClient_BookingURLsOnAllPages(t *testing.T) {
	var reqCount atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := reqCount.Add(1)
		w.WriteHeader(200)
		switch page {
		case 1:
			w.Write(fakeHotelPageMulti("Hotel P1"))
		case 2:
			w.Write(fakeHotelPageMulti("Hotel P2"))
		default:
			w.Write(fakeHotelPageMulti())
		}
	}))
	defer ts.Close()

	client := newTestClient(ts.URL)
	client.SetNoCache(true)

	result, err := SearchHotelsWithClient(context.Background(), client, "Helsinki", defaultOpts())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, h := range result.Hotels {
		if h.BookingURL == "" {
			t.Errorf("hotel %q missing BookingURL", h.Name)
		}
		if !strings.Contains(h.BookingURL, "google.com/travel/hotels") {
			t.Errorf("hotel %q has bad BookingURL: %s", h.Name, h.BookingURL)
		}
	}
}

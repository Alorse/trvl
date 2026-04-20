package deals

import (
	"context"
	"strings"
	"testing"
	"time"
)

// ============================================================
// MatchDeals — all branches (was 0% coverage)
// ============================================================

func TestMatchDeals_CachePopulatedAndMatches(t *testing.T) {
	now := time.Now()
	testDeals := []Deal{
		{Title: "HEL to BCN $99", Origin: "Helsinki", Destination: "Barcelona", Published: now},
		{Title: "HEL to NRT $499", Origin: "Helsinki", Destination: "Tokyo", Published: now},
		{Title: "AMS to BCN $79", Origin: "Amsterdam", Destination: "Barcelona", Published: now},
		{Title: "No origin", Origin: "", Destination: "Barcelona", Published: now},
		{Title: "No dest", Origin: "Helsinki", Destination: "", Published: now},
	}

	dealCache.Lock()
	dealCache.deals = testDeals
	dealCache.fetchedAt = time.Now()
	dealCache.Unlock()
	defer func() {
		dealCache.Lock()
		dealCache.deals = nil
		dealCache.fetchedAt = time.Time{}
		dealCache.Unlock()
	}()

	t.Run("exact match", func(t *testing.T) {
		matches := MatchDeals(context.Background(), "Helsinki", "Barcelona")
		if len(matches) != 1 {
			t.Errorf("expected 1 match, got %d", len(matches))
		}
	})

	t.Run("case insensitive", func(t *testing.T) {
		matches := MatchDeals(context.Background(), "helsinki", "barcelona")
		if len(matches) != 1 {
			t.Errorf("expected 1 match, got %d", len(matches))
		}
	})

	t.Run("substring match origin contains filter", func(t *testing.T) {
		matches := MatchDeals(context.Background(), "Hel", "Tokyo")
		if len(matches) != 1 {
			t.Errorf("expected 1 match for substring, got %d", len(matches))
		}
	})

	t.Run("no match", func(t *testing.T) {
		matches := MatchDeals(context.Background(), "Paris", "London")
		if len(matches) != 0 {
			t.Errorf("expected 0 matches, got %d", len(matches))
		}
	})

	t.Run("empty origin matches all deals with both fields set", func(t *testing.T) {
		// Empty origin: strings.Contains(x, "") is always true, so all deals
		// with both fields set and matching destination will match.
		// 3 deals have both origin+dest; 2 of those have Barcelona as dest.
		matches := MatchDeals(context.Background(), "", "Barcelona")
		if len(matches) != 2 {
			t.Errorf("expected 2 matches (empty origin matches everything), got %d", len(matches))
		}
	})

	t.Run("whitespace trimmed", func(t *testing.T) {
		matches := MatchDeals(context.Background(), "  Helsinki  ", "  Barcelona  ")
		if len(matches) != 1 {
			t.Errorf("expected 1 match with whitespace, got %d", len(matches))
		}
	})

	t.Run("destination substring", func(t *testing.T) {
		matches := MatchDeals(context.Background(), "Amsterdam", "Bar")
		if len(matches) != 1 {
			t.Errorf("expected 1 match for dest substring, got %d", len(matches))
		}
	})
}

func TestMatchDeals_EmptyCache(t *testing.T) {
	dealCache.Lock()
	dealCache.deals = nil
	dealCache.fetchedAt = time.Time{}
	dealCache.Unlock()

	// getCachedDeals with a cancelled context returns nil immediately.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	matches := MatchDeals(ctx, "Helsinki", "Barcelona")
	if matches != nil {
		t.Errorf("expected nil for empty cache + cancelled ctx, got %d matches", len(matches))
	}
}

// ============================================================
// getCachedDeals — cache hit path (was 0% coverage)
// ============================================================

func TestGetCachedDeals_CacheHit(t *testing.T) {
	testDeals := []Deal{
		{Title: "Test deal", Origin: "HEL", Destination: "BCN", Published: time.Now()},
	}

	dealCache.Lock()
	dealCache.deals = testDeals
	dealCache.fetchedAt = time.Now()
	dealCache.Unlock()
	defer func() {
		dealCache.Lock()
		dealCache.deals = nil
		dealCache.fetchedAt = time.Time{}
		dealCache.Unlock()
	}()

	deals := getCachedDeals(context.Background())
	if len(deals) != 1 {
		t.Errorf("expected 1 deal from cache, got %d", len(deals))
	}
	if deals[0].Title != "Test deal" {
		t.Errorf("deal title = %q, want 'Test deal'", deals[0].Title)
	}
}

func TestGetCachedDeals_StaleCache_FailedRefresh(t *testing.T) {
	dealCache.Lock()
	dealCache.deals = []Deal{{Title: "Stale"}}
	dealCache.fetchedAt = time.Now().Add(-31 * time.Minute)
	dealCache.Unlock()
	defer func() {
		dealCache.Lock()
		dealCache.deals = nil
		dealCache.fetchedAt = time.Time{}
		dealCache.Unlock()
	}()

	// Cancelled context: FetchDeals inside getCachedDeals fails immediately.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	deals := getCachedDeals(ctx)
	if deals != nil {
		t.Errorf("expected nil for stale cache + cancelled ctx, got %d", len(deals))
	}
}

// ============================================================
// normalizeStops — default branch (83.3% to 100%)
// ============================================================

func TestNormalizeStops_AllBranches(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"nonstop", "nonstop"},
		{"non-stop", "nonstop"},
		{"direct", "nonstop"},
		{"1 stop", "1 stop"},
		{"2 stops", "2 stops"},
		{"3 stops", "3 stops"},
	}
	for _, tt := range tests {
		got := normalizeStops(tt.input)
		if got != tt.want {
			t.Errorf("normalizeStops(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// ============================================================
// extractCityFromRoute — no-comma branch (75% to 100%)
// ============================================================

func TestExtractCityFromRoute_AllBranches(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Burbank, USA", "Burbank"},
		{"New York, USA", "New York"},
		{"Tokyo", "Tokyo"},
		{"  Paris  ", "Paris"},
		{"", ""},
	}
	for _, tt := range tests {
		got := extractCityFromRoute(tt.input)
		if got != tt.want {
			t.Errorf("extractCityFromRoute(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// ============================================================
// ParseRSS — long summary truncation (93.8% to higher)
// ============================================================

func TestParseRSS_LongSummaryTruncation(t *testing.T) {
	longDesc := strings.Repeat("word ", 100)
	rss := `<?xml version="1.0"?><rss version="2.0"><channel>
	<item><title>Test $99</title><link>https://test.local/deal</link>
	<pubDate>Thu, 03 Apr 2026 10:00:00 +0000</pubDate>
	<description>` + longDesc + `</description></item>
	</channel></rss>`

	deals, err := ParseRSS([]byte(rss), "test")
	if err != nil {
		t.Fatalf("ParseRSS error: %v", err)
	}
	if len(deals) != 1 {
		t.Fatalf("expected 1 deal, got %d", len(deals))
	}
	if !strings.HasSuffix(deals[0].Summary, "...") {
		t.Error("expected truncated summary ending with ...")
	}
	if len(deals[0].Summary) > 204 {
		t.Errorf("summary too long: %d chars", len(deals[0].Summary))
	}
}

// ============================================================
// originMatchesDeal — reverse IATA lookup branch (92.9% to 100%)
// ============================================================

func TestOriginMatchesDeal_ReverseIATAResolve(t *testing.T) {
	got := originMatchesDeal("CDG", []string{"paris"})
	if !got {
		t.Error("expected CDG to match 'paris' via reverse alias")
	}
}

func TestOriginMatchesDeal_MultipleFilters(t *testing.T) {
	got := originMatchesDeal("Helsinki", []string{"Paris", "HEL"})
	if !got {
		t.Error("expected Helsinki to match 'HEL' alias in filter list")
	}
}

// ============================================================
// extractFromCategories — mistake fare and empty category branches
// ============================================================

func TestExtractFromCategories_MistakeFare(t *testing.T) {
	d := Deal{Title: "Flights to Berlin"}
	extractFromCategories(&d, []string{"Mistake Fare"})
	if d.Type != "error_fare" {
		t.Errorf("type = %q, want error_fare for 'Mistake Fare' category", d.Type)
	}
}

func TestExtractFromCategories_EmptyCategory(t *testing.T) {
	d := Deal{Title: "Flights"}
	extractFromCategories(&d, []string{"", "  ", "Nonstop"})
	if d.Stops != "nonstop" {
		t.Errorf("stops = %q, want nonstop", d.Stops)
	}
}

func TestExtractFromCategories_DealCategoryNoOverride(t *testing.T) {
	d := Deal{Title: "Flights", Type: "error_fare"}
	extractFromCategories(&d, []string{"deal"})
	if d.Type != "error_fare" {
		t.Errorf("type = %q, should still be error_fare", d.Type)
	}
}

func TestExtractFromCategories_FlightDealCategoryNoType(t *testing.T) {
	d := Deal{Title: "Flights"}
	extractFromCategories(&d, []string{"Flight Deal"})
	// "flight deal" is in the "don't override" branch, so Type stays empty.
	if d.Type != "" {
		t.Errorf("type = %q, should be empty (flight deal doesnt set type)", d.Type)
	}
}

// ============================================================
// classifyDeal — does not overwrite existing type
// ============================================================

func TestClassifyDeal_DoesNotOverwriteSpecific(t *testing.T) {
	d := Deal{Title: "Cheap flights to Rome", Type: "error_fare"}
	classifyDeal(&d)
	if d.Type != "error_fare" {
		t.Errorf("type = %q, should keep error_fare", d.Type)
	}
}

func TestClassifyDeal_OverwritesDealWithSpecific(t *testing.T) {
	// If type was "deal" (generic), a more specific title type should replace it.
	d := Deal{Title: "Flash sale to Barcelona", Type: "deal"}
	classifyDeal(&d)
	if d.Type != "flash_sale" {
		t.Errorf("type = %q, want flash_sale (should overwrite generic deal)", d.Type)
	}
}

// ============================================================
// isLikelyCity — additional coverage
// ============================================================

func TestIsLikelyCity_NotCities(t *testing.T) {
	notCities := []string{"flights", "stop", "nonstop", "cheap", "return",
		"trip", "way", "fare", "deal", "sale", "error", "mistake",
		"flash", "direct", "travel", "book", "holiday", "package",
		"airline", "airport"}
	for _, word := range notCities {
		if isLikelyCity(word) {
			t.Errorf("isLikelyCity(%q) = true, want false", word)
		}
	}
}

func TestIsLikelyCity_ActualCities(t *testing.T) {
	cities := []string{"Helsinki", "Barcelona", "Tokyo", "Prague", "London"}
	for _, city := range cities {
		if !isLikelyCity(city) {
			t.Errorf("isLikelyCity(%q) = false, want true", city)
		}
	}
}

// ============================================================
// normalizeCabin — all branches (already 100% but validates)
// ============================================================

func TestNormalizeCabin_AllBranches(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"first class", "first"},
		{"First", "first"},
		{"business class", "business"},
		{"Business", "business"},
		{"premium economy", "premium_economy"},
		{"Premium Economy", "premium_economy"},
		{"economy", "economy"},
		{"Economy", "economy"},
		{"something else", "economy"},
	}
	for _, tt := range tests {
		got := normalizeCabin(tt.input)
		if got != tt.want {
			t.Errorf("normalizeCabin(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// ============================================================
// Price extraction — additional currency edge cases
// ============================================================

func TestExtractPrice_NZDollar(t *testing.T) {
	d := Deal{Title: "NZ$350 flights to Auckland"}
	extractPriceAndRoute(&d)
	if d.Price != 350 {
		t.Errorf("price = %.2f, want 350", d.Price)
	}
	// NZ$ falls through to USD since NZ is not in the prefix map.
	if d.Currency != "USD" {
		t.Errorf("currency = %q, want USD (NZ$ resolves to USD)", d.Currency)
	}
}

func TestExtractPrice_SEK(t *testing.T) {
	d := Deal{Title: "SEK 1299 flights to Oslo"}
	extractPriceAndRoute(&d)
	if d.Price != 1299 {
		t.Errorf("price = %.2f, want 1299", d.Price)
	}
	if d.Currency != "SEK" {
		t.Errorf("currency = %q, want SEK", d.Currency)
	}
}

func TestExtractPrice_AmountFirst(t *testing.T) {
	d := Deal{Title: "1299 SEK round trip flights"}
	extractPriceAndRoute(&d)
	if d.Price != 1299 {
		t.Errorf("price = %.2f, want 1299", d.Price)
	}
	if d.Currency != "SEK" {
		t.Errorf("currency = %q, want SEK", d.Currency)
	}
}

// ============================================================
// Route extraction — edge cases
// ============================================================

func TestExtractRoute_CityToCity_FalsePositive(t *testing.T) {
	// Words like "Flights" and "Deal" should not be extracted as cities.
	d := Deal{Title: "Flights to Barcelona from $99"}
	extractPriceAndRoute(&d)
	// "Flights" is not a likely city, so the "to" pattern should skip it.
	if d.Origin == "Flights" {
		t.Error("should not extract 'Flights' as origin")
	}
}

func TestExtractRoute_TwoWordCity(t *testing.T) {
	d := Deal{Title: "From New York to Barcelona for $199"}
	extractPriceAndRoute(&d)
	if d.Origin != "New York" {
		t.Errorf("origin = %q, want 'New York'", d.Origin)
	}
	if d.Destination != "Barcelona" {
		t.Errorf("destination = %q, want 'Barcelona'", d.Destination)
	}
}

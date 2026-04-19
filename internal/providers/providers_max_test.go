package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"

	"github.com/MikkoParkkola/trvl/internal/models"
)

// ===========================================================================
// decompressBody — Brotli encoding
// ===========================================================================

func TestDecompressBody_BrotliMax(t *testing.T) {
	// Exercises brotli with a larger payload to cover more of the reader path.
	original := strings.Repeat(`{"name":"Hotel","stars":5},`, 100)

	var buf bytes.Buffer
	bw := brotli.NewWriter(&buf)
	_, _ = bw.Write([]byte(original))
	bw.Close()

	resp := &http.Response{
		Header: http.Header{"Content-Encoding": {"br"}},
		Body:   io.NopCloser(&buf),
	}

	got, err := decompressBody(resp, int64(len(original)+1024))
	if err != nil {
		t.Fatalf("decompressBody(br): %v", err)
	}
	if string(got) != original {
		t.Errorf("got len=%d, want len=%d", len(got), len(original))
	}
}

// ===========================================================================
// decompressBody — Zstd encoding
// ===========================================================================

func TestDecompressBody_ZstdEncoding(t *testing.T) {
	original := `{"results": [{"name": "Zstd Hotel"}]}`

	var buf bytes.Buffer
	zw, err := zstd.NewWriter(&buf)
	if err != nil {
		t.Fatalf("zstd.NewWriter: %v", err)
	}
	_, _ = zw.Write([]byte(original))
	zw.Close()

	resp := &http.Response{
		Header: http.Header{
			"Content-Encoding": {"zstd"},
		},
		Body: io.NopCloser(&buf),
	}

	got, err := decompressBody(resp, 4096)
	if err != nil {
		t.Fatalf("decompressBody(zstd): %v", err)
	}
	if string(got) != original {
		t.Errorf("got %q, want %q", string(got), original)
	}
}

// ===========================================================================
// decompressBody — unknown encoding treated as identity
// ===========================================================================

func TestDecompressBody_UnknownEncoding(t *testing.T) {
	body := `plain text body`
	resp := &http.Response{
		Header: http.Header{
			"Content-Encoding": {"deflate"},
		},
		Body: io.NopCloser(strings.NewReader(body)),
	}

	got, err := decompressBody(resp, 4096)
	if err != nil {
		t.Fatalf("decompressBody(deflate): %v", err)
	}
	if string(got) != body {
		t.Errorf("got %q, want %q", string(got), body)
	}
}

// ===========================================================================
// decompressBody — limit enforcement
// ===========================================================================

func TestDecompressBody_LimitEnforced(t *testing.T) {
	body := strings.Repeat("x", 1000)
	resp := &http.Response{
		Header: http.Header{},
		Body:   io.NopCloser(strings.NewReader(body)),
	}

	got, err := decompressBody(resp, 100)
	if err != nil {
		t.Fatalf("decompressBody: %v", err)
	}
	if len(got) != 100 {
		t.Errorf("got %d bytes, want 100", len(got))
	}
}

// ===========================================================================
// saveCachedCookies + loadCachedCookies — round trip
// ===========================================================================

func TestSaveThenLoadCachedCookies_RoundTrip(t *testing.T) {
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}

	targetURL := "https://roundtrip-test.example.com/search"
	u, _ := url.Parse(targetURL)

	// Seed cookies into the jar.
	jar.SetCookies(u, []*http.Cookie{
		{Name: "session", Value: "abc123", Domain: ".example.com", Path: "/"},
		{Name: "csrf", Value: "tok456", Domain: ".example.com", Path: "/"},
	})

	// Save.
	saveCachedCookies(client, targetURL)
	t.Cleanup(func() {
		// Clean up the cache file.
		path, _ := cookieCachePath("roundtrip-test.example.com")
		os.Remove(path)
	})

	// Create a fresh client and load the cached cookies.
	jar2, _ := cookiejar.New(nil)
	client2 := &http.Client{Jar: jar2}

	loaded := loadCachedCookies(client2, targetURL)
	if !loaded {
		t.Fatal("expected loadCachedCookies to return true")
	}

	// Verify cookies are in the new jar.
	cookies := jar2.Cookies(u)
	if len(cookies) < 2 {
		t.Fatalf("expected at least 2 cookies, got %d", len(cookies))
	}

	names := map[string]string{}
	for _, c := range cookies {
		names[c.Name] = c.Value
	}
	if names["session"] != "abc123" {
		t.Errorf("session = %q, want 'abc123'", names["session"])
	}
	if names["csrf"] != "tok456" {
		t.Errorf("csrf = %q, want 'tok456'", names["csrf"])
	}
}

// ===========================================================================
// saveCachedCookies — edge cases
// ===========================================================================

func TestSaveCachedCookies_URLWithNoHost(t *testing.T) {
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	// URL parses but has no host — early return.
	saveCachedCookies(client, "/just-a-path")
}

func TestSaveCachedCookies_WritesAndLoads(t *testing.T) {
	// Verify that saveCachedCookies produces a file that loadCachedCookies can read.
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	targetURL := "https://save-writes-test.example.com/page"
	u, _ := url.Parse(targetURL)

	jar.SetCookies(u, []*http.Cookie{
		{Name: "tok", Value: "xyz", Domain: ".example.com", Path: "/"},
	})

	saveCachedCookies(client, targetURL)
	t.Cleanup(func() {
		path, _ := cookieCachePath("save-writes-test.example.com")
		os.Remove(path)
	})

	// Verify the file exists.
	path, _ := cookieCachePath("save-writes-test.example.com")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("cache file not created: %v", err)
	}

	// Load into fresh client.
	jar2, _ := cookiejar.New(nil)
	client2 := &http.Client{Jar: jar2}
	if !loadCachedCookies(client2, targetURL) {
		t.Fatal("loadCachedCookies returned false after save")
	}
}

func TestSaveCachedCookies_EmptyURL(t *testing.T) {
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	// Should not panic.
	saveCachedCookies(client, "")
}

func TestSaveCachedCookies_MalformedURL(t *testing.T) {
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	// Should not panic.
	saveCachedCookies(client, "::not-a-url::")
}

func TestSaveCachedCookies_NoCookies(t *testing.T) {
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	// No cookies in jar -> no-op, no panic.
	saveCachedCookies(client, "https://nocookies.example.com/page")
}

// ===========================================================================
// loadCachedCookies — edge cases
// ===========================================================================

func TestLoadCachedCookies_NoFile(t *testing.T) {
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	got := loadCachedCookies(client, "https://no-cache-file.example.com/page")
	if got {
		t.Error("expected false when no cache file exists")
	}
}

func TestLoadCachedCookies_Expired(t *testing.T) {
	// Write an expired cache file directly.
	targetURL := "https://expired-cache.example.com"
	path, err := cookieCachePath("expired-cache.example.com")
	if err != nil {
		t.Fatal(err)
	}

	expired := []cachedCookie{
		{
			Name:    "old",
			Value:   "stale",
			Domain:  ".example.com",
			Path:    "/",
			SavedAt: time.Now().Add(-48 * time.Hour), // 48h ago — beyond 24h TTL
		},
	}
	data, _ := json.Marshal(expired)
	os.MkdirAll(filepath.Dir(path), 0o700)
	os.WriteFile(path, data, 0o600)
	t.Cleanup(func() { os.Remove(path) })

	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	got := loadCachedCookies(client, targetURL)
	if got {
		t.Error("expected false for expired cache")
	}
}

func TestLoadCachedCookies_InvalidJSON(t *testing.T) {
	path, err := cookieCachePath("bad-json-cache.example.com")
	if err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Dir(path), 0o700)
	os.WriteFile(path, []byte(`{not valid json}`), 0o600)
	t.Cleanup(func() { os.Remove(path) })

	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	got := loadCachedCookies(client, "https://bad-json-cache.example.com")
	if got {
		t.Error("expected false for invalid JSON cache file")
	}
}

func TestLoadCachedCookies_EmptyArray(t *testing.T) {
	path, err := cookieCachePath("empty-array-cache.example.com")
	if err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Dir(path), 0o700)
	os.WriteFile(path, []byte(`[]`), 0o600)
	t.Cleanup(func() { os.Remove(path) })

	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	got := loadCachedCookies(client, "https://empty-array-cache.example.com")
	if got {
		t.Error("expected false for empty cookie array")
	}
}

func TestLoadCachedCookies_NilJarClient(t *testing.T) {
	// Write a valid cache file.
	path, err := cookieCachePath("nil-jar-cache.example.com")
	if err != nil {
		t.Fatal(err)
	}

	valid := []cachedCookie{
		{
			Name:    "ok",
			Value:   "fine",
			Domain:  ".example.com",
			Path:    "/",
			SavedAt: time.Now(),
		},
	}
	data, _ := json.Marshal(valid)
	os.MkdirAll(filepath.Dir(path), 0o700)
	os.WriteFile(path, data, 0o600)
	t.Cleanup(func() { os.Remove(path) })

	// Client with no jar -> can't set cookies.
	client := &http.Client{}
	got := loadCachedCookies(client, "https://nil-jar-cache.example.com")
	if got {
		t.Error("expected false when client has no jar")
	}
}

// ===========================================================================
// cookieCachePath — sanitization
// ===========================================================================

func TestCookieCachePath_SpecialChars(t *testing.T) {
	path, err := cookieCachePath("my:weird/host name")
	if err != nil {
		t.Fatal(err)
	}
	base := filepath.Base(path)
	// Colons, slashes, and spaces should be replaced with underscores.
	if strings.ContainsAny(base, ":/") {
		t.Errorf("path %q contains unsanitized characters", base)
	}
}

// ===========================================================================
// enrichRatings via httptest
// ===========================================================================

func TestEnrichRatings_FromJSONLD(t *testing.T) {
	jsonLD := `{
		"@type": "Hotel",
		"aggregateRating": {
			"ratingValue": 8.5,
			"reviewCount": 1234
		}
	}`
	html := fmt.Sprintf(`<html><head>
		<script type="application/ld+json">%s</script>
	</head><body></body></html>`, jsonLD)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, html)
	}))
	defer srv.Close()

	hotels := []models.HotelResult{
		{Name: "Rated Hotel", Rating: 9.0, BookingURL: srv.URL + "/hotel/1"},
		{Name: "Unrated Hotel", Rating: 0, BookingURL: srv.URL + "/hotel/2"},
		{Name: "No URL", Rating: 0, BookingURL: ""},
	}

	cfg := &ProviderConfig{ID: "test"}
	enrichRatings(context.Background(), srv.Client(), hotels, cfg)

	if hotels[0].Rating != 9.0 {
		t.Errorf("rated hotel should keep its rating, got %v", hotels[0].Rating)
	}
	if hotels[1].Rating != 8.5 {
		t.Errorf("unrated hotel should get enriched rating, got %v", hotels[1].Rating)
	}
	if hotels[1].ReviewCount != 1234 {
		t.Errorf("review count should be enriched, got %d", hotels[1].ReviewCount)
	}
}

func TestEnrichRatings_MaxEnrichments(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><script type="application/ld+json">{"aggregateRating":{"ratingValue":7.0}}</script></html>`)
	}))
	defer srv.Close()

	// Create 10 unrated hotels — only 5 should be enriched.
	hotels := make([]models.HotelResult, 10)
	for i := range hotels {
		hotels[i] = models.HotelResult{
			Name:       fmt.Sprintf("Hotel %d", i),
			BookingURL: srv.URL + fmt.Sprintf("/hotel/%d", i),
		}
	}

	cfg := &ProviderConfig{ID: "test"}
	enrichRatings(context.Background(), srv.Client(), hotels, cfg)

	enrichedCount := 0
	for _, h := range hotels {
		if h.Rating > 0 {
			enrichedCount++
		}
	}
	if enrichedCount != 5 {
		t.Errorf("expected exactly 5 enrichments (max), got %d", enrichedCount)
	}
}

func TestEnrichRatings_HTTP404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer srv.Close()

	hotels := []models.HotelResult{
		{Name: "Missing Hotel", Rating: 0, BookingURL: srv.URL + "/hotel/missing"},
	}

	cfg := &ProviderConfig{ID: "test"}
	enrichRatings(context.Background(), srv.Client(), hotels, cfg)

	if hotels[0].Rating != 0 {
		t.Errorf("rating should stay 0 on 404, got %v", hotels[0].Rating)
	}
}

// ===========================================================================
// fetchJSONLDRating — edge cases
// ===========================================================================

func TestFetchJSONLDRating_NoStructuredData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body>No structured data</body></html>`)
	}))
	defer srv.Close()

	_, _, err := fetchJSONLDRating(context.Background(), srv.Client(), srv.URL)
	if err == nil {
		t.Fatal("expected error when no JSON-LD is present")
	}
}

func TestFetchJSONLDRating_GraphArrayNested(t *testing.T) {
	jsonLD := `{
		"@graph": [
			{"@type": "WebPage"},
			{"@type": "Hotel", "aggregateRating": {"ratingValue": 9.2, "reviewCount": 500}}
		]
	}`
	html := fmt.Sprintf(`<html><script type="application/ld+json">%s</script></html>`, jsonLD)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, html)
	}))
	defer srv.Close()

	rating, count, err := fetchJSONLDRating(context.Background(), srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rating != 9.2 {
		t.Errorf("rating = %v, want 9.2", rating)
	}
	if count != 500 {
		t.Errorf("count = %d, want 500", count)
	}
}

func TestFetchJSONLDRating_MalformedJSON(t *testing.T) {
	html := `<html><script type="application/ld+json">not valid json</script></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, html)
	}))
	defer srv.Close()

	_, _, err := fetchJSONLDRating(context.Background(), srv.Client(), srv.URL)
	if err == nil {
		t.Fatal("expected error for invalid JSON-LD")
	}
}

// ===========================================================================
// enrichAirbnbDescriptions via httptest
// ===========================================================================

func TestEnrichAirbnbDescriptions_FromNiobe(t *testing.T) {
	niobeData := map[string]any{
		"niobeClientData": []any{
			[]any{
				"CacheKey:1",
				map[string]any{
					"data": map[string]any{
						"presentation": map[string]any{
							"stayProductDetailPage": map[string]any{
								"sections": map[string]any{
									"metadata": map[string]any{
										"sharingConfig": map[string]any{
											"description": "A beautiful beachfront apartment.",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	niobeJSON, _ := json.Marshal(niobeData)
	html := fmt.Sprintf(`<html><script data-deferred-state-0>%s</script></html>`, niobeJSON)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, html)
	}))
	defer srv.Close()

	hotels := []models.HotelResult{
		{Name: "Beach Apt", Description: "", BookingURL: srv.URL + "/rooms/12345"},
		{Name: "Already Described", Description: "Has a desc", BookingURL: srv.URL + "/rooms/67890"},
		{Name: "No URL", Description: ""},
	}

	enrichAirbnbDescriptions(context.Background(), srv.Client(), hotels)

	if hotels[0].Description != "A beautiful beachfront apartment." {
		t.Errorf("description = %q, want 'A beautiful beachfront apartment.'", hotels[0].Description)
	}
	if hotels[1].Description != "Has a desc" {
		t.Errorf("already-described should keep its description, got %q", hotels[1].Description)
	}
}

func TestEnrichAirbnbDescriptions_FallbackToSection(t *testing.T) {
	niobeData := map[string]any{
		"niobeClientData": []any{
			[]any{
				"CacheKey:1",
				map[string]any{
					"data": map[string]any{
						"presentation": map[string]any{
							"stayProductDetailPage": map[string]any{
								"sections": map[string]any{
									"metadata": map[string]any{
										"sharingConfig": map[string]any{},
									},
									"sections": []any{
										map[string]any{
											"sectionComponentType": "OTHER",
											"section": map[string]any{
												"title": "Not the description",
											},
										},
										map[string]any{
											"sectionComponentType": "DESCRIPTION_DEFAULT",
											"section": map[string]any{
												"body": map[string]any{
													"htmlText": "<b>Cozy</b> apartment with <br/>great views.",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	niobeJSON, _ := json.Marshal(niobeData)
	html := fmt.Sprintf(`<html><script data-deferred-state-0>%s</script></html>`, niobeJSON)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, html)
	}))
	defer srv.Close()

	hotels := []models.HotelResult{
		{Name: "Apt", Description: "", BookingURL: srv.URL + "/rooms/1"},
	}

	enrichAirbnbDescriptions(context.Background(), srv.Client(), hotels)

	// Should have HTML stripped.
	if !strings.Contains(hotels[0].Description, "Cozy") {
		t.Errorf("description = %q, want to contain 'Cozy'", hotels[0].Description)
	}
	if strings.Contains(hotels[0].Description, "<b>") {
		t.Errorf("description should have HTML stripped: %q", hotels[0].Description)
	}
}

func TestEnrichAirbnbDescriptions_MaxThreeEnrichments(t *testing.T) {
	niobeData := map[string]any{
		"niobeClientData": []any{
			[]any{
				"CacheKey:1",
				map[string]any{
					"data": map[string]any{
						"presentation": map[string]any{
							"stayProductDetailPage": map[string]any{
								"sections": map[string]any{
									"metadata": map[string]any{
										"sharingConfig": map[string]any{
											"description": "Test desc",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	niobeJSON, _ := json.Marshal(niobeData)
	html := fmt.Sprintf(`<html><script data-deferred-state-0>%s</script></html>`, niobeJSON)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, html)
	}))
	defer srv.Close()

	// Create 5 undescribed hotels — only 3 should be enriched.
	hotels := make([]models.HotelResult, 5)
	for i := range hotels {
		hotels[i] = models.HotelResult{
			Name:       fmt.Sprintf("Apt %d", i),
			BookingURL: srv.URL + fmt.Sprintf("/rooms/%d", i),
		}
	}

	enrichAirbnbDescriptions(context.Background(), srv.Client(), hotels)

	enrichedCount := 0
	for _, h := range hotels {
		if h.Description != "" {
			enrichedCount++
		}
	}
	if enrichedCount != 3 {
		t.Errorf("expected exactly 3 enrichments (max), got %d", enrichedCount)
	}
}

// ===========================================================================
// fetchAirbnbDescription — edge cases
// ===========================================================================

func TestFetchAirbnbDescription_NoNiobeScript(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body>No deferred state</body></html>`)
	}))
	defer srv.Close()

	_, err := fetchAirbnbDescription(context.Background(), srv.Client(), srv.URL)
	if err == nil {
		t.Fatal("expected error when no data-deferred-state-0 script")
	}
}

func TestFetchAirbnbDescription_HTTP500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	_, err := fetchAirbnbDescription(context.Background(), srv.Client(), srv.URL)
	if err == nil {
		t.Fatal("expected error on HTTP 500")
	}
}

func TestFetchAirbnbDescription_SubtitleFallback(t *testing.T) {
	niobeData := map[string]any{
		"niobeClientData": []any{
			[]any{
				"CacheKey:1",
				map[string]any{
					"data": map[string]any{
						"presentation": map[string]any{
							"stayProductDetailPage": map[string]any{
								"sections": map[string]any{
									"metadata": map[string]any{
										"sharingConfig": map[string]any{},
									},
									"sections": []any{
										map[string]any{
											"sectionComponentType": "DESCRIPTION_DEFAULT",
											"section": map[string]any{
												"subtitle": "A cozy place to stay",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	niobeJSON, _ := json.Marshal(niobeData)
	html := fmt.Sprintf(`<html><script data-deferred-state-0>%s</script></html>`, niobeJSON)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, html)
	}))
	defer srv.Close()

	desc, err := fetchAirbnbDescription(context.Background(), srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if desc != "A cozy place to stay" {
		t.Errorf("desc = %q, want 'A cozy place to stay'", desc)
	}
}

func TestFetchAirbnbDescription_TitleFallback(t *testing.T) {
	niobeData := map[string]any{
		"niobeClientData": []any{
			[]any{
				"CacheKey:1",
				map[string]any{
					"data": map[string]any{
						"presentation": map[string]any{
							"stayProductDetailPage": map[string]any{
								"sections": map[string]any{
									"metadata": map[string]any{
										"sharingConfig": map[string]any{},
									},
									"sections": []any{
										map[string]any{
											"sectionComponentType": "DESCRIPTION_DEFAULT",
											"section": map[string]any{
												"title": "About this space",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	niobeJSON, _ := json.Marshal(niobeData)
	html := fmt.Sprintf(`<html><script data-deferred-state-0>%s</script></html>`, niobeJSON)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, html)
	}))
	defer srv.Close()

	desc, err := fetchAirbnbDescription(context.Background(), srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if desc != "About this space" {
		t.Errorf("desc = %q, want 'About this space'", desc)
	}
}

// ===========================================================================
// stripHTMLTags — edge cases
// ===========================================================================

func TestStripHTMLTags_SelfClosingBr(t *testing.T) {
	got := stripHTMLTags("Hello<br/>World")
	if got != "Hello World" {
		t.Errorf("got %q, want 'Hello World'", got)
	}
}

func TestStripHTMLTags_MultipleTags(t *testing.T) {
	got := stripHTMLTags("<p>Hello</p><br><span>World</span>")
	if got != "Hello World" {
		t.Errorf("got %q, want 'Hello World'", got)
	}
}

func TestStripHTMLTags_NoTags(t *testing.T) {
	got := stripHTMLTags("Just plain text")
	if got != "Just plain text" {
		t.Errorf("got %q, want 'Just plain text'", got)
	}
}

func TestStripHTMLTags_Empty(t *testing.T) {
	got := stripHTMLTags("")
	if got != "" {
		t.Errorf("got %q, want empty string", got)
	}
}

// ===========================================================================
// defaultOpenURL — unsupported OS
// ===========================================================================

func TestDefaultOpenURL_UnsupportedOS(t *testing.T) {
	err := defaultOpenURL("freebsd", "", "https://example.com")
	if err == nil {
		t.Fatal("expected error for unsupported OS")
	}
	if !strings.Contains(err.Error(), "unsupported OS") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ===========================================================================
// resolveCityExtraFields via httptest
// ===========================================================================

func TestResolveCityExtraFields_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"results": []any{
				map[string]any{
					"dest_id":   "-2140479",
					"dest_type": "city",
					"city_name": "Helsinki",
				},
			},
		})
	}))
	defer srv.Close()

	cfg := &ProviderConfig{
		ID: "test",
		CityResolver: &CityResolverConfig{
			URL:        srv.URL + "/autocomplete?text=${location}",
			ResultPath: "results",
			IDField:    "dest_id",
			ExtraFields: map[string]string{
				"dest_type": "dest_type",
			},
		},
	}

	extras, err := resolveCityExtraFields(context.Background(), cfg, srv.Client(), "Helsinki")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if extras["dest_type"] != "city" {
		t.Errorf("dest_type = %q, want 'city'", extras["dest_type"])
	}
}

func TestResolveCityExtraFields_NilResolver(t *testing.T) {
	cfg := &ProviderConfig{ID: "test"}
	extras, err := resolveCityExtraFields(context.Background(), cfg, http.DefaultClient, "Helsinki")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if extras != nil {
		t.Errorf("expected nil extras for nil resolver, got %v", extras)
	}
}

func TestResolveCityExtraFields_NoExtraFields(t *testing.T) {
	cfg := &ProviderConfig{
		ID:           "test",
		CityResolver: &CityResolverConfig{},
	}
	extras, err := resolveCityExtraFields(context.Background(), cfg, http.DefaultClient, "Helsinki")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if extras != nil {
		t.Errorf("expected nil extras for empty extra_fields, got %v", extras)
	}
}

func TestResolveCityExtraFields_HTTP500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	cfg := &ProviderConfig{
		ID: "test",
		CityResolver: &CityResolverConfig{
			URL:        srv.URL + "/autocomplete?text=${location}",
			ResultPath: "results",
			IDField:    "dest_id",
			ExtraFields: map[string]string{
				"dest_type": "dest_type",
			},
		},
	}

	_, err := resolveCityExtraFields(context.Background(), cfg, srv.Client(), "Helsinki")
	if err == nil {
		t.Fatal("expected error on HTTP 500")
	}
}

// ===========================================================================
// resolveCityIDDynamic via httptest
// ===========================================================================

func TestResolveCityIDDynamic_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"results": []any{
				map[string]any{
					"dest_id":   float64(12345),
					"city_name": "Prague",
				},
			},
		})
	}))
	defer srv.Close()

	cfg := &ProviderConfig{
		ID: "test-resolver",
		CityResolver: &CityResolverConfig{
			URL:        srv.URL + "/autocomplete?text=${location}",
			ResultPath: "results",
			IDField:    "dest_id",
			NameField:  "city_name",
		},
	}

	cityID, err := resolveCityIDDynamic(context.Background(), cfg, srv.Client(), "Prague", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cityID != "12345" {
		t.Errorf("cityID = %q, want '12345'", cityID)
	}

	// Check it was cached.
	if cfg.CityLookup["prague"] != "12345" {
		t.Errorf("CityLookup[prague] = %q, want '12345'", cfg.CityLookup["prague"])
	}
}

func TestResolveCityIDDynamic_NilResolver(t *testing.T) {
	cfg := &ProviderConfig{ID: "test"}
	_, err := resolveCityIDDynamic(context.Background(), cfg, http.DefaultClient, "Prague", nil)
	if err == nil {
		t.Fatal("expected error for nil resolver")
	}
}

func TestResolveCityIDDynamic_EmptyURL(t *testing.T) {
	cfg := &ProviderConfig{
		ID:           "test",
		CityResolver: &CityResolverConfig{},
	}
	_, err := resolveCityIDDynamic(context.Background(), cfg, http.DefaultClient, "Prague", nil)
	if err == nil {
		t.Fatal("expected error for empty URL")
	}
}

func TestResolveCityIDDynamic_EmptyResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"results": []any{},
		})
	}))
	defer srv.Close()

	cfg := &ProviderConfig{
		ID: "test",
		CityResolver: &CityResolverConfig{
			URL:        srv.URL + "/autocomplete?text=${location}",
			ResultPath: "results",
			IDField:    "dest_id",
		},
	}

	_, err := resolveCityIDDynamic(context.Background(), cfg, srv.Client(), "NoCity", nil)
	if err == nil {
		t.Fatal("expected error for empty results")
	}
}

// ===========================================================================
// cookieDomainMatchesHost — additional cases
// ===========================================================================

func TestCookieDomainMatchesHost_CaseInsensitive(t *testing.T) {
	if !cookieDomainMatchesHost("BOOKING.COM", "booking.com") {
		t.Error("should match case-insensitively")
	}
	if !cookieDomainMatchesHost(".Booking.Com", "www.booking.com") {
		t.Error("should match case-insensitively with dot prefix")
	}
}

func TestCookieDomainMatchesHost_BothEmpty(t *testing.T) {
	if cookieDomainMatchesHost("", "") {
		t.Error("both empty should not match")
	}
}

// ===========================================================================
// isTestBinary — exercises the function
// ===========================================================================

func TestIsTestBinary_ReturnsTrue(t *testing.T) {
	// When running under go test, this should return true.
	if !isTestBinary() {
		t.Error("expected isTestBinary() to return true during tests")
	}
}

// ===========================================================================
// applyURLExtractions — invalid regex in URL extraction
// ===========================================================================

func TestApplyURLExtractions_InvalidRegex(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `some content`)
	}))
	defer srv.Close()

	extractions := map[string]Extraction{
		"bad": {
			Pattern:  `[invalid(regex`,
			Variable: "bad_var",
			URL:      srv.URL + "/bundle.js",
		},
	}
	authValues := make(map[string]string)

	matched := applyURLExtractions(context.Background(), srv.Client(), extractions, authValues)
	if matched != 0 {
		t.Errorf("matched = %d, want 0 for invalid regex", matched)
	}
}

// ===========================================================================
// applyURLExtractions — no match no default
// ===========================================================================

func TestApplyURLExtractions_NoMatchNoDefault(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `content without the pattern`)
	}))
	defer srv.Close()

	extractions := map[string]Extraction{
		"hash": {
			Pattern:  `sha256:([a-f0-9]{64})`,
			Variable: "sha",
			URL:      srv.URL + "/bundle.js",
			// No Default
		},
	}
	authValues := make(map[string]string)

	matched := applyURLExtractions(context.Background(), srv.Client(), extractions, authValues)
	if matched != 0 {
		t.Errorf("matched = %d, want 0 for no match with no default", matched)
	}
}

// ===========================================================================
// applyExtractions — variable fallback with default
// ===========================================================================

func TestApplyExtractions_DefaultFallbackToName(t *testing.T) {
	body := []byte(`no match here`)
	resp := &http.Response{Header: make(http.Header)}
	extractions := map[string]Extraction{
		"my_var": {
			Pattern: `never_matches`,
			Default: "fallback_value",
			// Variable deliberately empty — should use map key "my_var"
		},
	}
	authValues := make(map[string]string)

	matched := applyExtractions(extractions, resp, body, authValues)
	if matched != 1 {
		t.Errorf("matched = %d, want 1", matched)
	}
	if authValues["my_var"] != "fallback_value" {
		t.Errorf("my_var = %q, want 'fallback_value'", authValues["my_var"])
	}
}

// ===========================================================================
// anyToString — edge cases
// ===========================================================================

func TestAnyToString_Bool(t *testing.T) {
	if anyToString(true) != "true" {
		t.Errorf("anyToString(true) = %q", anyToString(true))
	}
	if anyToString(false) != "false" {
		t.Errorf("anyToString(false) = %q", anyToString(false))
	}
}

func TestAnyToString_Nil(t *testing.T) {
	if anyToString(nil) != "" {
		t.Errorf("anyToString(nil) = %q, want empty", anyToString(nil))
	}
}

func TestAnyToString_FractionalFloat(t *testing.T) {
	got := anyToString(3.14)
	if got != "3.14" {
		t.Errorf("anyToString(3.14) = %q, want '3.14'", got)
	}
}

func TestAnyToString_OtherType(t *testing.T) {
	got := anyToString([]int{1, 2, 3})
	if got != "[1 2 3]" {
		t.Errorf("anyToString([]int{1,2,3}) = %q", got)
	}
}

// ===========================================================================
// openURLInBrowser — defaultChromePreference on darwin
// ===========================================================================

func TestOpenURLInBrowser_DefaultChromeOnDarwin(t *testing.T) {
	var gotPref string
	withOpener(t, func(goos, pref, target string) error {
		gotPref = pref
		return nil
	})

	if err := openURLInBrowser("https://example.com", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// On darwin, empty preference should default to "Google Chrome".
	// On other OS, it stays empty.
	if runtime.GOOS == "darwin" && gotPref != "Google Chrome" {
		t.Errorf("on darwin, expected 'Google Chrome' default, got %q", gotPref)
	}
}

// ===========================================================================
// waitForFreshCookies — zero poll/max values
// ===========================================================================

func TestWaitForFreshCookies_ZeroPollInterval(t *testing.T) {
	prev := []*http.Cookie{{Name: "sid", Value: "v1"}}
	callCount := 0
	withCookieSource(t, func(string) []*http.Cookie {
		callCount++
		if callCount >= 2 {
			return []*http.Cookie{{Name: "sid", Value: "v2"}}
		}
		return []*http.Cookie{{Name: "sid", Value: "v1"}}
	})

	got, changed := waitForFreshCookies(context.Background(), "https://example.com",
		prev, 0, 0) // zero -> defaults to 1s poll, 10s max

	if !changed {
		t.Fatal("expected cookie change to be detected")
	}
	if len(got) != 1 || got[0].Value != "v2" {
		t.Errorf("unexpected cookies: %+v", got)
	}
}

// ===========================================================================
// cookieSnapshotKey — nil cookies
// ===========================================================================

func TestCookieSnapshotKey_NilCookies(t *testing.T) {
	key := cookieSnapshotKey(nil)
	if key != "" {
		t.Errorf("expected empty key for nil cookies, got %q", key)
	}
}

// ===========================================================================
// tryBrowserCookieRetry — exercises via httptest
// ===========================================================================

func TestTryBrowserCookieRetry_NoBrowserCookies(t *testing.T) {
	// When applyBrowserCookies returns false (no cookies found), retry should fail.
	jar, _ := cookiejar.New(nil)
	pc := &providerClient{
		config: &ProviderConfig{
			ID:      "test-retry",
			Cookies: CookieConfig{Browser: ""},
		},
		client:     &http.Client{Jar: jar},
		authValues: make(map[string]string),
	}

	auth := &AuthConfig{
		PreflightURL: "https://no-browser-cookies.example.invalid/page",
	}

	got := tryBrowserCookieRetry(context.Background(), pc, auth)
	if got {
		t.Error("expected false when no browser cookies available")
	}
}

// ===========================================================================
// tryWAFSolve — exercises via httptest
// ===========================================================================

func TestTryWAFSolve_NonChallengeStatus(t *testing.T) {
	jar, _ := cookiejar.New(nil)
	pc := &providerClient{
		config: &ProviderConfig{ID: "test-waf"},
		client: &http.Client{Jar: jar},
		authValues: make(map[string]string),
	}

	auth := &AuthConfig{
		PreflightURL: "https://example.com/page",
	}

	// Status 200 — not a WAF challenge.
	got := tryWAFSolve(context.Background(), pc, auth, 200, []byte("normal page"))
	if got {
		t.Error("expected false for non-challenge status code")
	}
}

func TestTryWAFSolve_403NoWAFMarkers(t *testing.T) {
	jar, _ := cookiejar.New(nil)
	pc := &providerClient{
		config: &ProviderConfig{ID: "test-waf"},
		client: &http.Client{Jar: jar},
		authValues: make(map[string]string),
	}

	auth := &AuthConfig{
		PreflightURL: "https://example.com/page",
	}

	// Status 403 without WAF markers — SolveAWSWAF should fail.
	got := tryWAFSolve(context.Background(), pc, auth, 403, []byte("<html>Access Denied</html>"))
	if got {
		t.Error("expected false when WAF solver doesn't find a token")
	}
}

// ===========================================================================
// tryBrowserEscapeHatch — exercises edges
// ===========================================================================

func TestTryBrowserEscapeHatch_NotInteractive(t *testing.T) {
	jar, _ := cookiejar.New(nil)
	pc := &providerClient{
		config: &ProviderConfig{
			ID: "test-escape",
			Auth: &AuthConfig{
				BrowserEscapeHatch: true,
			},
		},
		client:     &http.Client{Jar: jar},
		authValues: make(map[string]string),
	}

	auth := &AuthConfig{
		PreflightURL:       "https://example.com/page",
		BrowserEscapeHatch: true,
	}

	// Non-interactive context — should attempt since the function doesn't check
	// isInteractive (that's done by the caller in runPreflight).
	// But withOpener should prevent actual browser launch.
	withOpener(t, func(goos, pref, target string) error {
		return fmt.Errorf("browser launch blocked in test")
	})

	got := tryBrowserEscapeHatch(context.Background(), pc, auth)
	if got {
		t.Error("expected false when browser open fails")
	}
}

func TestTryBrowserEscapeHatch_ElicitDeclined(t *testing.T) {
	jar, _ := cookiejar.New(nil)
	pc := &providerClient{
		config: &ProviderConfig{
			ID:   "test-escape-elicit",
			Name: "TestProvider",
		},
		client:     &http.Client{Jar: jar},
		authValues: make(map[string]string),
	}

	auth := &AuthConfig{
		PreflightURL:       "https://example.com/page",
		BrowserEscapeHatch: true,
	}

	// Add elicit that declines.
	ctx := WithElicit(context.Background(), func(msg string) (bool, error) {
		return false, nil // user declined
	})

	got := tryBrowserEscapeHatch(ctx, pc, auth)
	if got {
		t.Error("expected false when user declines elicitation")
	}
}

func TestTryBrowserEscapeHatch_ElicitError(t *testing.T) {
	jar, _ := cookiejar.New(nil)
	pc := &providerClient{
		config: &ProviderConfig{
			ID:   "test-escape-elicit-err",
			Name: "TestProvider",
		},
		client:     &http.Client{Jar: jar},
		authValues: make(map[string]string),
	}

	auth := &AuthConfig{
		PreflightURL:       "https://example.com/page",
		BrowserEscapeHatch: true,
	}

	// Add elicit that returns an error.
	ctx := WithElicit(context.Background(), func(msg string) (bool, error) {
		return false, fmt.Errorf("elicitation failed")
	})

	got := tryBrowserEscapeHatch(ctx, pc, auth)
	if got {
		t.Error("expected false when elicitation errors")
	}
}

// ===========================================================================
// enrichRatings — no enrichable results
// ===========================================================================

func TestEnrichRatings_AllRated(t *testing.T) {
	hotels := []models.HotelResult{
		{Name: "Already Rated 1", Rating: 8.0, BookingURL: "https://example.com/1"},
		{Name: "Already Rated 2", Rating: 7.5, BookingURL: "https://example.com/2"},
	}

	cfg := &ProviderConfig{ID: "test"}
	enrichRatings(context.Background(), http.DefaultClient, hotels, cfg)

	// Ratings should remain unchanged.
	if hotels[0].Rating != 8.0 || hotels[1].Rating != 7.5 {
		t.Error("already-rated hotels should not be modified")
	}
}

func TestEnrichRatings_EmptySlice(t *testing.T) {
	cfg := &ProviderConfig{ID: "test"}
	// Should not panic.
	enrichRatings(context.Background(), http.DefaultClient, nil, cfg)
}

// ===========================================================================
// runPreflight — exercises cache valid path + extraction paths
// ===========================================================================

func TestRunPreflight_NilAuth(t *testing.T) {
	rt := NewRuntime(nil)
	jar, _ := cookiejar.New(nil)
	pc := &providerClient{
		config:     &ProviderConfig{ID: "test-nil-auth"},
		client:     &http.Client{Jar: jar},
		authValues: make(map[string]string),
	}

	// No auth config -> should return nil immediately.
	err := rt.runPreflight(context.Background(), pc, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPreflight_EmptyPreflightURL(t *testing.T) {
	rt := NewRuntime(nil)
	jar, _ := cookiejar.New(nil)
	pc := &providerClient{
		config: &ProviderConfig{
			ID:   "test-empty-preflight",
			Auth: &AuthConfig{PreflightURL: ""},
		},
		client:     &http.Client{Jar: jar},
		authValues: make(map[string]string),
	}

	err := rt.runPreflight(context.Background(), pc, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPreflight_SuccessfulExtraction(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><meta name="csrf" content="tok-999"></html>`)
	}))
	defer srv.Close()

	rt := NewRuntime(nil)
	jar, _ := cookiejar.New(nil)
	pc := &providerClient{
		config: &ProviderConfig{
			ID: "test-preflight-extract",
			Auth: &AuthConfig{
				PreflightURL: srv.URL + "/page",
				Extractions: map[string]Extraction{
					"csrf": {
						Pattern:  `content="(tok-[^"]+)"`,
						Variable: "csrf_token",
					},
				},
			},
		},
		client:     &http.Client{Jar: jar},
		authValues: make(map[string]string),
	}

	err := rt.runPreflight(context.Background(), pc, nil)
	if err != nil {
		t.Fatalf("runPreflight: %v", err)
	}
	if pc.authValues["csrf_token"] != "tok-999" {
		t.Errorf("csrf_token = %q, want 'tok-999'", pc.authValues["csrf_token"])
	}
}

func TestRunPreflight_CacheValid(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html>token=first</html>`)
	}))
	defer srv.Close()

	rt := NewRuntime(nil)
	jar, _ := cookiejar.New(nil)
	pc := &providerClient{
		config: &ProviderConfig{
			ID: "test-preflight-cache",
			Auth: &AuthConfig{
				PreflightURL: srv.URL + "/page",
			},
		},
		client:     &http.Client{Jar: jar},
		authValues: make(map[string]string),
	}

	// First call populates cache.
	if err := rt.runPreflight(context.Background(), pc, nil); err != nil {
		t.Fatalf("first preflight: %v", err)
	}

	// Second call should hit the cache (same URL, within expiry).
	if err := rt.runPreflight(context.Background(), pc, nil); err != nil {
		t.Fatalf("second preflight (cached): %v", err)
	}
}

// ===========================================================================
// doSearchRequest — nil GetBody
// ===========================================================================

func TestDoSearchRequest_NilGetBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"ok":true}`)
	}))
	defer srv.Close()

	orig, _ := http.NewRequestWithContext(context.Background(), "GET", srv.URL+"/search", nil)
	// GetBody is nil for GET requests without a body.
	resp, body, err := doSearchRequest(context.Background(), srv.Client(), orig)
	if err != nil {
		t.Fatalf("doSearchRequest: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(string(body), "ok") {
		t.Errorf("body = %q, want to contain 'ok'", string(body))
	}
}

// ===========================================================================
// applyBrowserCookies — nil client
// ===========================================================================

func TestApplyBrowserCookies_NilClient(t *testing.T) {
	if applyBrowserCookies(nil, "https://example.com", "") {
		t.Error("expected false for nil client")
	}
}

// ===========================================================================
// TestProvider — exercises the full diagnostic flow via httptest
// ===========================================================================

func TestTestProvider_FullFlow(t *testing.T) {
	// Serve a provider that returns valid JSON with results.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"results": []any{
				map[string]any{
					"name":  "Test Hotel",
					"price": 99.0,
					"id":    "hotel-1",
				},
			},
		})
	}))
	defer srv.Close()

	cfg := &ProviderConfig{
		ID:       "test-full",
		Name:     "Test Provider",
		Category: "hotel",
		Endpoint: srv.URL + "/search?location=${location}&checkin=${checkin}&checkout=${checkout}",
		Method:   "GET",
		ResponseMapping: ResponseMapping{
			ResultsPath: "results",
			Fields: map[string]string{
				"name":     "name",
				"price":    "price",
				"hotel_id": "id",
			},
		},
		RateLimit: RateLimitConfig{RequestsPerSecond: 10, Burst: 1},
	}

	result := TestProvider(context.Background(), cfg, "Helsinki", 60.17, 24.94,
		"2026-05-01", "2026-05-03", "EUR", 2)

	if !result.Success {
		t.Errorf("expected success, got error at step %q: %s", result.Step, result.Error)
	}
	if result.ResultsCount != 1 {
		t.Errorf("results count = %d, want 1", result.ResultsCount)
	}
}

func TestTestProvider_HTTP500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		fmt.Fprint(w, "Internal Server Error")
	}))
	defer srv.Close()

	cfg := &ProviderConfig{
		ID:       "test-500",
		Name:     "Failing Provider",
		Category: "hotel",
		Endpoint: srv.URL + "/search",
		Method:   "GET",
		ResponseMapping: ResponseMapping{
			ResultsPath: "results",
			Fields:      map[string]string{"name": "name"},
		},
	}

	result := TestProvider(context.Background(), cfg, "Helsinki", 60.17, 24.94,
		"2026-05-01", "2026-05-03", "EUR", 2)

	if result.Success {
		t.Error("expected failure on HTTP 500")
	}
	if result.HTTPStatus != 500 {
		t.Errorf("status = %d, want 500", result.HTTPStatus)
	}
}

func TestTestProvider_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `not json`)
	}))
	defer srv.Close()

	cfg := &ProviderConfig{
		ID:       "test-bad-json",
		Name:     "Bad JSON Provider",
		Category: "hotel",
		Endpoint: srv.URL + "/search",
		Method:   "GET",
		ResponseMapping: ResponseMapping{
			ResultsPath: "results",
			Fields:      map[string]string{"name": "name"},
		},
	}

	result := TestProvider(context.Background(), cfg, "Helsinki", 60.17, 24.94,
		"2026-05-01", "2026-05-03", "EUR", 2)

	if result.Success {
		t.Error("expected failure on malformed JSON")
	}
	if !strings.Contains(result.Error, "response_parse") {
		t.Errorf("error should mention response_parse, got %q", result.Error)
	}
}

func TestTestProvider_WrongResultsPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"items": []any{
					map[string]any{"name": "Hotel A"},
				},
			},
		})
	}))
	defer srv.Close()

	cfg := &ProviderConfig{
		ID:       "test-wrong-path",
		Name:     "Wrong Path Provider",
		Category: "hotel",
		Endpoint: srv.URL + "/search",
		Method:   "GET",
		ResponseMapping: ResponseMapping{
			ResultsPath: "nonexistent.path",
			Fields:      map[string]string{"name": "name"},
		},
	}

	result := TestProvider(context.Background(), cfg, "Helsinki", 60.17, 24.94,
		"2026-05-01", "2026-05-03", "EUR", 2)

	if result.Success {
		t.Error("expected failure on wrong results path")
	}
}

func TestTestProvider_WithPreflight(t *testing.T) {
	preflightCalls := 0
	searchCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/auth") {
			preflightCalls++
			fmt.Fprint(w, `<html>csrf_token=abc123</html>`)
			return
		}
		searchCalls++
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"results": []any{
				map[string]any{"name": "Preflight Hotel", "price": 150.0},
			},
		})
	}))
	defer srv.Close()

	cfg := &ProviderConfig{
		ID:       "test-preflight",
		Name:     "Preflight Provider",
		Category: "hotel",
		Endpoint: srv.URL + "/search",
		Method:   "GET",
		Auth: &AuthConfig{
			Type:         "preflight",
			PreflightURL: srv.URL + "/auth",
			Extractions: map[string]Extraction{
				"csrf": {
					Pattern:  `csrf_token=(\w+)`,
					Variable: "csrf",
				},
			},
		},
		ResponseMapping: ResponseMapping{
			ResultsPath: "results",
			Fields: map[string]string{
				"name":  "name",
				"price": "price",
			},
		},
	}

	result := TestProvider(context.Background(), cfg, "Helsinki", 60.17, 24.94,
		"2026-05-01", "2026-05-03", "EUR", 2)

	if !result.Success {
		t.Errorf("expected success with preflight, got error at step %q: %s", result.Step, result.Error)
	}
	if preflightCalls == 0 {
		t.Error("expected at least one preflight call")
	}
	if searchCalls == 0 {
		t.Error("expected at least one search call")
	}
}

func TestTestProvider_PostWithBody(t *testing.T) {
	var receivedBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		receivedBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"hotels": []any{
					map[string]any{"name": "POST Hotel"},
				},
			},
		})
	}))
	defer srv.Close()

	cfg := &ProviderConfig{
		ID:           "test-post",
		Name:         "POST Provider",
		Category:     "hotel",
		Endpoint:     srv.URL + "/graphql",
		Method:       "POST",
		BodyTemplate: `{"query":"hotels","variables":{"location":"${location}"}}`,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		ResponseMapping: ResponseMapping{
			ResultsPath: "data.hotels",
			Fields:      map[string]string{"name": "name"},
		},
	}

	result := TestProvider(context.Background(), cfg, "Helsinki", 60.17, 24.94,
		"2026-05-01", "2026-05-03", "EUR", 2)

	if !result.Success {
		t.Errorf("expected success, got error at step %q: %s", result.Step, result.Error)
	}
	if !strings.Contains(receivedBody, "Helsinki") {
		t.Errorf("body should contain 'Helsinki': %s", receivedBody)
	}
}

func TestTestProvider_WithHeaderOrder(t *testing.T) {
	var headerOrder []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Record the order headers arrive (Go's http.Header preserves order within values
		// but doesn't expose inter-key order; just verify all headers are present).
		for _, k := range []string{"Accept", "User-Agent", "X-Custom"} {
			if v := r.Header.Get(k); v != "" {
				headerOrder = append(headerOrder, k)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"results":[{"name":"Ordered Hotel"}]}`)
	}))
	defer srv.Close()

	cfg := &ProviderConfig{
		ID:       "test-header-order",
		Name:     "Header Order Provider",
		Category: "hotel",
		Endpoint: srv.URL + "/search",
		Method:   "GET",
		Headers: map[string]string{
			"Accept":     "application/json",
			"User-Agent": "trvl-test",
			"X-Custom":   "test-value",
		},
		HeaderOrder: []string{"Accept", "User-Agent", "X-Custom"},
		ResponseMapping: ResponseMapping{
			ResultsPath: "results",
			Fields:      map[string]string{"name": "name"},
		},
	}

	result := TestProvider(context.Background(), cfg, "Helsinki", 60.17, 24.94,
		"2026-05-01", "2026-05-03", "EUR", 2)

	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}
	if len(headerOrder) != 3 {
		t.Errorf("expected 3 ordered headers, got %d", len(headerOrder))
	}
}

func TestTestProvider_BodyExtractPattern(t *testing.T) {
	html := `<html><head></head><body>
	<script type="application/json" data-state>{"results":[{"name":"Extracted Hotel"}]}</script>
	</body></html>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, html)
	}))
	defer srv.Close()

	cfg := &ProviderConfig{
		ID:       "test-body-extract",
		Name:     "Body Extract Provider",
		Category: "hotel",
		Endpoint: srv.URL + "/search",
		Method:   "GET",
		ResponseMapping: ResponseMapping{
			ResultsPath:        "results",
			Fields:             map[string]string{"name": "name"},
			BodyExtractPattern: `<script[^>]*data-state[^>]*>([\s\S]*?)</script>`,
		},
	}

	result := TestProvider(context.Background(), cfg, "Helsinki", 60.17, 24.94,
		"2026-05-01", "2026-05-03", "EUR", 2)

	if !result.Success {
		t.Errorf("expected success, got error at step %q: %s", result.Step, result.Error)
	}
}

func TestTestProvider_GraphQLError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"errors": []any{
				map[string]any{
					"message": "PersistedQueryNotFound",
				},
			},
		})
	}))
	defer srv.Close()

	cfg := &ProviderConfig{
		ID:       "test-gql-error",
		Name:     "GraphQL Error Provider",
		Category: "hotel",
		Endpoint: srv.URL + "/graphql",
		Method:   "POST",
		ResponseMapping: ResponseMapping{
			ResultsPath: "data.results",
			Fields:      map[string]string{"name": "name"},
		},
	}

	result := TestProvider(context.Background(), cfg, "Helsinki", 60.17, 24.94,
		"2026-05-01", "2026-05-03", "EUR", 2)

	if result.Success {
		t.Error("expected failure on GraphQL error response")
	}
}

// ===========================================================================
// ReloadIfChanged — exercises the file-modified path
// ===========================================================================

func TestReloadIfChanged_ModifiedFile(t *testing.T) {
	dir := t.TempDir()

	// Create initial config.
	cfg := &ProviderConfig{
		ID:       "reload-test",
		Name:     "Initial Name",
		Category: "hotel",
		Endpoint: "https://example.com/search",
		ResponseMapping: ResponseMapping{ResultsPath: "results"},
	}
	data, _ := json.Marshal(cfg)
	path := filepath.Join(dir, "reload-test.json")
	os.WriteFile(path, data, 0o600)

	reg, err := NewRegistryAt(dir)
	if err != nil {
		t.Fatal(err)
	}

	// First call should return the existing config.
	got := reg.ReloadIfChanged("reload-test")
	if got == nil || got.Name != "Initial Name" {
		t.Fatalf("initial load: got %v", got)
	}

	// Modify the file with a future mtime.
	cfg.Name = "Updated Name"
	data2, _ := json.Marshal(cfg)
	os.WriteFile(path, data2, 0o600)

	// Touch the file to ensure the mtime is newer.
	futureTime := time.Now().Add(10 * time.Second)
	os.Chtimes(path, futureTime, futureTime)

	// Second call should reload.
	got2 := reg.ReloadIfChanged("reload-test")
	if got2 == nil || got2.Name != "Updated Name" {
		t.Errorf("reload: name = %q, want 'Updated Name'", got2.Name)
	}
}

func TestReloadIfChanged_NonexistentFile(t *testing.T) {
	dir := t.TempDir()

	// Create registry with a provider in memory but no file on disk.
	reg, _ := NewRegistryAt(dir)

	// Try to reload a non-existent provider.
	got := reg.ReloadIfChanged("nonexistent")
	if got != nil {
		t.Errorf("expected nil for non-existent provider, got %v", got)
	}
}

// ===========================================================================
// toFHTTPRequest / toStdResponse — exercises type conversion
// ===========================================================================

func TestToFHTTPRequest_RoundTrip(t *testing.T) {
	body := strings.NewReader("test body")
	req, _ := http.NewRequest("POST", "https://example.com/path", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/html")

	fReq, err := toFHTTPRequest(req)
	if err != nil {
		t.Fatalf("toFHTTPRequest: %v", err)
	}
	if fReq.Method != "POST" {
		t.Errorf("Method = %q, want POST", fReq.Method)
	}
	if fReq.URL.String() != "https://example.com/path" {
		t.Errorf("URL = %q", fReq.URL.String())
	}
}

func TestToStdResponse_Conversion(t *testing.T) {
	// We can't easily construct an fhttp.Response without importing
	// the bogdanfinn/fhttp package directly (it's an internal dep).
	// Coverage for toStdResponse is exercised indirectly when
	// fhttpBridgeTransport.RoundTrip is called in live integration tests.
	// This test verifies toFHTTPRequest doesn't panic on edge inputs.
	req, _ := http.NewRequest("GET", "https://example.com/", nil)
	fReq, err := toFHTTPRequest(req)
	if err != nil {
		t.Fatalf("toFHTTPRequest: %v", err)
	}
	if fReq == nil {
		t.Fatal("expected non-nil fhttp request")
	}
}

func TestDefaultOpenURL_LinuxPath(t *testing.T) {
	// Test the linux path returns an error (xdg-open may not exist in test env).
	err := defaultOpenURL("linux", "", "https://example.com")
	// We don't assert success/failure because xdg-open may or may not exist.
	_ = err
}

func TestDefaultOpenURL_WindowsPath(t *testing.T) {
	// Test the windows path returns an error (cmd not available on non-Windows).
	err := defaultOpenURL("windows", "", "https://example.com")
	_ = err
}

func TestDefaultOpenURL_DarwinWithPreference(t *testing.T) {
	// On darwin, with a non-existent browser preference, should fall back to "open".
	err := defaultOpenURL("darwin", "NonExistentBrowser12345", "https://example.com")
	// The "open" command should work on macOS regardless.
	_ = err
}

// ===========================================================================
// TestProvider — more edge cases for runTestPreflight branches
// ===========================================================================

func TestTestProvider_PreflightFails403(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/auth") {
			w.WriteHeader(403)
			fmt.Fprint(w, `<html>Access Denied</html>`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"results":[]}`)
	}))
	defer srv.Close()

	cfg := &ProviderConfig{
		ID:       "test-preflight-403",
		Name:     "Preflight 403 Provider",
		Category: "hotel",
		Endpoint: srv.URL + "/search",
		Method:   "GET",
		Auth: &AuthConfig{
			Type:         "preflight",
			PreflightURL: srv.URL + "/auth",
			Extractions: map[string]Extraction{
				"token": {
					Pattern:  `token=(\w+)`,
					Variable: "token",
				},
			},
		},
		ResponseMapping: ResponseMapping{
			ResultsPath: "results",
			Fields:      map[string]string{"name": "name"},
		},
	}

	result := TestProvider(context.Background(), cfg, "Helsinki", 60.17, 24.94,
		"2026-05-01", "2026-05-03", "EUR", 2)

	// The preflight gets 403 but extraction fails, triggering the no-match
	// fallback path. Should report extraction failure.
	if result.Success {
		t.Error("expected failure when preflight extraction fails")
	}
}

func TestTestProvider_PreflightExtractionNoMatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/auth") {
			// Returns 200 but body doesn't match the extraction pattern.
			fmt.Fprint(w, `<html>no token here</html>`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"results":[]}`)
	}))
	defer srv.Close()

	cfg := &ProviderConfig{
		ID:       "test-preflight-nomatch",
		Name:     "No Match Provider",
		Category: "hotel",
		Endpoint: srv.URL + "/search",
		Method:   "GET",
		Auth: &AuthConfig{
			Type:         "preflight",
			PreflightURL: srv.URL + "/auth",
			Extractions: map[string]Extraction{
				"token": {
					Pattern:  `token=(\w+)`,
					Variable: "token",
				},
			},
		},
		ResponseMapping: ResponseMapping{
			ResultsPath: "results",
			Fields:      map[string]string{"name": "name"},
		},
	}

	result := TestProvider(context.Background(), cfg, "Helsinki", 60.17, 24.94,
		"2026-05-01", "2026-05-03", "EUR", 2)

	if result.Success {
		t.Error("expected failure when extraction doesn't match")
	}
	if result.AuthTier != "" {
		// No tier succeeded.
	}
}

func TestTestProvider_ZeroResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"results": []any{},
		})
	}))
	defer srv.Close()

	cfg := &ProviderConfig{
		ID:       "test-empty-results",
		Name:     "Empty Results Provider",
		Category: "hotel",
		Endpoint: srv.URL + "/search",
		Method:   "GET",
		ResponseMapping: ResponseMapping{
			ResultsPath: "results",
			Fields:      map[string]string{"name": "name"},
		},
	}

	result := TestProvider(context.Background(), cfg, "Helsinki", 60.17, 24.94,
		"2026-05-01", "2026-05-03", "EUR", 2)

	if !result.Success {
		t.Errorf("expected success with empty results, got error: %s", result.Error)
	}
	if result.ResultsCount != 0 {
		t.Errorf("results count = %d, want 0", result.ResultsCount)
	}
}

func TestTestProvider_WithCityLookup(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"results":[{"name":"City Hotel"}]}`)
	}))
	defer srv.Close()

	cfg := &ProviderConfig{
		ID:       "test-city-lookup",
		Name:     "City Lookup Provider",
		Category: "hotel",
		Endpoint: srv.URL + "/search?city=${city_id}",
		Method:   "GET",
		CityLookup: map[string]string{
			"helsinki": "45",
		},
		ResponseMapping: ResponseMapping{
			ResultsPath: "results",
			Fields:      map[string]string{"name": "name"},
		},
	}

	result := TestProvider(context.Background(), cfg, "Helsinki", 60.17, 24.94,
		"2026-05-01", "2026-05-03", "EUR", 2)

	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}
}

func TestTestProvider_AkamaiChallenge(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(202)
		fmt.Fprint(w, `<html><script src="https://1234.awswaf.com/challenge.js"></script></html>`)
	}))
	defer srv.Close()

	cfg := &ProviderConfig{
		ID:       "test-akamai",
		Name:     "Akamai Provider",
		Category: "hotel",
		Endpoint: srv.URL + "/search",
		Method:   "GET",
		ResponseMapping: ResponseMapping{
			ResultsPath: "results",
			Fields:      map[string]string{"name": "name"},
		},
	}

	result := TestProvider(context.Background(), cfg, "Helsinki", 60.17, 24.94,
		"2026-05-01", "2026-05-03", "EUR", 2)

	if result.Success {
		t.Error("expected failure on Akamai challenge")
	}
	if !strings.Contains(result.Error, "WAF") {
		t.Errorf("error should mention WAF, got %q", result.Error)
	}
}

func TestTestProvider_WithQueryParams(t *testing.T) {
	var receivedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"results":[{"name":"QP Hotel"}]}`)
	}))
	defer srv.Close()

	cfg := &ProviderConfig{
		ID:       "test-query-params",
		Name:     "Query Params Provider",
		Category: "hotel",
		Endpoint: srv.URL + "/search",
		Method:   "GET",
		QueryParams: map[string]string{
			"checkin":  "${checkin}",
			"checkout": "${checkout}",
			"guests":   "${guests}",
		},
		ResponseMapping: ResponseMapping{
			ResultsPath: "results",
			Fields:      map[string]string{"name": "name"},
		},
	}

	result := TestProvider(context.Background(), cfg, "Helsinki", 60.17, 24.94,
		"2026-05-01", "2026-05-03", "EUR", 2)

	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}
	if !strings.Contains(receivedQuery, "checkin=2026-05-01") {
		t.Errorf("query should contain checkin, got %q", receivedQuery)
	}
}

func TestTestProvider_RatingScaleNormalization(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"results": []any{
				map[string]any{"name": "Scaled Hotel", "rating": 4.5},
			},
		})
	}))
	defer srv.Close()

	cfg := &ProviderConfig{
		ID:       "test-rating-scale",
		Name:     "Rating Scale Provider",
		Category: "hotel",
		Endpoint: srv.URL + "/search",
		Method:   "GET",
		ResponseMapping: ResponseMapping{
			ResultsPath: "results",
			Fields:      map[string]string{"name": "name", "rating": "rating"},
			RatingScale: 2.0, // multiply by 2 to normalize 0-5 -> 0-10
		},
	}

	result := TestProvider(context.Background(), cfg, "Helsinki", 60.17, 24.94,
		"2026-05-01", "2026-05-03", "EUR", 2)

	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}
}

// ===========================================================================
// TestProvider — Apollo denormalization and Niobe paths
// ===========================================================================

func TestTestProvider_WithApolloCache(t *testing.T) {
	apolloData := map[string]any{
		"ROOT_QUERY": map[string]any{
			"searchQueries": map[string]any{
				"search({})": map[string]any{
					"results": []any{
						map[string]any{
							"name":    "Apollo Hotel",
							"price":   120.0,
							"id":      "ap-1",
							"address": "123 Main St",
						},
					},
				},
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(apolloData)
	}))
	defer srv.Close()

	cfg := &ProviderConfig{
		ID:       "test-apollo",
		Name:     "Apollo Provider",
		Category: "hotel",
		Endpoint: srv.URL + "/search",
		Method:   "GET",
		ResponseMapping: ResponseMapping{
			ResultsPath: "ROOT_QUERY.searchQueries.search*.results",
			Fields: map[string]string{
				"name":    "name",
				"price":   "price",
				"hotel_id": "id",
				"address": "address",
			},
		},
	}

	result := TestProvider(context.Background(), cfg, "Helsinki", 60.17, 24.94,
		"2026-05-01", "2026-05-03", "EUR", 2)

	if !result.Success {
		t.Errorf("expected success, got error at step %q: %s", result.Step, result.Error)
	}
	if result.SampleResult["address"] != "123 Main St" {
		t.Errorf("sample address = %v, want '123 Main St'", result.SampleResult["address"])
	}
}

func TestTestProvider_WithNiobeSSR(t *testing.T) {
	niobeData := map[string]any{
		"niobeClientData": []any{
			[]any{
				"CacheKey:1",
				map[string]any{
					"data": map[string]any{
						"search": map[string]any{
							"results": []any{
								map[string]any{
									"name":  "Niobe Hotel",
									"price": 200.0,
								},
							},
						},
					},
				},
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(niobeData)
	}))
	defer srv.Close()

	cfg := &ProviderConfig{
		ID:       "test-niobe",
		Name:     "Niobe Provider",
		Category: "hotel",
		Endpoint: srv.URL + "/search",
		Method:   "GET",
		ResponseMapping: ResponseMapping{
			ResultsPath: "data.search.results",
			Fields: map[string]string{
				"name":  "name",
				"price": "price",
			},
		},
	}

	result := TestProvider(context.Background(), cfg, "Helsinki", 60.17, 24.94,
		"2026-05-01", "2026-05-03", "EUR", 2)

	if !result.Success {
		t.Errorf("expected success with Niobe SSR, got error at step %q: %s", result.Step, result.Error)
	}
	if result.ResultsCount != 1 {
		t.Errorf("results count = %d, want 1", result.ResultsCount)
	}
}

func TestTestProvider_WithCityResolver(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/autocomplete") {
			json.NewEncoder(w).Encode(map[string]any{
				"results": []any{
					map[string]any{
						"dest_id":   "-999",
						"city_name": "Helsinki",
					},
				},
			})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"results":[{"name":"Resolved Hotel"}]}`)
	}))
	defer srv.Close()

	cfg := &ProviderConfig{
		ID:       "test-city-resolver",
		Name:     "City Resolver Provider",
		Category: "hotel",
		Endpoint: srv.URL + "/search?city=${city_id}",
		Method:   "GET",
		CityResolver: &CityResolverConfig{
			URL:        srv.URL + "/autocomplete?text=${location}",
			ResultPath: "results",
			IDField:    "dest_id",
			NameField:  "city_name",
		},
		ResponseMapping: ResponseMapping{
			ResultsPath: "results",
			Fields:      map[string]string{"name": "name"},
		},
	}

	result := TestProvider(context.Background(), cfg, "Helsinki", 60.17, 24.94,
		"2026-05-01", "2026-05-03", "EUR", 2)

	if !result.Success {
		t.Errorf("expected success with city resolver, got error at step %q: %s", result.Step, result.Error)
	}
}

func TestCookieSnapshotKey_WithNilElement(t *testing.T) {
	cookies := []*http.Cookie{
		{Name: "a", Value: "1"},
		nil,
		{Name: "b", Value: "2"},
	}
	key := cookieSnapshotKey(cookies)
	// Should not panic and should produce a valid key ignoring the nil.
	if key == "" {
		t.Error("expected non-empty key")
	}
}

package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

func TestSubstituteVars(t *testing.T) {
	vars := map[string]string{
		"${checkin}":  "2025-06-01",
		"${checkout}": "2025-06-05",
		"${currency}": "USD",
		"${guests}":   "2",
		"${lat}":      "48.856613",
		"${lon}":      "2.352222",
	}

	input := "https://api.example.com/search?checkin=${checkin}&checkout=${checkout}&currency=${currency}&guests=${guests}"
	want := "https://api.example.com/search?checkin=2025-06-01&checkout=2025-06-05&currency=USD&guests=2"

	got := substituteVars(input, vars)
	if got != want {
		t.Errorf("substituteVars:\n got  %s\n want %s", got, want)
	}
}

func TestSubstituteVarsBodyTemplate(t *testing.T) {
	vars := map[string]string{
		"${ne_lat}": "49.006613",
		"${ne_lon}": "2.502222",
		"${sw_lat}": "48.706613",
		"${sw_lon}": "2.202222",
	}

	input := `{"bounds":{"ne":{"lat":${ne_lat},"lon":${ne_lon}},"sw":{"lat":${sw_lat},"lon":${sw_lon}}}}`
	want := `{"bounds":{"ne":{"lat":49.006613,"lon":2.502222},"sw":{"lat":48.706613,"lon":2.202222}}}`

	got := substituteVars(input, vars)
	if got != want {
		t.Errorf("substituteVars body template:\n got  %s\n want %s", got, want)
	}
}

func TestJSONPathSimple(t *testing.T) {
	data := map[string]any{
		"results": []any{
			map[string]any{"name": "Hotel A"},
			map[string]any{"name": "Hotel B"},
		},
	}

	got := jsonPath(data, "results")
	arr, ok := got.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", got)
	}
	if len(arr) != 2 {
		t.Fatalf("expected 2 results, got %d", len(arr))
	}
}

func TestJSONPathNested(t *testing.T) {
	data := map[string]any{
		"data": map[string]any{
			"search": map[string]any{
				"results": []any{
					map[string]any{"name": "Nested Hotel"},
				},
			},
		},
	}

	got := jsonPath(data, "data.search.results")
	arr, ok := got.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", got)
	}
	if len(arr) != 1 {
		t.Fatalf("expected 1 result, got %d", len(arr))
	}
	m := arr[0].(map[string]any)
	if m["name"] != "Nested Hotel" {
		t.Errorf("expected 'Nested Hotel', got %v", m["name"])
	}
}

func TestJSONPathMissing(t *testing.T) {
	data := map[string]any{"foo": "bar"}
	got := jsonPath(data, "missing.path")
	if got != nil {
		t.Errorf("expected nil for missing path, got %v", got)
	}
}

func TestJSONPathEmpty(t *testing.T) {
	data := map[string]any{"foo": "bar"}
	got := jsonPath(data, "")
	if got == nil {
		t.Error("expected data for empty path, got nil")
	}
}

func TestJSONPathScalar(t *testing.T) {
	data := map[string]any{
		"hotel": map[string]any{
			"name":   "Grand Plaza",
			"rating": 4.5,
		},
	}

	name := jsonPath(data, "hotel.name")
	if name != "Grand Plaza" {
		t.Errorf("expected 'Grand Plaza', got %v", name)
	}

	rating := jsonPath(data, "hotel.rating")
	if rating != 4.5 {
		t.Errorf("expected 4.5, got %v", rating)
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  ProviderConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: ProviderConfig{
				ID:       "test",
				Name:     "Test Provider",
				Category: "hotel",
				Endpoint: "https://api.example.com/search",
				ResponseMapping: ResponseMapping{
					ResultsPath: "results",
				},
			},
			wantErr: false,
		},
		{
			name: "missing id",
			config: ProviderConfig{
				Name:     "Test",
				Category: "hotel",
				Endpoint: "https://api.example.com",
				ResponseMapping: ResponseMapping{
					ResultsPath: "results",
				},
			},
			wantErr: true,
		},
		{
			name: "missing name",
			config: ProviderConfig{
				ID:       "test",
				Category: "hotel",
				Endpoint: "https://api.example.com",
				ResponseMapping: ResponseMapping{
					ResultsPath: "results",
				},
			},
			wantErr: true,
		},
		{
			name: "missing category",
			config: ProviderConfig{
				ID:       "test",
				Name:     "Test",
				Endpoint: "https://api.example.com",
				ResponseMapping: ResponseMapping{
					ResultsPath: "results",
				},
			},
			wantErr: true,
		},
		{
			name: "missing endpoint",
			config: ProviderConfig{
				ID:       "test",
				Name:     "Test",
				Category: "hotel",
				ResponseMapping: ResponseMapping{
					ResultsPath: "results",
				},
			},
			wantErr: true,
		},
		{
			name: "missing results_path",
			config: ProviderConfig{
				ID:       "test",
				Name:     "Test",
				Category: "hotel",
				Endpoint: "https://api.example.com",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEndpointDomain(t *testing.T) {
	cfg := &ProviderConfig{Endpoint: "https://api.example.com:8443/v1/search"}
	if got := cfg.EndpointDomain(); got != "api.example.com:8443" {
		t.Errorf("EndpointDomain() = %q, want %q", got, "api.example.com:8443")
	}

	cfg2 := &ProviderConfig{Endpoint: "https://hotels.example.com/search"}
	if got := cfg2.EndpointDomain(); got != "hotels.example.com" {
		t.Errorf("EndpointDomain() = %q, want %q", got, "hotels.example.com")
	}

	cfg3 := &ProviderConfig{Endpoint: "://invalid"}
	if got := cfg3.EndpointDomain(); got != "" {
		t.Errorf("EndpointDomain() for invalid URL = %q, want empty", got)
	}
}

func TestRateLimiterCreation(t *testing.T) {
	dir := t.TempDir()
	reg, err := NewRegistryAt(dir)
	if err != nil {
		t.Fatalf("NewRegistryAt: %v", err)
	}

	rt := NewRuntime(reg)

	// Custom rate.
	cfg := &ProviderConfig{
		ID:       "rate-test",
		Name:     "Rate Test",
		Category: "hotel",
		Endpoint: "https://example.com",
		RateLimit: RateLimitConfig{
			RequestsPerSecond: 5.0,
			Burst:             3,
		},
	}

	pc := rt.getOrCreateClient(cfg)
	if pc.limiter.Limit() != rate.Limit(5.0) {
		t.Errorf("limiter rate = %v, want 5.0", pc.limiter.Limit())
	}
	if pc.limiter.Burst() != 3 {
		t.Errorf("limiter burst = %d, want 3", pc.limiter.Burst())
	}

	// Default rate.
	cfgDefault := &ProviderConfig{
		ID:       "rate-default",
		Name:     "Rate Default",
		Category: "hotel",
		Endpoint: "https://example.com",
	}

	pcDefault := rt.getOrCreateClient(cfgDefault)
	if pcDefault.limiter.Limit() != rate.Limit(defaultRPS) {
		t.Errorf("default limiter rate = %v, want %v", pcDefault.limiter.Limit(), defaultRPS)
	}
	if pcDefault.limiter.Burst() != defaultBurst {
		t.Errorf("default limiter burst = %d, want %d", pcDefault.limiter.Burst(), defaultBurst)
	}
}

func TestBoundingBox(t *testing.T) {
	// The bounding box is computed inside searchProvider. We verify it through
	// variable substitution by checking that the mock server receives correct values.
	lat := 48.856613
	lon := 2.352222

	neLat := lat + boundingBoxOffset
	neLon := lon + boundingBoxOffset
	swLat := lat - boundingBoxOffset
	swLon := lon - boundingBoxOffset

	const eps = 1e-9
	assertClose := func(t *testing.T, name string, got, want float64) {
		t.Helper()
		diff := got - want
		if diff < -eps || diff > eps {
			t.Errorf("%s = %f, want %f (diff %e)", name, got, want, diff)
		}
	}
	assertClose(t, "ne_lat", neLat, 49.006613)
	assertClose(t, "ne_lon", neLon, 2.502222)
	assertClose(t, "sw_lat", swLat, 48.706613)
	assertClose(t, "sw_lon", swLon, 2.202222)
}

func TestSearchHotelsFullFlow(t *testing.T) {
	// Mock server returning hotel results.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify query params.
		q := r.URL.Query()
		if q.Get("checkin") != "2025-06-01" {
			t.Errorf("checkin = %q, want 2025-06-01", q.Get("checkin"))
		}
		if q.Get("checkout") != "2025-06-05" {
			t.Errorf("checkout = %q, want 2025-06-05", q.Get("checkout"))
		}

		resp := map[string]any{
			"data": map[string]any{
				"hotels": []any{
					map[string]any{
						"id":         "h1",
						"hotel_name": "Grand Plaza",
						"stars":      4.0,
						"rate":       4.8,
						"reviews":    120.0,
						"cost":       199.99,
						"curr":       "USD",
						"addr":       "123 Main St",
						"latitude":   48.856613,
						"longitude":  2.352222,
					},
					map[string]any{
						"id":         "h2",
						"hotel_name": "Budget Inn",
						"stars":      2.0,
						"rate":       3.5,
						"reviews":    45.0,
						"cost":       79.99,
						"curr":       "USD",
						"addr":       "456 Side St",
						"latitude":   48.857,
						"longitude":  2.353,
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	dir := t.TempDir()
	reg, err := NewRegistryAt(dir)
	if err != nil {
		t.Fatalf("NewRegistryAt: %v", err)
	}

	cfg := &ProviderConfig{
		ID:       "test-hotel",
		Name:     "Test Hotels",
		Category: "hotel",
		Endpoint: srv.URL + "/search",
		Method:   "GET",
		QueryParams: map[string]string{
			"checkin":  "${checkin}",
			"checkout": "${checkout}",
			"currency": "${currency}",
		},
		ResponseMapping: ResponseMapping{
			ResultsPath: "data.hotels",
			Fields: map[string]string{
				"hotel_id":     "id",
				"name":         "hotel_name",
				"stars":        "stars",
				"rating":       "rate",
				"review_count": "reviews",
				"price":        "cost",
				"currency":     "curr",
				"address":      "addr",
				"lat":          "latitude",
				"lon":          "longitude",
			},
		},
		RateLimit: RateLimitConfig{
			RequestsPerSecond: 100,
			Burst:             10,
		},
	}

	if err := reg.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	rt := NewRuntime(reg)
	hotels, err := rt.SearchHotels(context.Background(), "Paris", 48.856613, 2.352222, "2025-06-01", "2025-06-05", "USD", 2)
	if err != nil {
		t.Fatalf("SearchHotels: %v", err)
	}

	if len(hotels) != 2 {
		t.Fatalf("got %d hotels, want 2", len(hotels))
	}

	h := hotels[0]
	if h.Name != "Grand Plaza" && hotels[1].Name != "Grand Plaza" {
		t.Errorf("expected Grand Plaza in results, got %q and %q", hotels[0].Name, hotels[1].Name)
	}

	// Find Grand Plaza specifically.
	var gp *struct{ h int }
	for i, hotel := range hotels {
		if hotel.Name == "Grand Plaza" {
			idx := i
			gp = &struct{ h int }{h: idx}
			break
		}
	}
	if gp == nil {
		t.Fatal("Grand Plaza not found in results")
	}

	grand := hotels[gp.h]
	if grand.HotelID != "h1" {
		t.Errorf("hotel_id = %q, want h1", grand.HotelID)
	}
	if grand.Stars != 4 {
		t.Errorf("stars = %d, want 4", grand.Stars)
	}
	if grand.Rating != 4.8 {
		t.Errorf("rating = %f, want 4.8", grand.Rating)
	}
	if grand.ReviewCount != 120 {
		t.Errorf("review_count = %d, want 120", grand.ReviewCount)
	}
	if grand.Price != 199.99 {
		t.Errorf("price = %f, want 199.99", grand.Price)
	}
	if grand.Currency != "USD" {
		t.Errorf("currency = %q, want USD", grand.Currency)
	}
	if grand.Address != "123 Main St" {
		t.Errorf("address = %q, want '123 Main St'", grand.Address)
	}
}

func TestSearchHotelsNoProviders(t *testing.T) {
	dir := t.TempDir()
	reg, err := NewRegistryAt(dir)
	if err != nil {
		t.Fatalf("NewRegistryAt: %v", err)
	}

	rt := NewRuntime(reg)
	hotels, err := rt.SearchHotels(context.Background(), "Paris", 48.856613, 2.352222, "2025-06-01", "2025-06-05", "USD", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hotels != nil {
		t.Errorf("expected nil, got %d hotels", len(hotels))
	}
}

func TestPreflightAuthExtraction(t *testing.T) {
	// Mock preflight server returning a page with an API key.
	preflightSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Session-Token", "sess-abc123")
		fmt.Fprintf(w, `<html><script>var apiKey = "key-xyz789";</script></html>`)
	}))
	defer preflightSrv.Close()

	// Mock search server that verifies auth values were extracted.
	searchSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("X-Api-Key")
		session := r.Header.Get("X-Session")
		if apiKey != "key-xyz789" {
			t.Errorf("X-Api-Key = %q, want 'key-xyz789'", apiKey)
		}
		if session != "sess-abc123" {
			t.Errorf("X-Session = %q, want 'sess-abc123'", session)
		}

		resp := map[string]any{
			"results": []any{
				map[string]any{
					"name": "Auth Hotel",
					"id":   "ah1",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer searchSrv.Close()

	dir := t.TempDir()
	reg, err := NewRegistryAt(dir)
	if err != nil {
		t.Fatalf("NewRegistryAt: %v", err)
	}

	cfg := &ProviderConfig{
		ID:       "auth-test",
		Name:     "Auth Test",
		Category: "hotel",
		Endpoint: searchSrv.URL + "/search",
		Method:   "GET",
		Headers: map[string]string{
			"X-Api-Key": "${api_key}",
			"X-Session": "${session_token}",
		},
		Auth: &AuthConfig{
			Type:         "preflight",
			PreflightURL: preflightSrv.URL,
			Extractions: map[string]Extraction{
				"api_key": {
					Pattern:  `apiKey = "([^"]+)"`,
					Variable: "api_key",
				},
				"session_token": {
					Pattern:  `(.+)`,
					Variable: "session_token",
					Header:   "X-Session-Token",
				},
			},
		},
		ResponseMapping: ResponseMapping{
			ResultsPath: "results",
			Fields: map[string]string{
				"name":     "name",
				"hotel_id": "id",
			},
		},
		RateLimit: RateLimitConfig{
			RequestsPerSecond: 100,
			Burst:             10,
		},
	}

	if err := reg.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	rt := NewRuntime(reg)
	hotels, err := rt.SearchHotels(context.Background(), "Test", 0, 0, "2025-06-01", "2025-06-05", "USD", 2)
	if err != nil {
		t.Fatalf("SearchHotels: %v", err)
	}

	if len(hotels) != 1 {
		t.Fatalf("got %d hotels, want 1", len(hotels))
	}
	if hotels[0].Name != "Auth Hotel" {
		t.Errorf("name = %q, want 'Auth Hotel'", hotels[0].Name)
	}
}

func TestRegistryRoundTrip(t *testing.T) {
	dir := t.TempDir()
	reg, err := NewRegistryAt(dir)
	if err != nil {
		t.Fatalf("NewRegistryAt: %v", err)
	}

	cfg := &ProviderConfig{
		ID:       "round-trip",
		Name:     "Round Trip",
		Category: "hotel",
		Endpoint: "https://example.com/search",
		ResponseMapping: ResponseMapping{
			ResultsPath: "results",
			Fields:      map[string]string{"name": "name"},
		},
		Version: 1,
	}

	if err := reg.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Verify file exists.
	fpath := filepath.Join(dir, "round-trip.json")
	if _, err := os.Stat(fpath); err != nil {
		t.Fatalf("config file not found: %v", err)
	}

	// Create new registry from same directory — should load config.
	reg2, err := NewRegistryAt(dir)
	if err != nil {
		t.Fatalf("NewRegistryAt reload: %v", err)
	}

	got := reg2.Get("round-trip")
	if got == nil {
		t.Fatal("Get returned nil after reload")
	}
	if got.Name != "Round Trip" {
		t.Errorf("Name = %q, want 'Round Trip'", got.Name)
	}

	// Test ListByCategory.
	hotels := reg2.ListByCategory("hotel")
	if len(hotels) != 1 {
		t.Errorf("ListByCategory hotel = %d, want 1", len(hotels))
	}

	// Test Delete.
	if err := reg2.Delete("round-trip"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if reg2.Get("round-trip") != nil {
		t.Error("Get after Delete returned non-nil")
	}
	if _, err := os.Stat(fpath); !os.IsNotExist(err) {
		t.Error("config file still exists after Delete")
	}
}

func TestRegistryMarkSuccessAndError(t *testing.T) {
	dir := t.TempDir()
	reg, err := NewRegistryAt(dir)
	if err != nil {
		t.Fatalf("NewRegistryAt: %v", err)
	}

	cfg := &ProviderConfig{
		ID:       "mark-test",
		Name:     "Mark Test",
		Category: "hotel",
		Endpoint: "https://example.com",
		ResponseMapping: ResponseMapping{
			ResultsPath: "results",
		},
	}
	if err := reg.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Mark error.
	reg.MarkError("mark-test", "connection timeout")
	got := reg.Get("mark-test")
	if got.ErrorCount != 1 {
		t.Errorf("ErrorCount = %d, want 1", got.ErrorCount)
	}
	if got.LastError != "connection timeout" {
		t.Errorf("LastError = %q, want 'connection timeout'", got.LastError)
	}

	// Mark another error.
	reg.MarkError("mark-test", "server error")
	got = reg.Get("mark-test")
	if got.ErrorCount != 2 {
		t.Errorf("ErrorCount = %d, want 2", got.ErrorCount)
	}

	// Mark success — resets error count.
	reg.MarkSuccess("mark-test")
	got = reg.Get("mark-test")
	if got.ErrorCount != 0 {
		t.Errorf("ErrorCount after success = %d, want 0", got.ErrorCount)
	}
	if got.LastSuccess.IsZero() {
		t.Error("LastSuccess is zero after MarkSuccess")
	}
}

func TestMapHotelResult(t *testing.T) {
	raw := map[string]any{
		"hotel_name": "Test Hotel",
		"id":         "t1",
		"star_count": 3.0,
		"user_rate":  4.2,
		"num_review": 55.0,
		"nightly":    129.50,
		"money":      "EUR",
		"location":   "Berlin, Germany",
		"geo_lat":    52.520008,
		"geo_lon":    13.404954,
		"link":       "https://example.com/book/t1",
		"eco":        true,
	}

	fields := map[string]string{
		"name":          "hotel_name",
		"hotel_id":      "id",
		"stars":         "star_count",
		"rating":        "user_rate",
		"review_count":  "num_review",
		"price":         "nightly",
		"currency":      "money",
		"address":       "location",
		"lat":           "geo_lat",
		"lon":           "geo_lon",
		"booking_url":   "link",
		"eco_certified": "eco",
	}

	h := mapHotelResult(raw, fields)

	if h.Name != "Test Hotel" {
		t.Errorf("Name = %q, want 'Test Hotel'", h.Name)
	}
	if h.HotelID != "t1" {
		t.Errorf("HotelID = %q, want 't1'", h.HotelID)
	}
	if h.Stars != 3 {
		t.Errorf("Stars = %d, want 3", h.Stars)
	}
	if h.Rating != 4.2 {
		t.Errorf("Rating = %f, want 4.2", h.Rating)
	}
	if h.ReviewCount != 55 {
		t.Errorf("ReviewCount = %d, want 55", h.ReviewCount)
	}
	if h.Price != 129.50 {
		t.Errorf("Price = %f, want 129.50", h.Price)
	}
	if h.Currency != "EUR" {
		t.Errorf("Currency = %q, want 'EUR'", h.Currency)
	}
	if h.Address != "Berlin, Germany" {
		t.Errorf("Address = %q, want 'Berlin, Germany'", h.Address)
	}
	if h.Lat != 52.520008 {
		t.Errorf("Lat = %f, want 52.520008", h.Lat)
	}
	if h.Lon != 13.404954 {
		t.Errorf("Lon = %f, want 13.404954", h.Lon)
	}
	if h.BookingURL != "https://example.com/book/t1" {
		t.Errorf("BookingURL = %q, want 'https://example.com/book/t1'", h.BookingURL)
	}
	if !h.EcoCertified {
		t.Error("EcoCertified = false, want true")
	}
}

func TestToFloat64(t *testing.T) {
	tests := []struct {
		input any
		want  float64
	}{
		{3.14, 3.14},
		{42, 42.0},
		{"99.5", 99.5},
		{true, 0},
		{nil, 0},
	}
	for _, tt := range tests {
		got := toFloat64(tt.input)
		if got != tt.want {
			t.Errorf("toFloat64(%v) = %f, want %f", tt.input, got, tt.want)
		}
	}
}

func TestToInt(t *testing.T) {
	tests := []struct {
		input any
		want  int
	}{
		{3.0, 3},
		{42, 42},
		{"99", 99},
		{true, 0},
		{nil, 0},
	}
	for _, tt := range tests {
		got := toInt(tt.input)
		if got != tt.want {
			t.Errorf("toInt(%v) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestSearchHotelsProviderError(t *testing.T) {
	// Mock server returning 500.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "internal error")
	}))
	defer srv.Close()

	dir := t.TempDir()
	reg, err := NewRegistryAt(dir)
	if err != nil {
		t.Fatalf("NewRegistryAt: %v", err)
	}

	cfg := &ProviderConfig{
		ID:       "error-test",
		Name:     "Error Test",
		Category: "hotel",
		Endpoint: srv.URL + "/search",
		Method:   "GET",
		ResponseMapping: ResponseMapping{
			ResultsPath: "results",
		},
		RateLimit: RateLimitConfig{
			RequestsPerSecond: 100,
			Burst:             10,
		},
	}
	if err := reg.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	rt := NewRuntime(reg)
	_, err = rt.SearchHotels(context.Background(), "Test", 0, 0, "2025-06-01", "2025-06-05", "USD", 2)
	if err == nil {
		t.Fatal("expected error from provider returning 500")
	}

	// Verify error was marked.
	got := reg.Get("error-test")
	if got.ErrorCount != 1 {
		t.Errorf("ErrorCount = %d, want 1", got.ErrorCount)
	}
}

func TestSearchHotelsContextCanceled(t *testing.T) {
	dir := t.TempDir()
	reg, err := NewRegistryAt(dir)
	if err != nil {
		t.Fatalf("NewRegistryAt: %v", err)
	}

	cfg := &ProviderConfig{
		ID:       "ctx-test",
		Name:     "Ctx Test",
		Category: "hotel",
		Endpoint: "https://example.com/search",
		Method:   "GET",
		ResponseMapping: ResponseMapping{
			ResultsPath: "results",
		},
		RateLimit: RateLimitConfig{
			RequestsPerSecond: 0.001, // very slow limiter to force context cancellation
			Burst:             1,
		},
	}
	if err := reg.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	rt := NewRuntime(reg)

	// Exhaust the burst token.
	pc := rt.getOrCreateClient(cfg)
	pc.limiter.Allow()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err = rt.SearchHotels(ctx, "Test", 0, 0, "2025-06-01", "2025-06-05", "USD", 2)
	if err == nil {
		t.Fatal("expected error from canceled context")
	}
}

func TestSearchHotelsPostMethod(t *testing.T) {
	// Mock server that verifies POST body.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %q, want POST", r.Method)
		}

		body := make([]byte, 1024)
		n, _ := r.Body.Read(body)
		bodyStr := string(body[:n])

		if !containsSubstring(bodyStr, `"checkin":"2025-06-01"`) {
			t.Errorf("body does not contain checkin: %s", bodyStr)
		}

		resp := map[string]any{
			"results": []any{
				map[string]any{"name": "POST Hotel"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	dir := t.TempDir()
	reg, err := NewRegistryAt(dir)
	if err != nil {
		t.Fatalf("NewRegistryAt: %v", err)
	}

	cfg := &ProviderConfig{
		ID:           "post-test",
		Name:         "POST Test",
		Category:     "hotel",
		Endpoint:     srv.URL + "/search",
		Method:       "POST",
		BodyTemplate: `{"checkin":"${checkin}","checkout":"${checkout}"}`,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		ResponseMapping: ResponseMapping{
			ResultsPath: "results",
			Fields: map[string]string{
				"name": "name",
			},
		},
		RateLimit: RateLimitConfig{
			RequestsPerSecond: 100,
			Burst:             10,
		},
	}
	if err := reg.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	rt := NewRuntime(reg)
	hotels, err := rt.SearchHotels(context.Background(), "Test", 0, 0, "2025-06-01", "2025-06-05", "USD", 2)
	if err != nil {
		t.Fatalf("SearchHotels: %v", err)
	}
	if len(hotels) != 1 {
		t.Fatalf("got %d hotels, want 1", len(hotels))
	}
	if hotels[0].Name != "POST Hotel" {
		t.Errorf("name = %q, want 'POST Hotel'", hotels[0].Name)
	}
}

func TestSubstituteEnvVars(t *testing.T) {
	t.Run("basic substitution", func(t *testing.T) {
		t.Setenv("TRVL_TEST_VAR", "hello-world")
		got := substituteEnvVars("key=${env.TRVL_TEST_VAR}")
		if got != "key=hello-world" {
			t.Errorf("got %q, want %q", got, "key=hello-world")
		}
	})

	t.Run("missing env var replaced with empty string", func(t *testing.T) {
		got := substituteEnvVars("key=${env.TRVL_NONEXISTENT_VAR_12345}")
		if got != "key=" {
			t.Errorf("got %q, want %q", got, "key=")
		}
	})

	t.Run("no env vars returns unchanged", func(t *testing.T) {
		input := "plain string without env references"
		got := substituteEnvVars(input)
		if got != input {
			t.Errorf("got %q, want %q", got, input)
		}
	})

	t.Run("multiple env vars in one string", func(t *testing.T) {
		t.Setenv("TRVL_TEST_A", "alpha")
		t.Setenv("TRVL_TEST_B", "beta")
		got := substituteEnvVars("${env.TRVL_TEST_A}-and-${env.TRVL_TEST_B}")
		if got != "alpha-and-beta" {
			t.Errorf("got %q, want %q", got, "alpha-and-beta")
		}
	})

	t.Run("malformed pattern without closing brace", func(t *testing.T) {
		// ${env. without closing } should stop iteration and return what it has.
		input := "prefix${env.FOO_BAR"
		got := substituteEnvVars(input)
		// The function breaks out of the loop when it cannot find closing }.
		if got != input {
			t.Errorf("got %q, want %q", got, input)
		}
	})

	t.Run("empty var name", func(t *testing.T) {
		// ${env.} has an empty variable name -- os.Getenv("") returns "".
		got := substituteEnvVars("val=${env.}")
		if got != "val=" {
			t.Errorf("got %q, want %q", got, "val=")
		}
	})
}

func TestRunPreflight_POST(t *testing.T) {
	// Mock OAuth2-style token endpoint.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("preflight method = %q, want POST", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Errorf("Content-Type = %q, want application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("Accept") != "application/json" {
			t.Errorf("Accept = %q, want application/json", r.Header.Get("Accept"))
		}

		// Read and verify body.
		body := make([]byte, 1024)
		n, _ := r.Body.Read(body)
		bodyStr := string(body[:n])
		if bodyStr != "api_key=test-key" {
			t.Errorf("preflight body = %q, want %q", bodyStr, "api_key=test-key")
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token":"test-token-123","expires_in":"3600"}`)
	}))
	defer srv.Close()

	// Mock search server that verifies auth token was extracted and used.
	searchSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer test-token-123" {
			t.Errorf("Authorization = %q, want 'Bearer test-token-123'", authHeader)
		}
		resp := map[string]any{
			"results": []any{
				map[string]any{"name": "Token Hotel", "id": "th1"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer searchSrv.Close()

	dir := t.TempDir()
	reg, err := NewRegistryAt(dir)
	if err != nil {
		t.Fatalf("NewRegistryAt: %v", err)
	}

	cfg := &ProviderConfig{
		ID:       "post-preflight-test",
		Name:     "POST Preflight Test",
		Category: "hotel",
		Endpoint: searchSrv.URL + "/search",
		Method:   "GET",
		Headers: map[string]string{
			"Authorization": "Bearer ${auth_token}",
		},
		Auth: &AuthConfig{
			Type:            "preflight",
			PreflightURL:    srv.URL + "/auth",
			PreflightMethod: "POST",
			PreflightBody:   "api_key=test-key",
			PreflightHeaders: map[string]string{
				"Content-Type": "application/x-www-form-urlencoded",
				"Accept":       "application/json",
			},
			Extractions: map[string]Extraction{
				"auth_token": {
					Pattern:  `"access_token":"([^"]+)"`,
					Variable: "auth_token",
				},
			},
		},
		ResponseMapping: ResponseMapping{
			ResultsPath: "results",
			Fields: map[string]string{
				"name":     "name",
				"hotel_id": "id",
			},
		},
		RateLimit: RateLimitConfig{
			RequestsPerSecond: 100,
			Burst:             10,
		},
	}

	if err := reg.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	rt := NewRuntime(reg)
	hotels, err := rt.SearchHotels(context.Background(), "Test", 0, 0, "2025-06-01", "2025-06-05", "USD", 2)
	if err != nil {
		t.Fatalf("SearchHotels: %v", err)
	}
	if len(hotels) != 1 {
		t.Fatalf("got %d hotels, want 1", len(hotels))
	}
	if hotels[0].Name != "Token Hotel" {
		t.Errorf("name = %q, want 'Token Hotel'", hotels[0].Name)
	}
}

func TestTestProvider_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"data": map[string]any{
				"results": []any{
					map[string]any{
						"name":    "Paris Grand Hotel",
						"id":      "pg1",
						"rating":  4.5,
						"price":   250.0,
						"curr":    "EUR",
						"addr":    "1 Rue de Rivoli",
						"geo_lat": 48.8566,
						"geo_lon": 2.3522,
					},
					map[string]any{
						"name":    "Budget Paris",
						"id":      "bp1",
						"rating":  3.2,
						"price":   89.0,
						"curr":    "EUR",
						"geo_lat": 48.857,
						"geo_lon": 2.353,
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	cfg := &ProviderConfig{
		ID:       "test-provider-success",
		Name:     "Test Provider Success",
		Category: "hotel",
		Endpoint: srv.URL + "/search",
		Method:   "GET",
		ResponseMapping: ResponseMapping{
			ResultsPath: "data.results",
			Fields: map[string]string{
				"name":     "name",
				"hotel_id": "id",
				"rating":   "rating",
				"price":    "price",
				"currency": "curr",
				"address":  "addr",
				"lat":      "geo_lat",
				"lon":      "geo_lon",
			},
		},
	}

	result := TestProvider(context.Background(), cfg, "Paris", 48.8566, 2.3522, "2026-05-01", "2026-05-02", "EUR", 2)

	if !result.Success {
		t.Fatalf("expected Success=true, got false; step=%s error=%s", result.Step, result.Error)
	}
	if result.Step != "complete" {
		t.Errorf("Step = %q, want 'complete'", result.Step)
	}
	if result.ResultsCount != 2 {
		t.Errorf("ResultsCount = %d, want 2", result.ResultsCount)
	}
	if result.SampleResult == nil {
		t.Fatal("SampleResult is nil, want non-nil")
	}
	if result.SampleResult["name"] != "Paris Grand Hotel" {
		t.Errorf("SampleResult name = %v, want 'Paris Grand Hotel'", result.SampleResult["name"])
	}
}

func TestTestProvider_PreflightFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "server error")
	}))
	defer srv.Close()

	cfg := &ProviderConfig{
		ID:       "preflight-fail",
		Name:     "Preflight Fail",
		Category: "hotel",
		Endpoint: srv.URL + "/search",
		Method:   "GET",
		Auth: &AuthConfig{
			Type:         "preflight",
			PreflightURL: srv.URL + "/auth",
			Extractions: map[string]Extraction{
				"token": {
					Pattern:  `"token":"([^"]+)"`,
					Variable: "token",
				},
			},
		},
		ResponseMapping: ResponseMapping{
			ResultsPath: "results",
			Fields:      map[string]string{"name": "name"},
		},
	}

	result := TestProvider(context.Background(), cfg, "Paris", 48.8566, 2.3522, "2026-05-01", "2026-05-02", "EUR", 2)

	if result.Success {
		t.Fatal("expected Success=false for preflight failure")
	}
	// The TestProvider function sets step to "auth_extraction" after reading the body,
	// then checks extractions. Since the 500 body is "server error" and the pattern
	// won't match, the failure should be in the extraction step.
	if result.Step != "auth_extraction" {
		t.Errorf("Step = %q, want 'auth_extraction'", result.Step)
	}
	if result.HTTPStatus != 500 {
		t.Errorf("HTTPStatus = %d, want 500", result.HTTPStatus)
	}
}

func TestTestProvider_BadResponseParse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<html><body>Not JSON</body></html>")
	}))
	defer srv.Close()

	cfg := &ProviderConfig{
		ID:       "bad-parse",
		Name:     "Bad Parse",
		Category: "hotel",
		Endpoint: srv.URL + "/search",
		Method:   "GET",
		ResponseMapping: ResponseMapping{
			ResultsPath: "results",
			Fields:      map[string]string{"name": "name"},
		},
	}

	result := TestProvider(context.Background(), cfg, "Paris", 48.8566, 2.3522, "2026-05-01", "2026-05-02", "EUR", 2)

	if result.Success {
		t.Fatal("expected Success=false for bad JSON response")
	}
	if result.Step != "response_parse" {
		t.Errorf("Step = %q, want 'response_parse'", result.Step)
	}
	if result.Error == "" {
		t.Error("Error is empty, want non-empty error message")
	}
}

func TestTestProvider_EmptyResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"data": map[string]any{
				"results": []any{},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	cfg := &ProviderConfig{
		ID:       "empty-results",
		Name:     "Empty Results",
		Category: "hotel",
		Endpoint: srv.URL + "/search",
		Method:   "GET",
		ResponseMapping: ResponseMapping{
			ResultsPath: "data.results",
			Fields:      map[string]string{"name": "name"},
		},
	}

	result := TestProvider(context.Background(), cfg, "Paris", 48.8566, 2.3522, "2026-05-01", "2026-05-02", "EUR", 2)

	if !result.Success {
		t.Fatalf("expected Success=true for empty results, got error: %s", result.Error)
	}
	if result.ResultsCount != 0 {
		t.Errorf("ResultsCount = %d, want 0", result.ResultsCount)
	}
	if result.SampleResult != nil {
		t.Errorf("SampleResult = %v, want nil", result.SampleResult)
	}
}

func TestSubstituteEnvVars_InPreflight(t *testing.T) {
	t.Setenv("TRVL_TEST_API_KEY", "secret123")

	var receivedBody string
	preflightSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, 1024)
		n, _ := r.Body.Read(body)
		receivedBody = string(body[:n])

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"token":"tok-abc"}`)
	}))
	defer preflightSrv.Close()

	searchSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"results": []any{
				map[string]any{"name": "Env Hotel", "id": "eh1"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer searchSrv.Close()

	dir := t.TempDir()
	reg, err := NewRegistryAt(dir)
	if err != nil {
		t.Fatalf("NewRegistryAt: %v", err)
	}

	cfg := &ProviderConfig{
		ID:       "env-preflight-test",
		Name:     "Env Preflight Test",
		Category: "hotel",
		Endpoint: searchSrv.URL + "/search",
		Method:   "GET",
		Headers: map[string]string{
			"Authorization": "Bearer ${auth_tok}",
		},
		Auth: &AuthConfig{
			Type:            "preflight",
			PreflightURL:    preflightSrv.URL + "/auth",
			PreflightMethod: "POST",
			PreflightBody:   "api_key=${env.TRVL_TEST_API_KEY}",
			PreflightHeaders: map[string]string{
				"Content-Type": "application/x-www-form-urlencoded",
			},
			Extractions: map[string]Extraction{
				"auth_tok": {
					Pattern:  `"token":"([^"]+)"`,
					Variable: "auth_tok",
				},
			},
		},
		ResponseMapping: ResponseMapping{
			ResultsPath: "results",
			Fields: map[string]string{
				"name":     "name",
				"hotel_id": "id",
			},
		},
		RateLimit: RateLimitConfig{
			RequestsPerSecond: 100,
			Burst:             10,
		},
	}

	if err := reg.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	rt := NewRuntime(reg)
	hotels, err := rt.SearchHotels(context.Background(), "Test", 0, 0, "2025-06-01", "2025-06-05", "USD", 2)
	if err != nil {
		t.Fatalf("SearchHotels: %v", err)
	}

	// Verify env var was substituted in preflight body.
	if receivedBody != "api_key=secret123" {
		t.Errorf("preflight body = %q, want %q", receivedBody, "api_key=secret123")
	}

	if len(hotels) != 1 {
		t.Fatalf("got %d hotels, want 1", len(hotels))
	}
	if hotels[0].Name != "Env Hotel" {
		t.Errorf("name = %q, want 'Env Hotel'", hotels[0].Name)
	}
}

func TestStatus(t *testing.T) {
	t.Run("new provider", func(t *testing.T) {
		cfg := &ProviderConfig{
			ID:   "status-new",
			Name: "New Provider",
		}
		if got := cfg.Status(); got != "new" {
			t.Errorf("Status() = %q, want 'new'", got)
		}
	})

	t.Run("provider with errors", func(t *testing.T) {
		cfg := &ProviderConfig{
			ID:         "status-error",
			Name:       "Error Provider",
			ErrorCount: 3,
			LastError:  "connection refused",
		}
		if got := cfg.Status(); got != "error" {
			t.Errorf("Status() = %q, want 'error'", got)
		}
	})

	t.Run("provider with success", func(t *testing.T) {
		cfg := &ProviderConfig{
			ID:          "status-ok",
			Name:        "OK Provider",
			LastSuccess: time.Now(),
		}
		if got := cfg.Status(); got != "ok" {
			t.Errorf("Status() = %q, want 'ok'", got)
		}
	})

	t.Run("errors take precedence over success", func(t *testing.T) {
		cfg := &ProviderConfig{
			ID:          "status-both",
			Name:        "Both Provider",
			LastSuccess: time.Now(),
			ErrorCount:  1,
		}
		if got := cfg.Status(); got != "error" {
			t.Errorf("Status() = %q, want 'error' (errors should take precedence)", got)
		}
	})
}

func TestIsStale(t *testing.T) {
	t.Run("no errors is not stale", func(t *testing.T) {
		cfg := &ProviderConfig{
			ID:         "stale-ok",
			ErrorCount: 0,
		}
		if cfg.IsStale() {
			t.Error("IsStale() = true, want false for provider with no errors")
		}
	})

	t.Run("errors with no success is stale", func(t *testing.T) {
		cfg := &ProviderConfig{
			ID:         "stale-never-success",
			ErrorCount: 2,
		}
		if !cfg.IsStale() {
			t.Error("IsStale() = false, want true for provider with errors and no success")
		}
	})

	t.Run("errors with recent success is not stale", func(t *testing.T) {
		cfg := &ProviderConfig{
			ID:          "stale-recent",
			ErrorCount:  1,
			LastSuccess: time.Now().Add(-1 * time.Hour), // 1 hour ago
		}
		if cfg.IsStale() {
			t.Error("IsStale() = true, want false for provider with recent success")
		}
	})

	t.Run("errors with old success is stale", func(t *testing.T) {
		cfg := &ProviderConfig{
			ID:          "stale-old",
			ErrorCount:  1,
			LastSuccess: time.Now().Add(-48 * time.Hour), // 48 hours ago
		}
		if !cfg.IsStale() {
			t.Error("IsStale() = false, want true for provider with old success and errors")
		}
	})
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsIdx(s, substr))
}

func containsIdx(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

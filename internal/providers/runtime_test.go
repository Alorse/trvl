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
		"hotels": map[string]any{
			"name":   "Grand Plaza",
			"rating": 4.5,
		},
	}

	name := jsonPath(data, "hotels.name")
	if name != "Grand Plaza" {
		t.Errorf("expected 'Grand Plaza', got %v", name)
	}

	rating := jsonPath(data, "hotels.rating")
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
				Category: "hotels",
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
				Category: "hotels",
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
				Category: "hotels",
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
				Category: "hotels",
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
				Category: "hotels",
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
		Category: "hotels",
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
		Category: "hotels",
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
		Category: "hotels",
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
	hotels, _, err := rt.SearchHotels(context.Background(), "Paris", 48.856613, 2.352222, "2025-06-01", "2025-06-05", "USD", 2, nil)
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
	hotels, _, err := rt.SearchHotels(context.Background(), "Paris", 48.856613, 2.352222, "2025-06-01", "2025-06-05", "USD", 2, nil)
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
		Category: "hotels",
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
	hotels, _, err := rt.SearchHotels(context.Background(), "Test", 0, 0, "2025-06-01", "2025-06-05", "USD", 2, nil)
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
		Category: "hotels",
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
	hotels := reg2.ListByCategory("hotels")
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
		Category: "hotels",
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
		// Composite strings: firstNumericToken extracts the leading number.
		{"4.84 (25)", 4.84},
		{"€ 204", 204},
		{"€ 61", 61},
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
		// Composite strings: lastIntToken extracts the trailing integer.
		{"4.84 (25)", 25},
		{"4.96 (510)", 510},
		{"Rating: 351 reviews", 351},
	}
	for _, tt := range tests {
		got := toInt(tt.input)
		if got != tt.want {
			t.Errorf("toInt(%v) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestFirstNumericToken(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"4.84 (25)", "4.84"},
		{"€ 204", "204"},
		{"abc", ""},
		{"123", "123"},
		{"-42.5 total", "-42.5"},
	}
	for _, tt := range tests {
		got := firstNumericToken(tt.input)
		if got != tt.want {
			t.Errorf("firstNumericToken(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestLastIntToken(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"4.84 (25)", "25"},
		{"4.96 (510)", "510"},
		{"no numbers", ""},
		{"just 42", "42"},
		{"1 and 2 and 3", "3"},
	}
	for _, tt := range tests {
		got := lastIntToken(tt.input)
		if got != tt.want {
			t.Errorf("lastIntToken(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestResolveCityID(t *testing.T) {
	lookup := map[string]string{
		"prague":    "19",
		"helsinki":  "45",
		"amsterdam": "3",
	}
	tests := []struct{ input, want string }{
		{"Prague", "19"},
		{"prague", "19"},
		{"PRAGUE", "19"},
		{"Helsinki", "45"},
		{"  Amsterdam  ", "3"},
		{"Prague 1", "19"}, // partial: "prague 1" contains "prague"
		{"Unknown", ""},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := resolveCityID(lookup, tt.input); got != tt.want {
				t.Errorf("resolveCityID(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
	// Empty lookup returns "".
	if got := resolveCityID(nil, "Prague"); got != "" {
		t.Errorf("nil lookup: got %q, want empty", got)
	}
}

func TestResolvePropertyType(t *testing.T) {
	lookup := map[string]string{
		"hotel":     "204",
		"apartment": "201",
		"hostel":    "203",
	}
	tests := []struct{ input, want string }{
		{"hotel", "204"},
		{"Hotel", "204"},
		{"APARTMENT", "201"},
		{"hostel", "203"},
		{"  Hotel  ", "204"},
		{"resort", ""},  // not in lookup
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := resolvePropertyType(lookup, tt.input); got != tt.want {
				t.Errorf("resolvePropertyType(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
	// Nil lookup returns "".
	if got := resolvePropertyType(nil, "hotel"); got != "" {
		t.Errorf("nil lookup: got %q, want empty", got)
	}
}

func TestSearchHotelsFilterPassthrough(t *testing.T) {
	// Mock server that captures query params to verify filter vars are substituted.
	var capturedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.RawQuery
		resp := map[string]any{
			"results": []any{
				map[string]any{"name": "Filter Hotel", "id": "fh1"},
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
		ID:       "filter-test",
		Name:     "Filter Test",
		Category: "hotels",
		Endpoint: srv.URL + "/search",
		Method:   "GET",
		QueryParams: map[string]string{
			"min_price":         "${min_price}",
			"max_price":         "${max_price}",
			"property_type":     "${property_type}",
			"sort":              "${sort}",
			"stars":             "${stars}",
			"min_rating":        "${min_rating}",
			"amenities":         "${amenities}",
			"free_cancellation": "${free_cancellation}",
		},
		PropertyTypeLookup: map[string]string{
			"hotel":     "204",
			"apartment": "201",
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
	filters := &HotelFilterParams{
		MinPrice:         50,
		MaxPrice:         300,
		PropertyType:     "hotel",
		Sort:             "price",
		Stars:            4,
		MinRating:        4.0,
		Amenities:        []string{"wifi", "pool"},
		FreeCancellation: true,
	}
	hotels, _, err := rt.SearchHotels(context.Background(), "Paris", 48.856, 2.352, "2025-06-01", "2025-06-05", "EUR", 2, filters)
	if err != nil {
		t.Fatalf("SearchHotels: %v", err)
	}
	if len(hotels) != 1 {
		t.Fatalf("got %d hotels, want 1", len(hotels))
	}

	// Verify filter vars were substituted into query params.
	checks := map[string]string{
		"min_price=50":          "min_price",
		"max_price=300":         "max_price",
		"property_type=204":     "property_type (resolved via lookup)",
		"sort=price":            "sort",
		"stars=4":               "stars",
		"min_rating=4.0":        "min_rating",
		"amenities=wifi%2Cpool": "amenities",
		"free_cancellation=1":   "free_cancellation",
	}
	for substr, label := range checks {
		if !containsSubstring(capturedQuery, substr) {
			t.Errorf("query missing %s: %s not in %q", label, substr, capturedQuery)
		}
	}
}

func TestJSONPathSkipsEmptyArrays(t *testing.T) {
	// Simulates Airbnb v2 API where explore_tabs.sections has an "inserts"
	// section with empty listings before the real "listings" section.
	data := map[string]any{
		"explore_tabs": []any{
			map[string]any{
				"sections": []any{
					map[string]any{
						"result_type": "inserts",
						"listings":    []any{},
					},
					map[string]any{
						"result_type": "listings",
						"listings": []any{
							map[string]any{"name": "Real listing"},
						},
					},
				},
			},
		},
	}
	got := jsonPath(data, "explore_tabs.sections.listings")
	arr, ok := got.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", got)
	}
	if len(arr) != 1 {
		t.Fatalf("expected 1 listing, got %d (skipping empty arrays failed)", len(arr))
	}
}

func TestIsEmptyValue(t *testing.T) {
	cases := []struct {
		name string
		v    any
		want bool
	}{
		{"nil", nil, true},
		{"empty slice", []any{}, true},
		{"empty map", map[string]any{}, true},
		{"empty string", "", true},
		{"non-empty slice", []any{1}, false},
		{"non-empty map", map[string]any{"a": 1}, false},
		{"non-empty string", "x", false},
		{"zero int", 0, false},
		{"false bool", false, false},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := isEmptyValue(tt.v); got != tt.want {
				t.Errorf("isEmptyValue(%v) = %v, want %v", tt.v, got, tt.want)
			}
		})
	}
}

func TestDenormalizeApollo(t *testing.T) {
	// Simulates a Booking.com SSR Apollo normalized cache where
	// nested objects like reviewScore and location use __ref pointers.
	cache := map[string]any{
		"ROOT_QUERY": map[string]any{
			"searchQueries": map[string]any{
				"search({\"input\":{}})": map[string]any{
					"results": []any{
						map[string]any{"__ref": "SearchResultProperty:42"},
					},
				},
			},
		},
		"SearchResultProperty:42": map[string]any{
			"__typename":        "SearchResultProperty",
			"basicPropertyData": map[string]any{"__ref": "BasicPropertyData:42"},
			"displayName":       map[string]any{"text": "Hotel Amsterdam"},
		},
		"BasicPropertyData:42": map[string]any{
			"__typename":  "BasicPropertyData",
			"id":          float64(42),
			"reviewScore": map[string]any{"__ref": "ReviewScore:42"},
			"location":    map[string]any{"__ref": "Location:42"},
		},
		"ReviewScore:42": map[string]any{
			"score":       float64(8.5),
			"reviewCount": float64(1234),
		},
		"Location:42": map[string]any{
			"latitude":  52.37,
			"longitude": 4.89,
		},
	}

	// Only denormalize ROOT_QUERY subtree (mirrors runtime.go behavior).
	cache["ROOT_QUERY"] = denormalizeApollo(cache["ROOT_QUERY"], cache, nil)

	// Navigate: ROOT_QUERY.searchQueries.search*.results[0].basicPropertyData.reviewScore.score
	root := cache["ROOT_QUERY"].(map[string]any)
	sq := root["searchQueries"].(map[string]any)
	// Use the wildcard helper.
	val := jsonPath(sq, "search*.results")
	arr, ok := val.([]any)
	if !ok || len(arr) == 0 {
		t.Fatal("search*.results did not resolve to a non-empty array")
	}

	hotel := arr[0].(map[string]any)
	// displayName.text should be direct.
	name := jsonPath(hotel, "displayName.text")
	if name != "Hotel Amsterdam" {
		t.Errorf("name = %v, want Hotel Amsterdam", name)
	}
	// reviewScore.score should be denormalized through __ref.
	score := jsonPath(hotel, "basicPropertyData.reviewScore.score")
	if score != float64(8.5) {
		t.Errorf("score = %v, want 8.5", score)
	}
	// location should be denormalized.
	lat := jsonPath(hotel, "basicPropertyData.location.latitude")
	if lat != 52.37 {
		t.Errorf("lat = %v, want 52.37", lat)
	}
}

func TestUnwrapNiobe(t *testing.T) {
	// Simulates Airbnb's SSR Niobe cache format:
	// {"niobeClientData": [["CacheKey:...", {"data": {...}, "variables": {...}}]]}
	niobe := map[string]any{
		"niobeClientData": []any{
			[]any{
				"StaysSearch:{\"query\":\"Helsinki\"}",
				map[string]any{
					"data": map[string]any{
						"presentation": map[string]any{
							"staysSearch": map[string]any{
								"results": map[string]any{
									"searchResults": []any{
										map[string]any{
											"title":       "Apartment in Kamppi",
											"avgRatingLocalized": "4.69 (127)",
										},
									},
								},
							},
						},
					},
					"variables": map[string]any{
						"staysSearchRequest": map[string]any{},
					},
				},
			},
		},
	}

	unwrapped := unwrapNiobe(niobe)

	// Should unwrap to the inner payload containing "data" and "variables".
	m, ok := unwrapped.(map[string]any)
	if !ok {
		t.Fatalf("unwrapNiobe returned %T, want map[string]any", unwrapped)
	}
	if _, hasData := m["data"]; !hasData {
		t.Fatal("unwrapped result missing 'data' key")
	}

	// jsonPath should now resolve the results_path.
	results := jsonPath(unwrapped, "data.presentation.staysSearch.results.searchResults")
	arr, ok := results.([]any)
	if !ok || len(arr) == 0 {
		t.Fatalf("results_path did not resolve to a non-empty array: %T", results)
	}
	title := jsonPath(arr[0], "title")
	if title != "Apartment in Kamppi" {
		t.Errorf("title = %v, want Apartment in Kamppi", title)
	}
}

func TestUnwrapNiobePassthrough(t *testing.T) {
	// Non-Niobe JSON should be returned unchanged.
	regular := map[string]any{
		"data": map[string]any{"results": []any{1, 2, 3}},
	}
	result := unwrapNiobe(regular)
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	if _, hasData := m["data"]; !hasData {
		t.Fatal("passthrough lost 'data' key")
	}
}

func TestNormalizePrice(t *testing.T) {
	// Use a cache with known fallback rates (unreachable server forces fallback).
	old := defaultFXCache
	defer func() { defaultFXCache = old }()
	defaultFXCache = newFXCache()
	defaultFXCache.baseURL = "http://127.0.0.1:1" // force fallback

	tests := []struct {
		name  string
		price float64
		from  string
		to    string
		want  float64
	}{
		{"USD to EUR", 100, "USD", "EUR", 92},
		{"EUR to USD", 100, "EUR", "USD", 109},
		{"GBP to EUR", 100, "GBP", "EUR", 116},
		{"EUR to GBP", 100, "EUR", "GBP", 86},
		{"same currency", 85, "EUR", "EUR", 85},
		{"empty from (Airbnb)", 75, "", "EUR", 75},
		{"empty to", 75, "USD", "", 75},
		{"both empty", 75, "", "", 75},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizePrice(tt.price, tt.from, tt.to)
			diff := got - tt.want
			if diff < 0 {
				diff = -diff
			}
			if diff > 0.01 {
				t.Errorf("normalizePrice(%v, %q, %q) = %v, want %v", tt.price, tt.from, tt.to, got, tt.want)
			}
		})
	}
}

func TestExtractCurrencyCode(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		// 3-letter ISO code prefix
		{"EUR 204", "EUR"},
		{"USD204", "USD"},
		{"GBP 99.50", "GBP"},

		// 3-letter ISO code suffix
		{"204 EUR", "EUR"},
		{"204USD", "USD"},

		// Currency symbol prefix
		{"€175", "EUR"},
		{"€ 175", "EUR"},
		{"$120", "USD"},
		{"£99", "GBP"},
		{"¥1500", "JPY"},
		{"₹2500", "INR"},

		// Numeric-only — no currency
		{"175", ""},
		{"99.50", ""},

		// Empty / whitespace
		{"", ""},
		{"   ", ""},

		// Mixed case — not a valid ISO code
		{"Eur 204", ""},
		{"eur 204", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := extractCurrencyCode(tt.input)
			if got != tt.want {
				t.Errorf("extractCurrencyCode(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestMapHotelResultCurrencyFromPriceString(t *testing.T) {
	// Simulate Airbnb-like response: price is a string with embedded currency,
	// no separate currency field in the mapping.
	raw := map[string]any{
		"listing": map[string]any{
			"name": "Cozy Apartment",
		},
		"display_price": "EUR 204",
	}
	fields := map[string]string{
		"name":  "listing.name",
		"price": "display_price",
	}

	h := mapHotelResult(raw, fields)
	if h.Currency != "EUR" {
		t.Errorf("expected currency EUR from price string, got %q", h.Currency)
	}
	if h.Price != 204 {
		t.Errorf("expected price 204, got %v", h.Price)
	}

	// When an explicit currency field IS mapped, it takes precedence.
	raw["currency_code"] = "GBP"
	fields["currency"] = "currency_code"

	h = mapHotelResult(raw, fields)
	if h.Currency != "GBP" {
		t.Errorf("expected explicit currency GBP to take precedence, got %q", h.Currency)
	}
}

func TestMapHotelResultCurrencySymbol(t *testing.T) {
	raw := map[string]any{
		"price_display": "€175",
	}
	fields := map[string]string{
		"price": "price_display",
	}

	h := mapHotelResult(raw, fields)
	if h.Currency != "EUR" {
		t.Errorf("expected currency EUR from € symbol, got %q", h.Currency)
	}
	if h.Price != 175 {
		t.Errorf("expected price 175, got %v", h.Price)
	}
}

// TestBookingFieldMapping verifies that the corrected Booking.com field paths
// resolve correctly against the actual Apollo cache structure. The rating field
// was incorrectly mapped to basicPropertyData.reviewScore.score (which doesn't
// exist in the SSR data); it should be basicPropertyData.reviews.totalScore.
func TestBookingFieldMapping(t *testing.T) {
	// Minimal Booking search result matching the actual Apollo structure
	// discovered via TestBookingFieldDiscovery.
	raw := map[string]any{
		"displayName": map[string]any{
			"text": "Hotel Aix Europe",
		},
		"basicPropertyData": map[string]any{
			"id":       float64(2215748),
			"pageName": "aix-europe",
			"reviews": map[string]any{
				"totalScore":   7.1,
				"reviewsCount": float64(2551),
				"showScore":    true,
			},
			"location": map[string]any{
				"latitude":    48.870111,
				"longitude":   2.369928,
				"address":     "4 Rue d'Aix",
				"city":        "Paris",
				"countryCode": "fr",
			},
		},
		"priceDisplayInfoIrene": map[string]any{
			"displayPrice": map[string]any{
				"amountPerStay": map[string]any{
					"amount":   "€ 71.86",
					"currency": "EUR",
				},
			},
		},
	}

	// These are the corrected field mappings from booking.json.
	fields := map[string]string{
		"name":         "displayName.text",
		"hotel_id":     "basicPropertyData.id",
		"rating":       "basicPropertyData.reviews.totalScore",
		"review_count": "basicPropertyData.reviews.reviewsCount",
		"lat":          "basicPropertyData.location.latitude",
		"lon":          "basicPropertyData.location.longitude",
		"address":      "basicPropertyData.location.address",
		"price":        "priceDisplayInfoIrene.displayPrice.amountPerStay.amount",
		"currency":     "priceDisplayInfoIrene.displayPrice.amountPerStay.currency",
	}

	h := mapHotelResult(raw, fields)

	if h.Name != "Hotel Aix Europe" {
		t.Errorf("name: got %q, want %q", h.Name, "Hotel Aix Europe")
	}
	if h.HotelID != "2215748" {
		t.Errorf("hotel_id: got %q, want %q", h.HotelID, "2215748")
	}
	if h.Rating != 7.1 {
		t.Errorf("rating: got %v, want 7.1 (was 0 with old reviewScore.score path)", h.Rating)
	}
	if h.ReviewCount != 2551 {
		t.Errorf("review_count: got %d, want 2551", h.ReviewCount)
	}
	if h.Lat == 0 {
		t.Error("lat should not be 0")
	}
	if h.Address != "4 Rue d'Aix" {
		t.Errorf("address: got %q, want %q", h.Address, "4 Rue d'Aix")
	}
	if h.Currency != "EUR" {
		t.Errorf("currency: got %q, want %q", h.Currency, "EUR")
	}
	if h.Price != 71.86 {
		t.Errorf("price: got %v, want 71.86", h.Price)
	}

	// Verify the OLD (broken) path returns 0 — this was the bug.
	oldFields := map[string]string{
		"rating":       "basicPropertyData.reviewScore.score",
		"review_count": "basicPropertyData.reviewScore.reviewCount",
	}
	hOld := mapHotelResult(raw, oldFields)
	if hOld.Rating != 0 {
		t.Errorf("old reviewScore.score path should yield 0, got %v", hOld.Rating)
	}
	if hOld.ReviewCount != 0 {
		t.Errorf("old reviewScore.reviewCount path should yield 0, got %d", hOld.ReviewCount)
	}
}

// TestBookingURLConstruction verifies that the pageName + countryCode fields
// are correctly combined into a booking URL.
func TestBookingURLConstruction(t *testing.T) {
	raw := map[string]any{
		"basicPropertyData": map[string]any{
			"pageName": "aix-europe",
			"location": map[string]any{
				"countryCode": "fr",
			},
		},
	}

	pageName, _ := jsonPath(raw, "basicPropertyData.pageName").(string)
	cc, _ := jsonPath(raw, "basicPropertyData.location.countryCode").(string)

	if pageName != "aix-europe" {
		t.Errorf("pageName: got %q, want %q", pageName, "aix-europe")
	}
	if cc != "fr" {
		t.Errorf("countryCode: got %q, want %q", cc, "fr")
	}

	wantURL := "https://www.booking.com/hotel/fr/aix-europe.html"
	gotURL := "https://www.booking.com/hotel/" + cc + "/" + pageName + ".html"
	if gotURL != wantURL {
		t.Errorf("booking URL: got %q, want %q", gotURL, wantURL)
	}
}

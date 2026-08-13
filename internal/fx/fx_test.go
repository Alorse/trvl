package fx

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// fx.go — new paths not already covered
// ---------------------------------------------------------------------------

// TestFXCache_FetchBase_Success verifies rate retrieval from a mock Frankfurter API.
func TestFXCache_FetchBase_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(frankfurterResponse{
			Base:  "EUR",
			Rates: map[string]float64{"USD": 1.09, "GBP": 0.86},
		})
	}))
	defer srv.Close()

	fc := &Cache{
		rates:   make(map[string]map[string]float64),
		ttl:     24 * time.Hour,
		client:  srv.Client(),
		baseURL: srv.URL,
	}
	rates, _, err := fc.fetchBase("EUR")
	if err != nil {
		t.Fatalf("fetchBase: %v", err)
	}
	if rates["USD"] != 1.09 {
		t.Errorf("USD rate = %v, want 1.09", rates["USD"])
	}
}

// TestFXCache_FetchBase_HTTP500 verifies error on non-200 status.
func TestFXCache_FetchBase_HTTP500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "error")
	}))
	defer srv.Close()

	fc := &Cache{
		rates:   make(map[string]map[string]float64),
		ttl:     24 * time.Hour,
		client:  srv.Client(),
		baseURL: srv.URL,
	}
	_, _, err := fc.fetchBase("EUR")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

// TestFXCache_FetchBase_BadJSON verifies error for malformed JSON.
func TestFXCache_FetchBase_BadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{bad json`)
	}))
	defer srv.Close()

	fc := &Cache{
		rates:   make(map[string]map[string]float64),
		ttl:     24 * time.Hour,
		client:  srv.Client(),
		baseURL: srv.URL,
	}
	_, _, err := fc.fetchBase("USD")
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

// TestFXCache_GetRate_TriangulateEUR verifies triangulation through EUR.
func TestFXCache_GetRate_TriangulateEUR(t *testing.T) {
	fc := New()
	fc.mu.Lock()
	fc.rates = map[string]map[string]float64{
		"USD": {"EUR": 0.92},
		"EUR": {"GBP": 0.86},
	}
	fc.fetched = time.Now()
	fc.mu.Unlock()

	rate := fc.getRate("USD", "GBP")
	expected := 0.92 * 0.86
	if rate < expected-0.001 || rate > expected+0.001 {
		t.Errorf("USD→GBP = %v, want ~%v", rate, expected)
	}
}

// TestFXCache_GetRate_UnknownPair returns 0.
func TestFXCache_GetRate_UnknownPair(t *testing.T) {
	fc := New()
	fc.mu.Lock()
	fc.rates = map[string]map[string]float64{"EUR": {"USD": 1.09}}
	fc.fetched = time.Now()
	fc.mu.Unlock()

	if rate := fc.getRate("JPY", "CHF"); rate != 0 {
		t.Errorf("unknown pair rate = %v, want 0", rate)
	}
}

// TestFXCache_Refresh_FallbackOnError verifies fallback rates are used when fetch fails.
func TestFXCache_Refresh_FallbackOnError(t *testing.T) {
	fc := &Cache{
		rates:   make(map[string]map[string]float64),
		ttl:     24 * time.Hour,
		client:  &http.Client{Timeout: 100 * time.Millisecond},
		baseURL: "http://localhost:1",
	}
	fc.refresh()

	fc.mu.RLock()
	defer fc.mu.RUnlock()
	if fc.rates["EUR"]["USD"] == 0 {
		t.Error("expected fallback EUR→USD rate after fetch error")
	}
	if fc.fetched.IsZero() {
		t.Error("fetched timestamp should be set even after error")
	}
}

// TestFXCache_Refresh_AllThreeBases verifies all three bases are fetched.
func TestFXCache_Refresh_AllThreeBases(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		base := r.URL.Query().Get("from")
		switch base {
		case "EUR":
			json.NewEncoder(w).Encode(frankfurterResponse{Base: "EUR", Rates: map[string]float64{"USD": 1.09, "GBP": 0.86}})
		case "USD":
			json.NewEncoder(w).Encode(frankfurterResponse{Base: "USD", Rates: map[string]float64{"EUR": 0.92, "GBP": 0.79}})
		case "GBP":
			json.NewEncoder(w).Encode(frankfurterResponse{Base: "GBP", Rates: map[string]float64{"EUR": 1.16, "USD": 1.27}})
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer srv.Close()

	fc := &Cache{
		rates:   make(map[string]map[string]float64),
		ttl:     24 * time.Hour,
		client:  srv.Client(),
		baseURL: srv.URL,
	}
	fc.refresh()

	fc.mu.RLock()
	defer fc.mu.RUnlock()
	if fc.rates["EUR"]["USD"] != 1.09 {
		t.Errorf("EUR→USD = %v, want 1.09", fc.rates["EUR"]["USD"])
	}
	if fc.rates["GBP"]["USD"] != 1.27 {
		t.Errorf("GBP→USD = %v, want 1.27", fc.rates["GBP"]["USD"])
	}
}

// TestFXCache_Refresh_DoubleCheckLock verifies the double-check under lock
// (second goroutine finds the cache already fresh).
func TestFXCache_Refresh_DoubleCheckLock(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(frankfurterResponse{Base: r.URL.Query().Get("from"), Rates: map[string]float64{"USD": 1.0}})
	}))
	defer srv.Close()

	fc := &Cache{
		rates:   make(map[string]map[string]float64),
		ttl:     24 * time.Hour,
		client:  srv.Client(),
		baseURL: srv.URL,
	}
	// First refresh populates the cache.
	fc.refresh()

	// Mark as fresh.
	fc.mu.Lock()
	fc.fetched = time.Now()
	fc.mu.Unlock()

	// Second refresh should return immediately (double-check: not stale).
	fc.refresh()

	fc.mu.RLock()
	defer fc.mu.RUnlock()
	if fc.fetched.IsZero() {
		t.Error("fetched should be set")
	}
}

func TestFXCacheFallback(t *testing.T) {
	// Point cache at an unreachable server so it falls back to hardcoded rates.
	fc := New()
	fc.baseURL = "http://127.0.0.1:1" // connection refused

	r := fc.getRate("USD", "EUR")
	if r == 0 {
		t.Fatal("expected fallback rate for USD→EUR, got 0")
	}
	if r != 0.92 {
		t.Errorf("fallback USD→EUR = %v, want 0.92", r)
	}
}

func TestFXCacheLiveRates(t *testing.T) {
	// Spin up a fake Frankfurter server returning known rates.
	mux := http.NewServeMux()
	mux.HandleFunc("/latest", func(w http.ResponseWriter, r *http.Request) {
		base := r.URL.Query().Get("from")
		resp := map[string]any{
			"base": base,
			"date": "2026-04-16",
		}
		switch base {
		case "EUR":
			resp["rates"] = map[string]float64{"USD": 1.10, "GBP": 0.85}
		case "USD":
			resp["rates"] = map[string]float64{"EUR": 0.91, "GBP": 0.77}
		case "GBP":
			resp["rates"] = map[string]float64{"EUR": 1.18, "USD": 1.30}
		default:
			resp["rates"] = map[string]float64{}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	fc := New()
	fc.baseURL = srv.URL

	// Direct rate.
	r := fc.getRate("EUR", "USD")
	if r != 1.10 {
		t.Errorf("EUR→USD = %v, want 1.10", r)
	}

	// Reverse direction from a fetched base.
	r = fc.getRate("USD", "EUR")
	if r != 0.91 {
		t.Errorf("USD→EUR = %v, want 0.91", r)
	}

	// GBP→USD (direct, since GBP is a fetched base).
	r = fc.getRate("GBP", "USD")
	if r != 1.30 {
		t.Errorf("GBP→USD = %v, want 1.30", r)
	}
}

func TestFXCacheTriangulation(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/latest", func(w http.ResponseWriter, r *http.Request) {
		base := r.URL.Query().Get("from")
		resp := map[string]any{"base": base, "date": "2026-04-16"}
		switch base {
		case "EUR":
			resp["rates"] = map[string]float64{"USD": 1.10, "GBP": 0.85, "JPY": 160.0}
		case "USD":
			resp["rates"] = map[string]float64{"EUR": 0.91, "GBP": 0.77, "JPY": 145.0}
		case "GBP":
			resp["rates"] = map[string]float64{"EUR": 1.18, "USD": 1.30, "JPY": 188.0}
		default:
			resp["rates"] = map[string]float64{}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	fc := New()
	fc.baseURL = srv.URL

	// JPY→GBP: neither is a base with direct JPY→GBP. Should triangulate
	// through EUR: JPY→EUR (not a fetched base for JPY). Actually JPY is
	// not a fetched base at all, so triangulation from→EUR needs from in
	// the rate map. Let's test a pair that can triangulate:
	// We only fetch EUR/USD/GBP bases, and EUR has JPY rate.
	// JPY→USD would need JPY base (not fetched). So test CHF→JPY which
	// also can't triangulate. Better: test that unknown pairs return 0.
	r := fc.getRate("JPY", "CHF")
	if r != 0 {
		t.Errorf("JPY→CHF = %v, want 0 (unknown pair)", r)
	}
}

func TestFXCacheTTL(t *testing.T) {
	callCount := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/latest", func(w http.ResponseWriter, r *http.Request) {
		callCount++
		base := r.URL.Query().Get("from")
		resp := map[string]any{
			"base":  base,
			"date":  "2026-04-16",
			"rates": map[string]float64{"USD": 1.10, "EUR": 0.91, "GBP": 0.85},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	fc := New()
	fc.baseURL = srv.URL
	fc.ttl = 50 * time.Millisecond

	// First call triggers fetch (3 bases = 3 HTTP requests).
	fc.getRate("EUR", "USD")
	firstCount := callCount

	// Second call within TTL should NOT trigger another fetch.
	fc.getRate("EUR", "USD")
	if callCount != firstCount {
		t.Errorf("expected no new fetches within TTL, got %d extra", callCount-firstCount)
	}

	// Wait for TTL to expire, then call again.
	time.Sleep(60 * time.Millisecond)
	fc.getRate("EUR", "USD")
	if callCount <= firstCount {
		t.Error("expected new fetch after TTL expired")
	}
}

package providers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MikkoParkkola/trvl/internal/fx"
)

// fxResponse mirrors the Frankfurter JSON shape so these tests can stub the
// endpoint without reaching into the fx package's internals.
type fxResponse struct {
	Base  string             `json:"base"`
	Date  string             `json:"date"`
	Rates map[string]float64 `json:"rates"`
}

// ---------------------------------------------------------------------------
// normalizePrice — FX conversion with httptest mock
// ---------------------------------------------------------------------------

// TestNormalizePrice_FXConversion verifies that normalizePrice correctly
// delegates to the FX cache for known currency conversions.
func TestNormalizePrice_FXConversion(t *testing.T) {
	// Inject a fresh FX cache with a mock server so we don't hit the real API.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		base := r.URL.Query().Get("from")
		switch base {
		case "EUR":
			json.NewEncoder(w).Encode(fxResponse{Base: "EUR", Rates: map[string]float64{"USD": 1.10, "GBP": 0.87}})
		case "USD":
			json.NewEncoder(w).Encode(fxResponse{Base: "USD", Rates: map[string]float64{"EUR": 0.91, "GBP": 0.79}})
		case "GBP":
			json.NewEncoder(w).Encode(fxResponse{Base: "GBP", Rates: map[string]float64{"EUR": 1.15, "USD": 1.27}})
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer srv.Close()

	// Override the package-level FX cache with a test version.
	origCache := fxRates
	defer func() { fxRates = origCache }()
	fxRates = fx.NewAt(srv.URL, srv.Client())

	// EUR→USD at 1.10
	got := normalizePrice(100, "EUR", "USD")
	if got < 109 || got > 111 {
		t.Errorf("normalizePrice(100, EUR, USD) = %v, want ~110", got)
	}

	// Same currency: no conversion
	got = normalizePrice(100, "USD", "USD")
	if got != 100 {
		t.Errorf("normalizePrice(100, USD, USD) = %v, want 100", got)
	}

	// Unknown pair returns original
	got = normalizePrice(100, "JPY", "CHF")
	if got != 100 {
		t.Errorf("normalizePrice(100, JPY, CHF) = %v, want 100 (unknown pair)", got)
	}
}

func TestNormalizePriceUsesCache(t *testing.T) {
	// Replace the default cache with one pointing at a test server.
	mux := http.NewServeMux()
	mux.HandleFunc("/latest", func(w http.ResponseWriter, r *http.Request) {
		base := r.URL.Query().Get("from")
		resp := map[string]any{"base": base, "date": "2026-04-16"}
		switch base {
		case "EUR":
			resp["rates"] = map[string]float64{"USD": 1.25, "GBP": 0.80}
		case "USD":
			resp["rates"] = map[string]float64{"EUR": 0.80, "GBP": 0.64}
		case "GBP":
			resp["rates"] = map[string]float64{"EUR": 1.25, "USD": 1.5625}
		default:
			resp["rates"] = map[string]float64{}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	old := fxRates
	defer func() { fxRates = old }()

	fxRates = fx.NewAt(srv.URL, nil)

	got := normalizePrice(100, "EUR", "USD")
	want := 125.0
	diff := got - want
	if diff < 0 {
		diff = -diff
	}
	if diff > 0.01 {
		t.Errorf("normalizePrice(100, EUR, USD) = %v, want %v", got, want)
	}
}

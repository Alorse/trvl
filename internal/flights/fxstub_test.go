package flights

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MikkoParkkola/trvl/internal/fx"
)

// stubFXRates points the bag-fee converter at a stub publishing known rates,
// and returns a function restoring the previous one.
//
// Conversion has to be pinned rather than live: a test asserting an all-in
// total built from today's ECB rate would pass today and fail tomorrow for a
// reason that has nothing to do with the code. eurRates maps currency → units
// per EUR, matching how the ECB publishes its table.
func stubFXRates(t *testing.T, eurRates map[string]float64, asOf string) func() {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		base := r.URL.Query().Get("from")
		rates := map[string]float64{}
		if base == "EUR" {
			rates = eurRates
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"base": base, "date": asOf, "rates": rates,
		})
	}))

	prev := fxRates
	fxRates = fx.NewAt(srv.URL, srv.Client())
	return func() {
		fxRates = prev
		srv.Close()
	}
}

// approx compares money to the nearest hundredth, so a float representation
// detail never fails a test about baggage.
func approx(got, want float64) bool {
	d := got - want
	return d < 0.005 && d > -0.005
}

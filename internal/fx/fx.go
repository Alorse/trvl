// Package fx converts money between currencies, and says which rate it used
// and when that rate was published.
//
// The provenance matters as much as the number. A converted figure is not the
// same claim as a published one: an airline that charges GBP 65 for a bag has
// not stated a euro price, and a total built from that conversion is only as
// good as the rate behind it. Every conversion here therefore carries a Rate
// describing where it came from, so callers can pass that on rather than
// present a derived figure as if it were read off the airline's own page.
//
// Rates come from the Frankfurter API, which wraps the ECB's daily reference
// rates: free, no key, one fetch per 24h. When that fetch fails the cache falls
// back to hardcoded approximations covering only EUR, USD and GBP, marked as
// such so callers can tell an ECB rate from a guess.
package fx

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Source says where a rate came from, because the two are not equally good.
type Source string

const (
	// SourceECB — the ECB's daily reference rate for a stated day, via
	// Frankfurter.
	SourceECB Source = "ecb"
	// SourceFallback — a hardcoded approximation used when the live fetch
	// failed. Close enough to rank options, never good enough to bill from,
	// and carrying no date because it has none.
	SourceFallback Source = "fallback"
)

// Rate is a conversion factor together with its provenance.
type Rate struct {
	From   string  `json:"from"`
	To     string  `json:"to"`
	Rate   float64 `json:"rate"`
	AsOf   string  `json:"as_of,omitempty"` // YYYY-MM-DD the rate was published; empty for fallbacks
	Source Source  `json:"source"`
}

// String renders a rate the way it should appear in a citation.
func (r Rate) String() string {
	if r.AsOf != "" {
		return fmt.Sprintf("1 %s = %g %s (ECB %s)", r.From, r.Rate, r.To, r.AsOf)
	}
	return fmt.Sprintf("1 %s = %g %s (approximate, no live rate available)", r.From, r.Rate, r.To)
}

// Cache holds rates fetched from Frankfurter, refreshed at most once per ttl.
type Cache struct {
	mu      sync.RWMutex
	rates   map[string]map[string]float64 // base -> target -> rate
	asOf    map[string]string             // base -> YYYY-MM-DD the rates were published
	fetched time.Time
	ttl     time.Duration
	client  *http.Client
	baseURL string // overridable for tests
}

// frankfurterResponse is the JSON shape returned by
// https://api.frankfurter.app/latest?from=CUR
type frankfurterResponse struct {
	Base  string             `json:"base"`
	Date  string             `json:"date"`
	Rates map[string]float64 `json:"rates"`
}

// fallbackRates are used when the live fetch fails. They are intentionally
// approximate — close enough for cross-provider comparison, never for billing.
var fallbackRates = map[string]map[string]float64{
	"EUR": {"USD": 1.09, "GBP": 0.86},
	"USD": {"EUR": 0.92, "GBP": 0.79},
	"GBP": {"EUR": 1.16, "USD": 1.38},
}

// Default is the package-level cache.
var Default = New()

// New builds an empty cache pointed at the live Frankfurter API.
func New() *Cache {
	return &Cache{
		rates:   make(map[string]map[string]float64),
		asOf:    make(map[string]string),
		ttl:     24 * time.Hour,
		client:  &http.Client{Timeout: 5 * time.Second},
		baseURL: "https://api.frankfurter.app",
	}
}

// NewAt builds a cache pointed at a Frankfurter-compatible endpoint. Exported
// so that tests in other packages can substitute a stub server rather than
// reach the live API.
func NewAt(baseURL string, client *http.Client) *Cache {
	c := New()
	c.baseURL = baseURL
	if client != nil {
		c.client = client
	}
	return c
}

// Convert converts amount from one currency to another using the default
// cache, returning the rate it used. ok is false when no rate is known for the
// pair, in which case callers must not invent one.
//
// Identical currencies convert at 1 without consulting any source.
func Convert(amount float64, from, to string) (float64, Rate, bool) {
	return Default.Convert(amount, from, to)
}

// Convert converts amount between currencies, reporting the rate used.
func (c *Cache) Convert(amount float64, from, to string) (float64, Rate, bool) {
	r, ok := c.Rate(from, to)
	if !ok {
		return 0, Rate{}, false
	}
	return amount * r.Rate, r, true
}

// Rate returns the conversion factor from→to with its provenance.
func (c *Cache) Rate(from, to string) (Rate, bool) {
	if from == "" || to == "" {
		return Rate{}, false
	}
	if from == to {
		return Rate{From: from, To: to, Rate: 1, Source: SourceECB}, true
	}
	v := c.getRate(from, to)
	if v <= 0 {
		return Rate{}, false
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	r := Rate{From: from, To: to, Rate: v, Source: SourceFallback}
	// A rate is only as dated as the base it was derived from. Prefer the
	// base actually consulted; EUR is the one every derivation passes through.
	for _, base := range []string{from, to, "EUR"} {
		if d := c.asOf[base]; d != "" {
			r.AsOf, r.Source = d, SourceECB
			break
		}
	}
	return r, true
}

// getRate returns the conversion rate from→to as a bare number. It refreshes
// the cache when stale and falls back to hardcoded rates on error. Returns 0
// when no rate is known for the pair.
func (c *Cache) getRate(from, to string) float64 {
	c.mu.RLock()
	fresh := !c.fetched.IsZero() && time.Since(c.fetched) < c.ttl
	c.mu.RUnlock()

	if !fresh {
		c.refresh()
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	if from == to {
		return 1
	}

	// Direct rate: from→to.
	if targets, ok := c.rates[from]; ok {
		if r, ok := targets[to]; ok {
			return r
		}
	}

	// Only EUR, USD and GBP are fetched as bases, but the ECB's EUR table
	// quotes dozens of currencies — JPY, KRW, HKD among them. Inverting that
	// table reaches every one of them, which is what makes a bag fee published
	// in yen comparable to a fare quoted in euros.
	inv := func(base, cur string) float64 {
		if targets, ok := c.rates[base]; ok {
			if r, ok := targets[cur]; ok && r > 0 {
				return 1 / r
			}
		}
		return 0
	}

	if to == "EUR" {
		return inv("EUR", from)
	}
	if from == "EUR" {
		if targets, ok := c.rates["EUR"]; ok {
			return targets[to]
		}
		return 0
	}

	// Triangulate through EUR: from→EUR then EUR→to.
	fromToEUR := 0.0
	if targets, ok := c.rates[from]; ok {
		fromToEUR = targets["EUR"]
	}
	if fromToEUR == 0 {
		fromToEUR = inv("EUR", from)
	}
	eurToTo := 0.0
	if targets, ok := c.rates["EUR"]; ok {
		eurToTo = targets[to]
	}
	if fromToEUR > 0 && eurToTo > 0 {
		return fromToEUR * eurToTo
	}

	return 0
}

// refresh fetches live rates for the three base currencies most commonly seen
// in results (EUR, USD, GBP). On any error it populates the cache from
// fallbackRates so that conversion always has something to work with.
func (c *Cache) refresh() {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock — another goroutine may have
	// refreshed while we were waiting.
	if !c.fetched.IsZero() && time.Since(c.fetched) < c.ttl {
		return
	}

	bases := []string{"EUR", "USD", "GBP"}
	newRates := make(map[string]map[string]float64, len(bases))
	newAsOf := make(map[string]string, len(bases))
	ok := true

	for _, base := range bases {
		rates, date, err := c.fetchBase(base)
		if err != nil {
			ok = false
			break
		}
		newRates[base] = rates
		newAsOf[base] = date
	}

	if ok {
		c.rates, c.asOf = newRates, newAsOf
	} else {
		// Use fallback rates — copy so we don't alias the package var. No
		// dates: these figures were never published on any particular day,
		// and dating them would dress a guess up as a reading.
		for base, targets := range fallbackRates {
			m := make(map[string]float64, len(targets))
			for k, v := range targets {
				m[k] = v
			}
			newRates[base] = m
		}
		c.rates, c.asOf = newRates, map[string]string{}
	}
	c.fetched = time.Now()
}

// fetchBase calls the Frankfurter API for a single base currency, returning its
// rates and the day they were published.
func (c *Cache) fetchBase(base string) (map[string]float64, string, error) {
	url := fmt.Sprintf("%s/latest?from=%s", c.baseURL, base)
	resp, err := c.client.Get(url)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("frankfurter: %s", resp.Status)
	}

	var fr frankfurterResponse
	if err := json.NewDecoder(resp.Body).Decode(&fr); err != nil {
		return nil, "", err
	}
	return fr.Rates, fr.Date, nil
}

package destinations

import (
	"context"
	"testing"
	"time"
)

// seedCurrencyCache directly sets cache entries for testing without making HTTP calls.
func seedCurrencyCache(rates map[string]float64) {
	currencyCache.Lock()
	defer currencyCache.Unlock()
	currencyCache.rates = rates
	currencyCache.fetched = time.Now()
}

func TestConvertCurrency_SameCurrency(t *testing.T) {
	got, cur := ConvertCurrency(context.Background(), 100.0, "EUR", "EUR")
	if got != 100.0 || cur != "EUR" {
		t.Errorf("expected (100.0, EUR), got (%v, %v)", got, cur)
	}
}

func TestConvertCurrency_ZeroAmount(t *testing.T) {
	got, cur := ConvertCurrency(context.Background(), 0, "USD", "EUR")
	if got != 0 || cur != "EUR" {
		t.Errorf("expected (0, EUR), got (%v, %v)", got, cur)
	}
}

func TestConvertCurrency_EmptyFrom(t *testing.T) {
	got, cur := ConvertCurrency(context.Background(), 100.0, "", "EUR")
	if got != 100.0 || cur != "EUR" {
		t.Errorf("expected unchanged amount, got (%v, %v)", got, cur)
	}
}

func TestConvertCurrency_EmptyTo(t *testing.T) {
	got, cur := ConvertCurrency(context.Background(), 100.0, "USD", "")
	if got != 100.0 {
		t.Errorf("expected unchanged amount, got (%v, %v)", got, cur)
	}
}

func TestConvertCurrency_EURToUSD_CachedRates(t *testing.T) {
	seedCurrencyCache(map[string]float64{
		"EUR": 1.0,
		"USD": 1.1,
	})

	got, cur := ConvertCurrency(context.Background(), 100.0, "EUR", "USD")
	if cur != "USD" {
		t.Errorf("expected USD currency, got %q", cur)
	}
	// 100 EUR * 1.1 = 110 USD.
	if got < 109.0 || got > 111.0 {
		t.Errorf("expected ~110 USD, got %v", got)
	}
}

func TestConvertCurrency_USDToEUR_CachedRates(t *testing.T) {
	seedCurrencyCache(map[string]float64{
		"EUR": 1.0,
		"USD": 1.1,
	})

	got, cur := ConvertCurrency(context.Background(), 110.0, "USD", "EUR")
	if cur != "EUR" {
		t.Errorf("expected EUR currency, got %q", cur)
	}
	// 110 / 1.1 = 100 EUR.
	if got < 99.0 || got > 101.0 {
		t.Errorf("expected ~100 EUR, got %v", got)
	}
}

func TestConvertCurrency_CrossRate_CachedRates(t *testing.T) {
	// USD → GBP via EUR base: USD=1.1, GBP=0.85
	// 110 USD / 1.1 * 0.85 = 85 GBP
	seedCurrencyCache(map[string]float64{
		"EUR": 1.0,
		"USD": 1.1,
		"GBP": 0.85,
	})

	got, cur := ConvertCurrency(context.Background(), 110.0, "USD", "GBP")
	if cur != "GBP" {
		t.Errorf("expected GBP, got %q", cur)
	}
	if got < 84.0 || got > 86.0 {
		t.Errorf("expected ~85 GBP, got %v", got)
	}
}

func TestConvertCurrency_UnknownCurrency_Fallback(t *testing.T) {
	// When currency not in cache, returns original amount in original currency.
	seedCurrencyCache(map[string]float64{"EUR": 1.0})

	got, cur := ConvertCurrency(context.Background(), 100.0, "XYZ", "EUR")
	// from rate not found → returns original.
	if got != 100.0 || cur != "XYZ" {
		t.Errorf("expected (100, XYZ) fallback, got (%v, %v)", got, cur)
	}
}

func TestConvertToEUR_Passthrough(t *testing.T) {
	// Same from/to.
	got, cur := ConvertToEUR(context.Background(), 50.0, "EUR")
	if got != 50.0 || cur != "EUR" {
		t.Errorf("expected (50, EUR), got (%v, %v)", got, cur)
	}
}

func TestConvertToEUR_WithCachedRates(t *testing.T) {
	seedCurrencyCache(map[string]float64{
		"EUR": 1.0,
		"JPY": 160.0,
	})

	got, cur := ConvertToEUR(context.Background(), 1600.0, "JPY")
	if cur != "EUR" {
		t.Errorf("expected EUR, got %q", cur)
	}
	// 1600 JPY / 160 = 10 EUR.
	if got < 9.5 || got > 10.5 {
		t.Errorf("expected ~10 EUR, got %v", got)
	}
}

func TestFetchCurrency_EmptyCode(t *testing.T) {
	result, err := FetchCurrency(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.BaseCurrency != "EUR" {
		t.Errorf("expected EUR base, got %q", result.BaseCurrency)
	}
}

func TestFetchCurrency_FromCache(t *testing.T) {
	seedCurrencyCache(map[string]float64{
		"EUR": 1.0,
		"GBP": 0.85,
	})

	result, err := FetchCurrency(context.Background(), "GBP")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.LocalCurrency != "GBP" {
		t.Errorf("expected GBP, got %q", result.LocalCurrency)
	}
	if result.ExchangeRate < 0.8 || result.ExchangeRate > 0.9 {
		t.Errorf("expected ~0.85 rate, got %v", result.ExchangeRate)
	}
}

func TestFetchCurrency_CacheHitUnknownCurrency(t *testing.T) {
	// Cache is populated but doesn't contain ZZZ.
	seedCurrencyCache(map[string]float64{"EUR": 1.0, "USD": 1.1})

	result, err := FetchCurrency(context.Background(), "ZZZ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Returns zero-value exchange rate (not in cache), but no error.
	if result.ExchangeRate != 0 {
		t.Errorf("expected 0 rate for unknown currency, got %v", result.ExchangeRate)
	}
}

// currencyCache type is verified transitively by the test functions above
// that call currencyCache.Lock/Unlock/RLock directly.

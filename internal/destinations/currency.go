package destinations

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/MikkoParkkola/trvl/internal/models"
)

const exchangeRateURL = "https://api.exchangerate-api.com/v4/latest/EUR"

// currencyCache stores the full exchange rate table.
var currencyCache = struct {
	sync.RWMutex
	rates   map[string]float64
	fetched time.Time
}{rates: make(map[string]float64)}

const currencyCacheTTL = 6 * time.Hour

// exchangeRateResponse is the JSON shape from exchangerate-api.com.
type exchangeRateResponse struct {
	Base  string             `json:"base"`
	Rates map[string]float64 `json:"rates"`
}

// FetchCurrency retrieves the exchange rate for a currency code vs EUR.
func FetchCurrency(ctx context.Context, currencyCode string) (models.CurrencyInfo, error) {
	if currencyCode == "" {
		return models.CurrencyInfo{BaseCurrency: "EUR"}, nil
	}

	currencyCache.RLock()
	if len(currencyCache.rates) > 0 && time.Since(currencyCache.fetched) < currencyCacheTTL {
		rate, ok := currencyCache.rates[currencyCode]
		currencyCache.RUnlock()
		if ok {
			return models.CurrencyInfo{
				LocalCurrency: currencyCode,
				ExchangeRate:  rate,
				BaseCurrency:  "EUR",
			}, nil
		}
		return models.CurrencyInfo{
			LocalCurrency: currencyCode,
			BaseCurrency:  "EUR",
		}, nil
	}
	currencyCache.RUnlock()

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, exchangeRateAPIURL, nil)
	if err != nil {
		return models.CurrencyInfo{}, fmt.Errorf("create currency request: %w", err)
	}
	req.Header.Set("User-Agent", "trvl/1.0 (destination currency)")

	resp, err := client.Do(req)
	if err != nil {
		return models.CurrencyInfo{}, fmt.Errorf("currency request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return models.CurrencyInfo{}, fmt.Errorf("read currency response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return models.CurrencyInfo{}, fmt.Errorf("exchangerate-api returned status %d: %s", resp.StatusCode, string(body))
	}

	var erResp exchangeRateResponse
	if err := json.Unmarshal(body, &erResp); err != nil {
		return models.CurrencyInfo{}, fmt.Errorf("parse currency response: %w", err)
	}

	currencyCache.Lock()
	currencyCache.rates = erResp.Rates
	currencyCache.fetched = time.Now()
	currencyCache.Unlock()

	rate, ok := erResp.Rates[currencyCode]
	if !ok {
		return models.CurrencyInfo{
			LocalCurrency: currencyCode,
			BaseCurrency:  "EUR",
		}, nil
	}

	return models.CurrencyInfo{
		LocalCurrency: currencyCode,
		ExchangeRate:  rate,
		BaseCurrency:  "EUR",
	}, nil
}

package destinations

// This file provides test-only helpers for overriding API URLs and clearing caches.
// The package uses unexported variables for URLs to allow test substitution.

// Mutable URL variables (package-level constants replaced with vars for testability).
var (
	nominatimAPIURL     = nominatimURL
	openMeteoAPIURL     = openMeteoURL
	restCountriesAPIURL = restCountriesURL
	nagerDateAPIURL     = nagerDateURL
	travelAdvisoryAPIURL = travelAdvisoryURL
	exchangeRateAPIURL  = exchangeRateURL
)

// setTestNominatimURL overrides the Nominatim URL and returns the previous value.
func setTestNominatimURL(url string) string {
	prev := nominatimAPIURL
	nominatimAPIURL = url
	return prev
}

// setTestOpenMeteoURL overrides the Open-Meteo URL and returns the previous value.
func setTestOpenMeteoURL(url string) string {
	prev := openMeteoAPIURL
	openMeteoAPIURL = url
	return prev
}

// setTestRestCountriesURL overrides the REST Countries URL and returns the previous value.
func setTestRestCountriesURL(url string) string {
	prev := restCountriesAPIURL
	restCountriesAPIURL = url
	return prev
}

// setTestNagerDateURL overrides the Nager.Date URL and returns the previous value.
func setTestNagerDateURL(url string) string {
	prev := nagerDateAPIURL
	nagerDateAPIURL = url
	return prev
}

// setTestTravelAdvisoryURL overrides the travel-advisory.info URL and returns the previous value.
func setTestTravelAdvisoryURL(url string) string {
	prev := travelAdvisoryAPIURL
	travelAdvisoryAPIURL = url
	return prev
}

// setTestExchangeRateURL overrides the ExchangeRate URL and returns the previous value.
func setTestExchangeRateURL(url string) string {
	prev := exchangeRateAPIURL
	exchangeRateAPIURL = url
	return prev
}

// clearAllCaches resets all in-memory caches for test isolation.
func clearAllCaches() {
	geoCache.Lock()
	geoCache.entries = make(map[string]GeoResult)
	geoCache.Unlock()

	weatherCache.Lock()
	weatherCache.entries = make(map[string]weatherCacheEntry)
	weatherCache.Unlock()

	countryCache.Lock()
	countryCache.entries = make(map[string]countryCacheEntry)
	countryCache.Unlock()

	holidayCache.Lock()
	holidayCache.entries = make(map[string]holidayCacheEntry)
	holidayCache.Unlock()

	safetyCache.Lock()
	safetyCache.entries = make(map[string]safetyCacheEntry)
	safetyCache.Unlock()

	currencyCache.Lock()
	currencyCache.rates = make(map[string]float64)
	currencyCache.Unlock()
}

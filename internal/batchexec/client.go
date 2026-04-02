// Package batchexec provides an HTTP client for Google's internal batchexecute API.
//
// Google's travel frontends (Flights, Hotels) communicate via a protocol that
// POSTs form-encoded "f.req" payloads and returns JSON with an anti-XSSI prefix.
// This package handles TLS fingerprint impersonation (Chrome via utls), request
// encoding, and response decoding.
package batchexec

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/MikkoParkkola/trvl/internal/cache"
	utls "github.com/refraction-networking/utls"
	"golang.org/x/time/rate"
)

// Endpoint constants for Google Travel APIs.
const (
	FlightsURL          = "https://www.google.com/_/FlightsFrontendUi/data/travel.frontend.flights.FlightsFrontendService/GetShoppingResults"
	ExploreURL          = "https://www.google.com/_/FlightsFrontendUi/data/travel.frontend.flights.FlightsFrontendService/GetExploreDestinations"
	CalendarGraphURL    = "https://www.google.com/_/FlightsFrontendUi/data/travel.frontend.flights.FlightsFrontendService/GetCalendarGraph"
	CalendarGridURL     = "https://www.google.com/_/FlightsFrontendUi/data/travel.frontend.flights.FlightsFrontendService/GetCalendarGrid"
	HotelsURL           = "https://www.google.com/_/TravelFrontendUi/data/batchexecute"
)

// chromeUA is a recent Chrome User-Agent string.
const chromeUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

// Default retry configuration.
const (
	defaultMaxRetries  = 3
	defaultBaseBackoff = 1 * time.Second
)

// Cache TTL constants for different endpoint types.
const (
	FlightCacheTTL      = 5 * time.Minute
	HotelCacheTTL       = 10 * time.Minute
	DestinationCacheTTL = 1 * time.Hour
)

// Client wraps an http.Client with Chrome TLS fingerprint impersonation via utls.
// It includes a token bucket rate limiter, retry with exponential backoff,
// and an in-memory response cache.
type Client struct {
	http     *http.Client
	limiter  *rate.Limiter
	cache    *cache.Cache
	noCache  bool
}

// NewClient creates a Client that impersonates Chrome's TLS fingerprint.
//
// Chrome's ClientHello is used for TLS fingerprinting, but we force HTTP/1.1
// via ALPN to avoid the complexity of HTTP/2 framing with custom TLS connections.
// Google's servers support HTTP/1.1 and this is sufficient for API access.
//
// The client includes a token bucket rate limiter at 10 requests/second with
// burst of 1, and automatic retry with exponential backoff for 429/5xx errors.
func NewClient() *Client {
	transport := &http.Transport{
		DialTLSContext:      dialTLSChromeHTTP1,
		MaxIdleConns:        10,
		IdleConnTimeout:     30 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		// Force HTTP/1.1 — we handle TLS ourselves and net/http can't do HTTP/2
		// on externally-provided TLS connections without extra wiring.
		ForceAttemptHTTP2: false,
	}
	return &Client{
		http: &http.Client{
			Transport: transport,
			Timeout:   20 * time.Second,
		},
		limiter: rate.NewLimiter(rate.Limit(10), 1),
		cache:   cache.New(),
	}
}

// SetNoCache disables the response cache for this client.
func (c *Client) SetNoCache(disable bool) {
	c.noCache = disable
}

// getCached returns a cached response if available and caching is enabled.
func (c *Client) getCached(endpoint, payload string) ([]byte, bool) {
	if c.noCache || c.cache == nil {
		return nil, false
	}
	return c.cache.Get(cache.Key(endpoint, payload))
}

// setCached stores a response in the cache with the appropriate TTL.
func (c *Client) setCached(endpoint, payload string, data []byte, ttl time.Duration) {
	if c.noCache || c.cache == nil {
		return
	}
	c.cache.Set(cache.Key(endpoint, payload), data, ttl)
}

// SetRateLimit changes the rate limiter to allow rps requests per second.
// A burst of 1 is used to enforce strict spacing between requests.
func (c *Client) SetRateLimit(rps float64) {
	c.limiter = rate.NewLimiter(rate.Limit(rps), 1)
}

// dialTLSChromeHTTP1 dials a TCP connection and wraps it with a utls client
// that impersonates Chrome's TLS ClientHello but forces HTTP/1.1 via ALPN.
//
// We start from HelloChrome_Auto's spec (Chrome-like cipher suites, extensions,
// curves, etc.) but override the ALPN extension to only advertise "http/1.1".
// The UClient is created with HelloCustom so ApplyPreset installs our modified
// spec rather than ignoring it in favour of a built-in profile.
//
// Coverage exclusion: creates a raw TLS connection with Chrome fingerprint.
// Not unit-testable: requires real TCP connection + TLS handshake with remote server.
// Covered by integration tests (proof_test.go).
func dialTLSChromeHTTP1(ctx context.Context, network, addr string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	rawConn, err := dialer.DialContext(ctx, network, addr)
	if err != nil {
		return nil, fmt.Errorf("dial tcp: %w", err)
	}

	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		rawConn.Close()
		return nil, fmt.Errorf("split host: %w", err)
	}

	// Build a Chrome-like spec but with ALPN forced to HTTP/1.1.
	spec, err := utls.UTLSIdToSpec(utls.HelloChrome_Auto)
	if err != nil {
		rawConn.Close()
		return nil, fmt.Errorf("utls spec: %w", err)
	}
	for _, ext := range spec.Extensions {
		if alpn, ok := ext.(*utls.ALPNExtension); ok {
			alpn.AlpnProtocols = []string{"http/1.1"}
			break
		}
	}

	// HelloCustom tells utls to use our spec verbatim instead of a preset.
	uConn := utls.UClient(rawConn, &utls.Config{
		ServerName: host,
	}, utls.HelloCustom)

	if err := uConn.ApplyPreset(&spec); err != nil {
		uConn.Close()
		return nil, fmt.Errorf("apply preset: %w", err)
	}

	if err := uConn.HandshakeContext(ctx); err != nil {
		uConn.Close()
		return nil, fmt.Errorf("utls handshake: %w", err)
	}

	return uConn, nil
}

// Get performs a GET request with Chrome headers.
// The request is subject to rate limiting and automatic retry on 429/5xx.
func (c *Client) Get(ctx context.Context, url string) (int, []byte, error) {
	return c.doWithRetry(ctx, func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", chromeUA)
		return req, nil
	})
}

// PostForm sends a POST with form-encoded body to the given URL. It sets the
// Content-Type to application/x-www-form-urlencoded and uses a Chrome User-Agent.
// The request is subject to rate limiting and automatic retry on 429/5xx.
func (c *Client) PostForm(ctx context.Context, url, formBody string) (int, []byte, error) {
	return c.doWithRetry(ctx, func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(formBody))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=UTF-8")
		req.Header.Set("User-Agent", chromeUA)
		req.Header.Set("Accept", "*/*")
		req.Header.Set("Accept-Language", "en-US,en;q=0.9")
		req.Header.Set("Origin", "https://www.google.com")
		req.Header.Set("Referer", "https://www.google.com/travel/flights")
		return req, nil
	})
}

// doWithRetry executes an HTTP request with rate limiting and retry logic.
// It retries up to 3 times on 429 (rate limit) and 5xx (server error) responses,
// with exponential backoff (1s, 2s, 4s) plus jitter (+-25%).
// Client errors (4xx except 429) are not retried.
func (c *Client) doWithRetry(ctx context.Context, buildReq func() (*http.Request, error)) (int, []byte, error) {
	var lastStatus int
	var lastBody []byte
	var lastErr error

	for attempt := range defaultMaxRetries + 1 {
		// Wait for rate limiter before each attempt.
		if err := c.limiter.Wait(ctx); err != nil {
			return 0, nil, fmt.Errorf("rate limiter: %w", err)
		}

		req, err := buildReq()
		if err != nil {
			return 0, nil, err
		}

		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = err
			if attempt < defaultMaxRetries {
				if sleepErr := backoffSleep(ctx, attempt); sleepErr != nil {
					return 0, nil, sleepErr
				}
				continue
			}
			return 0, nil, lastErr
		}

		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024)) // 10MB limit
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			if attempt < defaultMaxRetries {
				if sleepErr := backoffSleep(ctx, attempt); sleepErr != nil {
					return 0, nil, sleepErr
				}
				continue
			}
			return 0, nil, lastErr
		}

		lastStatus = resp.StatusCode
		lastBody = body
		lastErr = nil

		// Don't retry on success or non-retryable client errors.
		if !isRetryable(resp.StatusCode) {
			return lastStatus, lastBody, nil
		}

		// Retryable error — backoff before next attempt (unless this was the last).
		if attempt < defaultMaxRetries {
			if sleepErr := backoffSleep(ctx, attempt); sleepErr != nil {
				return 0, nil, sleepErr
			}
		}
	}

	// All retries exhausted.
	if lastErr != nil {
		return 0, nil, lastErr
	}
	return lastStatus, lastBody, nil
}

// isRetryable returns true for HTTP status codes that should trigger a retry:
// 429 (Too Many Requests) and 5xx (server errors).
func isRetryable(statusCode int) bool {
	return statusCode == 429 || statusCode >= 500
}

// backoffSleep sleeps for exponential backoff duration with jitter.
// Base delay is 1s, doubling each attempt: 1s, 2s, 4s.
// Jitter adds +-25% randomness to prevent thundering herd.
func backoffSleep(ctx context.Context, attempt int) error {
	base := defaultBaseBackoff << attempt // 1s, 2s, 4s
	// Add jitter: +-25%
	jitter := time.Duration(float64(base) * (0.75 + rand.Float64()*0.5))

	timer := time.NewTimer(jitter)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// SearchFlights posts an encoded flight search payload to the Flights endpoint
// and returns the raw response body.
//
// Results are cached for 5 minutes. Use SetNoCache(true) to bypass.
//
// Coverage exclusion: thin wrapper around doWithRetry with endpoint-specific URL.
// doWithRetry is thoroughly tested (client_test.go, client_extra_test.go).
// Covered by integration proof tests.
func (c *Client) SearchFlights(ctx context.Context, encodedFilters string) (int, []byte, error) {
	payload := "f.req=" + encodedFilters
	if data, ok := c.getCached(FlightsURL, payload); ok {
		return 200, data, nil
	}
	status, body, err := c.PostForm(ctx, FlightsURL, payload)
	if err == nil && status == 200 {
		c.setCached(FlightsURL, payload, body, FlightCacheTTL)
	}
	return status, body, err
}

// BatchExecute posts an encoded batchexecute payload to the Hotels/Travel endpoint
// and returns the raw response body.
//
// Results are cached for 10 minutes. Use SetNoCache(true) to bypass.
//
// Coverage exclusion: thin wrapper around doWithRetry with endpoint-specific URL.
// doWithRetry is thoroughly tested (client_test.go, client_extra_test.go).
// Covered by integration proof tests.
func (c *Client) BatchExecute(ctx context.Context, encodedPayload string) (int, []byte, error) {
	payload := "f.req=" + encodedPayload
	if data, ok := c.getCached(HotelsURL, payload); ok {
		return 200, data, nil
	}
	status, body, err := c.PostForm(ctx, HotelsURL, payload)
	if err == nil && status == 200 {
		c.setCached(HotelsURL, payload, body, HotelCacheTTL)
	}
	return status, body, err
}

// PostExplore posts an encoded payload to the GetExploreDestinations endpoint.
//
// Results are cached for 1 hour. Use SetNoCache(true) to bypass.
//
// Coverage exclusion: thin wrapper around PostForm with endpoint-specific URL.
// PostForm and doWithRetry are thoroughly tested. Covered by integration proof tests.
func (c *Client) PostExplore(ctx context.Context, encodedPayload string) (int, []byte, error) {
	payload := "f.req=" + encodedPayload
	if data, ok := c.getCached(ExploreURL, payload); ok {
		return 200, data, nil
	}
	status, body, err := c.PostForm(ctx, ExploreURL, payload)
	if err == nil && status == 200 {
		c.setCached(ExploreURL, payload, body, DestinationCacheTTL)
	}
	return status, body, err
}

// PostCalendarGraph posts an encoded payload to the GetCalendarGraph endpoint.
//
// Results are cached for 5 minutes. Use SetNoCache(true) to bypass.
//
// Coverage exclusion: thin wrapper around PostForm with endpoint-specific URL.
// PostForm and doWithRetry are thoroughly tested. Covered by integration proof tests.
func (c *Client) PostCalendarGraph(ctx context.Context, encodedPayload string) (int, []byte, error) {
	payload := "f.req=" + encodedPayload
	if data, ok := c.getCached(CalendarGraphURL, payload); ok {
		return 200, data, nil
	}
	status, body, err := c.PostForm(ctx, CalendarGraphURL, payload)
	if err == nil && status == 200 {
		c.setCached(CalendarGraphURL, payload, body, FlightCacheTTL)
	}
	return status, body, err
}

// PostCalendarGrid posts an encoded payload to the GetCalendarGrid endpoint.
//
// Results are cached for 5 minutes. Use SetNoCache(true) to bypass.
//
// Coverage exclusion: thin wrapper around PostForm with endpoint-specific URL.
// PostForm and doWithRetry are thoroughly tested. Covered by integration proof tests.
func (c *Client) PostCalendarGrid(ctx context.Context, encodedPayload string) (int, []byte, error) {
	payload := "f.req=" + encodedPayload
	if data, ok := c.getCached(CalendarGridURL, payload); ok {
		return 200, data, nil
	}
	status, body, err := c.PostForm(ctx, CalendarGridURL, payload)
	if err == nil && status == 200 {
		c.setCached(CalendarGridURL, payload, body, FlightCacheTTL)
	}
	return status, body, err
}

// ErrBlocked is returned when Google responds with 403 Forbidden.
var ErrBlocked = errors.New("request blocked (403)")

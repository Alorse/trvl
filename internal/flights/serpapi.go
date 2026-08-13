package flights

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/MikkoParkkola/trvl/internal/batchexec"
	"github.com/MikkoParkkola/trvl/internal/models"
)

// serpKeyTimeout bounds how long we wait for the external serp-key router.
const serpKeyTimeout = 5 * time.Second

// serpAttempts is how many times SearchSerpApi tries (each with a fresh rotated
// key from serp-key) before giving up and letting the caller fall to Duffel.
const serpAttempts = 2

// serpAPIBaseURL is the SerpApi search endpoint; overridable in tests.
var serpAPIBaseURL = "https://serpapi.com/search"

// serpKeyCmd is the external rotating-key command. Defaults to "serp-key" on
// PATH; overridable via TRVL_SERP_KEY_CMD (also the test seam for a fake script).
func serpKeyCmd() string {
	if v := strings.TrimSpace(os.Getenv("TRVL_SERP_KEY_CMD")); v != "" {
		return v
	}
	return "serp-key"
}

// serpKeyFunc fetches one rotated SerpApi key. Indirected for tests.
var serpKeyFunc = execSerpKey

// execSerpKey runs the serp-key command and returns the single key on stdout.
// Contract: stdout = raw key, exit 0 on success, exit 1 if none available.
func execSerpKey(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, serpKeyTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, serpKeyCmd())
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("serp-key: %w (%s)", err, strings.TrimSpace(errb.String()))
	}
	key := strings.TrimSpace(out.String())
	if key == "" {
		return "", fmt.Errorf("serp-key: empty key")
	}
	return key, nil
}

// SerpEnabled reports whether the serp-key router command is available, gating
// SerpApi in the fallback chain. When false, SerpApi is silently skipped.
func SerpEnabled() bool {
	_, err := exec.LookPath(serpKeyCmd())
	return err == nil
}

type serpMultiCityLeg struct {
	DepartureID string `json:"departure_id"`
	ArrivalID   string `json:"arrival_id"`
	Date        string `json:"date"`
}

// SearchSerpApi fetches Google Flights data through SerpApi for the given slices,
// retrying once with a freshly rotated serp-key on failure. Returns mapped
// FlightResults, or an error so the caller falls back to Duffel. Round-trip v1
// returns the correct total price with outbound legs only.
func SearchSerpApi(ctx context.Context, slices []DuffelSlice, opts SearchOptions) ([]models.FlightResult, error) {
	opts.defaults()
	if len(slices) == 0 {
		return nil, fmt.Errorf("serpapi: at least one slice required")
	}
	var lastErr error
	for i := 0; i < serpAttempts; i++ {
		key, err := serpKeyFunc(ctx)
		if err != nil {
			lastErr = fmt.Errorf("serpapi: %w", err)
			slog.Warn("serpapi serp-key failed, retrying", "attempt", i, "error", err)
			continue
		}
		results, err := searchSerpOnce(ctx, key, slices, opts)
		if err != nil {
			lastErr = err
			slog.Warn("serpapi attempt failed, rotating key", "attempt", i, "error", err)
			continue
		}
		if len(results) == 0 {
			lastErr = fmt.Errorf("serpapi: no results")
			continue
		}
		return results, nil
	}
	return nil, fmt.Errorf("serpapi: all attempts failed: %w", lastErr)
}

// searchSerpOnce performs a single SerpApi request with one key.
func searchSerpOnce(ctx context.Context, key string, slices []DuffelSlice, opts SearchOptions) ([]models.FlightResult, error) {
	query, err := buildSerpQuery(slices, key, opts)
	if err != nil {
		return nil, fmt.Errorf("serpapi: build query: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, serpAPIBaseURL+"?"+query, nil)
	if err != nil {
		return nil, fmt.Errorf("serpapi: build request: %w", err)
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("serpapi: request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("serpapi: unexpected status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("serpapi: read response: %w", err)
	}
	if apiErr := serpAPIError(body); apiErr != nil {
		return nil, apiErr
	}

	// With output=html the body is Google's own batchexecute payload, so it goes
	// through the exact decoder the google_flights provider uses. SerpApi's
	// normalized JSON drops the bag allowance; the raw payload keeps it.
	inner, err := batchexec.DecodeFlightResponse(body)
	if err != nil {
		return nil, fmt.Errorf("serpapi: decode google payload: %w", err)
	}
	raw, err := batchexec.ExtractFlightData(inner)
	if err != nil {
		return nil, fmt.Errorf("serpapi: extract flights: %w", err)
	}

	results := parseFlights(raw)
	for i := range results {
		results[i].Provider = "google_serpapi"
		if results[i].Currency == "" {
			results[i].Currency = opts.Currency
		}
	}
	return results, nil
}

// serpAPIError reports SerpApi's JSON error envelope. Success bodies on the
// output=html path are not JSON, so a leading '{' is the discriminator.
func serpAPIError(body []byte) error {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return nil
	}
	var envelope struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(trimmed, &envelope); err != nil {
		return fmt.Errorf("serpapi: unrecognized response: %w", err)
	}
	if envelope.Error != "" {
		return fmt.Errorf("serpapi: api error: %s", envelope.Error)
	}
	return nil
}

// buildSerpQuery encodes the SerpApi google_flights query for the slices:
// 1 slice → one-way (type=2); 2 mirror slices → round-trip (type=1); otherwise
// multi-city (type=3, multi_city_json).
func buildSerpQuery(slices []DuffelSlice, key string, opts SearchOptions) (string, error) {
	v := url.Values{}
	v.Set("engine", "google_flights")
	v.Set("api_key", key)
	v.Set("hl", "en")
	// Ask for the unparsed upstream payload. SerpApi's normalized JSON has no
	// baggage field at all, while the raw Google payload carries the allowance
	// inline at offer[4][6] — the same slot the google_flights provider reads.
	v.Set("output", "html")
	if opts.Currency != "" {
		v.Set("currency", opts.Currency)
		v.Set("gl", CurrencyToGL(opts.Currency))
	}
	if opts.CabinClass != 0 {
		v.Set("travel_class", fmt.Sprintf("%d", int(opts.CabinClass)))
	}
	if opts.Adults > 0 {
		v.Set("adults", fmt.Sprintf("%d", opts.Adults))
	}
	// SerpApi's "bags" is carry-on only ("Parameter defines the number of
	// carry-on bags"); there is no checked-bag request parameter, so
	// opts.CheckedBags has nothing to map to and is enforced client-side by
	// filterFlightResults. Sending bags does reprice: on a live JFK-LAX probe
	// 5 of 32 itineraries changed price between bags=0 and bags=1.
	if opts.CarryOnBags > 0 {
		v.Set("bags", fmt.Sprintf("%d", opts.CarryOnBags))
	}
	if opts.ExcludeBasic {
		v.Set("exclude_basic", "true")
	}

	isRoundTrip := len(slices) == 2 &&
		slices[1].Origin == slices[0].Destination &&
		slices[1].Destination == slices[0].Origin

	switch {
	case len(slices) == 1:
		v.Set("type", "2")
		v.Set("departure_id", slices[0].Origin)
		v.Set("arrival_id", slices[0].Destination)
		v.Set("outbound_date", slices[0].DepartureDate)
	case isRoundTrip:
		v.Set("type", "1")
		v.Set("departure_id", slices[0].Origin)
		v.Set("arrival_id", slices[0].Destination)
		v.Set("outbound_date", slices[0].DepartureDate)
		v.Set("return_date", slices[1].DepartureDate)
	default:
		v.Set("type", "3")
		legs := make([]serpMultiCityLeg, 0, len(slices))
		for _, s := range slices {
			legs = append(legs, serpMultiCityLeg{DepartureID: s.Origin, ArrivalID: s.Destination, Date: s.DepartureDate})
		}
		raw, err := json.Marshal(legs)
		if err != nil {
			return "", err
		}
		v.Set("multi_city_json", string(raw))
	}
	return v.Encode(), nil
}

// Package tripwindow finds optimal travel windows by intersecting a search
// calendar with the user's busy/preferred intervals and estimating the cheapest
// trip cost for each candidate window.
//
// It is shared by the MCP tool (find_trip_window) and the CLI command
// (trvl when). The MCP path receives busy_intervals from the orchestrating LLM
// which first fetches them from the user's calendar tool. The CLI path accepts
// --busy flags or reads a "blocked" list from ~/.trvl/preferences.json.
package tripwindow

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/MikkoParkkola/trvl/internal/flights"
	"github.com/MikkoParkkola/trvl/internal/models"
)

const dateLayout = "2006-01-02"

// Interval is a half-open [Start, End) date range with an optional reason label.
type Interval struct {
	Start  string `json:"start"`            // YYYY-MM-DD inclusive
	End    string `json:"end"`              // YYYY-MM-DD inclusive
	Reason string `json:"reason,omitempty"` // display label (meeting, holiday, …)
}

// Input configures a trip-window search.
type Input struct {
	Origin             string     // IATA or city; resolved from preferences if empty
	Destination        string     // IATA or city (required)
	WindowStart        string     // earliest possible departure (YYYY-MM-DD)
	WindowEnd          string     // latest possible return (YYYY-MM-DD)
	BusyIntervals      []Interval // dates to avoid
	PreferredIntervals []Interval // dates to prefer (boost score)
	MinNights          int        // minimum trip length (default: 3)
	MaxNights          int        // maximum trip length (default: 7)
	MaxCandidates      int        // top N results to return (default: 5)
	BudgetEUR          float64    // 0 = no limit
	TransportModes     []string   // "flight", "train", "bus", "ferry"; empty = all
}

func (in *Input) applyDefaults() {
	if in.MinNights <= 0 {
		in.MinNights = 3
	}
	if in.MaxNights <= 0 {
		in.MaxNights = 7
	}
	if in.MaxNights < in.MinNights {
		in.MaxNights = in.MinNights
	}
	if in.MaxCandidates <= 0 {
		in.MaxCandidates = 5
	}
}

// Candidate is one feasible trip window with an estimated cheapest cost.
type Candidate struct {
	Start             string  `json:"start"`              // departure date YYYY-MM-DD
	End               string  `json:"end"`                // return date YYYY-MM-DD
	Nights            int     `json:"nights"`             // trip length
	EstimatedCost     float64 `json:"estimated_cost"`     // cheapest found; 0 if search failed
	Currency          string  `json:"currency"`           // currency of estimated_cost
	OverlapsPreferred bool    `json:"overlaps_preferred"` // true if inside a preferred interval
	Reasoning         string  `json:"reasoning"`          // brief explanation for ranking
}

// Find generates and ranks candidate travel windows within the input constraints.
//
// It enumerates all [MinNights, MaxNights] windows inside [WindowStart, WindowEnd],
// filters out those overlapping BusyIntervals, and for each remaining window
// queries the cheapest round-trip flight to Destination. Results are ranked by
// price (ascending) with preferred-interval windows boosted to the front.
func Find(ctx context.Context, in Input) ([]Candidate, error) {
	in.applyDefaults()

	if in.Destination == "" {
		return nil, fmt.Errorf("destination is required")
	}
	if in.WindowStart == "" || in.WindowEnd == "" {
		return nil, fmt.Errorf("window_start and window_end are required")
	}

	wsDate, err := time.Parse(dateLayout, in.WindowStart)
	if err != nil {
		return nil, fmt.Errorf("invalid window_start %q: %w", in.WindowStart, err)
	}
	weDate, err := time.Parse(dateLayout, in.WindowEnd)
	if err != nil {
		return nil, fmt.Errorf("invalid window_end %q: %w", in.WindowEnd, err)
	}
	if weDate.Before(wsDate) {
		return nil, fmt.Errorf("window_end must be on or after window_start")
	}

	// Parse busy and preferred intervals once.
	busy := mustParseIntervals(in.BusyIntervals)
	preferred := mustParseIntervals(in.PreferredIntervals)

	// Generate candidate windows.
	type rawCandidate struct {
		start, end time.Time
		nights     int
	}
	var candidates []rawCandidate

	for nights := in.MinNights; nights <= in.MaxNights; nights++ {
		for dep := wsDate; ; dep = dep.AddDate(0, 0, 1) {
			ret := dep.AddDate(0, 0, nights)
			if ret.After(weDate) {
				break
			}
			if !overlapsAny(dep, ret, busy) {
				candidates = append(candidates, rawCandidate{dep, ret, nights})
			}
		}
	}

	if len(candidates) == 0 {
		return nil, nil
	}

	// Query cheapest price for each candidate in parallel (bounded concurrency).
	type priceResult struct {
		idx   int
		price float64
		curr  string
	}
	results := make([]priceResult, len(candidates))
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup

	origin := in.Origin
	dest := in.Destination

	for i, c := range candidates {
		i, c := i, c
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			price, curr := cheapestFlight(ctx, origin, dest, c.start.Format(dateLayout), c.end.Format(dateLayout))
			results[i] = priceResult{i, price, curr}
		}()
	}
	wg.Wait()

	// Build Candidate list, applying budget filter.
	var out []Candidate
	for i, c := range candidates {
		pr := results[i]
		if in.BudgetEUR > 0 && pr.price > 0 && pr.price > in.BudgetEUR {
			continue
		}

		overlaps := overlapsAny(c.start, c.end, preferred)
		reasoning := buildReasoning(c.start, c.end, c.nights, pr.price, pr.curr, overlaps)

		out = append(out, Candidate{
			Start:             c.start.Format(dateLayout),
			End:               c.end.Format(dateLayout),
			Nights:            c.nights,
			EstimatedCost:     pr.price,
			Currency:          pr.curr,
			OverlapsPreferred: overlaps,
			Reasoning:         reasoning,
		})
	}

	// Rank: preferred windows first, then by price (ascending; 0 = unknown, sort last).
	sort.SliceStable(out, func(a, b int) bool {
		if out[a].OverlapsPreferred != out[b].OverlapsPreferred {
			return out[a].OverlapsPreferred
		}
		pa, pb := out[a].EstimatedCost, out[b].EstimatedCost
		if pa == 0 {
			return false
		}
		if pb == 0 {
			return true
		}
		return pa < pb
	})

	if len(out) > in.MaxCandidates {
		out = out[:in.MaxCandidates]
	}
	return out, nil
}

// --- helpers ---

type parsedInterval struct {
	start, end time.Time
}

func mustParseIntervals(ivs []Interval) []parsedInterval {
	out := make([]parsedInterval, 0, len(ivs))
	for _, iv := range ivs {
		s, errS := time.Parse(dateLayout, iv.Start)
		e, errE := time.Parse(dateLayout, iv.End)
		if errS != nil || errE != nil {
			continue // skip malformed entries silently
		}
		out = append(out, parsedInterval{s, e})
	}
	return out
}

// overlapsAny reports whether [start, end] overlaps any of the given intervals.
// All ranges are treated as inclusive on both ends.
func overlapsAny(start, end time.Time, ivs []parsedInterval) bool {
	for _, iv := range ivs {
		// Two inclusive ranges [a,b] and [c,d] overlap iff a<=d && c<=b.
		if !start.After(iv.end) && !iv.start.After(end) {
			return true
		}
	}
	return false
}

// cheapestFlight returns the cheapest round-trip price and currency for the
// given origin→destination on the given dates. Returns (0, "") on any error.
func cheapestFlight(ctx context.Context, origin, dest, depDate, retDate string) (float64, string) {
	if origin == "" || dest == "" || depDate == "" {
		return 0, ""
	}

	opts := flights.SearchOptions{
		ReturnDate: retDate,
	}

	result, err := flights.SearchFlights(ctx, origin, dest, depDate, opts)
	if err != nil || result == nil || !result.Success || len(result.Flights) == 0 {
		return 0, ""
	}

	var best float64
	var bestCurr string
	for _, f := range result.Flights {
		if f.Price > 0 && (best == 0 || f.Price < best) {
			best = f.Price
			bestCurr = f.Currency
		}
	}
	return best, bestCurr
}

func buildReasoning(start, end time.Time, nights int, price float64, curr string, preferred bool) string {
	msg := fmt.Sprintf("%s – %s (%d nights)",
		start.Format("Jan 2"), end.Format("Jan 2"), nights)
	if preferred {
		msg += "; overlaps a preferred window"
	}
	if price > 0 {
		msg += fmt.Sprintf("; est. %s %.0f RT", curr, price)
	} else {
		msg += "; price unavailable"
	}
	return msg
}

// ValidateInput returns an error if required fields are missing or logically invalid.
func ValidateInput(in Input) error {
	if in.Destination == "" {
		return fmt.Errorf("destination is required")
	}
	if in.WindowStart == "" {
		return fmt.Errorf("window_start is required")
	}
	if in.WindowEnd == "" {
		return fmt.Errorf("window_end is required")
	}
	ws, err := time.Parse(dateLayout, in.WindowStart)
	if err != nil {
		return fmt.Errorf("invalid window_start: %w", err)
	}
	we, err := time.Parse(dateLayout, in.WindowEnd)
	if err != nil {
		return fmt.Errorf("invalid window_end: %w", err)
	}
	if we.Before(ws) {
		return fmt.Errorf("window_end must be on or after window_start")
	}
	return nil
}

// ParseBusyFlag parses a "YYYY-MM-DD:YYYY-MM-DD" flag value into an Interval.
// Returns an error if the format is wrong.
func ParseBusyFlag(s string) (Interval, error) {
	if len(s) != 21 || s[10] != ':' {
		return Interval{}, fmt.Errorf("busy interval %q must be YYYY-MM-DD:YYYY-MM-DD", s)
	}
	start := s[:10]
	end := s[11:]
	if _, err := time.Parse(dateLayout, start); err != nil {
		return Interval{}, fmt.Errorf("busy interval start %q: %w", start, err)
	}
	if _, err := time.Parse(dateLayout, end); err != nil {
		return Interval{}, fmt.Errorf("busy interval end %q: %w", end, err)
	}
	return Interval{Start: start, End: end}, nil
}

// FlightSearchResult re-exports the models type for callers that want to
// inspect the raw result. Not used by Find itself — just a convenience alias.
type FlightSearchResult = models.FlightSearchResult

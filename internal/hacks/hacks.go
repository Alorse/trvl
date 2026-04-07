// Package hacks detects travel optimization opportunities alongside normal
// flight and route searches. Each detector is independent and runs in parallel.
package hacks

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
)

// Hack represents a detected travel optimization opportunity.
type Hack struct {
	Type        string   `json:"type"`                  // "throwaway", "hidden_city", "positioning", "split", "night_transport", "stopover", "date_flex", "open_jaw", "ferry_positioning", "multi_stop", "currency_arbitrage", "calendar_conflict", "tuesday_booking", "low_cost_carrier", "multimodal_skip_flight", "multimodal_positioning", "multimodal_open_jaw_ground", "multimodal_return_split"
	Title       string   `json:"title"`                 // human-readable hack name
	Description string   `json:"description"`           // explanation for the traveller
	Savings     float64  `json:"savings"`               // EUR saved vs naive booking
	Currency    string   `json:"currency"`              // currency for Savings
	Risks       []string `json:"risks,omitempty"`       // airline ToS, operational risks
	Steps       []string `json:"steps"`                 // how to execute
	Citations   []string `json:"citations,omitempty"`   // booking URLs / provider names
}

// DetectorInput carries all parameters shared across detectors.
type DetectorInput struct {
	Origin      string
	Destination string
	Date        string  // outbound YYYY-MM-DD
	ReturnDate  string  // round-trip return YYYY-MM-DD; empty = one-way search
	Currency    string  // defaults to EUR
	CarryOnOnly bool    // relevant for hidden-city (checked bags go to final dest)
	NaivePrice  float64 // baseline price for savings computation
}

func (in *DetectorInput) currency() string {
	if in.Currency != "" {
		return in.Currency
	}
	return "EUR"
}

// StopoverProgram describes an airline's free stopover offer.
type StopoverProgram struct {
	Airline      string
	Hub          string
	MaxNights    int
	Restrictions string
	URL          string
}

// stopoverPrograms is the static database of airline stopover programs.
var stopoverPrograms = map[string]StopoverProgram{
	"AY": {Airline: "Finnair", Hub: "HEL", MaxNights: 5, Restrictions: "Non-Finnish residents only", URL: "https://www.finnair.com/en/stopover"},
	"FI": {Airline: "Icelandair", Hub: "KEF", MaxNights: 7, Restrictions: "Free for transit passengers", URL: "https://www.icelandair.com/stopover"},
	"TP": {Airline: "TAP Portugal", Hub: "LIS", MaxNights: 10, Restrictions: "Free; book through TAP website", URL: "https://www.flytap.com/en-us/stopover"},
	"TK": {Airline: "Turkish Airlines", Hub: "IST", MaxNights: 2, Restrictions: "Free hotel for long layovers (TourIST program)", URL: "https://www.turkishairlines.com/en-int/any-questions/fly-and-smile/"},
	"QR": {Airline: "Qatar Airways", Hub: "DOH", MaxNights: 4, Restrictions: "Doha Stopover from +1 USD", URL: "https://www.qatarairways.com/en/destinations/qatar/doha-stopover.html"},
	"EK": {Airline: "Emirates", Hub: "DXB", MaxNights: 4, Restrictions: "Dubai Connect program", URL: "https://www.emirates.com/english/destinations/dubai/stopover/"},
	"SQ": {Airline: "Singapore Airlines", Hub: "SIN", MaxNights: 3, Restrictions: "Singapore Stopover Holiday program", URL: "https://www.singaporeair.com/en_UK/us/promotions/stopover-holiday/"},
	"EY": {Airline: "Etihad", Hub: "AUH", MaxNights: 2, Restrictions: "Abu Dhabi Stopover program", URL: "https://www.etihad.com/en-us/destinations/united-arab-emirates/abu-dhabi/stopover"},
}

// detectFn is the signature for individual hack detectors.
type detectFn func(ctx context.Context, in DetectorInput) []Hack

// DetectAll runs all detectors in parallel and returns every hack found.
// It respects ctx cancellation; detectors that finish after cancellation
// are discarded.
func DetectAll(ctx context.Context, in DetectorInput) []Hack {
	detectors := []detectFn{
		detectThrowaway,
		detectHiddenCity,
		detectPositioning,
		detectSplit,
		detectNightTransport,
		detectStopover,
		detectDateFlex,
		detectOpenJaw,
		detectFerryPositioning,
		detectMultiStop,
		detectCurrencyArbitrage,
		detectCalendarConflict,
		detectTuesdayBooking,
		detectLowCostCarrier,
		detectMultiModalSkipFlight,
		detectMultiModalPositioning,
		detectMultiModalOpenJawGround,
		detectMultiModalReturnSplit,
	}

	// Each detector gets a child context with a per-detector timeout so a
	// slow API call cannot block the entire hacks response.
	const detectorTimeout = 20 * time.Second

	type result struct {
		hacks []Hack
	}

	ch := make(chan result, len(detectors))
	var wg sync.WaitGroup

	for _, fn := range detectors {
		fn := fn
		wg.Add(1)
		go func() {
			defer wg.Done()
			dCtx, cancel := context.WithTimeout(ctx, detectorTimeout)
			defer cancel()
			h := fn(dCtx, in)
			ch <- result{hacks: h}
		}()
	}

	// Close channel once all goroutines complete.
	go func() {
		wg.Wait()
		close(ch)
	}()

	var all []Hack
	for r := range ch {
		all = append(all, r.hacks...)
	}
	return dedupHacks(all)
}

// dedupHacks removes hacks that are functionally identical. Two hacks are
// considered duplicates when they share the same Type, From/To airports (derived
// from their Steps), and a savings amount within EUR 5 of each other. When
// duplicates are found the one with more Steps (more detail) is kept.
func dedupHacks(hacks []Hack) []Hack {
	if len(hacks) <= 1 {
		return hacks
	}

	// extractKey returns a normalised signature for a hack. We use Type +
	// savings-bucket (rounded to nearest 5) + final destination airport derived
	// from the last Step that contains an IATA-like token (3 uppercase letters).
	extractKey := func(h Hack) string {
		bucket := math.Round(h.Savings/5) * 5
		// Find the last step that mentions a 3-letter uppercase airport code.
		airport := ""
		for _, s := range h.Steps {
			words := strings.Fields(s)
			for _, w := range words {
				// Strip punctuation
				clean := strings.Trim(w, "()[].,:-→>")
				if len(clean) == 3 && clean == strings.ToUpper(clean) {
					airport = clean
				}
			}
		}
		return fmt.Sprintf("%s|%.0f|%s", h.Type, bucket, airport)
	}

	seen := make(map[string]int) // key → index in result slice
	result := make([]Hack, 0, len(hacks))

	for _, h := range hacks {
		key := extractKey(h)
		if idx, exists := seen[key]; exists {
			// Keep the more detailed hack (more Steps).
			if len(h.Steps) > len(result[idx].Steps) {
				result[idx] = h
			}
		} else {
			seen[key] = len(result)
			result = append(result, h)
		}
	}
	return result
}

// roundSavings rounds to the nearest integer for display.
func roundSavings(v float64) float64 {
	return math.Round(v)
}

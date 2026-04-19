package optimizer

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/MikkoParkkola/trvl/internal/baggage"
	"github.com/MikkoParkkola/trvl/internal/batchexec"
	"github.com/MikkoParkkola/trvl/internal/flights"
	"github.com/MikkoParkkola/trvl/internal/hacks"
	"github.com/MikkoParkkola/trvl/internal/models"
)

// OptimizeInput configures a trip optimization search.
type OptimizeInput struct {
	Origin      string // primary origin IATA (e.g., "HEL")
	Destination string // primary destination IATA or city (e.g., "BCN")
	DepartDate  string // target departure (YYYY-MM-DD)
	ReturnDate  string // target return (YYYY-MM-DD), empty = one-way
	FlexDays    int    // date flexibility +/-N days (default 3)
	Guests      int    // passengers (default 1)
	Currency    string // display currency
	MaxResults  int    // top N results to return (default 5)
	MaxAPICalls int    // API call budget (default 15)

	// User context (from preferences).
	FFStatuses     []FFStatus // frequent flyer statuses
	NeedCheckedBag bool
	CarryOnOnly    bool
	HomeAirports   []string // user's home airports
}

// FFStatus represents a frequent flyer programme membership.
type FFStatus struct {
	Alliance string
	Tier     string
}

// BookingOption is a ranked booking strategy with all-in cost breakdown.
type BookingOption struct {
	Rank              int      `json:"rank"`
	Strategy          string   `json:"strategy"`
	Legs              []Leg    `json:"legs"`
	BaseCost          float64  `json:"base_cost"`
	BagCost           float64  `json:"bag_cost"`
	FFSavings         float64  `json:"ff_savings"`
	TransferCost      float64  `json:"transfer_cost"`
	AllInCost         float64  `json:"all_in_cost"`
	Currency          string   `json:"currency"`
	SavingsVsBaseline float64  `json:"savings_vs_baseline"`
	HacksApplied      []string `json:"hacks_applied"`
}

// Leg is a single transport segment in a booking option.
type Leg struct {
	Type     string  `json:"type"`
	From     string  `json:"from"`
	To       string  `json:"to"`
	Date     string  `json:"date"`
	Price    float64 `json:"price"`
	Currency string  `json:"currency"`
	Airline  string  `json:"airline,omitempty"`
	Duration int     `json:"duration_min,omitempty"`
	Notes    string  `json:"notes,omitempty"`
}

// OptimizeResult is the output of the optimization engine.
type OptimizeResult struct {
	Success  bool           `json:"success"`
	Options  []BookingOption `json:"options"`
	Baseline *BookingOption  `json:"baseline,omitempty"`
	Error    string         `json:"error,omitempty"`
}

// candidate is an internal search candidate generated during the EXPAND phase.
type candidate struct {
	origin       string
	dest         string
	departDate   string
	returnDate   string
	strategy     string
	hackTypes    []string
	transferCost float64
	transferTime int // minutes

	// prePriced marks candidates whose cost is known without a flight search
	// (e.g. rail corridors, ferry cabins). These skip the SEARCH phase.
	prePriced bool

	// Populated during SEARCH phase.
	searched bool
	flights  []models.FlightResult
	currency string

	// Populated during PRICE phase.
	baseCost  float64
	bagCost   float64
	ffSavings float64
	allInCost float64
}

// defaults fills zero-value fields with sensible defaults.
func (in *OptimizeInput) defaults() {
	if in.Guests <= 0 {
		in.Guests = 1
	}
	if in.FlexDays < 0 {
		in.FlexDays = 0
	}
	if in.FlexDays == 0 {
		in.FlexDays = 3
	}
	if in.MaxResults <= 0 {
		in.MaxResults = 5
	}
	if in.MaxAPICalls <= 0 {
		in.MaxAPICalls = 15
	}
	if in.Currency == "" {
		in.Currency = "EUR"
	}
}

// Optimize runs the 4-phase trip optimization engine.
// It expands candidates from pricing primitives, searches them in parallel,
// applies all-in cost adjustments, and ranks the results.
func Optimize(ctx context.Context, input OptimizeInput) (*OptimizeResult, error) {
	if err := validateInput(input); err != nil {
		return &OptimizeResult{Error: err.Error()}, err
	}

	input.defaults()

	// Apply 30s total timeout.
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Phase 1: EXPAND candidates.
	candidates := expandCandidates(input)

	// Phase 2: SEARCH candidates with budget.
	client := batchexec.NewClient()
	searchCandidates(ctx, candidates, client, input)

	// Phase 3: PRICE candidates.
	for _, c := range candidates {
		if c.searched {
			priceCandidate(c, input)
		}
	}

	// Phase 4: RANK by all-in cost.
	return rankCandidates(candidates, input), nil
}

func validateInput(in OptimizeInput) error {
	if in.Origin == "" {
		return fmt.Errorf("origin is required")
	}
	if in.Destination == "" {
		return fmt.Errorf("destination is required")
	}
	if in.DepartDate == "" {
		return fmt.Errorf("departure date is required")
	}
	if _, err := time.Parse("2006-01-02", in.DepartDate); err != nil {
		return fmt.Errorf("invalid departure date: %s", in.DepartDate)
	}
	if in.ReturnDate != "" {
		if _, err := time.Parse("2006-01-02", in.ReturnDate); err != nil {
			return fmt.Errorf("invalid return date: %s", in.ReturnDate)
		}
	}
	if strings.EqualFold(in.Origin, in.Destination) {
		return fmt.Errorf("origin and destination must differ")
	}
	return nil
}

// expandCandidates generates all candidate search parameters from applicable
// pricing primitives.
func expandCandidates(input OptimizeInput) []*candidate {
	origin := strings.ToUpper(input.Origin)
	dest := strings.ToUpper(input.Destination)

	var candidates []*candidate

	// 1. Baseline: direct search with given parameters.
	candidates = append(candidates, &candidate{
		origin:     origin,
		dest:       dest,
		departDate: input.DepartDate,
		returnDate: input.ReturnDate,
		strategy:   "Direct booking",
	})

	// 2. Alternative origins (positioning).
	for _, alt := range hacks.NearbyAirports(origin) {
		if alt.IATA == dest {
			continue
		}
		candidates = append(candidates, &candidate{
			origin:       alt.IATA,
			dest:         dest,
			departDate:   input.DepartDate,
			returnDate:   input.ReturnDate,
			strategy:     fmt.Sprintf("Fly from %s (%s via %s)", alt.City, alt.IATA, alt.Mode),
			hackTypes:    []string{alt.HackType},
			transferCost: alt.Cost,
			transferTime: alt.Minutes,
		})
	}

	// 3. Alternative destinations.
	for _, alt := range hacks.DestinationAlternatives(dest) {
		if alt.IATA == origin {
			continue
		}
		candidates = append(candidates, &candidate{
			origin:       origin,
			dest:         alt.IATA,
			departDate:   input.DepartDate,
			returnDate:   input.ReturnDate,
			strategy:     fmt.Sprintf("Fly to %s (%s) + %s to %s", alt.City, alt.IATA, alt.Mode, dest),
			hackTypes:    []string{"destination_airport"},
			transferCost: alt.Cost,
		})
	}

	// 4. Rail+fly stations (fare zone arbitrage).
	for _, station := range hacks.RailFlyStationsForHub(origin) {
		candidates = append(candidates, &candidate{
			origin:     station.IATA,
			dest:       dest,
			departDate: input.DepartDate,
			returnDate: input.ReturnDate,
			strategy:   fmt.Sprintf("Book via %s (%s fare zone, %s)", station.City, station.FareZone, station.AirlineName),
			hackTypes:  []string{"rail_fly_arbitrage"},
			// Rail segment is free — included in the airline ticket.
			transferCost: 0,
			transferTime: station.TrainMins,
		})
	}

	// 5. Date flexibility: generate candidates with shifted dates.
	// These are searched via CalendarGraph in the SEARCH phase (1 API call)
	// rather than N individual searches.
	if input.FlexDays > 0 {
		for d := -input.FlexDays; d <= input.FlexDays; d++ {
			if d == 0 {
				continue // baseline already covers d=0
			}
			shiftedDepart := shiftDate(input.DepartDate, d)
			shiftedReturn := shiftDate(input.ReturnDate, d)
			if shiftedDepart == "" {
				continue
			}
			if input.ReturnDate != "" && shiftedReturn == "" {
				continue
			}
			candidates = append(candidates, &candidate{
				origin:     origin,
				dest:       dest,
				departDate: shiftedDepart,
				returnDate: shiftedReturn,
				strategy:   fmt.Sprintf("Shift dates by %+d days", d),
				hackTypes:  []string{"date_flex"},
			})
		}
	}

	// 6. Hidden city: search to a beyond-destination via airline hub.
	// When the destination is a major airline hub, flights to cities
	// beyond the hub that connect through it can be cheaper.
	hiddenCityBeyond := map[string][]string{
		"AMS": {"HEL", "RIX", "TLL", "ARN"}, // KLM hub — search to Nordics/Baltics via AMS
		"FRA": {"PRG", "WAW", "BUD", "VIE"}, // Lufthansa hub — search to Central/Eastern Europe via FRA
		"CDG": {"BRU", "GVA", "LIS"},         // Air France hub
		"MUC": {"PRG", "VIE", "BUD"},         // Lufthansa hub
		"IST": {"TBS", "SOF", "OTP"},         // Turkish hub
		"CPH": {"ARN", "HEL", "OSL"},         // SAS hub
		"ZRH": {"MXP", "VIE", "MUC"},         // Swiss hub
	}

	if beyondCities, ok := hiddenCityBeyond[dest]; ok {
		for _, beyond := range beyondCities {
			if beyond == origin {
				continue
			}
			candidates = append(candidates, &candidate{
				origin:     origin,
				dest:       beyond,
				departDate: input.DepartDate,
				returnDate: input.ReturnDate,
				strategy:   fmt.Sprintf("Hidden city: book to %s, exit at %s", beyond, dest),
				hackTypes:  []string{"hidden_city"},
			})
		}
	}

	// 7. Departure tax: fly from a zero-tax country to avoid aviation tax.
	if taxEUR, _, ok := hacks.DepartureTaxSavings(origin); ok {
		for _, alt := range hacks.ZeroTaxAlternatives(origin) {
			if alt.IATA == dest {
				continue
			}
			// Only worth it when the tax saving exceeds the ground transport cost.
			if taxEUR <= alt.Cost {
				continue
			}
			candidates = append(candidates, &candidate{
				origin:       alt.IATA,
				dest:         dest,
				departDate:   input.DepartDate,
				returnDate:   input.ReturnDate,
				strategy:     fmt.Sprintf("Zero-tax departure from %s (%s) — saves €%.0f/person", alt.City, alt.IATA, taxEUR),
				hackTypes:    []string{"departure_tax", "positioning"},
				transferCost: alt.Cost,
				transferTime: alt.Minutes,
			})
		}
	}

	// 8. Rail competition: competitive rail corridor as ground alternative.
	if minFare, operators, ok := hacks.CompetitiveRailRoute(origin, dest); ok {
		candidates = append(candidates, &candidate{
			origin:     origin,
			dest:       dest,
			departDate: input.DepartDate,
			returnDate: input.ReturnDate,
			strategy:   fmt.Sprintf("Take train (%s) — fares from €%.0f", strings.Join(operators, ", "), minFare),
			hackTypes:  []string{"rail_competition", "ground_alternative"},
			prePriced:  true,
			baseCost:   minFare,
			currency:   "EUR",
			searched:   true,
		})
	}

	// 9. Ferry cabin: overnight ferry replaces a hotel night.
	if cabinEUR, hotelSavings, operator, ok := hacks.OvernightFerryRoute(origin, dest); ok {
		candidates = append(candidates, &candidate{
			origin:     origin,
			dest:       dest,
			departDate: input.DepartDate,
			returnDate: input.ReturnDate,
			strategy:   fmt.Sprintf("Overnight ferry (%s) — saves €%.0f vs hotel", operator, hotelSavings),
			hackTypes:  []string{"ferry_cabin_hotel"},
			prePriced:  true,
			baseCost:   cabinEUR,
			currency:   "EUR",
			searched:   true,
		})
	}

	return candidates
}

// shiftDate shifts a YYYY-MM-DD date string by the given number of days.
// Returns "" if the input is empty or unparseable.
func shiftDate(date string, days int) string {
	if date == "" {
		return ""
	}
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return ""
	}
	return t.AddDate(0, 0, days).Format("2006-01-02")
}

// searchCandidates executes flight searches for candidates within the API budget.
// It prioritizes the baseline (direct) search first, then alternatives.
//
// Date-flex candidates are resolved via a single CalendarGraph call (1 API call
// for the entire flex range) instead of N individual searches.
func searchCandidates(ctx context.Context, candidates []*candidate, client *batchexec.Client, input OptimizeInput) {
	budget := int64(input.MaxAPICalls)
	var used atomic.Int64

	// Pre-resolve date-flex candidates via CalendarGraph (1 API call).
	resolveFlexDatesViaCalendar(ctx, candidates, input, &used, budget)

	// Sort remaining candidates: baseline (no hacks) first, then by transfer cost.
	var remaining []*candidate
	for _, c := range candidates {
		if c.searched || c.prePriced {
			continue // already resolved (date-flex or pre-priced ground)
		}
		remaining = append(remaining, c)
	}
	sort.SliceStable(remaining, func(i, j int) bool {
		// Baseline first.
		if len(remaining[i].hackTypes) == 0 {
			return true
		}
		if len(remaining[j].hackTypes) == 0 {
			return false
		}
		return remaining[i].transferCost < remaining[j].transferCost
	})

	// Use semaphore for concurrency control (max 4 parallel searches).
	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup

	for _, c := range remaining {
		if used.Load() >= budget {
			break
		}
		if ctx.Err() != nil {
			break
		}

		c := c
		wg.Add(1)
		sem <- struct{}{} // acquire

		go func() {
			defer wg.Done()
			defer func() { <-sem }() // release

			if used.Add(1) > budget {
				return
			}

			opts := flights.SearchOptions{
				SortBy: models.SortCheapest,
				Adults: input.Guests,
			}
			if c.returnDate != "" {
				opts.ReturnDate = c.returnDate
			}

			result, err := flights.SearchFlightsWithClient(ctx, client, c.origin, c.dest, c.departDate, opts)
			if err != nil || result == nil || !result.Success || len(result.Flights) == 0 {
				return
			}

			c.searched = true
			c.flights = result.Flights
			if len(result.Flights) > 0 && result.Flights[0].Currency != "" {
				c.currency = result.Flights[0].Currency
			}
		}()
	}

	wg.Wait()
}

// resolveFlexDatesViaCalendar uses a single CalendarGraph API call to get
// prices for the entire flex date range. It populates date-flex candidates
// with synthetic FlightResult entries containing the calendar prices.
func resolveFlexDatesViaCalendar(ctx context.Context, candidates []*candidate, input OptimizeInput, used *atomic.Int64, budget int64) {
	if input.FlexDays <= 0 {
		return
	}

	// Collect date-flex candidates.
	flexCandidates := make(map[string]*candidate) // departDate -> candidate
	for _, c := range candidates {
		if len(c.hackTypes) == 1 && c.hackTypes[0] == "date_flex" {
			flexCandidates[c.departDate] = c
		}
	}
	if len(flexCandidates) == 0 {
		return
	}

	if used.Add(1) > budget {
		return
	}

	origin := strings.ToUpper(input.Origin)
	dest := strings.ToUpper(input.Destination)

	// Compute the date range covering all flex days.
	fromDate := shiftDate(input.DepartDate, -input.FlexDays)
	toDate := shiftDate(input.DepartDate, input.FlexDays)
	if fromDate == "" || toDate == "" {
		return
	}

	tripLength := 0
	roundTrip := input.ReturnDate != ""
	if roundTrip {
		departT, err1 := time.Parse("2006-01-02", input.DepartDate)
		returnT, err2 := time.Parse("2006-01-02", input.ReturnDate)
		if err1 == nil && err2 == nil {
			tripLength = int(returnT.Sub(departT).Hours() / 24)
		}
	}

	calResult, err := flights.SearchCalendar(ctx, origin, dest, flights.CalendarOptions{
		FromDate:   fromDate,
		ToDate:     toDate,
		TripLength: tripLength,
		RoundTrip:  roundTrip,
		Adults:     input.Guests,
	})
	if err != nil || calResult == nil || !calResult.Success || len(calResult.Dates) == 0 {
		return
	}

	// Map calendar prices to flex candidates.
	for _, dp := range calResult.Dates {
		if c, ok := flexCandidates[dp.Date]; ok && dp.Price > 0 {
			c.searched = true
			c.currency = dp.Currency
			c.flights = []models.FlightResult{
				{Price: dp.Price, Currency: dp.Currency},
			}
		}
	}
}

// priceCandidate computes all-in cost for a searched candidate.
func priceCandidate(c *candidate, input OptimizeInput) {
	// Pre-priced candidates (rail, ferry) already have baseCost set.
	// No bag fees for ground transport.
	if c.prePriced {
		c.allInCost = c.baseCost + c.transferCost
		return
	}

	if len(c.flights) == 0 {
		return
	}

	// Find cheapest flight.
	bestFlight := c.flights[0]
	for _, f := range c.flights[1:] {
		if f.Price > 0 && (bestFlight.Price <= 0 || f.Price < bestFlight.Price) {
			bestFlight = f
		}
	}
	if bestFlight.Price <= 0 {
		return
	}

	c.baseCost = bestFlight.Price
	c.currency = bestFlight.Currency

	// Compute baggage costs via AllInCost.
	ffStatuses := convertFFStatuses(input.FFStatuses)
	airlineCode := ""
	if len(bestFlight.Legs) > 0 {
		airlineCode = bestFlight.Legs[0].AirlineCode
	}

	allIn, _ := baggage.AllInCost(
		bestFlight.Price,
		airlineCode,
		input.NeedCheckedBag,
		!input.CarryOnOnly, // needCarryOn = opposite of carryOnOnly
		ffStatuses,
	)

	c.bagCost = allIn - bestFlight.Price
	if c.bagCost < 0 {
		c.bagCost = 0
	}

	// FF savings: difference between cost without FF and cost with FF.
	allInNoFF, _ := baggage.AllInCost(
		bestFlight.Price,
		airlineCode,
		input.NeedCheckedBag,
		!input.CarryOnOnly,
		nil, // no FF statuses
	)
	c.ffSavings = allInNoFF - allIn
	if c.ffSavings < 0 {
		c.ffSavings = 0
	}

	// All-in = base + bags - FF savings + transfer cost
	c.allInCost = allIn + c.transferCost
}

// rankCandidates sorts by all-in cost and returns the top N options.
func rankCandidates(candidates []*candidate, input OptimizeInput) *OptimizeResult {
	// Filter to only searched + priced candidates.
	var priced []*candidate
	for _, c := range candidates {
		if c.searched && c.allInCost > 0 {
			priced = append(priced, c)
		}
	}

	if len(priced) == 0 {
		return &OptimizeResult{
			Error: "no results found for any candidate strategy",
		}
	}

	// Identify baseline (the direct booking candidate) to determine the
	// reference currency for cross-candidate comparison.
	var baseline *candidate
	for _, c := range priced {
		if len(c.hackTypes) == 0 {
			baseline = c
			break
		}
	}

	// Determine the reference currency from the baseline (or the most common
	// currency). Candidates in a different currency can't be compared by raw
	// cost, so they sort after same-currency candidates.
	refCurrency := input.Currency
	if baseline != nil && baseline.currency != "" {
		refCurrency = baseline.currency
	}

	// Sort: same-currency candidates by all-in cost first, then cross-currency.
	sort.SliceStable(priced, func(i, j int) bool {
		iMatch := strings.EqualFold(priced[i].currency, refCurrency)
		jMatch := strings.EqualFold(priced[j].currency, refCurrency)
		if iMatch != jMatch {
			return iMatch // same-currency sorts before cross-currency
		}
		return priced[i].allInCost < priced[j].allInCost
	})

	// Build result.
	n := input.MaxResults
	if n > len(priced) {
		n = len(priced)
	}

	result := &OptimizeResult{
		Success: true,
		Options: make([]BookingOption, n),
	}

	for i := 0; i < n; i++ {
		c := priced[i]
		opt := candidateToOption(c, i+1, input)

		// Compute savings vs baseline (only when currencies match).
		if baseline != nil && baseline.allInCost > 0 &&
			strings.EqualFold(c.currency, baseline.currency) {
			opt.SavingsVsBaseline = math.Round(baseline.allInCost - opt.AllInCost)
		}

		result.Options[i] = opt
	}

	// Set baseline if it was found and priced.
	if baseline != nil {
		bl := candidateToOption(baseline, 0, input)
		result.Baseline = &bl
	}

	return result
}

// candidateToOption converts an internal candidate to the public BookingOption.
func candidateToOption(c *candidate, rank int, input OptimizeInput) BookingOption {
	var legs []Leg

	// Add transfer leg if there is a positioning cost.
	if c.transferCost > 0 {
		legs = append(legs, Leg{
			Type:     "ground",
			From:     strings.ToUpper(input.Origin),
			To:       c.origin,
			Date:     c.departDate,
			Price:    c.transferCost,
			Currency: c.currency,
			Notes:    "Ground transfer",
		})
	}

	// Add outbound leg: ground for pre-priced, flight otherwise.
	if c.prePriced {
		legs = append(legs, Leg{
			Type:     "ground",
			From:     c.origin,
			To:       c.dest,
			Date:     c.departDate,
			Price:    c.baseCost,
			Currency: c.currency,
			Notes:    c.strategy,
		})
	} else if len(c.flights) > 0 {
		best := cheapestFlight(c.flights)
		airline := ""
		duration := best.Duration
		if len(best.Legs) > 0 {
			airline = best.Legs[0].Airline
		}
		legs = append(legs, Leg{
			Type:     "flight",
			From:     c.origin,
			To:       c.dest,
			Date:     c.departDate,
			Price:    best.Price,
			Currency: best.Currency,
			Airline:  airline,
			Duration: duration,
		})
	}

	// Add destination transfer leg if it's an alternative destination.
	for _, h := range c.hackTypes {
		if h == "destination_airport" && c.transferCost > 0 {
			legs = append(legs, Leg{
				Type:     "ground",
				From:     c.dest,
				To:       strings.ToUpper(input.Destination),
				Date:     c.departDate,
				Price:    c.transferCost,
				Currency: c.currency,
				Notes:    "Ground transfer to final destination",
			})
			break
		}
	}

	return BookingOption{
		Rank:         rank,
		Strategy:     c.strategy,
		Legs:         legs,
		BaseCost:     math.Round(c.baseCost),
		BagCost:      math.Round(c.bagCost),
		FFSavings:    math.Round(c.ffSavings),
		TransferCost: math.Round(c.transferCost),
		AllInCost:    math.Round(c.allInCost),
		Currency:     c.currency,
		HacksApplied: c.hackTypes,
	}
}

// cheapestFlight returns the flight with the lowest positive price.
func cheapestFlight(flts []models.FlightResult) models.FlightResult {
	best := flts[0]
	for _, f := range flts[1:] {
		if f.Price > 0 && (best.Price <= 0 || f.Price < best.Price) {
			best = f
		}
	}
	return best
}

// convertFFStatuses converts optimizer FFStatus to baggage.FFStatus.
func convertFFStatuses(statuses []FFStatus) []baggage.FFStatus {
	out := make([]baggage.FFStatus, len(statuses))
	for i, s := range statuses {
		out[i] = baggage.FFStatus{
			Alliance: s.Alliance,
			Tier:     s.Tier,
		}
	}
	return out
}

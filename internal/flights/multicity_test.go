package flights

import (
	"context"
	"reflect"
	"testing"
)

const futureDate = "2999-01-01" // always valid/future for models.ValidateDate

func TestParseLeg_ValidIATA(t *testing.T) {
	leg, err := ParseLeg("CDG:HND:" + futureDate)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := leg.Origins, []string{"CDG"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Origins = %v, want %v", got, want)
	}
	if got, want := leg.Destinations, []string{"HND"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Destinations = %v, want %v", got, want)
	}
	if leg.Date != futureDate {
		t.Errorf("Date = %q, want %q", leg.Date, futureDate)
	}
}

func TestParseLeg_CityExpandsToAirports(t *testing.T) {
	leg, err := ParseLeg("Paris:HND:" + futureDate)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(leg.Origins) < 1 {
		t.Fatalf("expected Paris to resolve to at least one airport, got %v", leg.Origins)
	}
	found := false
	for _, a := range leg.Origins {
		if a == "CDG" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected CDG among Paris airports, got %v", leg.Origins)
	}
}

func TestParseLeg_MultiAirportEndpoint(t *testing.T) {
	leg, err := ParseLeg("CDG,ORY:HND:" + futureDate)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := leg.Origins, []string{"CDG", "ORY"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Origins = %v, want %v", got, want)
	}
}

func TestParseLeg_Errors(t *testing.T) {
	cases := []string{
		"CDG:HND",                // missing date
		"CDG:HND:2020-01-01",     // past date
		"CDG:HND:not-a-date",     // bad format
		":HND:" + futureDate,     // missing origin
		"CDG::" + futureDate,     // missing destination
	}
	for _, spec := range cases {
		if _, err := ParseLeg(spec); err == nil {
			t.Errorf("ParseLeg(%q) expected error, got nil", spec)
		}
	}
}

// TestBuildSegmentMulti_SingleAirportRegression verifies the refactor: a single
// airport via buildSegmentMulti must equal the original buildSegment output.
func TestBuildSegmentMulti_SingleAirportRegression(t *testing.T) {
	opts := SearchOptions{}
	opts.defaults()
	a := buildSegment("CDG", "HND", futureDate, opts)
	b := buildSegmentMulti([]string{"CDG"}, []string{"HND"}, futureDate, opts)
	if !reflect.DeepEqual(a, b) {
		t.Errorf("buildSegmentMulti single-airport output differs from buildSegment\n got: %#v\nwant: %#v", b, a)
	}
}

func TestBuildSegmentMulti_ListsAllAirports(t *testing.T) {
	opts := SearchOptions{}
	opts.defaults()
	seg := buildSegmentMulti([]string{"CDG", "ORY"}, []string{"HND", "NRT"}, futureDate, opts).([]any)

	depWant := []any{[]any{[]any{"CDG", 0}, []any{"ORY", 0}}}
	if !reflect.DeepEqual(seg[0], depWant) {
		t.Errorf("departure locations = %#v, want %#v", seg[0], depWant)
	}
	arrWant := []any{[]any{[]any{"HND", 0}, []any{"NRT", 0}}}
	if !reflect.DeepEqual(seg[1], arrWant) {
		t.Errorf("arrival locations = %#v, want %#v", seg[1], arrWant)
	}
	if seg[6] != futureDate {
		t.Errorf("date slot = %v, want %v", seg[6], futureDate)
	}
}

// TestBuildFiltersFromSegments_TripType verifies tripType=3 lands in outer[1][2]
// and the segments land in outer[1][13].
func TestBuildFiltersFromSegments_MultiCity(t *testing.T) {
	opts := SearchOptions{}
	opts.defaults()
	segs := []any{
		buildSegmentMulti([]string{"CDG"}, []string{"HND"}, futureDate, opts),
		buildSegmentMulti([]string{"HND"}, []string{"ICN"}, futureDate, opts),
		buildSegmentMulti([]string{"ICN"}, []string{"CDG"}, futureDate, opts),
	}
	filters := buildFiltersFromSegments(segs, 3, opts).([]any)
	settings := filters[1].([]any)
	if settings[2] != 3 {
		t.Errorf("tripType = %v, want 3", settings[2])
	}
	gotSegs, ok := settings[13].([]any)
	if !ok || len(gotSegs) != 3 {
		t.Errorf("segments slot = %#v, want 3 segments", settings[13])
	}
}

// TestBuildFilters_RegressionOneWayRoundTrip verifies the buildFilters refactor
// still emits tripType 2 (one-way) and 1 (round-trip) unchanged.
func TestBuildFilters_RegressionOneWayRoundTrip(t *testing.T) {
	opts := SearchOptions{}
	opts.defaults()

	oneWay := buildFilters("CDG", "HND", futureDate, opts).([]any)
	if oneWay[1].([]any)[2] != 2 {
		t.Errorf("one-way tripType = %v, want 2", oneWay[1].([]any)[2])
	}
	if got := oneWay[1].([]any)[13].([]any); len(got) != 1 {
		t.Errorf("one-way segments = %d, want 1", len(got))
	}

	rtOpts := opts
	rtOpts.ReturnDate = futureDate
	roundTrip := buildFilters("CDG", "HND", futureDate, rtOpts).([]any)
	if roundTrip[1].([]any)[2] != 1 {
		t.Errorf("round-trip tripType = %v, want 1", roundTrip[1].([]any)[2])
	}
	if got := roundTrip[1].([]any)[13].([]any); len(got) != 2 {
		t.Errorf("round-trip segments = %d, want 2", len(got))
	}
}

func TestSearchMultiCity_RequiresTwoLegs(t *testing.T) {
	one := []Leg{{Origins: []string{"CDG"}, Destinations: []string{"HND"}, Date: futureDate}}
	res, err := SearchMultiCity(context.Background(), one, SearchOptions{})
	if err == nil {
		t.Fatal("expected error for single leg, got nil")
	}
	if res == nil || res.Success {
		t.Errorf("expected unsuccessful result, got %#v", res)
	}
}

func TestSearchMultiCity_RejectsIncompleteLeg(t *testing.T) {
	legs := []Leg{
		{Origins: []string{"CDG"}, Destinations: []string{"HND"}, Date: futureDate},
		{Origins: nil, Destinations: []string{"ICN"}, Date: futureDate}, // missing origin
	}
	_, err := SearchMultiCity(context.Background(), legs, SearchOptions{})
	if err == nil {
		t.Fatal("expected error for incomplete leg, got nil")
	}
}

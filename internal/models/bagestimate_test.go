package models

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestBagEstimateJSONThreeStates pins the wire contract that downstream
// consumers read. The CLI's --format json output is a public interface: a price
// cache parses it and decides which flight to store, so what `included` says
// matters as much as what it means internally.
//
// The bug this guards against: trvl used to emit `"included": false` alongside
// `"source": "unknown"`. Those are two different claims — "this fare carries no
// checked bag" and "we do not know what this fare carries" — and a bool
// collapses the second into the first. Any consumer reading `included` without
// also reading `source` was being told something trvl cannot support.
func TestBagEstimateJSONThreeStates(t *testing.T) {
	cases := []struct {
		name string
		est  BagEstimate
		want string
		why  string
	}{
		{
			name: "included",
			est:  BagEstimate{Included: BagIncluded(), Source: BagSourceTableSourced},
			want: `"included":true`,
			why:  "an affirmative verdict serialises as true",
		},
		{
			name: "not included",
			est:  BagEstimate{Included: BagNotIncluded(), Source: BagSourceTableSourced},
			want: `"included":false`,
			why:  "a verdict of no bag is still a verdict",
		},
		{
			name: "unknown",
			est:  BagEstimate{Source: BagSourceUnknown},
			want: `"included":null`,
			why:  "no evidence must not read as evidence of no bag",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, err := json.Marshal(tc.est)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if !strings.Contains(string(b), tc.want) {
				t.Errorf("got %s, want it to contain %s — %s", b, tc.want, tc.why)
			}
		})
	}
}

// TestBagEstimatePredicates pins that the three helpers disagree where it
// matters. HasBag and LacksBag are deliberately not each other's negation:
// under an unknown verdict both are false, which is what stops a missing
// allowance from being priced as a fee-bearing one, or passed by a bag filter.
func TestBagEstimatePredicates(t *testing.T) {
	unknown := BagEstimate{Source: BagSourceUnknown}
	if unknown.HasBag() {
		t.Error("unknown must not pass a bag-required filter")
	}
	if unknown.LacksBag() {
		t.Error("unknown must not be priced as a fare that charges for a bag")
	}
	if !unknown.IsUnknown() {
		t.Error("unknown must report itself as such")
	}

	yes := BagEstimate{Included: BagIncluded()}
	if !yes.HasBag() || yes.LacksBag() || yes.IsUnknown() {
		t.Errorf("an included verdict reads wrong: %+v", yes)
	}

	no := BagEstimate{Included: BagNotIncluded()}
	if no.HasBag() || !no.LacksBag() || no.IsUnknown() {
		t.Errorf("a not-included verdict reads wrong: %+v", no)
	}
}

// TestBagEstimateZeroValueIsUnknown covers the failure mode the pointer
// introduces: a BagEstimate built without setting Included at all. That must
// read as unknown rather than as "no bag", so a construction site someone
// forgets to update degrades into honesty rather than into a false claim.
func TestBagEstimateZeroValueIsUnknown(t *testing.T) {
	var zero BagEstimate
	if !zero.IsUnknown() {
		t.Error("a zero-value estimate must claim nothing")
	}
	if zero.HasBag() || zero.LacksBag() {
		t.Error("a zero-value estimate must not assert either verdict")
	}
}

package optimizer

import (
	"testing"

	"github.com/MikkoParkkola/trvl/internal/models"
)

// TestPriceBaggageUsesFlightVerdict pins that ranking prices the checked bag
// from the per-flight verdict the search resolved, not from a second,
// airline-level calculation. Two paths computing the same number is how they
// drift apart, and this one decides which trip trvl recommends.
func TestPriceBaggageUsesFlightVerdict(t *testing.T) {
	ryanair := models.FlightResult{
		Price: 87, Currency: "EUR",
		Legs:        []models.FlightLeg{{AirlineCode: "FR"}},
		AllInMin:    96.49,
		AllInMax:    147,
		BagEstimate: &models.BagEstimate{Included: models.BagNotIncluded(), Source: models.BagSourceTableSourced, AmountMin: 9.49, AmountMax: 60, Currency: "EUR"},
	}

	// bagCost covers every extra: Ryanair charges for the overhead cabin bag
	// too, so the checked fee is the difference between the two cases.
	withBag := &candidate{}
	priceBaggage(withBag, ryanair, OptimizeInput{NeedCheckedBag: true})
	withoutBag := &candidate{}
	priceBaggage(withoutBag, ryanair, OptimizeInput{NeedCheckedBag: false})

	checkedComponent := withBag.bagCost - withoutBag.bagCost
	if diff := checkedComponent - 9.49; diff > 0.001 || diff < -0.001 {
		t.Errorf("checked-bag component = %v, want the floor of the resolved range (9.49)", checkedComponent)
	}
	if withBag.bagTermsUnknown {
		t.Error("a sourced range is not unknown terms")
	}
	if withoutBag.bagCost <= 0 {
		t.Error("the cabin-bag fee still applies when no checked bag is needed")
	}
	c := withBag

	// Unknown terms must be flagged, never silently priced at zero — that is
	// what let uncovered airlines rank as though their bag were free.
	unknown := models.FlightResult{
		Price: 121, Currency: "EUR",
		Legs:        []models.FlightLeg{{AirlineCode: "JU"}},
		BagEstimate: &models.BagEstimate{Included: models.BagNotIncluded(), Source: models.BagSourceUnknown},
	}
	c = &candidate{}
	priceBaggage(c, unknown, OptimizeInput{NeedCheckedBag: true})
	if !c.bagTermsUnknown {
		t.Error("an airline nobody covers must be marked as unknown terms")
	}
}

// TestPriceBaggageFrequentFlyerSaving checks that status waiving a bag shows up
// as a saving rather than vanishing into an unexplained cheaper total.
func TestPriceBaggageFrequentFlyerSaving(t *testing.T) {
	// Vueling: in the table with a sourced fee, and a Oneworld carrier so
	// status can waive it.
	vy := models.FlightResult{
		Price: 102, Currency: "EUR",
		Legs:     []models.FlightLeg{{AirlineCode: "VY"}},
		AllInMin: 120,
		AllInMax: 222,
		// As the search resolves it for a Oneworld member: status grants the bag.
		BagEstimate: &models.BagEstimate{Included: models.BagIncluded(), Source: models.BagSourceFrequentFlyer},
	}

	c := &candidate{}
	priceBaggage(c, vy, OptimizeInput{
		NeedCheckedBag: true,
		FFStatuses:     []FFStatus{{Alliance: "oneworld", Tier: "sapphire"}},
	})
	if c.ffSavings != 18 {
		t.Errorf("ffSavings = %v, want the 18 EUR fee the status waived", c.ffSavings)
	}
}

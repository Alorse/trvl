// Package baggage — resolving a flight's checked-bag situation from the best
// evidence available, and saying which evidence that was.
package baggage

import (
	"fmt"

	"github.com/MikkoParkkola/trvl/internal/models"
)

// ResolveCheckedBag decides whether a flight includes a free checked bag, and
// what a bag would cost if it does not, from the best evidence available.
//
// providerChecked is what the flight provider reported: a count, or nil when it
// did not state one. A frequent-flyer entitlement wins over everything; then
// provider data; then the airline table; an airline in neither is reported as
// unknown rather than guessed at.
func ResolveCheckedBag(providerChecked *int, airlineCode string, ffStatuses []FFStatus) models.BagEstimate {
	ab, inTable := Get(airlineCode)

	// Frequent-flyer status can grant a free checked bag outright, and that
	// beats every other source. It is applied here rather than after filtering,
	// where it used to run: a bag the traveller's status entitles them to could
	// not rescue a flight the bag filter had already dropped.
	if benefit := bestBenefitForAirline(airlineCode, ffStatuses); benefit.ExtraCheckedBags > 0 {
		return models.BagEstimate{
			Included:  true,
			Source:    models.BagSourceFrequentFlyer,
			Reference: fmt.Sprintf("%s alliance status grants %d extra checked bag(s)", airlineCode, benefit.ExtraCheckedBags),
		}
	}

	if providerChecked != nil {
		if *providerChecked >= 1 {
			return models.BagEstimate{Included: true, Source: models.BagSourceProvider}
		}
		// The provider is authoritative that no bag is included, but it does
		// not price one — so the fee, if we can offer one, is still an estimate
		// and is labelled with the table's own provenance.
		est := models.BagEstimate{Included: false, Source: models.BagSourceProvider}
		if inTable {
			applyFee(&est, ab)
		}
		return est
	}

	if !inTable {
		return models.BagEstimate{Included: false, Source: models.BagSourceUnknown,
			Reference: "airline not covered by the baggage table"}
	}

	if ab.CheckedIncluded >= 1 {
		return models.BagEstimate{
			Included:  true,
			Source:    inclusionSource(ab),
			Reference: inclusionReference(ab),
			Verified:  ab.CheckedVerified,
		}
	}

	est := models.BagEstimate{
		Included:  false,
		Source:    inclusionSource(ab),
		Reference: inclusionReference(ab),
		Verified:  ab.CheckedVerified,
	}
	applyFee(&est, ab)
	return est
}

// inclusionSource reports whether the airline's allowance figure was read from
// a primary source or is an uncited table value.
func inclusionSource(ab AirlineBaggage) models.BagSource {
	if ab.CheckedSource != "" {
		return models.BagSourceTableSourced
	}
	return models.BagSourceTableUnsourced
}

func inclusionReference(ab AirlineBaggage) string {
	if ab.CheckedSource != "" {
		return ab.CheckedSource
	}
	return fmt.Sprintf("%s baggage table: %d checked bag(s), no primary source", ab.Code, ab.CheckedIncluded)
}

// applyFee attaches the fee range and its provenance.
//
// Source is left alone: it describes the INCLUSION verdict, which the caller has
// already established. A cited fee does not make an uncited allowance cited —
// Turkish publishes a fee page but no allowance table, and reporting that as
// sourced would launder the weaker claim behind the stronger one.
func applyFee(est *models.BagEstimate, ab AirlineBaggage) {
	switch {
	case ab.FeeVaries:
		est.Reference = "airline publishes no fixed fee; it varies by route and date"
		if ab.FeeSource != "" {
			est.Reference += " — " + ab.FeeSource
			est.Verified = ab.FeeVerified
		}
	case ab.CheckedFeeMin > 0 && ab.CheckedFeeMax >= ab.CheckedFeeMin:
		est.AmountMin, est.AmountMax = ab.CheckedFeeMin, ab.CheckedFeeMax
		est.Currency = ab.FeeCurrency
		if est.Currency == "" {
			est.Currency = "EUR"
		}
		if ab.FeeSource != "" {
			est.Reference += " — fee: " + ab.FeeSource
		}
	case ab.CheckedFee > 0:
		est.AmountMin, est.AmountMax = ab.CheckedFee, ab.CheckedFee
		est.Currency = "EUR"
		est.Reference += " — fee: baggage table figure with no primary source"
	}
}

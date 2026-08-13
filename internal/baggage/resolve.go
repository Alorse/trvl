// Package baggage — resolving a flight's checked-bag situation from the best
// evidence available, and saying which evidence that was.
package baggage

import (
	"fmt"
	"time"

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

	return resolveFromTable(ab)
}

// nowFunc is the clock the staleness guard reads. Indirected for tests.
var nowFunc = time.Now

// Baggage allowances only ever shrink. Every change we have documented removed
// an allowance — ANA cut Europe fares from two pieces to one in November 2024,
// and nine carriers unbundled their cheapest long-haul brand to zero — and none
// restored one. So a table entry does not go stale in a random direction: it
// drifts toward claiming a bag the airline no longer gives.
//
// Downstream that is the expensive direction. A fare without a bag passes the
// filter, gets stored as the day's price, and a cache that only ratchets
// upward pins it there. A stale entry therefore manufactures exactly the
// failure the filter exists to prevent.
//
// Positive claims are doubted after bagClaimStaleAfter and stop being asserted
// after bagClaimExpiresAfter. Negative claims never expire: they cannot cause
// that failure, and airlines do not quietly start including bags again.
const (
	bagClaimStaleAfter   = 9 * 30 * 24 * time.Hour  // ~9 months: citation lapses
	bagClaimExpiresAfter = 18 * 30 * 24 * time.Hour // ~18 months: claim is dropped
)

// resolveFromTable turns a table entry into a verdict, applying the staleness
// guard to positive claims.
func resolveFromTable(ab AirlineBaggage) models.BagEstimate {
	est := models.BagEstimate{
		Included:  ab.CheckedIncluded >= 1,
		Source:    inclusionSource(ab),
		Reference: inclusionReference(ab),
		Verified:  ab.CheckedVerified,
	}

	if est.Included {
		switch age := claimAge(ab.CheckedVerified); {
		case age > bagClaimExpiresAfter:
			return models.BagEstimate{
				Included: false,
				Source:   models.BagSourceUnknown,
				Verified: ab.CheckedVerified,
				Reference: fmt.Sprintf("%s: allowance last verified %s and no longer relied on; allowances only shrink",
					ab.Code, ab.CheckedVerified),
			}
		case age > bagClaimStaleAfter:
			est.Source = models.BagSourceTableUnsourced
			est.Reference += fmt.Sprintf(" — verified %s, past its shelf life", ab.CheckedVerified)
		}
	}

	if !est.Included {
		applyFee(&est, ab)
	}
	return est
}

// claimAge reports how long ago a YYYY-MM verification stamp was taken. An
// absent or unparseable stamp counts as infinitely old, so an uncited positive
// claim is never treated as fresh.
func claimAge(verified string) time.Duration {
	if verified == "" {
		return 1<<62 - 1
	}
	t, err := time.Parse("2006-01", verified)
	if err != nil {
		return 1<<62 - 1
	}
	return nowFunc().Sub(t)
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

package points

import (
	"fmt"
	"strings"
)

// PointsValuation holds floor and ceiling CPP for a program.
// Kept for external callers that want raw valuation data.
type PointsValuation struct {
	Program    string
	FloorCPP   float64
	CeilingCPP float64
}

// Recommendation is the result of a points-vs-cash calculation.
type Recommendation struct {
	// ProgramSlug is the normalised program identifier.
	ProgramSlug string `json:"program_slug"`
	// ProgramName is the human-readable program name.
	ProgramName string `json:"program_name"`
	// CashPrice is the fare in USD (or whatever currency the caller supplies).
	CashPrice float64 `json:"cash_price"`
	// PointsRequired is the redemption cost.
	PointsRequired int `json:"points_required"`
	// CPP is the effective cents-per-point for this specific redemption.
	CPP float64 `json:"cpp"`
	// FloorCPP is the conservative baseline for this program.
	FloorCPP float64 `json:"floor_cpp"`
	// CeilingCPP is the aspirational sweet-spot value for this program.
	CeilingCPP float64 `json:"ceiling_cpp"`
	// Verdict is one of "use points", "pay cash", or "borderline".
	Verdict string `json:"verdict"`
	// Explanation provides a plain-English rationale.
	Explanation string `json:"explanation"`
}

// CalculateValue computes whether using points is worthwhile for a specific
// redemption.
//
// cashPrice is in any consistent currency unit (e.g. USD or EUR). The CPP
// analysis is currency-agnostic — what matters is the ratio.
//
// pointsRequired must be > 0. cashPrice must be > 0.
//
// Returns an error if program is unknown or inputs are invalid.
func CalculateValue(cashPrice float64, pointsRequired int, programSlug string) (*Recommendation, error) {
	slug := strings.ToLower(strings.TrimSpace(programSlug))

	prog := LookupProgram(slug)
	if prog == nil {
		// Build a helpful list of known slugs grouped by category.
		return nil, fmt.Errorf("unknown program %q — run `trvl points-value --list` to see supported programs", programSlug)
	}

	if cashPrice <= 0 {
		return nil, fmt.Errorf("cash price must be greater than 0")
	}
	if pointsRequired <= 0 {
		return nil, fmt.Errorf("points required must be greater than 0")
	}

	// CPP = (cashPrice * 100) / pointsRequired  →  cents per point.
	cpp := (cashPrice * 100) / float64(pointsRequired)

	verdict, explanation := evaluate(cpp, prog)

	return &Recommendation{
		ProgramSlug:    prog.Slug,
		ProgramName:    prog.Name,
		CashPrice:      cashPrice,
		PointsRequired: pointsRequired,
		CPP:            cpp,
		FloorCPP:       prog.FloorCPP,
		CeilingCPP:     prog.CeilingCPP,
		Verdict:        verdict,
		Explanation:    explanation,
	}, nil
}

// evaluate assigns a verdict and explanation based on how the effective CPP
// compares to the program's floor and ceiling.
func evaluate(cpp float64, prog *Program) (verdict, explanation string) {
	switch {
	case cpp >= prog.CeilingCPP:
		verdict = "use points"
		explanation = fmt.Sprintf(
			"Excellent redemption. You are getting %.2f¢/pt — at or above the sweet-spot value of %.2f¢/pt for %s. Use your points.",
			cpp, prog.CeilingCPP, prog.Name,
		)
	case cpp >= prog.FloorCPP:
		// Somewhere between floor and ceiling.
		midpoint := (prog.FloorCPP + prog.CeilingCPP) / 2
		if cpp >= midpoint {
			verdict = "use points"
			explanation = fmt.Sprintf(
				"Good redemption. You are getting %.2f¢/pt (floor %.2f¢/pt, ceiling %.2f¢/pt for %s). Above midpoint — worth using points.",
				cpp, prog.FloorCPP, prog.CeilingCPP, prog.Name,
			)
		} else {
			verdict = "borderline"
			explanation = fmt.Sprintf(
				"Below-average redemption. You are getting %.2f¢/pt — above the floor (%.2f¢/pt) but well below the sweet-spot (%.2f¢/pt) for %s. Consider saving points for a better opportunity.",
				cpp, prog.FloorCPP, prog.CeilingCPP, prog.Name,
			)
		}
	default:
		verdict = "pay cash"
		explanation = fmt.Sprintf(
			"Poor redemption. You are getting only %.2f¢/pt — below the floor value of %.2f¢/pt for %s. Pay cash and save your points.",
			cpp, prog.FloorCPP, prog.Name,
		)
	}
	return
}

// ListPrograms returns all programs, optionally filtered by category.
// Pass an empty category to get all programs.
func ListPrograms(category string) []Program {
	if category == "" {
		return Programs
	}
	var out []Program
	for _, p := range Programs {
		if strings.EqualFold(p.Category, category) {
			out = append(out, p)
		}
	}
	return out
}

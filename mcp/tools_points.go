package mcp

import (
	"context"
	"fmt"

	"github.com/MikkoParkkola/trvl/internal/points"
)

// --- Output schema ---

func calculatePointsValueOutputSchema() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"program_slug":    schemaString(),
			"program_name":    schemaString(),
			"cash_price":      schemaNum(),
			"points_required": schemaInt(),
			"cpp":             schemaNumDesc("Effective cents per point for this redemption"),
			"floor_cpp":       schemaNumDesc("Conservative baseline CPP for this program"),
			"ceiling_cpp":     schemaNumDesc("Sweet-spot CPP for this program"),
			"verdict":         map[string]interface{}{"type": "string", "enum": []string{"use points", "pay cash", "borderline"}},
			"explanation":     schemaString(),
		},
		"required": []string{"program_slug", "program_name", "cpp", "floor_cpp", "ceiling_cpp", "verdict", "explanation"},
	}
}

// --- Tool definition ---

func calculatePointsValueTool() ToolDef {
	return ToolDef{
		Name:  "calculate_points_value",
		Title: "Points vs Cash Calculator",
		Description: "Calculate whether redeeming loyalty points or paying cash is better for a specific redemption. " +
			"Returns the effective cents-per-point (cpp), program floor/ceiling valuations, a verdict, and a plain-English explanation. " +
			"Supports 20+ airline/hotel programs and 4 transferable currencies (Amex MR, Chase UR, Citi TYP, Capital One). " +
			"No API keys required — uses published valuation data.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"cash_price": {
					Type:        "number",
					Description: "The cash price of the redemption in your local currency (e.g. 450.00 for a $450 flight)",
				},
				"points_required": {
					Type:        "integer",
					Description: "Number of points or miles required for the redemption (e.g. 60000)",
				},
				"program": {
					Type:        "string",
					Description: "Loyalty program slug. Examples: finnair-plus, ana-mileage-club, world-of-hyatt, amex-mr, chase-ur.",
				},
			},
			Required: []string{"cash_price", "points_required", "program"},
		},
		OutputSchema: calculatePointsValueOutputSchema(),
		Annotations: &ToolAnnotations{
			Title:          "Points vs Cash Calculator",
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  false,
		},
	}
}

// --- Handler ---

func handleCalculatePointsValue(_ context.Context, args map[string]any, _ ElicitFunc, _ SamplingFunc, _ ProgressFunc) ([]ContentBlock, interface{}, error) {
	cashPrice := argFloat(args, "cash_price", 0)
	pointsRequired := argInt(args, "points_required", 0)
	programSlug := argString(args, "program")

	if programSlug == "" {
		return nil, nil, fmt.Errorf("program is required")
	}
	if cashPrice <= 0 {
		return nil, nil, fmt.Errorf("cash_price must be greater than 0")
	}
	if pointsRequired <= 0 {
		return nil, nil, fmt.Errorf("points_required must be greater than 0")
	}

	rec, err := points.CalculateValue(cashPrice, pointsRequired, programSlug)
	if err != nil {
		return nil, nil, err
	}

	summary := fmt.Sprintf(
		"%s: %.2f¢/pt (floor %.2f¢/pt, ceiling %.2f¢/pt) — %s. %s",
		rec.ProgramName, rec.CPP, rec.FloorCPP, rec.CeilingCPP, rec.Verdict, rec.Explanation,
	)

	content, err := buildAnnotatedContentBlocks(summary, rec)
	if err != nil {
		return nil, nil, err
	}

	return content, rec, nil
}

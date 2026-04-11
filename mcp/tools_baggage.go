package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/MikkoParkkola/trvl/internal/baggage"
)

// getBaggageRulesTool returns the MCP tool definition for airline baggage rules.
func getBaggageRulesTool() ToolDef {
	return ToolDef{
		Name:        "get_baggage_rules",
		Title:       "Airline Baggage Rules",
		Description: "Look up carry-on and checked baggage allowances for airlines. Covers full-service European carriers (KLM, Finnair, Lufthansa, BA, etc.), Gulf carriers (Emirates, Qatar, Singapore), and low-cost carriers (Ryanair, Wizz Air, easyJet, etc.). Pass airline IATA code for a specific airline, or \"all\" to list all.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"airline_code": {Type: "string", Description: "IATA airline code (e.g. KL, FR, U2) or \"all\" to list all airlines"},
			},
			Required: []string{"airline_code"},
		},
		OutputSchema: baggageOutputSchema(),
		Annotations: &ToolAnnotations{
			Title:          "Airline Baggage Rules",
			ReadOnlyHint:   true,
			OpenWorldHint:  false,
			IdempotentHint: true,
		},
	}
}

func baggageOutputSchema() interface{} {
	airlineSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"code":               map[string]interface{}{"type": "string"},
			"name":               map[string]interface{}{"type": "string"},
			"carry_on_max_kg":    map[string]interface{}{"type": "number"},
			"carry_on_dimensions": map[string]interface{}{"type": "string"},
			"personal_item":      map[string]interface{}{"type": "boolean"},
			"checked_included":   map[string]interface{}{"type": "integer"},
			"checked_fee_eur":    map[string]interface{}{"type": "number"},
			"overhead_only":      map[string]interface{}{"type": "boolean"},
			"notes":              map[string]interface{}{"type": "string"},
		},
	}
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"airline": airlineSchema,
			"airlines": map[string]interface{}{
				"type":  "array",
				"items": airlineSchema,
			},
			"found": map[string]interface{}{"type": "boolean"},
		},
	}
}

func handleGetBaggageRules(_ context.Context, args map[string]any, _ ElicitFunc, _ SamplingFunc, _ ProgressFunc) ([]ContentBlock, interface{}, error) {
	code := strings.ToUpper(strings.TrimSpace(argString(args, "airline_code")))

	if code == "ALL" || code == "" {
		airlines := baggage.All()
		summary := buildBaggageSummaryAll(airlines)
		type response struct {
			Airlines []baggage.AirlineBaggage `json:"airlines"`
			Found    bool                     `json:"found"`
		}
		resp := response{Airlines: airlines, Found: true}
		content := []ContentBlock{
			{Type: "text", Text: summary, Annotations: &ContentAnnotation{Audience: []string{"user"}, Priority: 1.0}},
			{Type: "text", Text: "Structured baggage data attached.", Annotations: &ContentAnnotation{Audience: []string{"assistant"}, Priority: 0.5}},
		}
		return content, resp, nil
	}

	ab, ok := baggage.Get(code)
	type response struct {
		Airline baggage.AirlineBaggage `json:"airline"`
		Found   bool                   `json:"found"`
	}
	resp := response{Airline: ab, Found: ok}

	var summary string
	if ok {
		summary = buildBaggageSummaryOne(ab)
	} else {
		summary = fmt.Sprintf("Airline %q not found in baggage database. Use airline_code=\"all\" to see all available airlines.", code)
	}

	content := []ContentBlock{
		{Type: "text", Text: summary, Annotations: &ContentAnnotation{Audience: []string{"user"}, Priority: 1.0}},
		{Type: "text", Text: "Structured baggage data attached.", Annotations: &ContentAnnotation{Audience: []string{"assistant"}, Priority: 0.5}},
	}
	return content, resp, nil
}

func buildBaggageSummaryOne(ab baggage.AirlineBaggage) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Baggage rules for %s (%s):\n\n", ab.Name, ab.Code))

	carryOn := "no weight limit"
	if ab.CarryOnMaxKg > 0 {
		carryOn = fmt.Sprintf("%.0f kg", ab.CarryOnMaxKg)
	}
	if ab.CarryOnDimensions != "" {
		carryOn += ", " + ab.CarryOnDimensions
	}
	sb.WriteString(fmt.Sprintf("  Carry-on: %s\n", carryOn))

	if ab.PersonalItem {
		sb.WriteString("  Personal item: yes (handbag/laptop bag)\n")
	} else {
		sb.WriteString("  Personal item: no\n")
	}

	if ab.CheckedIncluded > 0 {
		sb.WriteString(fmt.Sprintf("  Checked bags: %d included (23 kg)\n", ab.CheckedIncluded))
	} else if ab.CheckedFee > 0 {
		sb.WriteString(fmt.Sprintf("  Checked bags: not included, from EUR %.0f\n", ab.CheckedFee))
	} else {
		sb.WriteString("  Checked bags: not included\n")
	}

	if ab.OverheadOnly {
		sb.WriteString("\n  WARNING: Base fare only includes small under-seat bag. Overhead cabin bag costs extra.\n")
	}

	if ab.Notes != "" {
		sb.WriteString(fmt.Sprintf("\n  Notes: %s\n", ab.Notes))
	}
	return sb.String()
}

func buildBaggageSummaryAll(airlines []baggage.AirlineBaggage) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Baggage rules for %d airlines:\n\n", len(airlines)))
	for _, ab := range airlines {
		carryOn := "no limit"
		if ab.CarryOnMaxKg > 0 {
			carryOn = fmt.Sprintf("%.0fkg", ab.CarryOnMaxKg)
		}
		checked := "not included"
		if ab.CheckedIncluded > 0 {
			checked = fmt.Sprintf("%dx23kg included", ab.CheckedIncluded)
		} else if ab.CheckedFee > 0 {
			checked = fmt.Sprintf("~EUR%.0f", ab.CheckedFee)
		}
		lccMark := ""
		if ab.OverheadOnly {
			lccMark = " [LCC: overhead fee]"
		}
		sb.WriteString(fmt.Sprintf("  %s %-22s carry-on: %-6s  checked: %s%s\n",
			ab.Code, ab.Name, carryOn, checked, lccMark))
	}
	return sb.String()
}

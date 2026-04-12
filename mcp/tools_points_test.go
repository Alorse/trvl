package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	pointscalc "github.com/MikkoParkkola/trvl/internal/points"
)

func TestHandleCalculatePointsValue_Success(t *testing.T) {
	content, structured, err := handleCalculatePointsValue(context.Background(), map[string]any{
		"cash_price":      450.0,
		"points_required": 20000,
		"program":         "finnair-plus",
	}, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(content) != 2 {
		t.Fatalf("content len = %d, want 2", len(content))
	}
	if !strings.Contains(content[0].Text, "Finnair Plus") {
		t.Fatalf("summary = %q, want Finnair Plus", content[0].Text)
	}
	if !strings.Contains(content[0].Text, "2.25¢/pt") {
		t.Fatalf("summary = %q, want cpp text", content[0].Text)
	}

	rec, ok := structured.(*pointscalc.Recommendation)
	if !ok {
		t.Fatalf("structured type = %T, want *points.Recommendation", structured)
	}
	if rec.ProgramSlug != "finnair-plus" {
		t.Fatalf("program_slug = %q, want finnair-plus", rec.ProgramSlug)
	}
	if rec.Verdict != "use points" {
		t.Fatalf("verdict = %q, want use points", rec.Verdict)
	}
}

func TestHandleCalculatePointsValue_MissingProgram(t *testing.T) {
	_, _, err := handleCalculatePointsValue(context.Background(), map[string]any{
		"cash_price":      450.0,
		"points_required": 20000,
	}, nil, nil, nil)
	if err == nil {
		t.Fatal("expected missing program error")
	}
	if got := err.Error(); got != "program is required" {
		t.Fatalf("error = %q, want %q", got, "program is required")
	}
}

func TestHandleCalculatePointsValue_InvalidCashPrice(t *testing.T) {
	_, _, err := handleCalculatePointsValue(context.Background(), map[string]any{
		"cash_price":      0.0,
		"points_required": 20000,
		"program":         "finnair-plus",
	}, nil, nil, nil)
	if err == nil {
		t.Fatal("expected invalid cash price error")
	}
	if got := err.Error(); got != "cash_price must be greater than 0" {
		t.Fatalf("error = %q, want %q", got, "cash_price must be greater than 0")
	}
}

func TestHandleCalculatePointsValue_UnknownProgram(t *testing.T) {
	_, _, err := handleCalculatePointsValue(context.Background(), map[string]any{
		"cash_price":      450.0,
		"points_required": 20000,
		"program":         "unknown-program",
	}, nil, nil, nil)
	if err == nil {
		t.Fatal("expected unknown program error")
	}
	if !strings.Contains(err.Error(), "unknown program") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestToolsCallCalculatePointsValue(t *testing.T) {
	s := NewServer()
	resp := sendRequest(t, s, "tools/call", 42, ToolCallParams{
		Name: "calculate_points_value",
		Arguments: map[string]any{
			"cash_price":      450.0,
			"points_required": 20000,
			"program":         "finnair-plus",
		},
	})
	if resp == nil {
		t.Fatal("expected response")
	}
	if resp.Error != nil {
		t.Fatalf("unexpected top-level error: %+v", resp.Error)
	}

	resultJSON, err := json.Marshal(resp.Result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	var result ToolCallResult
	if err := json.Unmarshal(resultJSON, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.IsError {
		t.Fatal("expected isError=false")
	}
	if len(result.Content) < 2 {
		t.Fatalf("content len = %d, want at least 2", len(result.Content))
	}
	if result.StructuredContent == nil {
		t.Fatal("expected structuredContent")
	}

	structuredJSON, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structuredContent: %v", err)
	}
	var rec pointscalc.Recommendation
	if err := json.Unmarshal(structuredJSON, &rec); err != nil {
		t.Fatalf("unmarshal structuredContent: %v", err)
	}
	if rec.ProgramSlug != "finnair-plus" {
		t.Fatalf("program_slug = %q, want finnair-plus", rec.ProgramSlug)
	}
	if rec.Verdict != "use points" {
		t.Fatalf("verdict = %q, want use points", rec.Verdict)
	}
}

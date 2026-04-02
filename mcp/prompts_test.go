package mcp

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPromptsList(t *testing.T) {
	s := NewServer()
	resp := sendRequest(t, s, "prompts/list", 1, nil)
	if resp == nil {
		t.Fatal("expected response, got nil")
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}

	resultJSON, _ := json.Marshal(resp.Result)
	var result PromptsListResult
	if err := json.Unmarshal(resultJSON, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if len(result.Prompts) != 4 {
		t.Fatalf("expected 4 prompts, got %d", len(result.Prompts))
	}

	expected := map[string]bool{
		"plan-trip":           false,
		"where-should-i-go":  false,
		"find-cheapest-dates": false,
		"compare-hotels":     false,
	}
	for _, p := range result.Prompts {
		if _, ok := expected[p.Name]; !ok {
			t.Errorf("unexpected prompt: %s", p.Name)
		}
		expected[p.Name] = true
		if p.Description == "" {
			t.Errorf("prompt %s has empty description", p.Name)
		}
	}
	for name, found := range expected {
		if !found {
			t.Errorf("missing prompt: %s", name)
		}
	}
}

func TestPromptsGet_PlanTrip(t *testing.T) {
	s := NewServer()
	params := PromptsGetParams{
		Name: "plan-trip",
		Arguments: map[string]any{
			"origin":         "HEL",
			"destination":    "NRT",
			"departure_date": "2026-06-15",
			"return_date":    "2026-06-22",
			"budget":         "3000",
		},
	}
	resp := sendRequest(t, s, "prompts/get", 1, params)
	if resp == nil {
		t.Fatal("expected response, got nil")
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}

	resultJSON, _ := json.Marshal(resp.Result)
	var result PromptsGetResult
	if err := json.Unmarshal(resultJSON, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(result.Messages) == 0 {
		t.Fatal("expected messages")
	}
	if result.Messages[0].Role != "user" {
		t.Errorf("role = %q, want user", result.Messages[0].Role)
	}
	text := result.Messages[0].Content.Text
	if !strings.Contains(text, "HEL") || !strings.Contains(text, "NRT") {
		t.Error("prompt should contain origin and destination")
	}
	if !strings.Contains(text, "3000") {
		t.Error("prompt should contain budget")
	}
}

func TestPromptsGet_PlanTrip_MissingArgs(t *testing.T) {
	s := NewServer()
	params := PromptsGetParams{
		Name:      "plan-trip",
		Arguments: map[string]any{"origin": "HEL"},
	}
	resp := sendRequest(t, s, "prompts/get", 1, params)
	if resp == nil {
		t.Fatal("expected response, got nil")
	}
	if resp.Error == nil {
		t.Fatal("expected error for missing args")
	}
}

func TestPromptsGet_FindCheapestDates(t *testing.T) {
	s := NewServer()
	params := PromptsGetParams{
		Name: "find-cheapest-dates",
		Arguments: map[string]any{
			"origin":      "HEL",
			"destination": "NRT",
			"month":       "june-2026",
		},
	}
	resp := sendRequest(t, s, "prompts/get", 1, params)
	if resp == nil {
		t.Fatal("expected response, got nil")
	}
	if resp.Error != nil {
		t.Fatalf("error: %+v", resp.Error)
	}

	resultJSON, _ := json.Marshal(resp.Result)
	var result PromptsGetResult
	json.Unmarshal(resultJSON, &result)

	if len(result.Messages) == 0 {
		t.Fatal("expected messages")
	}
	if !strings.Contains(result.Messages[0].Content.Text, "june-2026") {
		t.Error("prompt should contain month")
	}
}

func TestPromptsGet_CompareHotels(t *testing.T) {
	s := NewServer()
	params := PromptsGetParams{
		Name: "compare-hotels",
		Arguments: map[string]any{
			"location":   "Tokyo",
			"check_in":   "2026-06-15",
			"check_out":  "2026-06-22",
			"priorities": "price,rating",
		},
	}
	resp := sendRequest(t, s, "prompts/get", 1, params)
	if resp == nil {
		t.Fatal("expected response, got nil")
	}
	if resp.Error != nil {
		t.Fatalf("error: %+v", resp.Error)
	}

	resultJSON, _ := json.Marshal(resp.Result)
	var result PromptsGetResult
	json.Unmarshal(resultJSON, &result)

	if len(result.Messages) == 0 {
		t.Fatal("expected messages")
	}
	if !strings.Contains(result.Messages[0].Content.Text, "price,rating") {
		t.Error("prompt should contain priorities")
	}
}

func TestPromptsGet_UnknownPrompt(t *testing.T) {
	s := NewServer()
	params := PromptsGetParams{Name: "nonexistent"}
	resp := sendRequest(t, s, "prompts/get", 1, params)
	if resp == nil {
		t.Fatal("expected response")
	}
	if resp.Error == nil {
		t.Fatal("expected error for unknown prompt")
	}
}

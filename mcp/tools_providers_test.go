package mcp

import (
	"context"
	"testing"

	"github.com/MikkoParkkola/trvl/internal/providers"
)

func testRegistry(t *testing.T) *providers.Registry {
	t.Helper()
	dir := t.TempDir()
	reg, err := providers.NewRegistryAt(dir)
	if err != nil {
		t.Fatalf("NewRegistryAt: %v", err)
	}
	return reg
}

func TestHandleConfigureProvider_NoElicitation(t *testing.T) {
	reg := testRegistry(t)
	args := map[string]any{
		"id":           "test-provider",
		"name":         "Test Provider",
		"category":     "hotels",
		"endpoint":     "https://api.example.com/search",
		"results_path": "$.results",
		"field_mapping": map[string]any{
			"name":  "$.hotel_name",
			"price": "$.price.total",
		},
	}

	// With elicit == nil, should return a CLI instruction message.
	content, _, err := handleConfigureProvider(context.Background(), args, nil, nil, nil, reg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(content) == 0 {
		t.Fatal("expected content blocks")
	}
	if content[0].Type != "text" {
		t.Errorf("expected text block, got %q", content[0].Type)
	}
	if got := content[0].Text; got == "" {
		t.Error("expected non-empty text")
	}
	// Should mention CLI command.
	if !containsString(content[0].Text, "trvl provider add") {
		t.Errorf("expected CLI instruction in response, got: %s", content[0].Text)
	}

	// Provider should NOT be saved.
	if reg.Get("test-provider") != nil {
		t.Error("provider should not be saved without elicitation")
	}
}

func TestHandleConfigureProvider_ElicitDecline(t *testing.T) {
	reg := testRegistry(t)
	args := map[string]any{
		"id":           "test-decline",
		"name":         "Decline Provider",
		"category":     "flights",
		"endpoint":     "https://api.example.com/flights",
		"results_path": "$.data",
		"field_mapping": map[string]any{
			"name": "$.flight_name",
		},
	}

	// Elicit returns nil (user dismissed).
	elicit := func(message string, schema map[string]interface{}) (map[string]interface{}, error) {
		return nil, nil
	}

	content, _, err := handleConfigureProvider(context.Background(), args, elicit, nil, nil, reg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(content) == 0 {
		t.Fatal("expected content blocks")
	}
	if !containsString(content[0].Text, "not enabled") {
		t.Errorf("expected decline message, got: %s", content[0].Text)
	}
	if reg.Get("test-decline") != nil {
		t.Error("provider should not be saved after decline")
	}
}

func TestHandleConfigureProvider_ElicitNo(t *testing.T) {
	reg := testRegistry(t)
	args := map[string]any{
		"id":           "test-no",
		"name":         "No Provider",
		"category":     "ground",
		"endpoint":     "https://api.example.com/ground",
		"results_path": "$.results",
		"field_mapping": map[string]any{
			"name": "$.route_name",
		},
	}

	elicit := func(message string, schema map[string]interface{}) (map[string]interface{}, error) {
		return map[string]interface{}{"enable": "no"}, nil
	}

	content, _, err := handleConfigureProvider(context.Background(), args, elicit, nil, nil, reg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !containsString(content[0].Text, "not enabled") {
		t.Errorf("expected decline message, got: %s", content[0].Text)
	}
	if reg.Get("test-no") != nil {
		t.Error("provider should not be saved after 'no'")
	}
}

func TestHandleConfigureProvider_ElicitYes(t *testing.T) {
	reg := testRegistry(t)
	args := map[string]any{
		"id":           "agoda-hotels",
		"name":         "Agoda",
		"category":     "hotels",
		"endpoint":     "https://api.agoda.com/search",
		"results_path": "$.results",
		"field_mapping": map[string]any{
			"name":  "$.hotel_name",
			"price": "$.price",
		},
		"rate_limit_rps": 1.0,
	}

	var elicitMessage string
	elicit := func(message string, schema map[string]interface{}) (map[string]interface{}, error) {
		elicitMessage = message
		return map[string]interface{}{"enable": "yes"}, nil
	}

	content, structured, err := handleConfigureProvider(context.Background(), args, elicit, nil, nil, reg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify elicitation message includes provider name and domain.
	if !containsString(elicitMessage, "Agoda") {
		t.Errorf("elicit message should mention provider name, got: %s", elicitMessage)
	}
	if !containsString(elicitMessage, "api.agoda.com") {
		t.Errorf("elicit message should mention domain, got: %s", elicitMessage)
	}

	// Verify success response.
	if len(content) == 0 {
		t.Fatal("expected content blocks")
	}
	if !containsString(content[0].Text, "enabled") {
		t.Errorf("expected success message, got: %s", content[0].Text)
	}

	// Verify structured output.
	if structured == nil {
		t.Fatal("expected structured output")
	}
	config, ok := structured.(*providers.ProviderConfig)
	if !ok {
		t.Fatalf("structured output type = %T, want *providers.ProviderConfig", structured)
	}
	if config.Consent == nil || !config.Consent.Granted {
		t.Error("consent should be granted")
	}
	if config.Consent == nil || config.Consent.Domain != "api.agoda.com" {
		t.Errorf("consent domain = %q, want api.agoda.com", config.Consent.Domain)
	}

	// Verify provider is saved in registry.
	saved := reg.Get("agoda-hotels")
	if saved == nil {
		t.Fatal("provider should be saved in registry")
	}
	if saved.Name != "Agoda" {
		t.Errorf("saved name = %q, want Agoda", saved.Name)
	}
	if saved.RateLimit.RequestsPerSecond != 1.0 {
		t.Errorf("saved rate_limit_rps = %v, want 1.0", saved.RateLimit.RequestsPerSecond)
	}
}

func TestHandleConfigureProvider_ValidationError(t *testing.T) {
	reg := testRegistry(t)
	args := map[string]any{
		"id":       "bad-provider",
		"name":     "Bad",
		"category": "invalid_category",
		"endpoint": "https://api.example.com",
		"results_path": "$.results",
		"field_mapping": map[string]any{
			"name": "$.name",
		},
	}

	_, _, err := handleConfigureProvider(context.Background(), args, nil, nil, nil, reg, nil)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !containsString(err.Error(), "invalid category") {
		t.Errorf("expected category validation error, got: %v", err)
	}
}

func TestHandleListProviders_Empty(t *testing.T) {
	reg := testRegistry(t)
	content, _, err := handleListProviders(context.Background(), nil, nil, nil, nil, reg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(content) == 0 {
		t.Fatal("expected content blocks")
	}
	if !containsString(content[0].Text, "No external providers configured") {
		t.Errorf("expected empty message, got: %s", content[0].Text)
	}
}

func TestHandleListProviders_WithProviders(t *testing.T) {
	reg := testRegistry(t)

	// Add a provider directly.
	config := &providers.ProviderConfig{
		ID:       "test-list",
		Name:     "Test List Provider",
		Category: "hotels",
		Endpoint: "https://api.example.com/search",
		Method:   "POST",
		ResponseMapping: providers.ResponseMapping{
			ResultsPath: "$.results",
			Fields: map[string]string{
				"name": "$.hotel_name",
			},
		},
		Consent: &providers.ConsentRecord{
			Granted: true,
			Domain:  "api.example.com",
		},
	}
	if err := reg.Save(config); err != nil {
		t.Fatalf("Save: %v", err)
	}

	content, structured, err := handleListProviders(context.Background(), nil, nil, nil, nil, reg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(content) < 2 {
		t.Fatal("expected annotated content blocks (summary + JSON)")
	}
	if !containsString(content[0].Text, "1 provider(s) configured") {
		t.Errorf("expected provider count in summary, got: %s", content[0].Text)
	}
	if !containsString(content[0].Text, "Test List Provider") {
		t.Errorf("expected provider name in summary, got: %s", content[0].Text)
	}
	if structured == nil {
		t.Error("expected structured output")
	}
}

func TestHandleRemoveProvider_Success(t *testing.T) {
	reg := testRegistry(t)

	// Add a provider.
	config := &providers.ProviderConfig{
		ID:       "to-remove",
		Name:     "Remove Me",
		Category: "flights",
		Endpoint: "https://api.example.com/flights",
		Method:   "POST",
		ResponseMapping: providers.ResponseMapping{
			ResultsPath: "$.data",
			Fields: map[string]string{
				"name": "$.flight_name",
			},
		},
	}
	if err := reg.Save(config); err != nil {
		t.Fatalf("Save: %v", err)
	}

	args := map[string]any{"id": "to-remove"}
	content, _, err := handleRemoveProvider(context.Background(), args, nil, nil, nil, reg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !containsString(content[0].Text, "removed") {
		t.Errorf("expected removal confirmation, got: %s", content[0].Text)
	}
	if reg.Get("to-remove") != nil {
		t.Error("provider should be removed from registry")
	}
}

func TestHandleRemoveProvider_NotFound(t *testing.T) {
	reg := testRegistry(t)
	args := map[string]any{"id": "non-existent"}
	_, _, err := handleRemoveProvider(context.Background(), args, nil, nil, nil, reg, nil)
	if err == nil {
		t.Fatal("expected error for non-existent provider")
	}
}

func TestHandleRemoveProvider_MissingID(t *testing.T) {
	reg := testRegistry(t)
	_, _, err := handleRemoveProvider(context.Background(), map[string]any{}, nil, nil, nil, reg, nil)
	if err == nil {
		t.Fatal("expected error for missing id")
	}
}

func TestExtractDomain(t *testing.T) {
	tests := []struct {
		endpoint string
		want     string
	}{
		{"https://api.agoda.com/search", "api.agoda.com"},
		{"https://booking.com/api/v2/search", "booking.com"},
		{"http://localhost:8080/search", "localhost"},
		{"not-a-url", "not-a-url"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.endpoint, func(t *testing.T) {
			got := extractDomain(tt.endpoint)
			if got != tt.want {
				t.Errorf("extractDomain(%q) = %q, want %q", tt.endpoint, got, tt.want)
			}
		})
	}
}

func TestParseStringMap(t *testing.T) {
	// From map[string]any.
	m := parseStringMap(map[string]any{
		"Accept":       "application/json",
		"Content-Type": "text/plain",
	})
	if m["Accept"] != "application/json" {
		t.Errorf("Accept = %q", m["Accept"])
	}

	// From JSON string.
	m2 := parseStringMap(`{"key":"value"}`)
	if m2["key"] != "value" {
		t.Errorf("key = %q", m2["key"])
	}

	// From empty string.
	m3 := parseStringMap("")
	if m3 != nil {
		t.Errorf("expected nil for empty string, got %v", m3)
	}

	// From nil.
	m4 := parseStringMap(nil)
	if m4 != nil {
		t.Errorf("expected nil for nil, got %v", m4)
	}
}

func TestParseAuthExtractions(t *testing.T) {
	// From map.
	m := parseAuthExtractions(map[string]any{
		"token": map[string]any{
			"pattern":  `"token":"([^"]+)"`,
			"variable": "auth_token",
			"header":   "Authorization",
		},
	})
	if m == nil {
		t.Fatal("expected non-nil result")
	}
	if m["token"].Pattern != `"token":"([^"]+)"` {
		t.Errorf("Pattern = %q", m["token"].Pattern)
	}
	if m["token"].Variable != "auth_token" {
		t.Errorf("Variable = %q", m["token"].Variable)
	}

	// From JSON string.
	m2 := parseAuthExtractions(`{"csrf":{"pattern":"csrf=([a-z0-9]+)","variable":"csrf","header":"X-CSRF"}}`)
	if m2 == nil {
		t.Fatal("expected non-nil result from JSON")
	}
	if m2["csrf"].Header != "X-CSRF" {
		t.Errorf("Header = %q", m2["csrf"].Header)
	}

	// Nil input.
	if parseAuthExtractions(nil) != nil {
		t.Error("expected nil for nil input")
	}
}

func TestTextContent(t *testing.T) {
	blocks := textContent("hello world")
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if blocks[0].Type != "text" {
		t.Errorf("Type = %q, want text", blocks[0].Type)
	}
	if blocks[0].Text != "hello world" {
		t.Errorf("Text = %q, want hello world", blocks[0].Text)
	}
}

func TestConfigureProviderTool_Definition(t *testing.T) {
	tool := configureProviderTool()
	if tool.Name != "configure_provider" {
		t.Errorf("Name = %q", tool.Name)
	}
	if len(tool.InputSchema.Required) != 6 {
		t.Errorf("Required fields = %d, want 6", len(tool.InputSchema.Required))
	}
	if tool.Annotations == nil {
		t.Fatal("annotations should be set")
	}
	if tool.Annotations.ReadOnlyHint {
		t.Error("ReadOnlyHint should be false for write tool")
	}
}

func TestListProvidersTool_Definition(t *testing.T) {
	tool := listProvidersTool()
	if tool.Name != "list_providers" {
		t.Errorf("Name = %q", tool.Name)
	}
	if tool.Annotations == nil {
		t.Fatal("annotations should be set")
	}
	if !tool.Annotations.ReadOnlyHint {
		t.Error("ReadOnlyHint should be true for read-only tool")
	}
}

func TestRemoveProviderTool_Definition(t *testing.T) {
	tool := removeProviderTool()
	if tool.Name != "remove_provider" {
		t.Errorf("Name = %q", tool.Name)
	}
	if tool.Annotations == nil {
		t.Fatal("annotations should be set")
	}
	if !tool.Annotations.DestructiveHint {
		t.Error("DestructiveHint should be true for remove tool")
	}
}

func TestTestProviderTool_Definition(t *testing.T) {
	tool := testProviderTool()
	if tool.Name != "test_provider" {
		t.Errorf("Name = %q", tool.Name)
	}
	if len(tool.InputSchema.Required) != 1 || tool.InputSchema.Required[0] != "id" {
		t.Errorf("Required = %v, want [id]", tool.InputSchema.Required)
	}
	if tool.Annotations == nil {
		t.Fatal("annotations should be set")
	}
	if tool.Annotations.ReadOnlyHint {
		t.Error("ReadOnlyHint should be false (makes HTTP requests)")
	}
	if !tool.Annotations.IdempotentHint {
		t.Error("IdempotentHint should be true")
	}
	if tool.OutputSchema == nil {
		t.Error("OutputSchema should be set")
	}
}

func TestHandleTestProvider_MissingID(t *testing.T) {
	reg := testRegistry(t)
	_, _, err := handleTestProvider(context.Background(), map[string]any{}, nil, nil, nil, reg, nil)
	if err == nil {
		t.Fatal("expected error for missing id")
	}
	if !containsString(err.Error(), "id is required") {
		t.Errorf("expected 'id is required' error, got: %v", err)
	}
}

func TestHandleTestProvider_NotFound(t *testing.T) {
	reg := testRegistry(t)
	_, _, err := handleTestProvider(context.Background(), map[string]any{"id": "nonexistent"}, nil, nil, nil, reg, nil)
	if err == nil {
		t.Fatal("expected error for nonexistent provider")
	}
	if !containsString(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestSuggestProviders_ConfigSkeletons(t *testing.T) {
	reg := testRegistry(t)
	content, structured, err := handleSuggestProviders(context.Background(), map[string]any{}, nil, nil, nil, reg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(content) == 0 {
		t.Fatal("expected content blocks")
	}

	suggestions, ok := structured.([]providerSuggestion)
	if !ok {
		t.Fatalf("structured type = %T, want []providerSuggestion", structured)
	}

	for _, s := range suggestions {
		if s.ConfigSkeleton == nil {
			t.Errorf("provider %q has nil ConfigSkeleton", s.ID)
		}
	}
}

func TestSuggestProviders_SkeletonHasResponseMapping(t *testing.T) {
	reg := testRegistry(t)
	_, structured, err := handleSuggestProviders(context.Background(), map[string]any{}, nil, nil, nil, reg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	suggestions := structured.([]providerSuggestion)
	for _, s := range suggestions {
		if s.ConfigSkeleton == nil {
			continue
		}
		rm, ok := s.ConfigSkeleton["response_mapping"]
		if !ok {
			t.Errorf("provider %q skeleton missing response_mapping", s.ID)
			continue
		}
		rmMap, ok := rm.(map[string]any)
		if !ok {
			t.Errorf("provider %q response_mapping is not a map", s.ID)
			continue
		}
		if _, ok := rmMap["results_path"]; !ok {
			t.Errorf("provider %q response_mapping missing results_path", s.ID)
		}
		if _, ok := rmMap["fields"]; !ok {
			t.Errorf("provider %q response_mapping missing fields", s.ID)
		}
	}
}

// containsString checks if s contains sub.
func containsString(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsSubstring(s, sub))
}

func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

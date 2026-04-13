package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/MikkoParkkola/trvl/internal/providers"
)

// textContent returns a single-element text content block slice.
func textContent(s string) []ContentBlock {
	return []ContentBlock{{Type: "text", Text: s}}
}

// providerHandler is a tool handler that also receives the provider registry
// and the provider runtime.
type providerHandler func(ctx context.Context, args map[string]any, elicit ElicitFunc, sampling SamplingFunc, progress ProgressFunc, reg *providers.Registry, rt *providers.Runtime) ([]ContentBlock, interface{}, error)

// wrapProviderHandler adapts a providerHandler into a ToolHandler by injecting
// the server's provider registry and runtime.
func (s *Server) wrapProviderHandler(handler providerHandler) ToolHandler {
	return func(ctx context.Context, args map[string]any, elicit ElicitFunc, sampling SamplingFunc, progress ProgressFunc) ([]ContentBlock, interface{}, error) {
		if s.providerRegistry == nil {
			return nil, nil, fmt.Errorf("provider registry not available")
		}
		return handler(ctx, args, elicit, sampling, progress, s.providerRegistry, s.providerRuntime)
	}
}

// --- configure_provider ---

// configureProviderTool returns the MCP tool definition for configure_provider.
func configureProviderTool() ToolDef {
	return ToolDef{
		Name:  "configure_provider",
		Title: "Configure External Provider",
		Description: "Configure an external data provider for accommodation, transport, or restaurant search. " +
			"The user will be asked directly to confirm before the provider is enabled.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"id":                {Type: "string", Description: "Unique identifier for this provider (e.g. \"agoda-hotels\")."},
				"name":              {Type: "string", Description: "Human-readable provider name (e.g. \"Agoda\")."},
				"category":          {Type: "string", Description: "Provider category: hotels, flights, ground, restaurants, or reviews."},
				"endpoint":          {Type: "string", Description: "Full URL of the provider's search endpoint."},
				"method":            {Type: "string", Description: "HTTP method (default: POST)."},
				"headers":           {Type: "object", Description: "Extra HTTP headers as key-value pairs."},
				"query_params":      {Type: "object", Description: "URL query parameters as key-value pairs."},
				"body_template":     {Type: "string", Description: "Request body template with {{placeholder}} variables."},
				"auth_type":         {Type: "string", Description: "Authentication type: none, header, or preflight."},
				"auth_preflight_url": {Type: "string", Description: "URL for preflight auth request (when auth_type=preflight)."},
				"auth_extractions":  {Type: "object", Description: "Map of extraction name to {pattern, variable, header} for preflight auth."},
				"results_path":      {Type: "string", Description: "JSONPath to the results array in the response (e.g. \"$.data.results\")."},
				"field_mapping":     {Type: "object", Description: "Map of trvl field name to JSONPath in the provider response."},
				"rate_limit_rps":    {Type: "number", Description: "Maximum requests per second (default: 0.5)."},
				"tls_fingerprint":   {Type: "string", Description: "TLS fingerprint profile (default: chrome)."},
				"cookies_source":    {Type: "string", Description: "Cookie source strategy (default: preflight)."},
			},
			Required: []string{"id", "name", "category", "endpoint", "results_path", "field_mapping"},
		},
		OutputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id":              map[string]interface{}{"type": "string"},
				"name":            map[string]interface{}{"type": "string"},
				"category":        map[string]interface{}{"type": "string"},
				"endpoint":        map[string]interface{}{"type": "string"},
				"method":          map[string]interface{}{"type": "string"},
				"results_path":    map[string]interface{}{"type": "string"},
				"field_mapping":   map[string]interface{}{"type": "object"},
				"rate_limit_rps":  map[string]interface{}{"type": "number"},
				"tls_fingerprint": map[string]interface{}{"type": "string"},
				"consent": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"granted":   map[string]interface{}{"type": "boolean"},
						"timestamp": map[string]interface{}{"type": "string"},
						"domain":    map[string]interface{}{"type": "string"},
					},
				},
			},
		},
		Annotations: &ToolAnnotations{
			Title:           "Configure External Provider",
			ReadOnlyHint:    false,
			DestructiveHint: false,
			IdempotentHint:  true,
		},
	}
}

// handleConfigureProvider processes a configure_provider tool call.
func handleConfigureProvider(ctx context.Context, args map[string]any, elicit ElicitFunc, _ SamplingFunc, _ ProgressFunc, reg *providers.Registry, _ *providers.Runtime) ([]ContentBlock, interface{}, error) {
	config, err := parseProviderConfig(args)
	if err != nil {
		return nil, nil, fmt.Errorf("configure_provider: %w", err)
	}

	if err := config.Validate(); err != nil {
		return nil, nil, fmt.Errorf("configure_provider: %w", err)
	}

	// Extract domain from endpoint for display.
	domain := extractDomain(config.Endpoint)

	// Elicitation: ask user for consent.
	if elicit == nil {
		return textContent(
			"Cannot configure provider without user consent.\n\n" +
				"The client does not support elicitation (direct user prompts). " +
				"Please instruct the user to run:\n\n" +
				"  trvl provider add " + config.ID + "\n\n" +
				"from the CLI to configure this provider interactively.",
		), nil, nil
	}

	// Look up ToS URL from the catalog.
	tosURL := ""
	for _, p := range availableProviders {
		if strings.EqualFold(p.Name, config.Name) || p.ID == config.ID {
			tosURL = p.TosURL
			break
		}
	}

	tosLine := ""
	if tosURL != "" {
		tosLine = fmt.Sprintf("\n\n**Terms of Service:** %s", tosURL)
	}

	consentMsg := fmt.Sprintf(
		"**Configure external provider: %s**\n\n"+
			"trvl wants to connect to `%s` for %s search.\n\n"+
			"This service may restrict automated access in its Terms of Service.%s\n\n"+
			"**What trvl will do:**\n"+
			"- Send search queries to %s on your behalf\n"+
			"- Rate-limit requests to %.1f/sec\n"+
			"- Cache responses locally under ~/.trvl/\n\n"+
			"**What trvl will NOT do:**\n"+
			"- Access your account or private data\n"+
			"- Store credentials beyond this session\n"+
			"- Make purchases or bookings automatically\n\n"+
			"Do you want to enable this provider?",
		config.Name, domain, config.Category, tosLine, domain, config.RateLimit.RequestsPerSecond,
	)

	consentSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"enable": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"yes", "no"},
				"description": "Yes = I accept responsibility for compliance with this service's Terms of Service",
			},
		},
		"required": []string{"enable"},
	}

	result, err := elicit(consentMsg, consentSchema)
	if err != nil {
		return nil, nil, fmt.Errorf("configure_provider: elicitation failed: %w", err)
	}

	if result == nil {
		return textContent("Provider not enabled: user declined or dismissed the prompt."), nil, nil
	}

	enableVal, _ := result["enable"].(string)
	if enableVal != "yes" {
		return textContent("Provider not enabled: user chose not to enable " + config.Name + "."), nil, nil
	}

	// Record consent.
	config.Consent = &providers.ConsentRecord{
		Granted:   true,
		Timestamp: time.Now(),
		Domain:    domain,
	}

	// Save to registry.
	if err := reg.Save(config); err != nil {
		return nil, nil, fmt.Errorf("configure_provider: save: %w", err)
	}

	summary := fmt.Sprintf("Provider %q enabled for %s search (domain: %s, rate limit: %.1f rps).",
		config.Name, config.Category, domain, config.RateLimit.RequestsPerSecond)
	return textContent(summary), config, nil
}

// parseProviderConfig extracts a ProviderConfig from MCP tool arguments.
func parseProviderConfig(args map[string]any) (*providers.ProviderConfig, error) {
	config := &providers.ProviderConfig{
		ID:           argString(args, "id"),
		Name:         argString(args, "name"),
		Category:     argString(args, "category"),
		Endpoint:     argString(args, "endpoint"),
		Method:       argString(args, "method"),
		BodyTemplate: argString(args, "body_template"),
		ResponseMapping: providers.ResponseMapping{
			ResultsPath: argString(args, "results_path"),
		},
		RateLimit: providers.RateLimitConfig{
			RequestsPerSecond: argFloat(args, "rate_limit_rps", 0),
		},
		TLS: providers.TLSConfig{
			Fingerprint: argString(args, "tls_fingerprint"),
		},
		Cookies: providers.CookieConfig{
			Source: argString(args, "cookies_source"),
		},
	}

	// Build Auth config if auth_type is provided.
	if authType := argString(args, "auth_type"); authType != "" {
		config.Auth = &providers.AuthConfig{
			Type:         authType,
			PreflightURL: argString(args, "auth_preflight_url"),
		}
		if v, ok := args["auth_extractions"]; ok {
			config.Auth.Extractions = parseAuthExtractions(v)
		}
	}

	// Parse headers (map[string]string).
	if v, ok := args["headers"]; ok {
		config.Headers = parseStringMap(v)
	}

	// Parse query_params (map[string]string).
	if v, ok := args["query_params"]; ok {
		config.QueryParams = parseStringMap(v)
	}

	// Parse field_mapping into ResponseMapping.Fields.
	if v, ok := args["field_mapping"]; ok {
		config.ResponseMapping.Fields = parseStringMap(v)
	}

	return config, nil
}

// parseStringMap converts a map[string]any to map[string]string.
// Also handles a JSON string encoding.
func parseStringMap(v any) map[string]string {
	switch val := v.(type) {
	case map[string]any:
		result := make(map[string]string, len(val))
		for k, v := range val {
			if s, ok := v.(string); ok {
				result[k] = s
			}
		}
		return result
	case string:
		if val == "" {
			return nil
		}
		var result map[string]string
		if err := json.Unmarshal([]byte(val), &result); err != nil {
			return nil
		}
		return result
	default:
		return nil
	}
}

// parseAuthExtractions converts a map[string]any to map[string]Extraction.
func parseAuthExtractions(v any) map[string]providers.Extraction {
	m, ok := v.(map[string]any)
	if !ok {
		// Try JSON string.
		if s, ok := v.(string); ok && s != "" {
			var result map[string]providers.Extraction
			if err := json.Unmarshal([]byte(s), &result); err != nil {
				return nil
			}
			return result
		}
		return nil
	}

	result := make(map[string]providers.Extraction, len(m))
	for name, val := range m {
		em, ok := val.(map[string]any)
		if !ok {
			continue
		}
		ext := providers.Extraction{}
		if s, ok := em["pattern"].(string); ok {
			ext.Pattern = s
		}
		if s, ok := em["variable"].(string); ok {
			ext.Variable = s
		}
		if s, ok := em["header"].(string); ok {
			ext.Header = s
		}
		result[name] = ext
	}
	return result
}

// extractDomain returns the hostname from a URL, or the URL itself if parsing fails.
func extractDomain(endpoint string) string {
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" {
		// Fallback: try to extract something useful.
		parts := strings.SplitN(endpoint, "/", 4)
		if len(parts) >= 3 {
			return parts[2]
		}
		return endpoint
	}
	return u.Hostname()
}

// --- list_providers ---

// listProvidersTool returns the MCP tool definition for list_providers.
func listProvidersTool() ToolDef {
	return ToolDef{
		Name:        "list_providers",
		Title:       "List External Providers",
		Description: "List all configured external data providers with their status, consent, and error counts.",
		InputSchema: InputSchema{
			Type:       "object",
			Properties: map[string]Property{},
		},
		OutputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"providers": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"id":           map[string]interface{}{"type": "string"},
							"name":         map[string]interface{}{"type": "string"},
							"category":     map[string]interface{}{"type": "string"},
							"domain":       map[string]interface{}{"type": "string"},
							"consent":      map[string]interface{}{"type": "boolean"},
							"last_success": map[string]interface{}{"type": "string"},
							"error_count":  map[string]interface{}{"type": "integer"},
						},
					},
				},
			},
		},
		Annotations: &ToolAnnotations{
			Title:          "List External Providers",
			ReadOnlyHint:   true,
			IdempotentHint: true,
		},
	}
}

// handleListProviders processes a list_providers tool call.
func handleListProviders(_ context.Context, _ map[string]any, _ ElicitFunc, _ SamplingFunc, _ ProgressFunc, reg *providers.Registry, _ *providers.Runtime) ([]ContentBlock, interface{}, error) {
	configs := reg.List()

	if len(configs) == 0 {
		return textContent("No external providers configured. Use configure_provider to add one."), nil, nil
	}

	type providerSummary struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Category    string `json:"category"`
		Domain      string `json:"domain"`
		Consent     bool   `json:"consent"`
		LastSuccess string `json:"last_success,omitempty"`
		ErrorCount  int    `json:"error_count"`
	}

	summaries := make([]providerSummary, 0, len(configs))
	var lines []string

	for _, c := range configs {
		domain := extractDomain(c.Endpoint)
		lastSuccess := ""
		if !c.LastSuccess.IsZero() {
			lastSuccess = c.LastSuccess.Format(time.RFC3339)
		}

		consentGranted := c.Consent != nil && c.Consent.Granted
		summaries = append(summaries, providerSummary{
			ID:          c.ID,
			Name:        c.Name,
			Category:    c.Category,
			Domain:      domain,
			Consent:     consentGranted,
			LastSuccess: lastSuccess,
			ErrorCount:  c.ErrorCount,
		})

		status := "enabled"
		if !consentGranted {
			status = "no consent"
		}
		line := fmt.Sprintf("- %s (%s) [%s] %s", c.Name, c.Category, status, domain)
		if c.ErrorCount > 0 {
			line += fmt.Sprintf(" (%d errors)", c.ErrorCount)
		}
		lines = append(lines, line)
	}

	summary := fmt.Sprintf("%d provider(s) configured:\n%s", len(configs), strings.Join(lines, "\n"))
	content, err := buildAnnotatedContentBlocks(summary, summaries)
	if err != nil {
		return nil, nil, err
	}
	return content, summaries, nil
}

// --- remove_provider ---

// removeProviderTool returns the MCP tool definition for remove_provider.
func removeProviderTool() ToolDef {
	return ToolDef{
		Name:        "remove_provider",
		Title:       "Remove External Provider",
		Description: "Remove a configured external data provider by ID. No confirmation needed.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"id": {Type: "string", Description: "ID of the provider to remove."},
			},
			Required: []string{"id"},
		},
		OutputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"message": map[string]interface{}{"type": "string"},
			},
		},
		Annotations: &ToolAnnotations{
			Title:           "Remove External Provider",
			ReadOnlyHint:    false,
			DestructiveHint: true,
			IdempotentHint:  true,
		},
	}
}

// handleRemoveProvider processes a remove_provider tool call.
func handleRemoveProvider(_ context.Context, args map[string]any, _ ElicitFunc, _ SamplingFunc, _ ProgressFunc, reg *providers.Registry, _ *providers.Runtime) ([]ContentBlock, interface{}, error) {
	id := argString(args, "id")
	if id == "" {
		return nil, nil, fmt.Errorf("remove_provider: id is required")
	}

	if err := reg.Delete(id); err != nil {
		return nil, nil, fmt.Errorf("remove_provider: %w", err)
	}

	return textContent(fmt.Sprintf("Provider %q removed.", id)), nil, nil
}

// --- test_provider ---

// testProviderTool returns the MCP tool definition for test_provider.
func testProviderTool() ToolDef {
	return ToolDef{
		Name:  "test_provider",
		Title: "Test Provider Configuration",
		Description: "Test a configured provider by making a single search request. " +
			"Returns detailed diagnostics including which step succeeded or failed " +
			"(preflight, auth extraction, search request, response parsing, field mapping). " +
			"Use this after configure_provider to verify the config works, and iterate on " +
			"failures without requiring re-consent.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"id":       {Type: "string", Description: "Provider ID to test."},
				"location": {Type: "string", Description: "Test location (default: Paris)."},
				"checkin":  {Type: "string", Description: "Test check-in date (default: tomorrow, YYYY-MM-DD)."},
				"checkout": {Type: "string", Description: "Test check-out date (default: day after tomorrow, YYYY-MM-DD)."},
			},
			Required: []string{"id"},
		},
		OutputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"success":            map[string]interface{}{"type": "boolean"},
				"step":               map[string]interface{}{"type": "string"},
				"http_status":        map[string]interface{}{"type": "integer"},
				"results_count":      map[string]interface{}{"type": "integer"},
				"error":              map[string]interface{}{"type": "string"},
				"extraction_results": map[string]interface{}{"type": "object"},
				"body_snippet":       map[string]interface{}{"type": "string"},
				"sample_result":      map[string]interface{}{"type": "object"},
			},
		},
		Annotations: &ToolAnnotations{
			Title:           "Test Provider Configuration",
			ReadOnlyHint:    false,
			DestructiveHint: false,
			IdempotentHint:  true,
		},
	}
}

// handleTestProvider processes a test_provider tool call.
func handleTestProvider(ctx context.Context, args map[string]any, _ ElicitFunc, _ SamplingFunc, _ ProgressFunc, reg *providers.Registry, _ *providers.Runtime) ([]ContentBlock, interface{}, error) {
	id := argString(args, "id")
	if id == "" {
		return nil, nil, fmt.Errorf("test_provider: id is required")
	}

	cfg := reg.Get(id)
	if cfg == nil {
		return nil, nil, fmt.Errorf("test_provider: provider %q not found", id)
	}

	// Default test parameters.
	location := argString(args, "location")
	if location == "" {
		location = "Paris"
	}
	checkin := argString(args, "checkin")
	checkout := argString(args, "checkout")
	if checkin == "" {
		checkin = time.Now().AddDate(0, 0, 1).Format("2006-01-02")
		checkout = time.Now().AddDate(0, 0, 2).Format("2006-01-02")
	}
	if checkout == "" {
		// checkin was provided but checkout was not.
		ci, err := time.Parse("2006-01-02", checkin)
		if err == nil {
			checkout = ci.AddDate(0, 0, 1).Format("2006-01-02")
		} else {
			checkout = time.Now().AddDate(0, 0, 2).Format("2006-01-02")
		}
	}

	// Paris coordinates.
	lat, lon := 48.8566, 2.3522

	result := providers.TestProvider(ctx, cfg, location, lat, lon, checkin, checkout, "EUR", 2)

	// Build summary text.
	var summary string
	if result.Success {
		summary = fmt.Sprintf("Provider %q test passed: %d results found at step %q.", id, result.ResultsCount, result.Step)
	} else {
		summary = fmt.Sprintf("Provider %q test failed at step %q: %s", id, result.Step, result.Error)
	}

	content, err := buildAnnotatedContentBlocks(summary, result)
	if err != nil {
		return nil, nil, err
	}
	return content, result, nil
}

// --- suggest_providers ---

// providerSuggestion describes an available provider that the user can configure.
// Returned by suggest_providers to help any LLM suggest and configure providers.
type providerSuggestion struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	Category       string         `json:"category"`
	Description    string         `json:"description"`
	AuthPattern    string         `json:"auth_pattern"`
	AuthHint       string         `json:"auth_hint"`
	Reference      string         `json:"reference"`
	TosURL         string         `json:"tos_url"`
	TLS            string         `json:"tls"`
	RateLimit      string         `json:"rate_limit"`
	Configured     bool           `json:"configured"`
	ConfigSkeleton map[string]any `json:"config_skeleton,omitempty"`
}

// availableProviders is the built-in catalog of providers that users can configure.
// Each entry contains enough metadata for any LLM to understand the provider and
// generate a working configure_provider call by consulting the reference project.
var availableProviders = []providerSuggestion{
	{
		ID:          "booking",
		Name:        "Booking.com",
		Category:    "hotels",
		Description: "Hotels and apartments worldwide.",
		AuthPattern: "graphql_csrf",
		AuthHint:    "GraphQL with session token",
		Reference:   "github.com/opentabs-dev/opentabs",
		TosURL:      "https://www.booking.com/content/terms.html",
		TLS:         "chrome",
		RateLimit:   "0.5 req/s",
		ConfigSkeleton: skeletonGraphQLCSRF(),
	},
	{
		ID:          "airbnb",
		Name:        "Airbnb",
		Category:    "hotels",
		Description: "Vacation rentals, apartments, and unique stays.",
		AuthPattern: "graphql_apikey",
		AuthHint:    "GraphQL with page-embedded key",
		Reference:   "github.com/johnbalvin/gobnb",
		TosURL:      "https://www.airbnb.com/terms",
		TLS:         "chrome",
		RateLimit:   "0.5 req/s",
		ConfigSkeleton: skeletonGraphQLAPIKey(),
	},
	{
		ID:          "vrbo",
		Name:        "VRBO",
		Category:    "hotels",
		Description: "Vacation rentals (Expedia Group).",
		AuthPattern: "graphql_headers",
		AuthHint:    "GraphQL with browser-like headers",
		Reference:   "search GitHub for vrbo graphql",
		TosURL:      "https://www.vrbo.com/legal/terms-and-conditions",
		TLS:         "chrome",
		RateLimit:   "0.5 req/s",
		ConfigSkeleton: skeletonGraphQLHeaders(),
	},
	{
		ID:          "hostelworld",
		Name:        "Hostelworld",
		Category:    "hotels",
		Description: "Hostels and budget accommodation worldwide.",
		AuthPattern: "rest_apikey",
		AuthHint:    "REST API with key from page source",
		Reference:   "search GitHub for hostelworld api",
		TosURL:      "https://www.hostelworld.com/securityprivacy/terms-and-conditions",
		TLS:         "standard",
		RateLimit:   "1 req/s",
		ConfigSkeleton: skeletonRESTAPIKey(),
	},
	{
		ID:          "tripadvisor",
		Name:        "TripAdvisor",
		Category:    "reviews",
		Description: "Hotel and restaurant reviews and ratings.",
		AuthPattern: "graphql_queryid",
		AuthHint:    "GraphQL with query IDs",
		Reference:   "search GitHub for tripadvisor graphql",
		TosURL:      "https://www.tripadvisor.com/pages/terms.html",
		TLS:         "standard",
		RateLimit:   "1 req/s",
		ConfigSkeleton: skeletonGraphQLQueryID(),
	},
	{
		ID:          "blablacar",
		Name:        "BlaBlaCar",
		Category:    "ground",
		Description: "Ridesharing across Europe.",
		AuthPattern: "rest_public",
		AuthHint:    "Public REST API, no authentication needed",
		Reference:   "search GitHub for blablacar api",
		TosURL:      "https://www.blablacar.com/about-us/terms-and-conditions",
		TLS:         "standard",
		RateLimit:   "2 req/s",
		ConfigSkeleton: skeletonRESTPublic(),
	},
	{
		ID:          "opentable",
		Name:        "OpenTable",
		Category:    "restaurants",
		Description: "Restaurant availability and reservations.",
		AuthPattern: "graphql_csrf",
		AuthHint:    "GraphQL with session token",
		Reference:   "search GitHub for opentable api",
		TosURL:      "https://www.opentable.com/legal/terms-and-conditions",
		TLS:         "chrome",
		RateLimit:   "0.5 req/s",
		ConfigSkeleton: skeletonGraphQLCSRF(),
	},
}

// --- Config skeleton builders ---

// skeletonResponseMapping returns the common response_mapping skeleton.
func skeletonResponseMapping() map[string]any {
	return map[string]any{
		"results_path": "FILL: dot-notation path to results array in response JSON",
		"fields": map[string]any{
			"name":     "FILL: path to hotel/property name",
			"hotel_id": "FILL: path to unique ID",
			"rating":   "FILL: path to rating score",
			"price":    "FILL: path to price amount",
			"currency": "FILL: path to currency code",
			"lat":      "FILL: path to latitude",
			"lon":      "FILL: path to longitude",
		},
	}
}

func skeletonGraphQLCSRF() map[string]any {
	return map[string]any{
		"auth": map[string]any{
			"type":          "preflight",
			"preflight_url": "FILL: URL of a search results page on the target service",
			"extractions": map[string]any{
				"csrf_token": map[string]any{
					"pattern":  "FILL: regex with capture group to extract CSRF/session token from HTML",
					"variable": "csrf_token",
				},
			},
		},
		"endpoint": "FILL: GraphQL endpoint URL",
		"method":   "POST",
		"headers": map[string]any{
			"Content-Type":    "application/json",
			"FILL_csrf_header": "${csrf_token}",
		},
		"body_template":    "FILL: GraphQL query JSON with ${checkin}, ${checkout}, ${location} or ${ne_lat}/${sw_lat} variables",
		"response_mapping": skeletonResponseMapping(),
	}
}

func skeletonGraphQLAPIKey() map[string]any {
	return map[string]any{
		"auth": map[string]any{
			"type":          "preflight",
			"preflight_url": "FILL: homepage or page that contains the API key in source",
			"extractions": map[string]any{
				"api_key": map[string]any{
					"pattern":  "FILL: regex to extract API key from page source",
					"variable": "api_key",
				},
			},
		},
		"endpoint": "FILL: GraphQL endpoint URL",
		"method":   "POST",
		"headers": map[string]any{
			"Content-Type":   "application/json",
			"FILL_key_header": "${api_key}",
		},
		"body_template":    "FILL: GraphQL persisted query JSON with ${checkin}, ${checkout}, ${ne_lat}/${sw_lat} variables",
		"response_mapping": skeletonResponseMapping(),
	}
}

func skeletonGraphQLHeaders() map[string]any {
	return map[string]any{
		"endpoint": "FILL: GraphQL endpoint URL",
		"method":   "POST",
		"headers": map[string]any{
			"Content-Type": "application/json",
			"FILL_header1": "FILL: browser-like header value",
			"FILL_header2": "FILL: additional required header",
		},
		"body_template":    "FILL: GraphQL persisted query JSON with ${checkin}, ${checkout}, ${ne_lat}/${sw_lat} variables",
		"response_mapping": skeletonResponseMapping(),
	}
}

func skeletonRESTAPIKey() map[string]any {
	return map[string]any{
		"auth": map[string]any{
			"type":          "preflight",
			"preflight_url": "FILL: page that contains the API key in source or headers",
			"extractions": map[string]any{
				"api_key": map[string]any{
					"pattern":  "FILL: regex to extract API key",
					"variable": "api_key",
				},
			},
		},
		"endpoint": "FILL: REST API endpoint URL with path parameters",
		"method":   "GET",
		"headers": map[string]any{
			"FILL_key_header": "${api_key}",
		},
		"query_params": map[string]any{
			"FILL_location_param": "${location}",
			"FILL_checkin_param":  "${checkin}",
			"FILL_checkout_param": "${checkout}",
			"FILL_guests_param":   "${guests}",
			"FILL_currency_param": "${currency}",
		},
		"response_mapping": skeletonResponseMapping(),
	}
}

func skeletonRESTPublic() map[string]any {
	return map[string]any{
		"endpoint": "FILL: public REST API endpoint URL",
		"method":   "GET",
		"query_params": map[string]any{
			"FILL_from_param": "${location}",
			"FILL_date_param": "${checkin}",
		},
		"response_mapping": skeletonResponseMapping(),
	}
}

func skeletonGraphQLQueryID() map[string]any {
	return map[string]any{
		"endpoint": "FILL: GraphQL endpoint URL",
		"method":   "POST",
		"headers": map[string]any{
			"Content-Type": "application/json",
		},
		"body_template":    "FILL: GraphQL query with pre-registered query ID, includes ${checkin}, ${checkout}, ${location} variables",
		"response_mapping": skeletonResponseMapping(),
	}
}

// suggestProvidersTool returns the MCP tool definition for suggest_providers.
func suggestProvidersTool() ToolDef {
	return ToolDef{
		Name:  "suggest_providers",
		Title: "Suggest Available Providers",
		Description: "Returns a catalog of external data providers that the user can enable " +
			"for additional hotel, transport, restaurant, and review sources. " +
			"Call this proactively after hotel searches to suggest additional sources, " +
			"or when the user asks about expanding their search coverage. " +
			"Each provider includes an auth pattern description and a reference to an " +
			"open-source project where the API integration details can be found. " +
			"Use configure_provider to enable a suggested provider (requires user consent).",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"category": {Type: "string", Description: "Filter by category: hotels, ground, restaurants, reviews. Empty returns all."},
			},
		},
		OutputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"providers": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"id":              map[string]interface{}{"type": "string"},
							"name":            map[string]interface{}{"type": "string"},
							"category":        map[string]interface{}{"type": "string"},
							"description":     map[string]interface{}{"type": "string"},
							"auth_pattern":    map[string]interface{}{"type": "string"},
							"auth_hint":       map[string]interface{}{"type": "string"},
							"reference":       map[string]interface{}{"type": "string"},
							"tls":             map[string]interface{}{"type": "string"},
							"rate_limit":      map[string]interface{}{"type": "string"},
							"configured":      map[string]interface{}{"type": "boolean"},
							"config_skeleton": map[string]interface{}{"type": "object"},
						},
					},
				},
			},
		},
		Annotations: &ToolAnnotations{
			Title:          "Suggest Available Providers",
			ReadOnlyHint:   true,
			IdempotentHint: true,
		},
	}
}

// handleSuggestProviders processes a suggest_providers tool call.
func handleSuggestProviders(_ context.Context, args map[string]any, _ ElicitFunc, _ SamplingFunc, _ ProgressFunc, reg *providers.Registry, _ *providers.Runtime) ([]ContentBlock, interface{}, error) {
	category := argString(args, "category")

	// Mark which providers are already configured.
	configured := make(map[string]bool)
	for _, c := range reg.List() {
		configured[c.ID] = true
	}

	suggestions := make([]providerSuggestion, 0, len(availableProviders))
	for _, p := range availableProviders {
		if category != "" && p.Category != category {
			continue
		}
		s := p // copy
		s.Configured = configured[p.ID]
		suggestions = append(suggestions, s)
	}

	if len(suggestions) == 0 {
		return textContent("No providers available for category: " + category), nil, nil
	}

	var lines []string
	for _, s := range suggestions {
		status := "available"
		if s.Configured {
			status = "configured"
		}
		lines = append(lines, fmt.Sprintf("- %s (%s) [%s] — %s", s.Name, s.Category, status, s.Description))
	}

	summary := fmt.Sprintf("%d provider(s) available:\n%s\n\nTo enable a provider, use configure_provider. "+
		"Consult the reference project for each provider to find the current API endpoint, "+
		"authentication details, query structure, and response field paths.",
		len(suggestions), strings.Join(lines, "\n"))

	content, err := buildAnnotatedContentBlocks(summary, suggestions)
	if err != nil {
		return nil, nil, err
	}
	return content, suggestions, nil
}

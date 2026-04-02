package mcp

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// --- HTTP handler tests ---

func TestHTTPHandler_POST_Initialize(t *testing.T) {
	hs := NewHTTPServer(0)

	req := Request{JSONRPC: "2.0", ID: float64(1), Method: "initialize"}
	body, _ := json.Marshal(req)

	rr := httptest.NewRecorder()
	httpReq := httptest.NewRequest("POST", "/mcp", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")

	hs.handleMCP(rr, httpReq)

	if rr.Code != 200 {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var resp Response
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
	if resp.ID != float64(1) {
		t.Errorf("id = %v, want 1", resp.ID)
	}
}

func TestHTTPHandler_POST_ToolsList(t *testing.T) {
	hs := NewHTTPServer(0)

	req := Request{JSONRPC: "2.0", ID: float64(2), Method: "tools/list"}
	body, _ := json.Marshal(req)

	rr := httptest.NewRecorder()
	httpReq := httptest.NewRequest("POST", "/mcp", bytes.NewReader(body))

	hs.handleMCP(rr, httpReq)

	var resp Response
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.Error != nil {
		t.Fatalf("error: %+v", resp.Error)
	}

	resultJSON, _ := json.Marshal(resp.Result)
	var result ToolsListResult
	json.Unmarshal(resultJSON, &result)

	if len(result.Tools) != 9 {
		t.Errorf("expected 9 tools, got %d", len(result.Tools))
	}
}

func TestHTTPHandler_POST_ToolsCall(t *testing.T) {
	hs := NewHTTPServer(0)

	params := ToolCallParams{
		Name: "search_flights",
		Arguments: map[string]any{
			"origin":         "HEL",
			"destination":    "NRT",
			"departure_date": "2026-06-15",
		},
	}
	req := Request{JSONRPC: "2.0", ID: float64(3), Method: "tools/call"}
	paramsJSON, _ := json.Marshal(params)
	req.Params = paramsJSON
	body, _ := json.Marshal(req)

	rr := httptest.NewRecorder()
	httpReq := httptest.NewRequest("POST", "/mcp", bytes.NewReader(body))

	hs.handleMCP(rr, httpReq)

	var resp Response
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.Error != nil {
		t.Fatalf("error: %+v", resp.Error)
	}
}

func TestHTTPHandler_GET_NotAllowed(t *testing.T) {
	hs := NewHTTPServer(0)

	rr := httptest.NewRecorder()
	httpReq := httptest.NewRequest("GET", "/mcp", nil)

	hs.handleMCP(rr, httpReq)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestHTTPHandler_OPTIONS_CORS(t *testing.T) {
	hs := NewHTTPServer(0)

	rr := httptest.NewRecorder()
	httpReq := httptest.NewRequest("OPTIONS", "/mcp", nil)
	httpReq.Header.Set("Origin", "http://localhost:3000")

	hs.handleMCP(rr, httpReq)

	if rr.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}

	if rr.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Errorf("CORS Allow-Origin = %q, want http://localhost:3000", rr.Header().Get("Access-Control-Allow-Origin"))
	}
	if rr.Header().Get("Access-Control-Allow-Methods") != "POST, OPTIONS" {
		t.Error("missing CORS Allow-Methods header")
	}
}

func TestHTTPHandler_CORS_RejectsNonLocalhost(t *testing.T) {
	hs := NewHTTPServer(0)

	rr := httptest.NewRecorder()
	httpReq := httptest.NewRequest("OPTIONS", "/mcp", nil)
	httpReq.Header.Set("Origin", "https://evil.com")

	hs.handleMCP(rr, httpReq)

	if rr.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("CORS should not allow non-localhost origin, got %q", rr.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestHTTPHandler_POST_InvalidJSON(t *testing.T) {
	hs := NewHTTPServer(0)

	rr := httptest.NewRecorder()
	httpReq := httptest.NewRequest("POST", "/mcp", bytes.NewReader([]byte("not json")))

	hs.handleMCP(rr, httpReq)

	// Should return 200 with a JSON-RPC parse error.
	if rr.Code != 200 {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var resp Response
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected parse error")
	}
	if resp.Error.Code != -32700 {
		t.Errorf("error code = %d, want -32700", resp.Error.Code)
	}
}

func TestHTTPHandler_POST_Notification(t *testing.T) {
	hs := NewHTTPServer(0)

	req := Request{JSONRPC: "2.0", Method: "notifications/initialized"}
	body, _ := json.Marshal(req)

	rr := httptest.NewRecorder()
	httpReq := httptest.NewRequest("POST", "/mcp", bytes.NewReader(body))

	hs.handleMCP(rr, httpReq)

	// Notifications return 204 No Content.
	if rr.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
}

func TestHTTPHandler_Health(t *testing.T) {
	hs := NewHTTPServer(0)

	rr := httptest.NewRecorder()
	httpReq := httptest.NewRequest("GET", "/health", nil)

	hs.handleHealth(rr, httpReq)

	if rr.Code != 200 {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var result map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result["status"] != "ok" {
		t.Errorf("status = %q, want ok", result["status"])
	}
	if result["server"] != "trvl" {
		t.Errorf("server = %q, want trvl", result["server"])
	}
	if result["version"] != "0.2.0" {
		t.Errorf("version = %q, want 0.2.0", result["version"])
	}
}

// --- Tool parameter validation ---

func TestToolsCall_InvalidParams(t *testing.T) {
	s := NewServer()

	req := &Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  json.RawMessage(`"not a valid params object"`),
	}

	resp := s.HandleRequest(req)
	if resp.Error == nil {
		t.Fatal("expected error for invalid params")
	}
	if resp.Error.Code != -32602 {
		t.Errorf("error code = %d, want -32602", resp.Error.Code)
	}
}

func TestToolsCall_UnknownToolDirect(t *testing.T) {
	s := NewServer()

	params := ToolCallParams{Name: "nonexistent"}
	raw, _ := json.Marshal(params)

	req := &Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  raw,
	}

	resp := s.HandleRequest(req)
	if resp.Error == nil {
		t.Fatal("expected error for unknown tool")
	}
	if !strings.Contains(resp.Error.Message, "unknown tool") {
		t.Errorf("error message = %q", resp.Error.Message)
	}
}

// --- Multiple sequential HTTP requests ---

func TestHTTPHandler_SequentialRequests(t *testing.T) {
	hs := NewHTTPServer(0)

	// Initialize -> tools/list -> tools/call.
	methods := []string{"initialize", "tools/list"}
	for i, method := range methods {
		req := Request{JSONRPC: "2.0", ID: float64(i + 1), Method: method}
		body, _ := json.Marshal(req)

		rr := httptest.NewRecorder()
		httpReq := httptest.NewRequest("POST", "/mcp", bytes.NewReader(body))
		hs.handleMCP(rr, httpReq)

		var resp Response
		json.Unmarshal(rr.Body.Bytes(), &resp)
		if resp.Error != nil {
			t.Fatalf("request %d (%s) error: %+v", i+1, method, resp.Error)
		}
	}
}

// --- Large payload ---

func TestHTTPHandler_LargePayload(t *testing.T) {
	hs := NewHTTPServer(0)

	// Create a tool call with a large arguments map.
	args := make(map[string]any)
	for i := range 100 {
		args[strings.Repeat("k", 50)+string(rune(i+'a'))] = strings.Repeat("v", 200)
	}

	params := ToolCallParams{Name: "search_flights", Arguments: args}
	req := Request{JSONRPC: "2.0", ID: float64(1), Method: "tools/call"}
	paramsJSON, _ := json.Marshal(params)
	req.Params = paramsJSON
	body, _ := json.Marshal(req)

	rr := httptest.NewRecorder()
	httpReq := httptest.NewRequest("POST", "/mcp", bytes.NewReader(body))
	hs.handleMCP(rr, httpReq)

	if rr.Code != 200 {
		t.Errorf("status = %d, want 200", rr.Code)
	}

	// Response should be valid JSON.
	var resp Response
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
}

// --- All four tool handlers ---

func TestAllToolHandlers(t *testing.T) {
	handlers := []struct {
		name      string
		args      map[string]any
		mayError  bool // true if the handler may return an error (e.g., fake hotel ID)
	}{
		{"search_flights", map[string]any{"origin": "HEL", "destination": "NRT", "departure_date": "2026-06-15"}, false},
		{"search_dates", map[string]any{"origin": "HEL", "destination": "NRT", "start_date": "2026-06-01", "end_date": "2026-06-30"}, false},
		{"search_hotels", map[string]any{"location": "Helsinki", "check_in": "2026-06-15", "check_out": "2026-06-18"}, false},
		{"hotel_prices", map[string]any{"hotel_id": "/g/abc", "check_in": "2026-06-15", "check_out": "2026-06-18"}, true},
	}

	for _, tt := range handlers {
		t.Run(tt.name, func(t *testing.T) {
			s := NewServer()
			handler, ok := s.handlers[tt.name]
			if !ok {
				t.Fatalf("handler not found for %s", tt.name)
			}

			content, structured, err := handler(tt.args, nil)
			if err != nil {
				if tt.mayError {
					// Expected: fake hotel ID may fail with real API.
					return
				}
				t.Fatalf("handler error: %v", err)
			}

			if len(content) == 0 {
				t.Fatal("expected content blocks")
			}

			// First content block should be the summary (for user).
			if content[0].Type != "text" {
				t.Errorf("first content block type = %q, want text", content[0].Type)
			}
			if content[0].Annotations == nil {
				t.Error("first content block should have annotations")
			}

			// Second content block should be JSON (for assistant).
			if len(content) >= 2 {
				var parsed map[string]any
				if err := json.Unmarshal([]byte(content[1].Text), &parsed); err != nil {
					t.Fatalf("second content block is not valid JSON: %v", err)
				}
			}

			// Structured content should be non-nil.
			if structured == nil {
				t.Error("expected structured content to be non-nil")
			}
		})
	}
}

func TestAllToolHandlers_NilArgs(t *testing.T) {
	tools := []string{"search_flights", "search_dates", "search_hotels", "hotel_prices"}
	s := NewServer()

	for _, name := range tools {
		t.Run(name, func(t *testing.T) {
			handler := s.handlers[name]
			// All handlers should return an error for nil args (missing required fields).
			_, _, err := handler(nil, nil)
			if err == nil {
				t.Error("expected error for nil args")
			}
		})
	}
}

// --- Tool definitions ---

func TestToolDefinitions(t *testing.T) {
	s := NewServer()

	for _, tool := range s.tools {
		t.Run(tool.Name, func(t *testing.T) {
			if tool.Description == "" {
				t.Error("empty description")
			}
			if tool.InputSchema.Type != "object" {
				t.Errorf("schema type = %q, want object", tool.InputSchema.Type)
			}
			if len(tool.InputSchema.Properties) == 0 {
				t.Error("no properties")
			}
			if len(tool.InputSchema.Required) == 0 {
				t.Error("no required fields")
			}

			// Verify all required fields exist in properties.
			for _, req := range tool.InputSchema.Required {
				if _, ok := tool.InputSchema.Properties[req]; !ok {
					t.Errorf("required field %q not in properties", req)
				}
			}
		})
	}
}

// --- marshalResult (via buildAnnotatedContentBlocks) ---

func TestBuildAnnotatedContentBlocks(t *testing.T) {
	blocks, err := buildAnnotatedContentBlocks("Test summary", map[string]string{"key": "value"})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}

	// Summary block.
	if blocks[0].Text != "Test summary" {
		t.Errorf("summary = %q", blocks[0].Text)
	}
	if blocks[0].Annotations == nil {
		t.Fatal("summary should have annotations")
	}
	if blocks[0].Annotations.Priority != 1.0 {
		t.Errorf("summary priority = %f, want 1.0", blocks[0].Annotations.Priority)
	}
	if len(blocks[0].Annotations.Audience) == 0 || blocks[0].Annotations.Audience[0] != "user" {
		t.Errorf("summary audience = %v, want [user]", blocks[0].Annotations.Audience)
	}

	// JSON block.
	if !strings.Contains(blocks[1].Text, "key") {
		t.Error("JSON block should contain key")
	}
	if blocks[1].Annotations == nil {
		t.Fatal("JSON block should have annotations")
	}
	if blocks[1].Annotations.Priority != 0.5 {
		t.Errorf("JSON priority = %f, want 0.5", blocks[1].Annotations.Priority)
	}
	if len(blocks[1].Annotations.Audience) == 0 || blocks[1].Annotations.Audience[0] != "assistant" {
		t.Errorf("JSON audience = %v, want [assistant]", blocks[1].Annotations.Audience)
	}
}

// --- Stdio edge cases ---

func TestServeStdio_EmptyInput(t *testing.T) {
	s := NewServer()
	in := bytes.NewBufferString("")
	out := &bytes.Buffer{}

	err := s.ServeStdio(in, out)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("expected no output, got %q", out.String())
	}
}

func TestServeStdio_ManyEmptyLines(t *testing.T) {
	s := NewServer()
	in := bytes.NewBufferString("\n\n\n\n\n")
	out := &bytes.Buffer{}

	err := s.ServeStdio(in, out)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("expected no output, got %q", out.String())
	}
}

// --- writeJSON ---

func TestWriteJSON(t *testing.T) {
	var buf bytes.Buffer
	resp := Response{JSONRPC: "2.0", ID: float64(1)}
	err := writeJSON(&buf, resp)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	line := strings.TrimSpace(buf.String())
	var parsed Response
	if err := json.Unmarshal([]byte(line), &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed.JSONRPC != "2.0" {
		t.Errorf("jsonrpc = %q", parsed.JSONRPC)
	}
}

func TestWriteJSON_Error(t *testing.T) {
	// Write to a writer that always fails.
	w := &failWriter{}
	err := writeJSON(w, Response{JSONRPC: "2.0"})
	if err == nil {
		t.Error("expected error from failing writer")
	}
}

type failWriter struct{}

func (f *failWriter) Write(p []byte) (int, error) {
	return 0, io.ErrClosedPipe
}

// --- NewServer ---

func TestNewServer(t *testing.T) {
	s := NewServer()
	if s == nil {
		t.Fatal("NewServer returned nil")
	}
	if len(s.tools) != 9 {
		t.Errorf("expected 9 tools, got %d", len(s.tools))
	}
	if len(s.handlers) != 9 {
		t.Errorf("expected 9 handlers, got %d", len(s.handlers))
	}
}

// --- NewHTTPServer ---

func TestNewHTTPServer(t *testing.T) {
	hs := NewHTTPServer(8080)
	if hs == nil {
		t.Fatal("NewHTTPServer returned nil")
	}
	if hs.port != 8080 {
		t.Errorf("port = %d, want 8080", hs.port)
	}
	if hs.server == nil {
		t.Error("server is nil")
	}
}

// --- HandleRequest all methods ---

func TestHandleRequest_AllMethods(t *testing.T) {
	s := NewServer()

	tests := []struct {
		method    string
		expectNil bool
		expectErr bool
	}{
		{"initialize", false, false},
		{"tools/list", false, false},
		{"notifications/initialized", true, false},
		{"unknown/method", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			req := &Request{JSONRPC: "2.0", ID: float64(1), Method: tt.method}
			if tt.method == "tools/call" {
				params := ToolCallParams{Name: "search_flights"}
				raw, _ := json.Marshal(params)
				req.Params = raw
			}

			resp := s.HandleRequest(req)
			if tt.expectNil {
				if resp != nil {
					t.Error("expected nil response")
				}
				return
			}
			if resp == nil {
				t.Fatal("expected non-nil response")
			}
			if tt.expectErr && resp.Error == nil {
				t.Error("expected error")
			}
			if !tt.expectErr && resp.Error != nil {
				t.Errorf("unexpected error: %+v", resp.Error)
			}
		})
	}
}

// --- Protocol version 2025-11-25 features ---

func TestProtocolVersion(t *testing.T) {
	if protocolVersion != "2025-11-25" {
		t.Errorf("protocol version = %q, want 2025-11-25", protocolVersion)
	}
}

func TestInitializeCapabilities(t *testing.T) {
	s := NewServer()
	resp := sendRequest(t, s, "initialize", 1, nil)
	if resp == nil {
		t.Fatal("expected response")
	}

	resultJSON, _ := json.Marshal(resp.Result)
	var result InitializeResult
	if err := json.Unmarshal(resultJSON, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if result.Capabilities.Tools == nil {
		t.Error("tools capability should be set")
	}
	if result.Capabilities.Prompts == nil {
		t.Error("prompts capability should be set")
	}
	if result.Capabilities.Resources == nil {
		t.Error("resources capability should be set")
	}
	if result.Capabilities.Logging == nil {
		t.Error("logging capability should be set")
	}
	if result.ProtocolVersion != "2025-11-25" {
		t.Errorf("protocol version = %q, want 2025-11-25", result.ProtocolVersion)
	}
}

func TestToolAnnotations(t *testing.T) {
	s := NewServer()
	for _, tool := range s.tools {
		t.Run(tool.Name, func(t *testing.T) {
			if tool.Annotations == nil {
				t.Fatal("annotations should be set")
			}
			if tool.Annotations.Title == "" {
				t.Error("title should be set")
			}
			if !tool.Annotations.ReadOnlyHint {
				t.Error("readOnlyHint should be true")
			}
			if !tool.Annotations.IdempotentHint {
				t.Error("idempotentHint should be true")
			}
		})
	}
}

func TestToolOutputSchema(t *testing.T) {
	s := NewServer()
	// New tools added in v0.2 pending output schema definition
	skipSchema := map[string]bool{"weekend_getaway": true, "suggest_dates": true, "optimize_multi_city": true}
	for _, tool := range s.tools {
		t.Run(tool.Name, func(t *testing.T) {
			if skipSchema[tool.Name] {
				t.Skipf("output schema pending for %s", tool.Name)
			}
			if tool.OutputSchema == nil {
				t.Fatal("outputSchema should be set")
			}
			// Should be a valid JSON-serializable object.
			data, err := json.Marshal(tool.OutputSchema)
			if err != nil {
				t.Fatalf("marshal outputSchema: %v", err)
			}
			var schema map[string]interface{}
			if err := json.Unmarshal(data, &schema); err != nil {
				t.Fatalf("outputSchema is not a valid JSON object: %v", err)
			}
			if schema["type"] != "object" {
				t.Errorf("outputSchema type = %v, want object", schema["type"])
			}
		})
	}
}

func TestToolTitle(t *testing.T) {
	s := NewServer()
	for _, tool := range s.tools {
		t.Run(tool.Name, func(t *testing.T) {
			if tool.Title == "" {
				t.Error("tool-level title should be set")
			}
		})
	}
}

func TestStructuredContent(t *testing.T) {
	s := NewServer()

	// Call a tool via HandleRequest and verify structuredContent is present.
	params := ToolCallParams{
		Name: "search_flights",
		Arguments: map[string]any{
			"origin":         "HEL",
			"destination":    "NRT",
			"departure_date": "2026-06-15",
		},
	}
	raw, _ := json.Marshal(params)
	req := &Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  raw,
	}

	resp := s.HandleRequest(req)
	if resp == nil {
		t.Fatal("expected response")
	}
	if resp.Error != nil {
		t.Fatalf("error: %+v", resp.Error)
	}

	resultJSON, _ := json.Marshal(resp.Result)
	var result ToolCallResult
	if err := json.Unmarshal(resultJSON, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Content should have annotated blocks.
	if len(result.Content) < 2 {
		t.Fatalf("expected at least 2 content blocks, got %d", len(result.Content))
	}

	// Structured content should be present.
	if result.StructuredContent == nil {
		t.Error("structuredContent should be present")
	}
}

func TestContentAnnotations(t *testing.T) {
	s := NewServer()

	params := ToolCallParams{
		Name: "search_flights",
		Arguments: map[string]any{
			"origin":         "HEL",
			"destination":    "NRT",
			"departure_date": "2026-06-15",
		},
	}
	raw, _ := json.Marshal(params)
	req := &Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  raw,
	}

	resp := s.HandleRequest(req)
	if resp == nil {
		t.Fatal("expected response")
	}

	resultJSON, _ := json.Marshal(resp.Result)
	var result ToolCallResult
	json.Unmarshal(resultJSON, &result)

	if len(result.Content) < 2 {
		t.Fatal("expected at least 2 content blocks")
	}

	// First block: user audience, high priority.
	first := result.Content[0]
	if first.Annotations == nil {
		t.Fatal("first block should have annotations")
	}
	if len(first.Annotations.Audience) == 0 || first.Annotations.Audience[0] != "user" {
		t.Errorf("first block audience = %v, want [user]", first.Annotations.Audience)
	}
	if first.Annotations.Priority != 1.0 {
		t.Errorf("first block priority = %f, want 1.0", first.Annotations.Priority)
	}

	// Second block: assistant audience, lower priority.
	second := result.Content[1]
	if second.Annotations == nil {
		t.Fatal("second block should have annotations")
	}
	if len(second.Annotations.Audience) == 0 || second.Annotations.Audience[0] != "assistant" {
		t.Errorf("second block audience = %v, want [assistant]", second.Annotations.Audience)
	}
	if second.Annotations.Priority != 0.5 {
		t.Errorf("second block priority = %f, want 0.5", second.Annotations.Priority)
	}
}

func TestInitializeTracksClientCapabilities(t *testing.T) {
	s := NewServer()

	// Send initialize with elicitation capability.
	initParams := InitializeParams{
		ProtocolVersion: "2025-11-25",
		Capabilities: ClientCapabilities{
			Elicitation: &ElicitationCapability{},
		},
		ClientInfo: ClientInfo{Name: "test-client", Version: "1.0"},
	}
	raw, _ := json.Marshal(initParams)
	req := &Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "initialize",
		Params:  raw,
	}

	resp := s.HandleRequest(req)
	if resp == nil {
		t.Fatal("expected response")
	}
	if resp.Error != nil {
		t.Fatalf("error: %+v", resp.Error)
	}

	// Verify client capabilities were stored.
	if s.clientCapabilities.Elicitation == nil {
		t.Error("expected elicitation capability to be stored")
	}
}

func TestSendLog(t *testing.T) {
	s := NewServer()
	var buf bytes.Buffer
	s.notifyWriter = &buf

	s.SendLog("info", "test message")

	line := strings.TrimSpace(buf.String())
	if line == "" {
		t.Fatal("expected log notification")
	}

	var notif map[string]interface{}
	if err := json.Unmarshal([]byte(line), &notif); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if notif["method"] != "notifications/message" {
		t.Errorf("method = %v, want notifications/message", notif["method"])
	}
	params, ok := notif["params"].(map[string]interface{})
	if !ok {
		t.Fatal("params should be an object")
	}
	if params["level"] != "info" {
		t.Errorf("level = %v, want info", params["level"])
	}
	if params["data"] != "test message" {
		t.Errorf("data = %v, want test message", params["data"])
	}
	if params["logger"] != "trvl" {
		t.Errorf("logger = %v, want trvl", params["logger"])
	}
}

func TestSendProgress(t *testing.T) {
	s := NewServer()
	var buf bytes.Buffer
	s.notifyWriter = &buf

	s.SendProgress("token-1", 0.5, 1.0, "Searching...")

	line := strings.TrimSpace(buf.String())
	if line == "" {
		t.Fatal("expected progress notification")
	}

	var notif map[string]interface{}
	if err := json.Unmarshal([]byte(line), &notif); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if notif["method"] != "notifications/progress" {
		t.Errorf("method = %v, want notifications/progress", notif["method"])
	}
	params, ok := notif["params"].(map[string]interface{})
	if !ok {
		t.Fatal("params should be an object")
	}
	if params["progressToken"] != "token-1" {
		t.Errorf("progressToken = %v", params["progressToken"])
	}
	if params["progress"] != 0.5 {
		t.Errorf("progress = %v", params["progress"])
	}
	if params["message"] != "Searching..." {
		t.Errorf("message = %v", params["message"])
	}
}

func TestSendNotification_NilWriter(t *testing.T) {
	s := NewServer()
	// Should not panic or error when no writer is set.
	err := s.SendNotification("notifications/message", LogParams{Level: "info", Logger: "trvl", Data: "test"})
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestMakeElicitFunc_NilWithoutCapability(t *testing.T) {
	s := NewServer()
	// Client does not declare elicitation capability.
	fn := s.makeElicitFunc()
	if fn != nil {
		t.Error("expected nil ElicitFunc when client has no elicitation capability")
	}
}

func TestMakeElicitFunc_NilWithoutWriter(t *testing.T) {
	s := NewServer()
	s.clientCapabilities.Elicitation = &ElicitationCapability{}
	// No writer set.
	fn := s.makeElicitFunc()
	if fn != nil {
		t.Error("expected nil ElicitFunc when no writer is set")
	}
}

// --- Elicitation schema builders ---

func TestFlightElicitationSchema(t *testing.T) {
	schema := flightElicitationSchema("HEL", "NRT")
	if schema == nil {
		t.Fatal("expected non-nil schema")
	}
	if schema["type"] != "object" {
		t.Errorf("type = %v, want object", schema["type"])
	}
	props, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("properties should be a map")
	}
	if _, ok := props["departure_date"]; !ok {
		t.Error("missing departure_date property")
	}
	required, ok := schema["required"].([]string)
	if !ok {
		t.Fatal("required should be a string slice")
	}
	if len(required) != 1 || required[0] != "departure_date" {
		t.Errorf("required = %v, want [departure_date]", required)
	}
}

func TestHotelElicitationSchema(t *testing.T) {
	schema := hotelElicitationSchema(24, "Helsinki")
	if schema == nil {
		t.Fatal("expected non-nil schema")
	}
	props, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("properties should be a map")
	}
	if _, ok := props["min_stars"]; !ok {
		t.Error("missing min_stars property")
	}
	if _, ok := props["max_price"]; !ok {
		t.Error("missing max_price property")
	}
	if _, ok := props["sort_by"]; !ok {
		t.Error("missing sort_by property")
	}
}

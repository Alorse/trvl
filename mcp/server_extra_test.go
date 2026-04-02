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

	if len(result.Tools) != 4 {
		t.Errorf("expected 4 tools, got %d", len(result.Tools))
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

	hs.handleMCP(rr, httpReq)

	if rr.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}

	if rr.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("missing CORS Allow-Origin header")
	}
	if rr.Header().Get("Access-Control-Allow-Methods") != "POST, OPTIONS" {
		t.Error("missing CORS Allow-Methods header")
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
	if result["version"] != "0.1.0" {
		t.Errorf("version = %q, want 0.1.0", result["version"])
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
		name string
		args map[string]any
	}{
		{"search_flights", map[string]any{"origin": "HEL", "destination": "NRT", "departure_date": "2026-06-15"}},
		{"search_dates", map[string]any{"origin": "HEL", "destination": "NRT", "start_date": "2026-06-01", "end_date": "2026-06-30"}},
		{"search_hotels", map[string]any{"location": "Helsinki", "check_in": "2026-06-15", "check_out": "2026-06-18"}},
		{"hotel_prices", map[string]any{"hotel_id": "/g/abc", "check_in": "2026-06-15", "check_out": "2026-06-18"}},
	}

	for _, tt := range handlers {
		t.Run(tt.name, func(t *testing.T) {
			s := NewServer()
			handler, ok := s.handlers[tt.name]
			if !ok {
				t.Fatalf("handler not found for %s", tt.name)
			}

			result, err := handler(tt.args)
			if err != nil {
				t.Fatalf("handler error: %v", err)
			}

			// Result should be valid JSON.
			var parsed map[string]any
			if err := json.Unmarshal([]byte(result), &parsed); err != nil {
				t.Fatalf("result not valid JSON: %v", err)
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
			result, err := handler(nil)
			if err != nil {
				t.Fatalf("handler error: %v", err)
			}
			if result == "" {
				t.Error("expected non-empty result")
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

// --- marshalResult ---

func TestMarshalResult_ValidInput(t *testing.T) {
	result, err := marshalResult(map[string]string{"key": "value"})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !strings.Contains(result, "key") {
		t.Error("missing key in result")
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
	if len(s.tools) != 4 {
		t.Errorf("expected 4 tools, got %d", len(s.tools))
	}
	if len(s.handlers) != 4 {
		t.Errorf("expected 4 handlers, got %d", len(s.handlers))
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

// --- HandleRequest unknown method ---

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

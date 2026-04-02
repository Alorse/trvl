package mcp

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// sendRequest writes a JSON-RPC request to the server and returns the response.
func sendRequest(t *testing.T, s *Server, method string, id any, params any) *Response {
	t.Helper()

	req := Request{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
	}
	if params != nil {
		raw, err := json.Marshal(params)
		if err != nil {
			t.Fatalf("marshal params: %v", err)
		}
		req.Params = raw
	}

	reqBytes, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	in := bytes.NewBuffer(append(reqBytes, '\n'))
	out := &bytes.Buffer{}

	if err := s.ServeStdio(in, out); err != nil {
		t.Fatalf("ServeStdio: %v", err)
	}

	line := strings.TrimSpace(out.String())
	if line == "" {
		return nil
	}

	var resp Response
	if err := json.Unmarshal([]byte(line), &resp); err != nil {
		t.Fatalf("unmarshal response %q: %v", line, err)
	}
	return &resp
}

func TestInitialize(t *testing.T) {
	s := NewServer()
	resp := sendRequest(t, s, "initialize", 1, nil)
	if resp == nil {
		t.Fatal("expected response, got nil")
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
	if resp.ID != float64(1) { // JSON numbers decode as float64
		t.Errorf("expected id=1, got %v", resp.ID)
	}

	// Verify the result structure.
	resultJSON, err := json.Marshal(resp.Result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	var result InitializeResult
	if err := json.Unmarshal(resultJSON, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if result.ProtocolVersion != protocolVersion {
		t.Errorf("protocol version: got %q, want %q", result.ProtocolVersion, protocolVersion)
	}
	if result.ServerInfo.Name != "trvl" {
		t.Errorf("server name: got %q, want %q", result.ServerInfo.Name, "trvl")
	}
	if result.ServerInfo.Version != "0.1.0" {
		t.Errorf("server version: got %q, want %q", result.ServerInfo.Version, "0.1.0")
	}
	if result.Capabilities.Tools == nil {
		t.Error("expected tools capability to be set")
	}
}

func TestToolsList(t *testing.T) {
	s := NewServer()
	resp := sendRequest(t, s, "tools/list", 2, nil)
	if resp == nil {
		t.Fatal("expected response, got nil")
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}

	resultJSON, err := json.Marshal(resp.Result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	var result ToolsListResult
	if err := json.Unmarshal(resultJSON, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if len(result.Tools) != 4 {
		t.Fatalf("expected 4 tools, got %d", len(result.Tools))
	}

	expected := map[string]bool{
		"search_flights": false,
		"search_dates":   false,
		"search_hotels":  false,
		"hotel_prices":   false,
	}
	for _, tool := range result.Tools {
		if _, ok := expected[tool.Name]; !ok {
			t.Errorf("unexpected tool: %s", tool.Name)
		}
		expected[tool.Name] = true

		if tool.Description == "" {
			t.Errorf("tool %s has empty description", tool.Name)
		}
		if tool.InputSchema.Type != "object" {
			t.Errorf("tool %s schema type: got %q, want %q", tool.Name, tool.InputSchema.Type, "object")
		}
		if len(tool.InputSchema.Properties) == 0 {
			t.Errorf("tool %s has no properties", tool.Name)
		}
		if len(tool.InputSchema.Required) == 0 {
			t.Errorf("tool %s has no required fields", tool.Name)
		}
	}

	for name, found := range expected {
		if !found {
			t.Errorf("missing tool: %s", name)
		}
	}
}

func TestToolsCallSearchFlights(t *testing.T) {
	s := NewServer()
	params := ToolCallParams{
		Name: "search_flights",
		Arguments: map[string]any{
			"origin":         "HEL",
			"destination":    "NRT",
			"departure_date": "2026-05-15",
		},
	}
	resp := sendRequest(t, s, "tools/call", 3, params)
	if resp == nil {
		t.Fatal("expected response, got nil")
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}

	resultJSON, err := json.Marshal(resp.Result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	var result ToolCallResult
	if err := json.Unmarshal(resultJSON, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if len(result.Content) == 0 {
		t.Fatal("expected content blocks")
	}
	if result.Content[0].Type != "text" {
		t.Errorf("content type: got %q, want %q", result.Content[0].Type, "text")
	}

	// Stub returns "not yet implemented" — verify it's valid JSON.
	var stub map[string]any
	if err := json.Unmarshal([]byte(result.Content[0].Text), &stub); err != nil {
		t.Fatalf("content is not valid JSON: %v", err)
	}
	if stub["success"] != false {
		t.Errorf("expected success=false, got %v", stub["success"])
	}
}

func TestToolsCallSearchDates(t *testing.T) {
	s := NewServer()
	params := ToolCallParams{
		Name: "search_dates",
		Arguments: map[string]any{
			"origin":      "HEL",
			"destination": "NRT",
			"start_date":  "2026-05-01",
			"end_date":    "2026-05-31",
		},
	}
	resp := sendRequest(t, s, "tools/call", 4, params)
	if resp == nil {
		t.Fatal("expected response, got nil")
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
}

func TestToolsCallSearchHotels(t *testing.T) {
	s := NewServer()
	params := ToolCallParams{
		Name: "search_hotels",
		Arguments: map[string]any{
			"location":  "Helsinki",
			"check_in":  "2026-05-15",
			"check_out": "2026-05-18",
		},
	}
	resp := sendRequest(t, s, "tools/call", 5, params)
	if resp == nil {
		t.Fatal("expected response, got nil")
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
}

func TestToolsCallHotelPrices(t *testing.T) {
	s := NewServer()
	params := ToolCallParams{
		Name: "hotel_prices",
		Arguments: map[string]any{
			"hotel_id":  "abc123",
			"check_in":  "2026-05-15",
			"check_out": "2026-05-18",
		},
	}
	resp := sendRequest(t, s, "tools/call", 6, params)
	if resp == nil {
		t.Fatal("expected response, got nil")
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
}

func TestToolsCallUnknownTool(t *testing.T) {
	s := NewServer()
	params := ToolCallParams{
		Name:      "nonexistent",
		Arguments: map[string]any{},
	}
	resp := sendRequest(t, s, "tools/call", 7, params)
	if resp == nil {
		t.Fatal("expected response, got nil")
	}
	if resp.Error == nil {
		t.Fatal("expected error for unknown tool")
	}
	if resp.Error.Code != -32602 {
		t.Errorf("error code: got %d, want %d", resp.Error.Code, -32602)
	}
}

func TestUnknownMethod(t *testing.T) {
	s := NewServer()
	resp := sendRequest(t, s, "unknown/method", 8, nil)
	if resp == nil {
		t.Fatal("expected response, got nil")
	}
	if resp.Error == nil {
		t.Fatal("expected error for unknown method")
	}
	if resp.Error.Code != -32601 {
		t.Errorf("error code: got %d, want %d", resp.Error.Code, -32601)
	}
}

func TestNotificationNoResponse(t *testing.T) {
	s := NewServer()
	// notifications/initialized should produce no response line.
	req := Request{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	}
	reqBytes, _ := json.Marshal(req)
	in := bytes.NewBuffer(append(reqBytes, '\n'))
	out := &bytes.Buffer{}

	if err := s.ServeStdio(in, out); err != nil {
		t.Fatalf("ServeStdio: %v", err)
	}

	if out.Len() != 0 {
		t.Errorf("expected no output for notification, got %q", out.String())
	}
}

func TestParseError(t *testing.T) {
	s := NewServer()
	in := bytes.NewBufferString("not valid json\n")
	out := &bytes.Buffer{}

	if err := s.ServeStdio(in, out); err != nil {
		t.Fatalf("ServeStdio: %v", err)
	}

	line := strings.TrimSpace(out.String())
	var resp Response
	if err := json.Unmarshal([]byte(line), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected parse error")
	}
	if resp.Error.Code != -32700 {
		t.Errorf("error code: got %d, want %d", resp.Error.Code, -32700)
	}
}

func TestMultipleRequests(t *testing.T) {
	s := NewServer()

	// Send initialize + tools/list in sequence.
	initReq := Request{JSONRPC: "2.0", ID: float64(1), Method: "initialize"}
	listReq := Request{JSONRPC: "2.0", ID: float64(2), Method: "tools/list"}

	initBytes, _ := json.Marshal(initReq)
	listBytes, _ := json.Marshal(listReq)

	input := string(initBytes) + "\n" + string(listBytes) + "\n"
	in := bytes.NewBufferString(input)
	out := &bytes.Buffer{}

	if err := s.ServeStdio(in, out); err != nil {
		t.Fatalf("ServeStdio: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 response lines, got %d: %q", len(lines), out.String())
	}

	// Verify first response is initialize.
	var resp1 Response
	if err := json.Unmarshal([]byte(lines[0]), &resp1); err != nil {
		t.Fatalf("unmarshal resp1: %v", err)
	}
	if resp1.ID != float64(1) {
		t.Errorf("resp1 id: got %v, want 1", resp1.ID)
	}

	// Verify second response is tools/list.
	var resp2 Response
	if err := json.Unmarshal([]byte(lines[1]), &resp2); err != nil {
		t.Fatalf("unmarshal resp2: %v", err)
	}
	if resp2.ID != float64(2) {
		t.Errorf("resp2 id: got %v, want 2", resp2.ID)
	}
}

func TestEmptyLinesIgnored(t *testing.T) {
	s := NewServer()

	req := Request{JSONRPC: "2.0", ID: float64(1), Method: "initialize"}
	reqBytes, _ := json.Marshal(req)

	// Empty lines should be skipped.
	input := "\n\n" + string(reqBytes) + "\n\n"
	in := bytes.NewBufferString(input)
	out := &bytes.Buffer{}

	if err := s.ServeStdio(in, out); err != nil {
		t.Fatalf("ServeStdio: %v", err)
	}

	line := strings.TrimSpace(out.String())
	var resp Response
	if err := json.Unmarshal([]byte(line), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Error != nil {
		t.Errorf("unexpected error: %+v", resp.Error)
	}
}

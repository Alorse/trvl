// Package mcp implements an MCP (Model Context Protocol) server for trvl.
//
// It supports two transports:
//   - stdio: JSON-RPC messages over stdin/stdout (one JSON object per line)
//   - HTTP:  JSON-RPC messages via POST /mcp
//
// The server exposes eighteen tools: search_flights, search_dates, search_hotels,
// hotel_prices, hotel_reviews, destination_info, calculate_trip_cost,
// weekend_getaway, suggest_dates, optimize_multi_city, nearby_places,
// travel_guide, local_events, search_ground, search_airport_transfers,
// search_restaurants, search_deals, and plan_trip. It also provides prompts and resources.
//
// Protocol version: 2025-11-25
// Key features: structured output, elicitation, content annotations,
// progress notifications, and logging.
package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"github.com/MikkoParkkola/trvl/internal/watch"
)

const (
	serverName      = "trvl"
	serverVersion   = "0.3.0"
	protocolVersion = "2025-11-25"
)

// --- Server ---

// Server handles MCP JSON-RPC requests.
type Server struct {
	tools              []ToolDef
	handlers           map[string]ToolHandler
	prompts            []PromptDef
	resources          []ResourceDef
	clientCapabilities ClientCapabilities

	// Notification writer, set during ServeStdio for server-to-client messages.
	notifyWriter io.Writer
	notifyMu     sync.Mutex

	// For elicitation: reader set during ServeStdio.
	elicitReader *bufio.Scanner

	// Session state for trip planning.
	tripState  TripState
	priceCache *priceCache

	// Watch store for price tracking resources.
	watchStore *watch.Store
}

// ToolHandler processes a tool call and returns content blocks, optional
// structured content, and an error.
// The elicit parameter may be nil if the client does not support elicitation.
// The sampling parameter may be nil if the client does not support sampling.
type ToolHandler func(args map[string]any, elicit ElicitFunc, sampling SamplingFunc) ([]ContentBlock, interface{}, error)

// NewServer creates a new MCP server with the standard trvl tools registered.
func NewServer() *Server {
	s := &Server{
		handlers:   make(map[string]ToolHandler),
		priceCache: newPriceCache(),
	}

	// Initialize watch store (best-effort; nil store is handled gracefully).
	if ws, err := watch.DefaultStore(); err == nil {
		_ = ws.Load()
		s.watchStore = ws
	}

	registerTools(s)
	registerPrompts(s)
	registerResources(s)
	return s
}

// recordSearch adds a search record to the session trip state.
func (s *Server) recordSearch(typ, query string, bestPrice float64, currency string) {
	s.tripState.mu.Lock()
	defer s.tripState.mu.Unlock()
	s.tripState.Searches = append(s.tripState.Searches, SearchRecord{
		Type:      typ,
		Query:     query,
		BestPrice: bestPrice,
		Currency:  currency,
		Time:      time.Now(),
	})
}

// SendNotification writes a JSON-RPC notification to the client (server->client).
func (s *Server) SendNotification(method string, params interface{}) error {
	s.notifyMu.Lock()
	defer s.notifyMu.Unlock()

	if s.notifyWriter == nil {
		return nil // No writer available (HTTP mode or not started).
	}

	notif := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
	}
	return writeJSON(s.notifyWriter, notif)
}

// SendProgress sends a progress notification to the client.
func (s *Server) SendProgress(token string, progress, total float64, message string) {
	_ = s.SendNotification("notifications/progress", ProgressParams{
		ProgressToken: token,
		Progress:      progress,
		Total:         total,
		Message:       message,
	})
}

// SendLog sends a log notification to the client.
func (s *Server) SendLog(level, message string) {
	_ = s.SendNotification("notifications/message", LogParams{
		Level:  level,
		Logger: "trvl",
		Data:   message,
	})
}

// makeElicitFunc returns an ElicitFunc for the current transport mode.
//
// In stdio mode, elicitation is disabled because the elicitation response
// would be read from the same scanner as ServeStdio's main loop, causing
// stream desync if a regular request arrives while waiting for the
// elicitation response. Tool handlers fall back to progressive disclosure
// suggestions (which already work well) when elicit is nil.
//
// TODO: To re-enable stdio elicitation, the transport needs stream
// multiplexing — either a dedicated response channel or tagging messages
// with correlation IDs so the scanner can route them correctly.
//
// Elicitation works correctly in HTTP mode since each request is independent.
func (s *Server) makeElicitFunc() ElicitFunc {
	// Disabled in stdio mode to prevent stream desync.
	return nil
}

// makeSamplingFunc returns a SamplingFunc if the client declared the sampling
// capability. Like elicitation, sampling is disabled in stdio mode to prevent
// stream desync.
func (s *Server) makeSamplingFunc() SamplingFunc {
	if s.clientCapabilities.Sampling == nil {
		return nil
	}
	// Disabled in stdio mode to prevent stream desync (same as elicitation).
	return nil
}

// HandleRequest processes a single JSON-RPC request and returns the response.
func (s *Server) HandleRequest(req *Request) *Response {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "notifications/initialized":
		return nil
	case "notifications/cancelled":
		return nil // Client cancelled a request; acknowledged.
	case "ping":
		return &Response{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{}}
	case "logging/setLevel":
		return s.handleLoggingSetLevel(req)
	case "completion/complete":
		return s.handleCompletionComplete(req)
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(req)
	case "prompts/list":
		return s.handlePromptsList(req)
	case "prompts/get":
		return s.handlePromptsGet(req)
	case "resources/list":
		return s.handleResourcesList(req)
	case "resources/read":
		return s.handleResourcesRead(req)
	default:
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &Error{Code: -32601, Message: fmt.Sprintf("method not found: %s", req.Method)},
		}
	}
}

func (s *Server) handleInitialize(req *Request) *Response {
	// Parse client capabilities from the initialize request.
	if req.Params != nil {
		var params InitializeParams
		if err := json.Unmarshal(req.Params, &params); err == nil {
			s.clientCapabilities = params.Capabilities
			if params.Capabilities.Sampling != nil {
				s.SendLog("info", "Client supports sampling/createMessage — AI-powered result curation enabled")
			}
		}
	}

	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: InitializeResult{
			ProtocolVersion: protocolVersion,
			Capabilities: Capabilities{
				Tools:     &ToolsCapability{ListChanged: false},
				Prompts:   &PromptsCapability{ListChanged: false},
				Resources: &ResourcesCapability{Subscribe: false, ListChanged: false},
				Logging:   &LoggingCapability{},
			},
			ServerInfo: ServerInfo{
				Name:    serverName,
				Version: serverVersion,
			},
		},
	}
}

func (s *Server) handleToolsList(req *Request) *Response {
	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  ToolsListResult{Tools: s.tools},
	}
}

func (s *Server) handleToolsCall(req *Request) *Response {
	var params ToolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &Error{Code: -32602, Message: fmt.Sprintf("invalid params: %v", err)},
		}
	}

	handler, ok := s.handlers[params.Name]
	if !ok {
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &Error{Code: -32602, Message: fmt.Sprintf("unknown tool: %s", params.Name)},
		}
	}

	// Log the tool call.
	s.SendLog("info", fmt.Sprintf("Calling tool: %s", params.Name))

	// Build elicit and sampling functions based on client capabilities.
	elicit := s.makeElicitFunc()
	sampling := s.makeSamplingFunc()

	content, structured, err := handler(params.Arguments, elicit, sampling)
	if err != nil {
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: ToolCallResult{
				Content: []ContentBlock{{Type: "text", Text: err.Error()}},
				IsError: true,
			},
		}
	}

	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: ToolCallResult{
			Content:           content,
			StructuredContent: structured,
		},
	}
}

func (s *Server) handlePromptsList(req *Request) *Response {
	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  PromptsListResult{Prompts: s.prompts},
	}
}

func (s *Server) handlePromptsGet(req *Request) *Response {
	var params PromptsGetParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &Error{Code: -32602, Message: fmt.Sprintf("invalid params: %v", err)},
		}
	}

	result, err := getPrompt(params.Name, params.Arguments)
	if err != nil {
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &Error{Code: -32602, Message: err.Error()},
		}
	}

	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

func (s *Server) handleResourcesList(req *Request) *Response {
	// Combine static resources with dynamic ones from trip state.
	resources := s.listResources()
	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  ResourcesListResult{Resources: resources},
	}
}

func (s *Server) handleResourcesRead(req *Request) *Response {
	var params ResourcesReadParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &Error{Code: -32602, Message: fmt.Sprintf("invalid params: %v", err)},
		}
	}

	result, err := s.readResource(params.URI)
	if err != nil {
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &Error{Code: -32002, Message: err.Error()},
		}
	}

	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

// --- logging/setLevel handler ---

// logLevel stores the current minimum log level.
var logLevel = "info"

func (s *Server) handleLoggingSetLevel(req *Request) *Response {
	var params struct {
		Level string `json:"level"`
	}
	if req.Params != nil {
		_ = json.Unmarshal(req.Params, &params)
	}
	if params.Level != "" {
		logLevel = params.Level
		s.SendLog("info", fmt.Sprintf("Log level set to %s", params.Level))
	}
	return &Response{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{}}
}

// --- completion/complete handler ---

// handleCompletionComplete provides argument auto-completion for tools and prompts.
func (s *Server) handleCompletionComplete(req *Request) *Response {
	var params struct {
		Ref struct {
			Type string `json:"type"` // "ref/prompt" or "ref/resource"
			Name string `json:"name"`
			URI  string `json:"uri"`
		} `json:"ref"`
		Argument struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"argument"`
	}
	if req.Params != nil {
		_ = json.Unmarshal(req.Params, &params)
	}

	var values []string

	// Provide completions for known argument patterns.
	switch params.Argument.Name {
	case "origin", "destination", "from", "to":
		// Return matching IATA airport codes.
		values = completeAirport(params.Argument.Value)
	case "cabin_class":
		values = []string{"economy", "premium_economy", "business", "first"}
	case "sort":
		values = []string{"cheapest", "rating", "distance", "stars"}
	case "type":
		values = []string{"bus", "train"}
	case "provider":
		values = []string{"flixbus", "regiojet"}
	case "currency":
		values = []string{"EUR", "USD", "GBP", "CZK", "PLN", "SEK", "NOK", "DKK", "CHF", "JPY"}
	}

	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]any{
			"completion": map[string]any{
				"values":  values,
				"hasMore": false,
				"total":   len(values),
			},
		},
	}
}

// completeAirport returns IATA codes matching the given prefix.
func completeAirport(prefix string) []string {
	if prefix == "" {
		return nil
	}
	prefix = toUpper(prefix)
	var matches []string
	for code := range airportCompletionMap {
		if len(matches) >= 20 {
			break
		}
		if len(code) >= len(prefix) && code[:len(prefix)] == prefix {
			matches = append(matches, code)
		}
	}
	return matches
}

func toUpper(s string) string {
	b := make([]byte, len(s))
	for i := range len(s) {
		c := s[i]
		if c >= 'a' && c <= 'z' {
			c -= 32
		}
		b[i] = c
	}
	return string(b)
}

// airportCompletionMap is populated from the models package at init time.
var airportCompletionMap map[string]string

func init() {
	// Build airport completion map lazily on first access.
	airportCompletionMap = make(map[string]string, 250)
	// Common airports — populated from models.AirportNames if available,
	// otherwise a static subset for completion.
	commonAirports := map[string]string{
		"HEL": "Helsinki", "AMS": "Amsterdam", "PRG": "Prague", "KRK": "Krakow",
		"CDG": "Paris CDG", "ORY": "Paris Orly", "LHR": "London Heathrow",
		"LGW": "London Gatwick", "STN": "London Stansted", "FCO": "Rome",
		"BCN": "Barcelona", "MAD": "Madrid", "VIE": "Vienna", "BUD": "Budapest",
		"WAW": "Warsaw", "BER": "Berlin", "MUC": "Munich", "FRA": "Frankfurt",
		"ZRH": "Zurich", "CPH": "Copenhagen", "OSL": "Oslo", "ARN": "Stockholm",
		"DUB": "Dublin", "BRU": "Brussels", "LIS": "Lisbon", "ATH": "Athens",
		"IST": "Istanbul", "JFK": "New York JFK", "EWR": "Newark", "LAX": "Los Angeles",
		"SFO": "San Francisco", "ORD": "Chicago", "NRT": "Tokyo Narita",
		"HND": "Tokyo Haneda", "ICN": "Seoul", "SIN": "Singapore", "BKK": "Bangkok",
		"HKG": "Hong Kong", "SYD": "Sydney", "DXB": "Dubai", "DOH": "Doha",
	}
	for code, name := range commonAirports {
		airportCompletionMap[code] = name
	}
}

// ServeStdio runs the MCP server over stdin/stdout.
// Each line of input is a JSON-RPC request; each response is written as a single JSON line.
func (s *Server) ServeStdio(in io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(in)
	// Allow up to 1MB per line for large tool call results.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	// Set up notification writer and elicitation reader for server->client.
	s.notifyWriter = out
	s.elicitReader = scanner

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			resp := Response{
				JSONRPC: "2.0",
				Error:   &Error{Code: -32700, Message: fmt.Sprintf("parse error: %v", err)},
			}
			if writeErr := writeJSON(out, resp); writeErr != nil {
				return writeErr
			}
			continue
		}

		resp := s.HandleRequest(&req)
		if resp == nil {
			// Notification -- no response.
			continue
		}
		if err := writeJSON(out, resp); err != nil {
			return err
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read stdin: %w", err)
	}
	return nil
}

// writeJSON marshals v as a single JSON line to w.
func writeJSON(w io.Writer, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal response: %w", err)
	}
	if _, err := fmt.Fprintf(w, "%s\n", data); err != nil {
		return fmt.Errorf("write response: %w", err)
	}
	return nil
}

// Run starts the MCP server on stdin/stdout. This is the main entry point
// for the stdio transport.
//
// Coverage exclusion: blocking stdio entry point.
// ServeStdio (which Run calls) is tested via buffer I/O in server_test.go.
func Run() error {
	s := NewServer()
	log.SetOutput(io.Discard) // Suppress log output on stdio transport.
	return s.ServeStdio(os.Stdin, os.Stdout)
}

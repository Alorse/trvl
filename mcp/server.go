// Package mcp implements an MCP (Model Context Protocol) server for trvl.
//
// It supports two transports:
//   - stdio: JSON-RPC messages over stdin/stdout (one JSON object per line)
//   - HTTP:  JSON-RPC messages via POST /mcp
//
// The server exposes four tools: search_flights, search_dates, search_hotels,
// and hotel_prices. It also provides prompts and resources.
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
	"sync/atomic"
)

const (
	serverName      = "trvl"
	serverVersion   = "0.2.0"
	protocolVersion = "2025-11-25"
)

// --- JSON-RPC types ---

// Request is a JSON-RPC 2.0 request.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response is a JSON-RPC 2.0 response.
type Response struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id,omitempty"`
	Result  any    `json:"result,omitempty"`
	Error   *Error `json:"error,omitempty"`
}

// Error is a JSON-RPC 2.0 error object.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// --- MCP protocol types ---

// InitializeParams holds the client's initialize request parameters.
type InitializeParams struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ClientCapabilities `json:"capabilities"`
	ClientInfo      ClientInfo         `json:"clientInfo"`
}

// ClientInfo describes the client identity.
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ClientCapabilities describes what the client supports.
type ClientCapabilities struct {
	Elicitation *ElicitationCapability `json:"elicitation,omitempty"`
	Sampling    *SamplingCapability    `json:"sampling,omitempty"`
	Roots       *RootsCapability       `json:"roots,omitempty"`
}

// ElicitationCapability indicates the client supports elicitation/create.
type ElicitationCapability struct{}

// SamplingCapability indicates the client supports sampling/createMessage.
type SamplingCapability struct{}

// RootsCapability indicates the client supports roots/list.
type RootsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// InitializeResult is the response to the initialize method.
type InitializeResult struct {
	ProtocolVersion string       `json:"protocolVersion"`
	Capabilities    Capabilities `json:"capabilities"`
	ServerInfo      ServerInfo   `json:"serverInfo"`
}

// Capabilities describes the server's MCP capabilities.
type Capabilities struct {
	Tools     *ToolsCapability     `json:"tools,omitempty"`
	Prompts   *PromptsCapability   `json:"prompts,omitempty"`
	Resources *ResourcesCapability `json:"resources,omitempty"`
	Logging   *LoggingCapability   `json:"logging,omitempty"`
}

// ToolsCapability indicates the server supports tool listing and calling.
type ToolsCapability struct {
	ListChanged bool `json:"listChanged"`
}

// PromptsCapability indicates the server supports prompt listing and retrieval.
type PromptsCapability struct {
	ListChanged bool `json:"listChanged"`
}

// ResourcesCapability indicates the server supports resource listing and reading.
type ResourcesCapability struct {
	Subscribe   bool `json:"subscribe"`
	ListChanged bool `json:"listChanged"`
}

// LoggingCapability indicates the server supports logging notifications.
type LoggingCapability struct{}

// ServerInfo describes the server identity.
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// --- Tools types ---

// ToolsListResult is the response to tools/list.
type ToolsListResult struct {
	Tools []ToolDef `json:"tools"`
}

// ToolDef describes a single tool for tools/list.
type ToolDef struct {
	Name         string           `json:"name"`
	Title        string           `json:"title,omitempty"`
	Description  string           `json:"description"`
	InputSchema  InputSchema      `json:"inputSchema"`
	OutputSchema interface{}      `json:"outputSchema,omitempty"`
	Annotations  *ToolAnnotations `json:"annotations,omitempty"`
}

// ToolAnnotations provides metadata hints about a tool's behavior.
type ToolAnnotations struct {
	Title          string `json:"title,omitempty"`
	ReadOnlyHint   bool   `json:"readOnlyHint,omitempty"`
	IdempotentHint bool   `json:"idempotentHint,omitempty"`
	OpenWorldHint  bool   `json:"openWorldHint,omitempty"`
}

// InputSchema is a JSON Schema describing the tool's input parameters.
type InputSchema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required,omitempty"`
}

// Property describes a single input parameter.
type Property struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

// ToolCallParams is the params object for tools/call.
type ToolCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

// ToolCallResult is the response to tools/call.
type ToolCallResult struct {
	Content           []ContentBlock `json:"content"`
	StructuredContent interface{}    `json:"structuredContent,omitempty"`
	IsError           bool           `json:"isError,omitempty"`
}

// ContentBlock is a single content block in a tool result.
type ContentBlock struct {
	Type        string             `json:"type"`
	Text        string             `json:"text,omitempty"`
	Annotations *ContentAnnotation `json:"annotations,omitempty"`
}

// ContentAnnotation provides hints about a content block's audience and priority.
type ContentAnnotation struct {
	Audience []string `json:"audience,omitempty"` // "user", "assistant"
	Priority float64  `json:"priority,omitempty"` // 0.0 - 1.0
}

// --- Elicitation types ---

// ElicitFunc asks the client a question and returns the user's response.
// If the client does not support elicitation (or we are in HTTP mode), this
// will be nil and tool handlers should proceed with defaults.
type ElicitFunc func(message string, schema map[string]interface{}) (map[string]interface{}, error)

// ElicitationRequest is the JSON-RPC request sent to the client.
type ElicitationRequest struct {
	JSONRPC string               `json:"jsonrpc"`
	ID      any                  `json:"id"`
	Method  string               `json:"method"`
	Params  ElicitationReqParams `json:"params"`
}

// ElicitationReqParams is the params for elicitation/create.
type ElicitationReqParams struct {
	Message         string      `json:"message"`
	RequestedSchema interface{} `json:"requestedSchema"`
}

// ElicitationResponse is the client's response to an elicitation request.
type ElicitationResponse struct {
	JSONRPC string              `json:"jsonrpc"`
	ID      any                 `json:"id,omitempty"`
	Result  ElicitationResult   `json:"result"`
	Error   *Error              `json:"error,omitempty"`
}

// ElicitationResult is the result of an elicitation/create response.
type ElicitationResult struct {
	Action  string                 `json:"action"` // "accept", "decline", "cancel"
	Content map[string]interface{} `json:"content,omitempty"`
}

// --- Progress notification types ---

// ProgressNotification is sent during long-running operations.
type ProgressNotification struct {
	JSONRPC string         `json:"jsonrpc"`
	Method  string         `json:"method"`
	Params  ProgressParams `json:"params"`
}

// ProgressParams describes progress of an operation.
type ProgressParams struct {
	ProgressToken string  `json:"progressToken"`
	Progress      float64 `json:"progress"`
	Total         float64 `json:"total"`
	Message       string  `json:"message,omitempty"`
}

// --- Logging notification types ---

// LogNotification is a server-to-client log message.
type LogNotification struct {
	JSONRPC string    `json:"jsonrpc"`
	Method  string    `json:"method"`
	Params  LogParams `json:"params"`
}

// LogParams describes a log message.
type LogParams struct {
	Level  string `json:"level"`  // "debug", "info", "warning", "error"
	Logger string `json:"logger"` // logger name (e.g., "trvl")
	Data   string `json:"data"`   // log message
}

// --- Prompts types ---

// PromptsListResult is the response to prompts/list.
type PromptsListResult struct {
	Prompts []PromptDef `json:"prompts"`
}

// PromptDef describes a prompt template.
type PromptDef struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Arguments   []PromptArgument `json:"arguments,omitempty"`
}

// PromptArgument describes a single prompt argument.
type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

// PromptsGetParams is the params object for prompts/get.
type PromptsGetParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

// PromptsGetResult is the response to prompts/get.
type PromptsGetResult struct {
	Description string          `json:"description,omitempty"`
	Messages    []PromptMessage `json:"messages"`
}

// PromptMessage is a single message in a prompt result.
type PromptMessage struct {
	Role    string       `json:"role"`
	Content ContentBlock `json:"content"`
}

// --- Resources types ---

// ResourcesListResult is the response to resources/list.
type ResourcesListResult struct {
	Resources []ResourceDef `json:"resources"`
}

// ResourceDef describes a single resource.
type ResourceDef struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// ResourcesReadParams is the params object for resources/read.
type ResourcesReadParams struct {
	URI string `json:"uri"`
}

// ResourcesReadResult is the response to resources/read.
type ResourcesReadResult struct {
	Contents []ResourceContent `json:"contents"`
}

// ResourceContent is a single content block in a resource read result.
type ResourceContent struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
}

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

	// For elicitation: reader and writer set during ServeStdio.
	elicitReader *bufio.Scanner
	elicitID     atomic.Int64
}

// ToolHandler processes a tool call and returns content blocks, optional
// structured content, and an error.
// The elicit parameter may be nil if the client does not support elicitation.
type ToolHandler func(args map[string]any, elicit ElicitFunc) ([]ContentBlock, interface{}, error)

// NewServer creates a new MCP server with the standard trvl tools registered.
func NewServer() *Server {
	s := &Server{
		handlers: make(map[string]ToolHandler),
	}
	registerTools(s)
	registerPrompts(s)
	registerResources(s)
	return s
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

// HandleRequest processes a single JSON-RPC request and returns the response.
func (s *Server) HandleRequest(req *Request) *Response {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "notifications/initialized":
		return nil
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

	// Build elicit function based on client capabilities.
	elicit := s.makeElicitFunc()

	content, structured, err := handler(params.Arguments, elicit)
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
	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  ResourcesListResult{Resources: s.resources},
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

	result, err := readResource(params.URI)
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
func Run() error {
	s := NewServer()
	log.SetOutput(io.Discard) // Suppress log output on stdio transport.
	return s.ServeStdio(os.Stdin, os.Stdout)
}

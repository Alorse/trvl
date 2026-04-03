package mcp

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// HTTPServer wraps an MCP Server with an HTTP transport.
type HTTPServer struct {
	server *Server
	port   int
}

// NewHTTPServer creates an HTTP transport for the MCP server on the given port.
func NewHTTPServer(port int) *HTTPServer {
	return &HTTPServer{
		server: NewServer(),
		port:   port,
	}
}

// ListenAndServe starts the HTTP server. It blocks until the server exits.
//
// Coverage exclusion: blocking HTTP server entry point.
// The handler logic (handleMCP, handleHealth) is tested via httptest in server_extra_test.go.
func (h *HTTPServer) ListenAndServe() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", h.handleMCP)
	mux.HandleFunc("/health", h.handleHealth)

	addr := fmt.Sprintf(":%d", h.port)
	log.Printf("trvl MCP server listening on http://localhost%s/mcp", addr)
	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	return srv.ListenAndServe()
}

func (h *HTTPServer) handleMCP(w http.ResponseWriter, r *http.Request) {
	// CORS headers — restrict to localhost origins only.
	origin := r.Header.Get("Origin")
	if isLocalhostOrigin(origin) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
	}
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Limit request body to 1MB to prevent abuse.
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var req Request
	if err := json.Unmarshal(body, &req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := Response{
			JSONRPC: "2.0",
			Error:   &Error{Code: -32700, Message: fmt.Sprintf("parse error: %v", err)},
		}
		json.NewEncoder(w).Encode(resp)
		return
	}

	resp := h.server.HandleRequest(&req)
	if resp == nil {
		// Notification — return 204 No Content.
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *HTTPServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"server":  serverName,
		"version": serverVersion,
		"tools":   len(h.server.tools),
	})
}

// isLocalhostOrigin checks if the origin is a localhost URL (any port).
func isLocalhostOrigin(origin string) bool {
	if origin == "" {
		return false
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	host := strings.Split(u.Host, ":")[0]
	return host == "localhost" || host == "127.0.0.1" || host == "[::1]"
}

// RunHTTP starts the MCP server in HTTP mode on the given port.
//
// Coverage exclusion: blocking HTTP server entry point.
// Calls ListenAndServe, whose handler logic is tested via httptest in server_extra_test.go.
func RunHTTP(port int) error {
	return NewHTTPServer(port).ListenAndServe()
}

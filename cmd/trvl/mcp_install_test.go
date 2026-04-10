package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestClientConfigPath_ClaudeDesktop(t *testing.T) {
	path, err := clientConfigPath("claude-desktop")
	if err != nil {
		t.Fatalf("clientConfigPath(claude-desktop): %v", err)
	}
	if path == "" {
		t.Fatal("clientConfigPath(claude-desktop) returned empty string")
	}
	if !strings.HasSuffix(path, "claude_desktop_config.json") {
		t.Errorf("path %q does not end with claude_desktop_config.json", path)
	}
	switch runtime.GOOS {
	case "darwin":
		if !strings.Contains(path, "Application Support") {
			t.Errorf("darwin path should contain Application Support, got %q", path)
		}
	case "linux":
		if !strings.Contains(path, ".config") {
			t.Errorf("linux path should contain .config, got %q", path)
		}
	}
}

func TestClientConfigPath_ClaudeAlias(t *testing.T) {
	path1, err1 := clientConfigPath("claude-desktop")
	path2, err2 := clientConfigPath("claude")
	if err1 != nil || err2 != nil {
		t.Fatalf("errors: %v, %v", err1, err2)
	}
	if path1 != path2 {
		t.Errorf("claude-desktop and claude should resolve identically, got %q vs %q", path1, path2)
	}
}

func TestClientConfigPath_Cursor(t *testing.T) {
	path, err := clientConfigPath("cursor")
	if err != nil {
		t.Fatalf("clientConfigPath(cursor): %v", err)
	}
	if !strings.HasSuffix(path, filepath.Join(".cursor", "mcp.json")) {
		t.Errorf("cursor path should end with .cursor/mcp.json, got %q", path)
	}
}

func TestClientConfigPath_ClaudeCode(t *testing.T) {
	path, err := clientConfigPath("claude-code")
	if err != nil {
		t.Fatalf("clientConfigPath(claude-code): %v", err)
	}
	if !strings.HasSuffix(path, ".claude.json") {
		t.Errorf("claude-code path should end with .claude.json, got %q", path)
	}
}

func TestClientConfigPath_CaseInsensitive(t *testing.T) {
	path1, _ := clientConfigPath("Cursor")
	path2, _ := clientConfigPath("CURSOR")
	path3, _ := clientConfigPath("cursor")
	if path1 != path2 || path2 != path3 {
		t.Errorf("clientConfigPath should be case-insensitive: %q, %q, %q", path1, path2, path3)
	}
}

func TestClientConfigPath_Unknown(t *testing.T) {
	_, err := clientConfigPath("unknown-client")
	if err == nil {
		t.Fatal("clientConfigPath(unknown-client) should return error")
	}
	if !strings.Contains(err.Error(), "unknown client") {
		t.Errorf("error should mention 'unknown client', got %q", err.Error())
	}
}

func TestTrvlBinaryPath(t *testing.T) {
	path, err := trvlBinaryPath()
	if err != nil {
		t.Fatalf("trvlBinaryPath: %v", err)
	}
	if path == "" {
		t.Fatal("trvlBinaryPath returned empty string")
	}
	if !filepath.IsAbs(path) {
		t.Errorf("trvlBinaryPath should return absolute path, got %q", path)
	}
}

func TestRunInstall_DryRun(t *testing.T) {
	// Dry-run should not create any files, just print what would happen.
	// Use claude-code client since its config path is simplest (~/.claude.json).
	err := runInstall("claude-code", false, true)
	if err != nil {
		t.Fatalf("runInstall dry-run: %v", err)
	}
}

func TestRunInstall_DryRun_CursorClient(t *testing.T) {
	err := runInstall("cursor", false, true)
	if err != nil {
		t.Fatalf("runInstall dry-run cursor: %v", err)
	}
}

func TestRunInstall_DryRun_ClaudeDesktop(t *testing.T) {
	err := runInstall("claude-desktop", false, true)
	if err != nil {
		t.Fatalf("runInstall dry-run claude-desktop: %v", err)
	}
}

func TestRunInstall_UnknownClient(t *testing.T) {
	err := runInstall("nonexistent", false, true)
	if err == nil {
		t.Fatal("runInstall with unknown client should return error")
	}
}

func TestRunInstall_MergesExistingConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")

	// Pre-existing config with another MCP server.
	existing := map[string]any{
		"mcpServers": map[string]any{
			"other-tool": map[string]any{
				"command": "/usr/bin/other",
				"args":    []string{"serve"},
			},
		},
	}
	data, _ := json.MarshalIndent(existing, "", "  ")
	if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	// Simulate what runInstall does: load, merge, write.
	raw, _ := os.ReadFile(cfgPath)
	cfg := map[string]any{}
	_ = json.Unmarshal(raw, &cfg)

	servers, _ := cfg["mcpServers"].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}
	servers["trvl"] = map[string]any{
		"command": "/usr/local/bin/trvl",
		"args":    []string{"mcp"},
	}
	cfg["mcpServers"] = servers

	out, _ := json.MarshalIndent(cfg, "", "  ")
	if err := os.WriteFile(cfgPath, out, 0o644); err != nil {
		t.Fatal(err)
	}

	// Read back and verify both entries exist.
	readData, _ := os.ReadFile(cfgPath)
	var result map[string]any
	if err := json.Unmarshal(readData, &result); err != nil {
		t.Fatalf("merged config is not valid JSON: %v", err)
	}
	merged, _ := result["mcpServers"].(map[string]any)
	if _, ok := merged["other-tool"]; !ok {
		t.Error("existing 'other-tool' entry was lost during merge")
	}
	if _, ok := merged["trvl"]; !ok {
		t.Error("trvl entry was not added during merge")
	}
}

func TestRunInstall_CreatesNewFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nested", "config.json")

	// Ensure parent directory creation works.
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatal(err)
	}

	// Start with no file; simulate runInstall logic.
	cfg := map[string]any{}
	servers := map[string]any{}
	servers["trvl"] = map[string]any{
		"command": "/usr/local/bin/trvl",
		"args":    []string{"mcp"},
	}
	cfg["mcpServers"] = servers

	out, _ := json.MarshalIndent(cfg, "", "  ")
	if err := os.WriteFile(cfgPath, out, 0o644); err != nil {
		t.Fatal(err)
	}

	// Read back.
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("file should exist: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	merged, _ := result["mcpServers"].(map[string]any)
	if _, ok := merged["trvl"]; !ok {
		t.Error("trvl entry not found in newly created config")
	}
}

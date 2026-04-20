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

func setInstallTestHome(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	setTestHome(t, dir)
	t.Setenv("APPDATA", filepath.Join(dir, "AppData", "Roaming"))
}

func installConfigPath(t *testing.T, client string) string {
	t.Helper()

	cfgPath, err := clientConfigPath(client)
	if err != nil {
		t.Fatalf("clientConfigPath(%q): %v", client, err)
	}
	return cfgPath
}

func readJSONConfig(t *testing.T, path string) map[string]any {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", path, err)
	}

	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("Unmarshal(%q): %v", path, err)
	}
	return cfg
}

func TestRunInstall_DryRun(t *testing.T) {
	setInstallTestHome(t)
	cfgPath := installConfigPath(t, "claude-code")

	err := runInstall("claude-code", false, true)
	if err != nil {
		t.Fatalf("runInstall dry-run: %v", err)
	}
	if _, err := os.Stat(cfgPath); !os.IsNotExist(err) {
		t.Fatalf("dry-run should not create %s, stat err = %v", cfgPath, err)
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
	setInstallTestHome(t)
	cfgPath := installConfigPath(t, "claude-code")

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
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := runInstall("claude-code", false, false); err != nil {
		t.Fatalf("runInstall merge existing config: %v", err)
	}

	result := readJSONConfig(t, cfgPath)
	merged, _ := result["mcpServers"].(map[string]any)
	if _, ok := merged["other-tool"]; !ok {
		t.Error("existing 'other-tool' entry was lost during merge")
	}
	trvlEntry, ok := merged["trvl"].(map[string]any)
	if !ok {
		t.Error("trvl entry was not added during merge")
	} else {
		binary, err := trvlBinaryPath()
		if err != nil {
			t.Fatalf("trvlBinaryPath: %v", err)
		}
		if got := trvlEntry["command"]; got != binary {
			t.Errorf("trvl command = %v, want %q", got, binary)
		}
		args, _ := trvlEntry["args"].([]any)
		if len(args) != 1 || args[0] != "mcp" {
			t.Errorf("trvl args = %v, want [mcp]", trvlEntry["args"])
		}
	}

	backup, err := os.ReadFile(cfgPath + ".trvl.bak")
	if err != nil {
		t.Fatalf("expected backup file: %v", err)
	}
	if string(backup) != string(data) {
		t.Error("backup file should preserve original config contents")
	}
}

func TestRunInstall_CreatesNewFile(t *testing.T) {
	setInstallTestHome(t)
	cfgPath := installConfigPath(t, "claude-code")

	if err := runInstall("claude-code", false, false); err != nil {
		t.Fatalf("runInstall create config: %v", err)
	}

	result := readJSONConfig(t, cfgPath)
	merged, _ := result["mcpServers"].(map[string]any)
	if _, ok := merged["trvl"]; !ok {
		t.Error("trvl entry not found in newly created config")
	}
}

func TestRunInstall_InvalidJSONWithoutForce(t *testing.T) {
	setInstallTestHome(t)
	cfgPath := installConfigPath(t, "claude-code")

	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, []byte("{invalid"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := runInstall("claude-code", false, false)
	if err == nil {
		t.Fatal("runInstall should reject invalid JSON without --force")
	}
	if !strings.Contains(err.Error(), "use --force to overwrite") {
		t.Fatalf("error = %q, want overwrite hint", err.Error())
	}
}

func TestRunInstall_ForceOverwritesInvalidJSON(t *testing.T) {
	setInstallTestHome(t)
	cfgPath := installConfigPath(t, "claude-code")
	invalid := []byte("{invalid")

	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, invalid, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := runInstall("claude-code", true, false); err != nil {
		t.Fatalf("runInstall force overwrite invalid JSON: %v", err)
	}

	result := readJSONConfig(t, cfgPath)
	merged, _ := result["mcpServers"].(map[string]any)
	if _, ok := merged["trvl"]; !ok {
		t.Fatal("forced install should replace invalid config with trvl entry")
	}

	backup, err := os.ReadFile(cfgPath + ".trvl.bak")
	if err != nil {
		t.Fatalf("expected backup of invalid config: %v", err)
	}
	if string(backup) != string(invalid) {
		t.Fatalf("backup contents = %q, want original invalid config %q", string(backup), string(invalid))
	}
}

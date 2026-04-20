package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikkoParkkola/trvl/internal/providers"
)

// writeTestProviderV19 creates a minimal provider config JSON in the temp HOME.
func writeTestProviderV19(t *testing.T, tmp, id string) {
	t.Helper()
	dir := filepath.Join(tmp, ".trvl", "providers")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir providers: %v", err)
	}
	cfg := providers.ProviderConfig{
		ID:       id,
		Name:     "Test Provider " + id,
		Category: "hotels",
		Endpoint: "https://example.com/api",
		Method:   "GET",
		Consent: &providers.ConsentRecord{
			Granted:   true,
			Timestamp: time.Now(),
			Domain:    "example.com",
		},
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal provider: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, id+".json"), data, 0o600); err != nil {
		t.Fatalf("write provider file: %v", err)
	}
}

// ---------------------------------------------------------------------------
// shouldShowNudge — pure function with injectable deps
// ---------------------------------------------------------------------------

func TestShouldShowNudge_NotSearchCommand(t *testing.T) {
	got := shouldShowNudge("profile", "", func(string) string { return "" }, 0, func(int) bool { return true })
	if got {
		t.Error("expected false for non-search command")
	}
}

func TestShouldShowNudge_SuppressedByEnv(t *testing.T) {
	got := shouldShowNudge("flights", "", func(key string) string {
		if key == "TRVL_NO_NUDGE" {
			return "1"
		}
		return ""
	}, 0, func(int) bool { return true })
	if got {
		t.Error("expected false when TRVL_NO_NUDGE=1")
	}
}

func TestShouldShowNudge_MCPCommandV4(t *testing.T) {
	got := shouldShowNudge("mcp", "", func(string) string { return "" }, 0, func(int) bool { return true })
	if got {
		t.Error("expected false for mcp command")
	}
}

func TestShouldShowNudge_JSONFormatV4(t *testing.T) {
	got := shouldShowNudge("flights", "json", func(string) string { return "" }, 0, func(int) bool { return true })
	if got {
		t.Error("expected false for json format")
	}
}

func TestShouldShowNudge_NotTerminal(t *testing.T) {
	got := shouldShowNudge("flights", "", func(string) string { return "" }, 0, func(int) bool { return false })
	if got {
		t.Error("expected false when not a terminal")
	}
}

func TestShouldShowNudge_ShouldShow(t *testing.T) {
	got := shouldShowNudge("flights", "", func(string) string { return "" }, 0, func(int) bool { return true })
	if !got {
		t.Error("expected true for search command + terminal + no suppression")
	}
}

func TestShouldShowNudge_AllSearchCommandsV4(t *testing.T) {
	for cmd := range searchCommands {
		got := shouldShowNudge(cmd, "", func(string) string { return "" }, 0, func(int) bool { return true })
		if !got {
			t.Errorf("expected true for search command %q", cmd)
		}
	}
}

func TestShouldShowNudge_NotSearchCmdV14(t *testing.T) {
	got := shouldShowNudge("setup", "", os.Getenv, os.Stderr.Fd(), func(int) bool { return true })
	if got {
		t.Error("expected false for non-search command")
	}
}

func TestShouldShowNudge_TrvlNoNudgeEnvV14(t *testing.T) {
	got := shouldShowNudge("flights", "", func(key string) string {
		if key == "TRVL_NO_NUDGE" {
			return "1"
		}
		return ""
	}, os.Stderr.Fd(), func(int) bool { return true })
	if got {
		t.Error("expected false when TRVL_NO_NUDGE=1")
	}
}

func TestShouldShowNudge_NotSearchCommandV24(t *testing.T) {
	got := shouldShowNudge("prefs", "", os.Getenv, 2, func(int) bool { return true })
	if got {
		t.Error("expected false for non-search command")
	}
}

func TestShouldShowNudge_NoNudgeEnvV24(t *testing.T) {
	t.Setenv("TRVL_NO_NUDGE", "1")
	got := shouldShowNudge("flights", "", os.Getenv, 2, func(int) bool { return true })
	if got {
		t.Error("expected false when TRVL_NO_NUDGE=1")
	}
}

func TestShouldShowNudge_JSONFormatV24(t *testing.T) {
	got := shouldShowNudge("flights", "json", os.Getenv, 2, func(int) bool { return true })
	if got {
		t.Error("expected false when format=json")
	}
}

func TestShouldShowNudge_NotTerminalV24(t *testing.T) {
	got := shouldShowNudge("flights", "", os.Getenv, 2, func(int) bool { return false })
	if got {
		t.Error("expected false when not a terminal")
	}
}

func TestShouldShowNudge_ReturnsTrueV24(t *testing.T) {
	t.Setenv("TRVL_NO_NUDGE", "")
	got := shouldShowNudge("hotels", "", func(key string) string {
		if key == "TRVL_NO_NUDGE" {
			return ""
		}
		return ""
	}, 2, func(int) bool { return true })
	if !got {
		t.Error("expected true for search command with terminal and no suppression")
	}
}

func TestShouldShowNudge_SearchCommand_Terminal_V28(t *testing.T) {
	got := shouldShowNudge("flights", "table", func(string) string { return "" }, 0, func(int) bool { return true })
	if !got {
		t.Error("expected true for search command on terminal")
	}
}

func TestShouldShowNudge_NonSearch_V28(t *testing.T) {
	got := shouldShowNudge("version", "table", func(string) string { return "" }, 0, func(int) bool { return true })
	if got {
		t.Error("expected false for non-search command")
	}
}

func TestShouldShowNudge_EnvVarSuppressed_V28(t *testing.T) {
	got := shouldShowNudge("flights", "table", func(k string) string {
		if k == "TRVL_NO_NUDGE" {
			return "1"
		}
		return ""
	}, 0, func(int) bool { return true })
	if got {
		t.Error("expected false when TRVL_NO_NUDGE=1")
	}
}

func TestMaybeShowStarNudge_JSONFormatNoOp(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	maybeShowStarNudge("flights", "json")
}

// ---------------------------------------------------------------------------
// nudgePath — pure helper
// ---------------------------------------------------------------------------

func TestNudgePath_ReturnsPathV24(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	p, err := nudgePath()
	if err != nil {
		t.Fatalf("nudgePath: %v", err)
	}
	if !strings.HasSuffix(p, "nudge.json") {
		t.Errorf("expected path ending in nudge.json, got %s", p)
	}
}

// ---------------------------------------------------------------------------
// loadNudgeState / saveNudgeState — disk round-trip
// ---------------------------------------------------------------------------

func TestLoadNudgeState_InvalidJSONV14(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "nudge.json")
	_ = os.WriteFile(p, []byte("not-json"), 0o600)
	s := loadNudgeState(p)
	if s.SearchCount != 0 || s.Shown {
		t.Errorf("expected zero nudgeState for invalid JSON, got %+v", s)
	}
}

func TestSaveAndLoadNudgeStateV14(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "nudge.json")
	original := nudgeState{SearchCount: 2, Shown: false}
	saveNudgeState(p, original)
	loaded := loadNudgeState(p)
	if loaded.SearchCount != 2 {
		t.Errorf("expected SearchCount=2, got %d", loaded.SearchCount)
	}
}

func TestSaveNudgeState_ShownTrueV14(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "nudge.json")
	s := nudgeState{SearchCount: 5, Shown: true, ShownAt: time.Now()}
	saveNudgeState(p, s)
	loaded := loadNudgeState(p)
	if !loaded.Shown {
		t.Error("expected Shown=true after save")
	}
}

func TestSaveAndLoadNudgeState_V24(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "nudge.json")

	s := nudgeState{SearchCount: 3, Shown: true}
	saveNudgeState(path, s)

	loaded := loadNudgeState(path)
	if loaded.SearchCount != 3 {
		t.Errorf("expected SearchCount=3, got %d", loaded.SearchCount)
	}
	if !loaded.Shown {
		t.Error("expected Shown=true")
	}
}

func TestLoadNudgeState_MissingFileV24(t *testing.T) {
	s := loadNudgeState("/tmp/nonexistent-nudge-xyz.json")
	if s.SearchCount != 0 || s.Shown {
		t.Errorf("expected zero state for missing file, got %+v", s)
	}
}

func TestLoadNudgeState_MissingFile_V28(t *testing.T) {
	s := loadNudgeState("/tmp/trvl-nonexistent-nudge-v28-xyz.json")
	if s.SearchCount != 0 || s.Shown {
		t.Errorf("expected zero state for missing file, got %+v", s)
	}
}

func TestLoadNudgeState_InvalidJSON_V28(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nudge.json")
	_ = os.WriteFile(path, []byte("not json"), 0o600)
	s := loadNudgeState(path)
	if s.SearchCount != 0 || s.Shown {
		t.Errorf("expected zero state for invalid JSON, got %+v", s)
	}
}

func TestSaveAndLoadNudgeState_RoundTrip_V28(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nudge.json")

	want := nudgeState{SearchCount: 2, Shown: false}
	saveNudgeState(path, want)
	got := loadNudgeState(path)

	if got.SearchCount != want.SearchCount || got.Shown != want.Shown {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, want)
	}
}

func TestSaveNudgeState_ShownTrue_V28(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "nudge.json")

	want := nudgeState{SearchCount: 3, Shown: true, ShownAt: time.Now()}
	saveNudgeState(path, want)
	got := loadNudgeState(path)

	if !got.Shown || got.SearchCount != 3 {
		t.Errorf("expected Shown=true SearchCount=3, got %+v", got)
	}
}

// ---------------------------------------------------------------------------
// clientConfigPath — remaining client types
// ---------------------------------------------------------------------------

func TestClientConfigPath_ZedClient(t *testing.T) {
	path, err := clientConfigPath("zed")
	if err != nil {
		t.Fatalf("clientConfigPath(zed): %v", err)
	}
	if path == "" {
		t.Error("expected non-empty path for zed")
	}
}

func TestClientConfigPath_LMStudio(t *testing.T) {
	path, err := clientConfigPath("lm-studio")
	if err != nil {
		t.Fatalf("clientConfigPath(lm-studio): %v", err)
	}
	if path == "" {
		t.Error("expected non-empty path for lm-studio")
	}
}

func TestClientConfigPath_Gemini(t *testing.T) {
	path, err := clientConfigPath("gemini")
	if err != nil {
		t.Fatalf("clientConfigPath(gemini): %v", err)
	}
	if path == "" {
		t.Error("expected non-empty path for gemini")
	}
}

func TestClientConfigPath_AmazonQ(t *testing.T) {
	path, err := clientConfigPath("amazon-q")
	if err != nil {
		t.Fatalf("clientConfigPath(amazon-q): %v", err)
	}
	if path == "" {
		t.Error("expected non-empty path for amazon-q")
	}
}

func TestClientConfigPath_VSCode(t *testing.T) {
	path, err := clientConfigPath("vscode")
	if err != nil {
		t.Fatalf("clientConfigPath(vscode): %v", err)
	}
	if path == "" {
		t.Error("expected non-empty path for vscode")
	}
}

func TestClientConfigPath_Windsurf(t *testing.T) {
	path, err := clientConfigPath("windsurf")
	if err != nil {
		t.Fatalf("clientConfigPath(windsurf): %v", err)
	}
	if path == "" {
		t.Error("expected non-empty path for windsurf")
	}
}

func TestClientConfigPath_WindsurfV19(t *testing.T) {
	p, err := clientConfigPath("windsurf")
	if err != nil {
		t.Fatalf("clientConfigPath(windsurf): %v", err)
	}
	if p == "" {
		t.Error("expected non-empty path for windsurf")
	}
}

func TestClientConfigPath_CodexV19(t *testing.T) {
	p, err := clientConfigPath("codex")
	if err != nil {
		t.Fatalf("clientConfigPath(codex): %v", err)
	}
	if p == "" {
		t.Error("expected non-empty path for codex")
	}
}

func TestClientConfigPath_GeminiV19(t *testing.T) {
	p, err := clientConfigPath("gemini")
	if err != nil {
		t.Fatalf("clientConfigPath(gemini): %v", err)
	}
	if p == "" {
		t.Error("expected non-empty path for gemini")
	}
}

func TestClientConfigPath_AmazonQV19(t *testing.T) {
	p, err := clientConfigPath("amazon-q")
	if err != nil {
		t.Fatalf("clientConfigPath(amazon-q): %v", err)
	}
	if p == "" {
		t.Error("expected non-empty path for amazon-q")
	}
}

func TestClientConfigPath_ZedV19(t *testing.T) {
	p, err := clientConfigPath("zed")
	if err != nil {
		t.Fatalf("clientConfigPath(zed): %v", err)
	}
	if p == "" {
		t.Error("expected non-empty path for zed")
	}
}

func TestClientConfigPath_LMStudioV19(t *testing.T) {
	p, err := clientConfigPath("lm-studio")
	if err != nil {
		t.Fatalf("clientConfigPath(lm-studio): %v", err)
	}
	if p == "" {
		t.Error("expected non-empty path for lm-studio")
	}
}

func TestClientConfigPath_VSCodeV19(t *testing.T) {
	p, err := clientConfigPath("vscode")
	if err != nil {
		t.Fatalf("clientConfigPath(vscode): %v", err)
	}
	if p == "" {
		t.Error("expected non-empty path for vscode")
	}
}

func TestClientConfigPath_KnownClients_V28(t *testing.T) {
	cases := []string{"cursor", "claude-code", "windsurf", "vscode", "vs-code", "copilot", "gemini", "amazon-q", "q", "lm-studio"}
	for _, c := range cases {
		p, err := clientConfigPath(c)
		if err != nil {
			t.Errorf("clientConfigPath(%q) err: %v", c, err)
		}
		if p == "" {
			t.Errorf("clientConfigPath(%q) returned empty path", c)
		}
	}
}

func TestClientConfigPath_Unknown_V28(t *testing.T) {
	_, err := clientConfigPath("definitely-not-a-real-client-v28")
	if err == nil {
		t.Error("expected error for unknown client")
	}
	if !strings.Contains(err.Error(), "unknown client") {
		t.Errorf("error should mention 'unknown client', got: %v", err)
	}
}

func TestClientConfigPath_Zed_V28(t *testing.T) {
	p, err := clientConfigPath("zed")
	if err != nil {
		t.Fatalf("clientConfigPath(zed): %v", err)
	}
	if !strings.Contains(p, "zed") && !strings.Contains(p, "Zed") {
		t.Errorf("zed path should contain 'zed' or 'Zed', got %q", p)
	}
}

func TestClientConfigPath_Claude_V28(t *testing.T) {
	p, err := clientConfigPath("claude")
	if err != nil {
		t.Fatalf("clientConfigPath(claude): %v", err)
	}
	if !strings.Contains(p, "Claude") {
		t.Errorf("claude path should contain 'Claude', got %q", p)
	}
}

// ---------------------------------------------------------------------------
// mcpConfigKey — remaining branches
// ---------------------------------------------------------------------------

func TestMCPConfigKey_Zed(t *testing.T) {
	got := mcpConfigKey("zed")
	if got != "context_servers" {
		t.Errorf("mcpConfigKey(zed) = %q, want %q", got, "context_servers")
	}
}

func TestMCPConfigKey_Default(t *testing.T) {
	got := mcpConfigKey("claude-desktop")
	if got != "mcpServers" {
		t.Errorf("mcpConfigKey(claude-desktop) = %q, want %q", got, "mcpServers")
	}
}

func TestMcpConfigKey_V24(t *testing.T) {
	cases := []struct {
		client string
		want   string
	}{
		{"vscode", "servers"},
		{"vs-code", "servers"},
		{"copilot", "servers"},
		{"zed", "context_servers"},
		{"claude-desktop", "mcpServers"},
		{"windsurf", "mcpServers"},
		{"codex", "mcpServers"},
	}
	for _, tc := range cases {
		got := mcpConfigKey(tc.client)
		if got != tc.want {
			t.Errorf("mcpConfigKey(%q) = %q, want %q", tc.client, got, tc.want)
		}
	}
}

func TestMCPConfigKey_V28(t *testing.T) {
	cases := []struct {
		client string
		want   string
	}{
		{"vscode", "servers"},
		{"vs-code", "servers"},
		{"copilot", "servers"},
		{"zed", "context_servers"},
		{"claude", "mcpServers"},
		{"cursor", "mcpServers"},
		{"claude-code", "mcpServers"},
	}
	for _, tt := range cases {
		got := mcpConfigKey(tt.client)
		if got != tt.want {
			t.Errorf("mcpConfigKey(%q) = %q, want %q", tt.client, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// isCodexTOML — pure predicate
// ---------------------------------------------------------------------------

func TestIsCodexTOML_V28(t *testing.T) {
	if !isCodexTOML("codex") {
		t.Error("expected true for 'codex'")
	}
	if !isCodexTOML("Codex") {
		t.Error("expected true for 'Codex'")
	}
	if isCodexTOML("claude") {
		t.Error("expected false for 'claude'")
	}
}

// ---------------------------------------------------------------------------
// loadJSONConfig — file-backed, no network
// ---------------------------------------------------------------------------

func TestLoadJSONConfig_NonExistentFile_V28(t *testing.T) {
	cfg, data, err := loadJSONConfig("/tmp/trvl-nonexistent-config-v28-xyz.json", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data) != 0 {
		t.Error("expected nil data for missing file")
	}
	if len(cfg) != 0 {
		t.Error("expected empty config for missing file")
	}
}

func TestLoadJSONConfig_ValidJSON_V28(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	_ = os.WriteFile(path, []byte(`{"mcpServers":{}}`), 0o600)

	cfg, data, err := loadJSONConfig(path, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty data")
	}
	if _, ok := cfg["mcpServers"]; !ok {
		t.Error("expected mcpServers key in config")
	}
}

func TestLoadJSONConfig_EmptyFile_V28(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	_ = os.WriteFile(path, []byte{}, 0o600)

	cfg, _, err := loadJSONConfig(path, false)
	if err != nil {
		t.Fatalf("unexpected error for empty file: %v", err)
	}
	if len(cfg) != 0 {
		t.Error("expected empty config for empty file")
	}
}

func TestLoadJSONConfig_InvalidJSON_NoForce_V28(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	_ = os.WriteFile(path, []byte("not json"), 0o600)

	_, _, err := loadJSONConfig(path, false)
	if err == nil {
		t.Error("expected error for invalid JSON without force")
	}
}

func TestLoadJSONConfig_InvalidJSON_WithForce_V28(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	_ = os.WriteFile(path, []byte("not json"), 0o600)

	cfg, _, err := loadJSONConfig(path, true)
	if err != nil {
		t.Fatalf("expected no error with force: %v", err)
	}
	if len(cfg) != 0 {
		t.Error("expected empty config when force-overwriting invalid JSON")
	}
}

// ---------------------------------------------------------------------------
// runInstallCodexTOML — file-backed, no network
// ---------------------------------------------------------------------------

func TestRunInstallCodexTOML_DryRun_New_V28(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	err := runInstallCodexTOML(path, "/usr/local/bin/trvl", false, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("dry-run should not create file")
	}
}

func TestRunInstallCodexTOML_Write_V28(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	err := runInstallCodexTOML(path, "/usr/local/bin/trvl", false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "[mcp_servers.trvl]") {
		t.Errorf("expected TOML entry in %s, got: %s", path, string(data))
	}
}

func TestRunInstallCodexTOML_AlreadyExists_NoForce_V28(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	_ = os.WriteFile(path, []byte("[mcp_servers.trvl]\ncommand = \"/old/trvl\"\n"), 0o644)

	err := runInstallCodexTOML(path, "/new/trvl", false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), "/new/trvl") {
		t.Error("should not overwrite without --force")
	}
}

func TestRunInstallCodexTOML_AlreadyExists_DryRun_V28(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	_ = os.WriteFile(path, []byte("[mcp_servers.trvl]\ncommand = \"/old/trvl\"\n"), 0o644)

	err := runInstallCodexTOML(path, "/new/trvl", false, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunInstallCodexTOML_Force_V28(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	_ = os.WriteFile(path, []byte("[mcp_servers.trvl]\ncommand = \"/old/trvl\"\n"), 0o644)

	err := runInstallCodexTOML(path, "/new/trvl", true, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// runInstall — codex TOML path
// ---------------------------------------------------------------------------

func TestRunInstall_CodexDryRunV19(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)
	err := runInstall("codex", false, true)
	if err != nil {
		t.Errorf("runInstall codex dry-run: %v", err)
	}
}

func TestRunInstall_CodexCreatesConfigV19(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)
	err := runInstall("codex", false, false)
	if err != nil {
		t.Errorf("runInstall codex create: %v", err)
	}
}

// ---------------------------------------------------------------------------
// trvlBinaryPath — covers the os.Executable path
// ---------------------------------------------------------------------------

func TestTrvlBinaryPath_ReturnsNonEmpty(t *testing.T) {
	path, err := trvlBinaryPath()
	if err != nil {
		t.Skipf("trvlBinaryPath error (expected in test env): %v", err)
	}
	if path == "" {
		t.Error("expected non-empty path")
	}
}

func TestTrvlBinaryPath_V24(t *testing.T) {
	p, err := trvlBinaryPath()
	if err != nil {
		t.Fatalf("trvlBinaryPath: %v", err)
	}
	if p == "" {
		t.Error("expected non-empty binary path")
	}
}

// ---------------------------------------------------------------------------
// secureTempPath / keysPath / saveKeys — basic coverage
// ---------------------------------------------------------------------------

func TestSecureTempPath_ReturnsPath(t *testing.T) {
	tmp := t.TempDir()
	path, err := secureTempPath(tmp, "trvl-test-")
	if err != nil {
		t.Fatalf("secureTempPath: %v", err)
	}
	if path == "" {
		t.Error("expected non-empty secureTempPath")
	}
}

func TestSecureTempPath_V24(t *testing.T) {
	tmp := t.TempDir()
	p, err := secureTempPath(tmp, "keys.json.tmp-")
	if err != nil {
		t.Fatalf("secureTempPath: %v", err)
	}
	if !strings.HasPrefix(filepath.Base(p), "keys.json.tmp-") {
		t.Errorf("unexpected prefix in %s", p)
	}
}

func TestKeysPath_ReturnsPath(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	path, err := keysPath()
	if err != nil {
		t.Fatalf("keysPath: %v", err)
	}
	if path == "" {
		t.Error("expected non-empty keysPath")
	}
}

func TestKeysPath_V24(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	p, err := keysPath()
	if err != nil {
		t.Fatalf("keysPath: %v", err)
	}
	if !strings.HasSuffix(p, "keys.json") {
		t.Errorf("expected keys.json suffix, got %s", p)
	}
}

func TestLoadExistingKeys_NonexistentFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	keys := loadExistingKeys()
	_ = keys
}

func TestLoadExistingKeys_MissingFileV24(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	keys := loadExistingKeys()
	if keys.SeatsAero != "" || keys.Kiwi != "" {
		t.Errorf("expected empty keys for missing file, got %+v", keys)
	}
}

func TestSaveKeys_WritesFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	keys := APIKeys{
		SeatsAero: "test-key",
	}
	if err := saveKeys(keys); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	path, _ := keysPath()
	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		t.Error("expected keys file to be written")
	}
}

func TestSaveKeysTo_V24(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, ".trvl", "keys.json")
	keys := APIKeys{SeatsAero: "test-key", Kiwi: "kiwi-key"}
	if err := saveKeysTo(path, keys); err != nil {
		t.Fatalf("saveKeysTo: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("keys.json not created: %v", err)
	}
}

// ---------------------------------------------------------------------------
// runSetup — non-interactive mode
// ---------------------------------------------------------------------------

func TestRunSetup_NonInteractiveV24(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cfg := setupConfig{
		nonInteractive: true,
		homeFlag:       "HEL",
		currencyFlag:   "EUR",
		cabinFlag:      "economy",
		stdin:          os.Stdin,
		stdout:         os.Stdout,
	}
	if err := runSetup(cfg); err != nil {
		t.Errorf("runSetup non-interactive: %v", err)
	}
}

func TestRunSetup_NonInteractiveBusinessClassV24(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cfg := setupConfig{
		nonInteractive: true,
		homeFlag:       "JFK",
		currencyFlag:   "USD",
		cabinFlag:      "business",
		stdin:          os.Stdin,
		stdout:         os.Stdout,
	}
	if err := runSetup(cfg); err != nil {
		t.Errorf("runSetup non-interactive business: %v", err)
	}
}

// ---------------------------------------------------------------------------
// createGist — gh not found → fallback print path (no network)
// ---------------------------------------------------------------------------

func TestCreateGist_NoGhV24(t *testing.T) {
	err := createGist("# Test trip\n\nSome markdown content here.")
	_ = err
}

// ---------------------------------------------------------------------------
// mcpInstallCmd — unknown client
// ---------------------------------------------------------------------------

func TestMCPInstallCmd_UnknownClient(t *testing.T) {
	cmd := mcpInstallCmd()
	cmd.SetArgs([]string{"--client", "totally-unknown-client"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for unknown client")
	}
}

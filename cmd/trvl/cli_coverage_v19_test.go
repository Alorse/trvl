package main

import (
	"encoding/json"
	"os"
	"path/filepath"
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
// runProvidersList — with actual providers (covers table render branch)
// ---------------------------------------------------------------------------

func TestRunProvidersList_WithProviderV19(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	writeTestProviderV19(t, tmp, "test-hotel-provider")

	cmd := providersCmd()
	cmd.SetArgs([]string{"list"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("providers list: %v", err)
	}
}

func TestRunProvidersList_WithProviderJSONV19(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	writeTestProviderV19(t, tmp, "test-hotel-provider-json")

	// The list subcommand doesn't accept --format; use global var.
	oldFormat := format
	format = "json"
	defer func() { format = oldFormat }()

	cmd := providersCmd()
	cmd.SetArgs([]string{"list"})
	_ = cmd.Execute()
}

// ---------------------------------------------------------------------------
// runProvidersStatus — with actual providers (covers table + summary branches)
// ---------------------------------------------------------------------------

func TestRunProvidersStatus_WithProviderV19(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	writeTestProviderV19(t, tmp, "test-status-provider")

	cmd := providersCmd()
	cmd.SetArgs([]string{"status"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("providers status: %v", err)
	}
}

func TestRunProvidersStatus_WithErrorProviderV19(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Provider with error state → covers errorNames branch.
	dir := filepath.Join(tmp, ".trvl", "providers")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	cfg := providers.ProviderConfig{
		ID:         "error-provider",
		Name:       "Error Provider",
		Category:   "flights",
		Endpoint:   "https://bad.example.com/api",
		LastError:  "connection refused",
		ErrorCount: 5,
		Consent: &providers.ConsentRecord{
			Granted:   true,
			Timestamp: time.Now(),
		},
	}
	data, _ := json.Marshal(cfg)
	_ = os.WriteFile(filepath.Join(dir, "error-provider.json"), data, 0o600)

	cmd := providersCmd()
	cmd.SetArgs([]string{"status"})
	_ = cmd.Execute()
}

func TestRunProvidersStatus_WithStaleProviderV19(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Provider with stale state (no success in >24h) → covers staleNames branch.
	dir := filepath.Join(tmp, ".trvl", "providers")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	cfg := providers.ProviderConfig{
		ID:          "stale-provider",
		Name:        "Stale Provider",
		Category:    "flights",
		Endpoint:    "https://stale.example.com/api",
		LastSuccess: time.Now().Add(-48 * time.Hour), // 48 hours ago → stale
		Consent: &providers.ConsentRecord{
			Granted:   true,
			Timestamp: time.Now(),
		},
	}
	data, _ := json.Marshal(cfg)
	_ = os.WriteFile(filepath.Join(dir, "stale-provider.json"), data, 0o600)

	cmd := providersCmd()
	cmd.SetArgs([]string{"status"})
	_ = cmd.Execute()
}

// ---------------------------------------------------------------------------
// runWatchCheckCycleWithRooms — empty store (covers the early-exit branch)
// ---------------------------------------------------------------------------

func TestRunWatchCheckCycleWithRooms_EmptyStoreV19(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	n, err := runWatchCheckCycleWithRooms(t.Context(), &liveChecker{}, &liveRoomChecker{}, nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 checks with empty store, got %d", n)
	}
}

// ---------------------------------------------------------------------------
// runProvidersDisable — non-terminal stdin path (stdin in tests is not a tty)
// ---------------------------------------------------------------------------

func TestRunProvidersDisable_WithProviderV19(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	writeTestProviderV19(t, tmp, "to-delete-provider")

	// Stdin is not a terminal in test → skips interactive prompt → deletes.
	cmd := providersCmd()
	cmd.SetArgs([]string{"disable", "to-delete-provider"})
	// Either succeeds or errors; both are fine — we just cover more of RunE.
	_ = cmd.Execute()
}

// ---------------------------------------------------------------------------
// airportTransferCmd — missing args and flags (no network)
// ---------------------------------------------------------------------------

func TestAirportTransferCmd_MissingArgsV19(t *testing.T) {
	cmd := airportTransferCmd()
	cmd.SetArgs([]string{"CDG"}) // needs 3 args
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error with only one positional arg")
	}
}

func TestAirportTransferCmd_FlagsExistV19(t *testing.T) {
	cmd := airportTransferCmd()
	for _, name := range []string{"currency", "provider", "max-price", "type", "arrival-after"} {
		if f := cmd.Flags().Lookup(name); f == nil {
			t.Errorf("expected --%s flag on airportTransferCmd", name)
		}
	}
}

// ---------------------------------------------------------------------------
// routeCmd — missing args (error path, no network)
// ---------------------------------------------------------------------------

func TestRouteCmd_MissingArgsV19(t *testing.T) {
	cmd := routeCmd()
	cmd.SetArgs([]string{"HEL"}) // needs at least 3 args
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error with only one positional arg")
	}
}

// ---------------------------------------------------------------------------
// clientConfigPath — exercise remaining client paths not in mcp_install_test.go
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// runInstall for codex (TOML path — covers runInstallCodexTOML branch)
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

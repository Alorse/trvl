package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikkoParkkola/trvl/internal/destinations"
	"github.com/MikkoParkkola/trvl/internal/models"
)

// ---------------------------------------------------------------------------
// shouldShowNudge — pure logic, no I/O (not yet covered variants)
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// loadNudgeState / saveNudgeState — disk round-trip
// ---------------------------------------------------------------------------

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
	// Sub-directory tests MkdirAll inside saveNudgeState.
	path := filepath.Join(dir, "sub", "nudge.json")

	want := nudgeState{SearchCount: 3, Shown: true, ShownAt: time.Now()}
	saveNudgeState(path, want)
	got := loadNudgeState(path)

	if !got.Shown || got.SearchCount != 3 {
		t.Errorf("expected Shown=true SearchCount=3, got %+v", got)
	}
}

// ---------------------------------------------------------------------------
// clientConfigPath — pure switch logic (new variants not yet covered)
// ---------------------------------------------------------------------------

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
// mcpConfigKey — pure switch
// ---------------------------------------------------------------------------

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
	// Dry-run should NOT create the file.
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
	// Old value should be preserved (no force).
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
// formatDestinationCard — pure output, no network
// ---------------------------------------------------------------------------

func TestFormatDestinationCard_Empty_V28(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = old }()

	err := formatDestinationCard(&models.DestinationInfo{Location: "Tokyo"})
	w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "Tokyo") {
		t.Error("expected location in output")
	}
}

func TestFormatDestinationCard_FullInfo_V28(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = old }()

	info := &models.DestinationInfo{
		Location: "Paris",
		Timezone: "Europe/Paris",
		Country: models.CountryInfo{
			Name:       "France",
			Code:       "FR",
			Region:     "Western Europe",
			Capital:    "Paris",
			Languages:  []string{"French"},
			Currencies: []string{"EUR"},
		},
		Weather: models.WeatherInfo{
			Forecast: []models.WeatherDay{
				{Date: "2026-07-01", TempHigh: 28, TempLow: 18, Precipitation: 2.5, Description: "Sunny"},
			},
		},
		Holidays: []models.Holiday{
			{Date: "2026-07-14", Name: "Bastille Day", Type: "National"},
		},
		Safety: models.SafetyInfo{
			Level:       2.5,
			Advisory:    "Exercise normal precautions",
			Source:      "State Dept",
			LastUpdated: "2026-01-01",
		},
		Currency: models.CurrencyInfo{
			BaseCurrency:  "USD",
			LocalCurrency: "EUR",
			ExchangeRate:  0.92,
		},
	}
	err := formatDestinationCard(info)
	w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "France") {
		t.Error("expected country in output")
	}
	if !strings.Contains(out, "Bastille Day") {
		t.Error("expected holiday in output")
	}
	if !strings.Contains(out, "CURRENCY") {
		t.Error("expected currency section in output")
	}
}

// ---------------------------------------------------------------------------
// formatGuideCard / printWrapped — pure output
// ---------------------------------------------------------------------------

func TestFormatGuideCard_Basic_V28(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = old }()

	guide := &models.WikivoyageGuide{
		Location: "Barcelona",
		URL:      "https://en.wikivoyage.org/wiki/Barcelona",
		Summary:  "Great city on the Mediterranean.",
		Sections: map[string]string{
			"See":    "Sagrada Família, Park Güell",
			"Get in": "By air: El Prat airport.",
			"Custom": "Some extra info.",
		},
	}
	err := formatGuideCard(guide)
	w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Barcelona") {
		t.Error("expected location in output")
	}
	if !strings.Contains(out, "Sagrada") {
		t.Error("expected See section content")
	}
	if !strings.Contains(out, "Custom") {
		t.Error("expected custom section in output")
	}
}

func TestFormatGuideCard_EmptySections_V28(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = old }()

	guide := &models.WikivoyageGuide{
		Location: "Nowhere",
		URL:      "https://en.wikivoyage.org/wiki/Nowhere",
		Sections: map[string]string{
			"See": "", // empty — should be skipped
		},
	}
	_ = formatGuideCard(guide)
	w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	if !strings.Contains(buf.String(), "Nowhere") {
		t.Error("expected location in output")
	}
}

// ---------------------------------------------------------------------------
// formatNearbyCard — pure output (new variants)
// ---------------------------------------------------------------------------

func TestFormatNearbyCard_Empty_V28(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = old }()

	_ = formatNearbyCard(&destinations.NearbyResult{})
	w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	if !strings.Contains(buf.String(), "No nearby") {
		t.Error("expected 'No nearby' message")
	}
}

func TestFormatNearbyCard_WithPOIs_V28(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = old }()

	result := &destinations.NearbyResult{
		POIs: []models.NearbyPOI{
			{Name: "La Boqueria", Type: "market", Distance: 200, Cuisine: "", Hours: "8:00-20:00"},
		},
		RatedPlaces: []models.RatedPlace{
			{Name: "Bar El Xampanyet", Rating: 8.5, Category: "bar", PriceLevel: 2, Distance: 300},
		},
		Attractions: []models.Attraction{
			{Name: "Sagrada Familia", Kind: "church", Distance: 500},
		},
	}
	_ = formatNearbyCard(result)
	w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	out := buf.String()
	if !strings.Contains(out, "La Boqueria") {
		t.Error("expected POI in output")
	}
	if !strings.Contains(out, "Bar El Xampanyet") {
		t.Error("expected rated place in output")
	}
	if !strings.Contains(out, "Sagrada Familia") {
		t.Error("expected attraction in output")
	}
}

// ---------------------------------------------------------------------------
// watchDaemon — pure logic paths without network
// ---------------------------------------------------------------------------

func TestRunWatchDaemon_ZeroInterval_V28(t *testing.T) {
	ctx := context.Background()
	var buf bytes.Buffer
	err := runWatchDaemon(ctx, &buf, 0, false, func(context.Context) (int, error) { return 0, nil }, nil)
	if err == nil {
		t.Error("expected error for zero interval")
	}
	if !strings.Contains(err.Error(), "greater than zero") {
		t.Errorf("expected 'greater than zero' error, got: %v", err)
	}
}

func TestRunWatchDaemon_NilCycleFunc_V28(t *testing.T) {
	ctx := context.Background()
	var buf bytes.Buffer
	err := runWatchDaemon(ctx, &buf, time.Minute, false, nil, nil)
	if err == nil {
		t.Error("expected error for nil cycle func")
	}
}

func TestRunWatchDaemon_CancelledCtx_NoRunNow_V28(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var buf bytes.Buffer
	called := false
	err := runWatchDaemon(ctx, &buf, time.Minute, false, func(context.Context) (int, error) {
		called = true
		return 1, nil
	}, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Error("cycle func should not be called when runNow=false and ctx is already cancelled")
	}
	if !strings.Contains(buf.String(), "stopped") {
		t.Errorf("expected 'stopped' in output, got: %q", buf.String())
	}
}

func TestRunWatchDaemon_RunNow_CancelAfterFirst_V28(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var buf bytes.Buffer
	called := false

	err := runWatchDaemon(ctx, &buf, time.Minute, true, func(ctx2 context.Context) (int, error) {
		called = true
		cancel() // cancel after first run so daemon stops
		return 1, nil
	}, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("cycle func should be called when runNow=true")
	}
}

func TestRunWatchDaemon_CycleError_V28(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var buf bytes.Buffer
	ticker := &fakeDaemonTickerV28{ch: make(chan time.Time, 1)}
	ticker.ch <- time.Now() // trigger one tick

	_ = runWatchDaemon(ctx, &buf, time.Minute, false, func(ctx2 context.Context) (int, error) {
		cancel() // stop after first scheduled run
		return 0, io.ErrUnexpectedEOF
	}, func(d time.Duration) watchDaemonTicker {
		return ticker
	})

	if !strings.Contains(buf.String(), "watch check failed") {
		t.Errorf("expected 'watch check failed' in output, got: %q", buf.String())
	}
}

func TestRunWatchDaemon_ZeroWatchesMessage_V28(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var buf bytes.Buffer
	ticker := &fakeDaemonTickerV28{ch: make(chan time.Time, 1)}
	ticker.ch <- time.Now()

	_ = runWatchDaemon(ctx, &buf, time.Minute, false, func(ctx2 context.Context) (int, error) {
		cancel()
		return 0, nil // zero watches
	}, func(d time.Duration) watchDaemonTicker {
		return ticker
	})

	if !strings.Contains(buf.String(), "no active watches") {
		t.Errorf("expected 'no active watches' in output, got: %q", buf.String())
	}
}

// fakeDaemonTickerV28 is a test double for watchDaemonTicker.
type fakeDaemonTickerV28 struct {
	ch chan time.Time
}

func (f *fakeDaemonTickerV28) Chan() <-chan time.Time { return f.ch }
func (f *fakeDaemonTickerV28) Stop()                 {}

// ---------------------------------------------------------------------------
// destinationCmd — cancelled context (exercises runDestination path)
// ---------------------------------------------------------------------------

func TestDestinationCmd_CancelledCtx_V28(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cmd := destinationCmd()
	cmd.SetArgs([]string{"Tokyo"})
	_ = cmd.ExecuteContext(ctx)
}

func TestDestinationCmd_WithDates_CancelledCtx_V28(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cmd := destinationCmd()
	cmd.SetArgs([]string{"Barcelona", "--dates", "2026-08-01,2026-08-08"})
	_ = cmd.ExecuteContext(ctx)
}

func TestDestinationCmd_SingleDate_V28(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cmd := destinationCmd()
	cmd.SetArgs([]string{"Paris", "--dates", "2026-08-01"})
	_ = cmd.ExecuteContext(ctx)
}

// ---------------------------------------------------------------------------
// guideCmd — cancelled context
// ---------------------------------------------------------------------------

func TestGuideCmd_CancelledCtx_V28(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cmd := guideCmd()
	cmd.SetArgs([]string{"Rome"})
	_ = cmd.ExecuteContext(ctx)
}

// ---------------------------------------------------------------------------
// eventsCmd — no API key path (returns early with an error)
// ---------------------------------------------------------------------------

func TestEventsCmd_NoAPIKey_V28(t *testing.T) {
	orig := os.Getenv("TICKETMASTER_API_KEY")
	_ = os.Unsetenv("TICKETMASTER_API_KEY")
	defer func() {
		if orig != "" {
			_ = os.Setenv("TICKETMASTER_API_KEY", orig)
		}
	}()

	cmd := eventsCmd()
	cmd.SetArgs([]string{"Barcelona", "--from", "2026-07-01", "--to", "2026-07-08"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when TICKETMASTER_API_KEY is not set")
	}
	if !strings.Contains(err.Error(), "TICKETMASTER_API_KEY") {
		t.Errorf("expected TICKETMASTER_API_KEY error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// nearbyCmd — invalid coordinates (returns early, pure validation)
// ---------------------------------------------------------------------------

func TestNearbyCmd_InvalidLat_V28(t *testing.T) {
	cmd := nearbyCmd()
	cmd.SetArgs([]string{"notanum", "2.17"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid latitude")
	}
}

func TestNearbyCmd_InvalidLon_V28(t *testing.T) {
	cmd := nearbyCmd()
	cmd.SetArgs([]string{"41.38", "notanum"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid longitude")
	}
}

// ---------------------------------------------------------------------------
// airportTransferCmd — cancelled context
// ---------------------------------------------------------------------------

func TestAirportTransferCmd_CancelledCtx_V28(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cmd := airportTransferCmd()
	cmd.SetArgs([]string{"HEL", "2026-08-01"})
	_ = cmd.ExecuteContext(ctx)
}

// ---------------------------------------------------------------------------
// upgradeCmd — cancelled context
// ---------------------------------------------------------------------------

func TestUpgradeCmd_CancelledCtx_V28(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cmd := upgradeCmd()
	cmd.SetArgs([]string{})
	_ = cmd.ExecuteContext(ctx)
}

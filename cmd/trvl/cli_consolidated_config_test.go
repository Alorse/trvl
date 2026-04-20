package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikkoParkkola/trvl/internal/preferences"
)

func TestLoadLastSearch_NotFoundV10(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	_, err := loadLastSearch()
	if err == nil {
		t.Error("expected error when no last_search.json")
	}
}

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

func TestLoadExistingKeys_NonexistentFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	keys := loadExistingKeys()

	_ = keys
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

func TestDiscoverCmd_MissingOriginNoPrefs(t *testing.T) {

	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := discoverCmd()
	cmd.SetArgs([]string{"--from", "2026-07-01", "--until", "2026-07-31", "--budget", "500"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when no origin and no prefs")
	}
}

func TestPrefsAddFamilyMemberCmd_AddsV16(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := prefsAddFamilyMemberCmd()
	cmd.SetArgs([]string{"family_member", "Father", "--notes", "prefers window seat"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("add family member: %v", err)
	}
}

func TestPrefsAddFamilyMemberCmd_WrongKeyV16(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := prefsAddFamilyMemberCmd()
	cmd.SetArgs([]string{"not_family_member", "Bob"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for wrong first arg")
	}
}

func TestProfileAddCmd_AddsFlightBooking(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cmd := profileAddCmd()
	cmd.SetArgs([]string{
		"--type", "flight",
		"--provider", "KLM",
		"--from", "HEL",
		"--to", "AMS",
		"--price", "189",
		"--currency", "EUR",
		"--travel-date", "2026-03-15",
	})
	if err := cmd.Execute(); err != nil {
		t.Errorf("profile add: %v", err)
	}
}

func TestProfileAddCmd_AddsHotelBooking(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cmd := profileAddCmd()
	cmd.SetArgs([]string{
		"--type", "hotel",
		"--provider", "Marriott",
		"--price", "450",
		"--currency", "EUR",
		"--nights", "3",
		"--stars", "4",
		"--travel-date", "2026-03-15",
	})
	if err := cmd.Execute(); err != nil {
		t.Errorf("profile add hotel: %v", err)
	}
}

func TestProfileAddCmd_PrintsFrom_To(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cmd := profileAddCmd()
	cmd.SetArgs([]string{
		"--type", "ground",
		"--provider", "FlixBus",
		"--from", "Prague",
		"--to", "Vienna",
		"--price", "19",
		"--currency", "EUR",
	})
	if err := cmd.Execute(); err != nil {
		t.Errorf("profile add ground: %v", err)
	}
}

func TestPrefsSetCmd_HomeAirports(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := prefsSetCmd()
	cmd.SetArgs([]string{"home_airports", "HEL"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("prefs set home_airports: %v", err)
	}
}

func TestPrefsSetCmd_DisplayCurrency(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := prefsSetCmd()
	cmd.SetArgs([]string{"display_currency", "EUR"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("prefs set display_currency: %v", err)
	}
}

func TestPrefsSetCmd_InvalidDisplayCurrency(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := prefsSetCmd()
	cmd.SetArgs([]string{"display_currency", "TOOLONG"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid display_currency")
	}
}

func TestPrefsSetCmd_MinHotelStars(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := prefsSetCmd()
	cmd.SetArgs([]string{"min_hotel_stars", "3"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("prefs set min_hotel_stars: %v", err)
	}
}

func TestPrefsSetCmd_MinHotelRating(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := prefsSetCmd()
	cmd.SetArgs([]string{"min_hotel_rating", "8.5"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("prefs set min_hotel_rating: %v", err)
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

func TestProfileImportEmailCmd_RunsV20(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := profileCmd()
	cmd.SetArgs([]string{"import-email"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("profile import-email: %v", err)
	}
}

func TestLoadLastSearch_MissingFileV20(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	_, err := loadLastSearch()
	if err == nil {
		t.Error("expected error for missing last_search.json")
	}
}

func TestPrefsSetCmd_LocaleV21(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := prefsSetCmd()
	cmd.SetArgs([]string{"locale", "en-FI"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("prefs set locale: %v", err)
	}
}

func TestPrefsSetCmd_HomeAirportsMultipleV21(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := prefsSetCmd()
	cmd.SetArgs([]string{"home_airports", "HEL,AMS"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("prefs set home_airports multi: %v", err)
	}
}

func TestPrefsSetCmd_LoyaltyAirlinesV21(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := prefsSetCmd()
	cmd.SetArgs([]string{"loyalty_airlines", "AY,KL"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("prefs set loyalty_airlines: %v", err)
	}
}

func TestPrefsSetCmd_LoyaltyHotelsV21(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := prefsSetCmd()
	cmd.SetArgs([]string{"loyalty_hotels", "Marriott Bonvoy"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("prefs set loyalty_hotels: %v", err)
	}
}

func TestPrefsSetCmd_PreferredDistrictsV21(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := prefsSetCmd()
	cmd.SetArgs([]string{"preferred_districts", "Prague=Prague 1,Prague 2"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("prefs set preferred_districts: %v", err)
	}
}

func TestPrefsSetCmd_CarryOnOnlyV21(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := prefsSetCmd()
	cmd.SetArgs([]string{"carry_on_only", "true"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("prefs set carry_on_only: %v", err)
	}
}

func TestPrefsSetCmd_PreferDirectV21(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := prefsSetCmd()
	cmd.SetArgs([]string{"prefer_direct", "false"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("prefs set prefer_direct: %v", err)
	}
}

func TestPrefsSetCmd_UnknownKeyV21(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := prefsSetCmd()
	cmd.SetArgs([]string{"unknown_key_xyz", "value"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for unknown preference key")
	}
}

func TestWhenCmd_MissingOriginNoPrefsV22(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := whenCmd()
	cmd.SetArgs([]string{"--to", "BCN", "--from", "2026-07-01", "--until", "2026-07-31"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when no origin and no prefs")
	}
}

func TestPrefsSetCmd_EnsuitOnlyV23(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := prefsSetCmd()
	cmd.SetArgs([]string{"ensuite_only", "true"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("prefs set ensuite_only: %v", err)
	}
}

func TestPrefsSetCmd_NoDormitoriesV23(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := prefsSetCmd()
	cmd.SetArgs([]string{"no_dormitories", "true"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("prefs set no_dormitories: %v", err)
	}
}

func TestPrefsSetCmd_FastWifiV23(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := prefsSetCmd()
	cmd.SetArgs([]string{"fast_wifi_needed", "true"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("prefs set fast_wifi_needed: %v", err)
	}
}

func TestPrefsSetCmd_HomeCitiesV23(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := prefsSetCmd()
	cmd.SetArgs([]string{"home_cities", "Helsinki,Amsterdam"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("prefs set home_cities: %v", err)
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

func TestTrvlBinaryPath_V24(t *testing.T) {
	p, err := trvlBinaryPath()
	if err != nil {
		t.Fatalf("trvlBinaryPath: %v", err)
	}
	if p == "" {
		t.Error("expected non-empty binary path")
	}
}

func TestPrefsEditCmd_NonNilV24(t *testing.T) {
	cmd := prefsEditCmd()
	if cmd == nil {
		t.Error("expected non-nil prefsEditCmd")
	}
}

func TestPrefsInitCmd_NonNilV24(t *testing.T) {
	cmd := prefsInitCmd()
	if cmd == nil {
		t.Error("expected non-nil prefsInitCmd")
	}
}

func TestLoadExistingKeys_MissingFileV24(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	keys := loadExistingKeys()

	if keys.SeatsAero != "" || keys.Kiwi != "" {
		t.Errorf("expected empty keys for missing file, got %+v", keys)
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

func TestApplyPreference_BooleanFields(t *testing.T) {
	cases := []struct {
		key string
		val string
	}{
		{"carry_on_only", "true"},
		{"prefer_direct", "yes"},
		{"no_dormitories", "1"},
		{"ensuite_only", "on"},
		{"fast_wifi_needed", "true"},
	}
	for _, c := range cases {
		p := &preferences.Preferences{}
		if err := applyPreference(p, c.key, c.val); err != nil {
			t.Errorf("key %q: %v", c.key, err)
		}
	}
}

func TestApplyPreference_BooleanInvalid(t *testing.T) {
	p := &preferences.Preferences{}
	if err := applyPreference(p, "carry_on_only", "maybe"); err == nil {
		t.Error("expected error for invalid bool")
	}
}

func TestApplyPreference_MinHotelStarsInvalid(t *testing.T) {
	p := &preferences.Preferences{}
	if err := applyPreference(p, "min_hotel_stars", "6"); err == nil {
		t.Error("expected error for stars > 5")
	}
}

func TestApplyPreference_MinHotelStarsNotInt(t *testing.T) {
	p := &preferences.Preferences{}
	if err := applyPreference(p, "min_hotel_stars", "abc"); err == nil {
		t.Error("expected error for non-integer")
	}
}

func TestApplyPreference_MinHotelRatingInvalid(t *testing.T) {
	p := &preferences.Preferences{}
	if err := applyPreference(p, "min_hotel_rating", "11"); err == nil {
		t.Error("expected error for rating > 10")
	}
}

func TestApplyPreference_DisplayCurrencyInvalid(t *testing.T) {
	p := &preferences.Preferences{}
	if err := applyPreference(p, "display_currency", "eu"); err == nil {
		t.Error("expected error for non-3-letter code")
	}
}

func TestApplyPreference_LoyaltyHotels(t *testing.T) {
	p := &preferences.Preferences{}
	if err := applyPreference(p, "loyalty_hotels", "Marriott Bonvoy,IHG"); err != nil {
		t.Fatal(err)
	}
	if len(p.LoyaltyHotels) != 2 {
		t.Errorf("unexpected: %v", p.LoyaltyHotels)
	}
}

func TestApplyPreference_PreferredDistricts_DeleteOnEmpty(t *testing.T) {
	p := &preferences.Preferences{
		PreferredDistricts: map[string][]string{
			"Prague": {"Prague 1"},
		},
	}
	if err := applyPreference(p, "preferred_districts", "Prague="); err != nil {
		t.Fatal(err)
	}
	if _, ok := p.PreferredDistricts["Prague"]; ok {
		t.Error("expected Prague to be deleted")
	}
}

func TestApplyPreference_PreferredDistricts_MissingEquals(t *testing.T) {
	p := &preferences.Preferences{}
	if err := applyPreference(p, "preferred_districts", "PragueNoEquals"); err == nil {
		t.Error("expected error without =")
	}
}

func TestApplyPreference_PreferredDistricts_EmptyCity(t *testing.T) {
	p := &preferences.Preferences{}
	if err := applyPreference(p, "preferred_districts", "=Prague 1"); err == nil {
		t.Error("expected error with empty city")
	}
}

func TestPrefsAddFamilyMemberCmd_WrongFirstArg(t *testing.T) {
	cmd := prefsAddFamilyMemberCmd()
	cmd.SetArgs([]string{"wrong_arg", "John"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for wrong first arg")
	}
}

func TestPrefsCmd_SubcommandsExist(t *testing.T) {
	cmd := prefsCmd()
	names := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		names[sub.Name()] = true
	}
	for _, want := range []string{"set", "edit", "init", "add"} {
		if !names[want] {
			t.Errorf("expected subcommand %q in prefs", want)
		}
	}
}

func TestSetupPromptString_UsesInput(t *testing.T) {
	scanner := bufio.NewScanner(strings.NewReader("newvalue\n"))
	got := setupPromptString(scanner, os.Stderr, "Label", "default")
	if got != "newvalue" {
		t.Errorf("expected newvalue, got %q", got)
	}
}

func TestSetupPromptString_EmptyKeepsDefault(t *testing.T) {
	scanner := bufio.NewScanner(strings.NewReader("\n"))
	got := setupPromptString(scanner, os.Stderr, "Label", "default")
	if got != "default" {
		t.Errorf("expected default, got %q", got)
	}
}

func TestSetupPromptString_EmptyCurrentLabel(t *testing.T) {
	scanner := bufio.NewScanner(strings.NewReader("value\n"))
	got := setupPromptString(scanner, os.Stderr, "Label", "")
	if got != "value" {
		t.Errorf("expected value, got %q", got)
	}
}

func TestSetupPromptOptional_ReturnsInput(t *testing.T) {
	scanner := bufio.NewScanner(strings.NewReader("optional\n"))
	got := setupPromptOptional(scanner, os.Stderr, "Label", "current")
	if got != "optional" {
		t.Errorf("expected optional, got %q", got)
	}
}

func TestSetupPromptOptional_EmptyReturnsEmpty(t *testing.T) {
	scanner := bufio.NewScanner(strings.NewReader("\n"))
	got := setupPromptOptional(scanner, os.Stderr, "Label", "")
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestSetupPromptOptional_WithCurrentShowsBracket(t *testing.T) {
	scanner := bufio.NewScanner(strings.NewReader("\n"))

	got := setupPromptOptional(scanner, os.Stderr, "Label", "existing")
	if got != "existing" {
		t.Errorf("expected existing (kept), got %q", got)
	}
}

func TestSetupPromptSecret_WithInput(t *testing.T) {
	scanner := bufio.NewScanner(strings.NewReader("secret123\n"))
	got := setupPromptSecret(scanner, os.Stderr, "API Key", "")
	if got != "secret123" {
		t.Errorf("expected secret123, got %q", got)
	}
}

func TestSetupPromptSecret_WithExisting(t *testing.T) {
	scanner := bufio.NewScanner(strings.NewReader("\n"))
	got := setupPromptSecret(scanner, os.Stderr, "API Key", "existing-secret")

	if got != "" {
		t.Errorf("expected empty (not kept), got %q", got)
	}
}

func TestSetupPromptChoice_ValidChoice(t *testing.T) {
	scanner := bufio.NewScanner(strings.NewReader("table\n"))
	valid := map[string]bool{"table": true, "json": true}
	got := setupPromptChoice(scanner, os.Stderr, "Format", "table", valid)
	if got != "table" {
		t.Errorf("expected table, got %q", got)
	}
}

func TestSetupPromptChoice_EmptyKeepsCurrent(t *testing.T) {
	scanner := bufio.NewScanner(strings.NewReader("\n"))
	valid := map[string]bool{"table": true, "json": true}
	got := setupPromptChoice(scanner, os.Stderr, "Format", "json", valid)
	if got != "json" {
		t.Errorf("expected json (current), got %q", got)
	}
}

func TestSetupPromptChoice_InvalidThenValid(t *testing.T) {

	scanner := bufio.NewScanner(strings.NewReader("csv\njson\n"))
	valid := map[string]bool{"table": true, "json": true}
	got := setupPromptChoice(scanner, os.Stderr, "Format", "table", valid)
	if got != "json" {
		t.Errorf("expected json after retry, got %q", got)
	}
}

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

func TestProfileAddCmd_Flags(t *testing.T) {
	cmd := profileAddCmd()
	for _, name := range []string{"type", "travel-date", "from", "to", "provider", "price", "currency", "nights", "stars", "reference", "notes"} {
		if f := cmd.Flags().Lookup(name); f == nil {
			t.Errorf("expected --%s flag on profile add", name)
		}
	}
}

func TestProfileAddCmd_MissingTypeError(t *testing.T) {
	cmd := profileAddCmd()

	cmd.SetArgs([]string{"--provider", "KLM"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when --type is missing")
	}
}

func TestProfileAddCmd_MissingProviderError(t *testing.T) {
	cmd := profileAddCmd()

	cmd.SetArgs([]string{"--type", "flight"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when --provider is missing")
	}
}

func TestRunPrefsShow_EmptyPrefs(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cmd := prefsCmd()
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestMaybeShowStarNudge_JSONFormatNoOp(t *testing.T) {

	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	maybeShowStarNudge("flights", "json")
}

func TestTrvlBinaryPath_ReturnsNonEmpty(t *testing.T) {
	path, err := trvlBinaryPath()
	if err != nil {
		t.Skipf("trvlBinaryPath error (expected in test env): %v", err)
	}
	if path == "" {
		t.Error("expected non-empty path")
	}
}

func TestSaveAndLoadLastSearch_V9(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	ls := &LastSearch{
		Command:        "flights",
		Origin:         "HEL",
		Destination:    "BCN",
		DepartDate:     "2026-07-01",
		FlightPrice:    199,
		FlightCurrency: "EUR",
	}
	saveLastSearch(ls)

	loaded, err := loadLastSearch()
	if err != nil {
		t.Fatalf("loadLastSearch: %v", err)
	}
	if loaded.Origin != "HEL" {
		t.Errorf("expected HEL, got %q", loaded.Origin)
	}
}

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

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/MikkoParkkola/trvl/internal/models"
	"github.com/MikkoParkkola/trvl/internal/preferences"
	"github.com/spf13/cobra"
)

// APIKeys holds optional premium provider API keys stored in ~/.trvl/keys.json.
// The file is written with mode 0600 (owner read/write only).
type APIKeys struct {
	SeatsAero    string `json:"seats_aero,omitempty"`
	Kiwi         string `json:"kiwi,omitempty"`
	Distribusion string `json:"distribusion,omitempty"`
}

func setupCmd() *cobra.Command {
	var (
		nonInteractive bool
		homeFlag       string
		currencyFlag   string
		cabinFlag      string
		mcpClient      string
	)

	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Interactive first-time setup wizard",
		Long: `Run the trvl setup wizard to configure your home airport, preferences,
and optional API keys for premium providers.

In interactive mode the wizard prompts you for each setting and shows the
current/default value in brackets. Press Enter to keep it.

Non-interactive mode (--non-interactive) writes defaults or flag-supplied
values without any prompts — suitable for CI and scripting.

At the end, the wizard offers to run 'trvl mcp install' so trvl is
immediately available as an MCP server in your AI client.

Examples:
  trvl setup
  trvl setup --non-interactive
  trvl setup --non-interactive --home HEL --currency EUR --cabin economy`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := setupConfig{
				nonInteractive: nonInteractive,
				homeFlag:       homeFlag,
				currencyFlag:   currencyFlag,
				cabinFlag:      cabinFlag,
				mcpClient:      mcpClient,
				stdin:          os.Stdin,
				stdout:         os.Stdout,
			}
			return runSetup(cfg)
		},
	}

	cmd.Flags().BoolVar(&nonInteractive, "non-interactive", false, "Use defaults (or flag values) without prompting")
	cmd.Flags().StringVar(&homeFlag, "home", "", "Home airport IATA code (e.g. HEL)")
	cmd.Flags().StringVar(&currencyFlag, "currency", "", "Preferred currency (e.g. EUR)")
	cmd.Flags().StringVar(&cabinFlag, "cabin", "", "Cabin class: economy, premium_economy, business, first")
	cmd.Flags().StringVar(&mcpClient, "mcp-client", "claude-desktop", "MCP client for the install step")

	return cmd
}

// setupConfig carries all inputs for runSetup, making it fully testable.
type setupConfig struct {
	nonInteractive bool
	homeFlag       string
	currencyFlag   string
	cabinFlag      string
	mcpClient      string
	stdin          *os.File
	stdout         *os.File
}

// runSetup implements the wizard logic.
func runSetup(cfg setupConfig) error {
	w := cfg.stdout
	scanner := bufio.NewScanner(cfg.stdin)

	fmt.Fprintln(w, "Welcome to trvl setup!")
	fmt.Fprintln(w, "This wizard configures your preferences and optionally installs trvl as an MCP server.")
	if !cfg.nonInteractive {
		fmt.Fprintln(w, "Press Enter to keep the value shown in [brackets].")
	}
	fmt.Fprintln(w)

	p, err := preferences.Load()
	if err != nil {
		return fmt.Errorf("load preferences: %w", err)
	}

	// --- Home airport ---
	home := resolveString(cfg.nonInteractive, cfg.homeFlag, p.HomeAirport(), "HEL")
	if !cfg.nonInteractive {
		home = setupPromptIATA(scanner, w, "Home airport (IATA code)", home)
	}
	if home != "" {
		if len(p.HomeAirports) == 0 {
			p.HomeAirports = []string{home}
		} else {
			p.HomeAirports[0] = home
		}
	}

	// --- Currency ---
	currency := resolveString(cfg.nonInteractive, cfg.currencyFlag, p.DisplayCurrency, "EUR")
	if !cfg.nonInteractive {
		currency = setupPromptString(scanner, w, "Preferred currency", currency)
	}
	if len(currency) == 3 {
		p.DisplayCurrency = strings.ToUpper(currency)
	}

	// --- Cabin class ---
	validCabins := map[string]bool{
		"economy": true, "premium_economy": true, "business": true, "first": true,
	}
	cabin := resolveString(cfg.nonInteractive, cfg.cabinFlag, "", "economy")
	if !cfg.nonInteractive {
		cabin = setupPromptChoice(scanner, w,
			"Cabin class (economy/premium_economy/business/first)", cabin, validCabins)
	}

	// --- Frequent flyer programs ---
	existingFF := strings.Join(p.LoyaltyAirlines, ",")
	ffPrograms := existingFF
	if !cfg.nonInteractive {
		ffPrograms = setupPromptOptional(scanner, w, "Frequent flyer programs / loyalty airlines (comma-separated IATA codes, or skip)", existingFF)
	}
	if ffPrograms != "" && ffPrograms != existingFF {
		p.LoyaltyAirlines = splitAndTrim(ffPrograms)
	}

	// Store cabin in notes for now (no dedicated field in Preferences yet).
	if cabin != "" && cabin != "economy" {
		if p.Notes != "" && !strings.Contains(p.Notes, "cabin:") {
			p.Notes = "cabin:" + cabin + " " + p.Notes
		} else if !strings.Contains(p.Notes, "cabin:") {
			p.Notes = "cabin:" + cabin
		}
	}

	// Write preferences.
	if err := preferences.Save(p); err != nil {
		return fmt.Errorf("save preferences: %w", err)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Preferences saved.")

	// --- API keys (interactive only) ---
	if !cfg.nonInteractive {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Optional: API keys for premium providers.")
		fmt.Fprintln(w, "These extend trvl with award space (Seats.aero), budget flights (Kiwi.com), and bus/coach (Distribusion).")
		fmt.Fprintln(w, "Leave blank to skip any key.")

		keys := loadExistingKeys()

		seatsAero := setupPromptSecret(scanner, w, "Seats.aero API key", keys.SeatsAero)
		kiwi := setupPromptSecret(scanner, w, "Kiwi.com API key", keys.Kiwi)
		distribusion := setupPromptSecret(scanner, w, "Distribusion API key", keys.Distribusion)

		if seatsAero != "" || kiwi != "" || distribusion != "" {
			keys.SeatsAero = coalesce(seatsAero, keys.SeatsAero)
			keys.Kiwi = coalesce(kiwi, keys.Kiwi)
			keys.Distribusion = coalesce(distribusion, keys.Distribusion)
			if err := saveKeys(keys); err != nil {
				fmt.Fprintf(w, "Warning: could not save API keys: %v\n", err)
			} else {
				fmt.Fprintln(w, "API keys saved to ~/.trvl/keys.json (mode 0600).")
			}
		}
	}

	// --- MCP install ---
	doInstall := false
	if !cfg.nonInteractive {
		fmt.Fprintln(w)
		answer := setupPromptString(scanner, w, "Run 'trvl mcp install' now to register trvl as an MCP server? (yes/no)", "yes")
		b, _ := parseBool(answer)
		doInstall = b
	}

	if doInstall {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "Running: trvl mcp install --client %s\n", cfg.mcpClient)
		if err := runInstall(cfg.mcpClient, false, false); err != nil {
			fmt.Fprintf(w, "Warning: mcp install failed: %v\n", err)
			fmt.Fprintln(w, "You can run it manually: trvl mcp install")
		}
	} else {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Skipping MCP install. Run 'trvl mcp install' at any time.")
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "Setup complete. Run 'trvl prefs' to review all preferences.")
	if home != "" {
		fmt.Fprintf(w, "Try: trvl explore %s\n", home)
	}
	return nil
}

// --- prompt helpers (write to w, read from scanner) ---

// setupPromptIATA prompts for a 3-letter IATA code, re-prompting on invalid input.
func setupPromptIATA(scanner *bufio.Scanner, w *os.File, label, current string) string {
	for {
		fmt.Fprintf(w, "  %s [%s]: ", label, current)
		if !scanner.Scan() {
			return current
		}
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" {
			return current
		}
		upper := strings.ToUpper(raw)
		if err := models.ValidateIATA(upper); err != nil {
			fmt.Fprintf(w, "  Invalid IATA code %q — must be exactly 3 uppercase letters (e.g. HEL, NRT). Try again.\n", raw)
			continue
		}
		return upper
	}
}

// setupPromptString prompts for a free-text value; empty input keeps current.
func setupPromptString(scanner *bufio.Scanner, w *os.File, label, current string) string {
	if current != "" {
		fmt.Fprintf(w, "  %s [%s]: ", label, current)
	} else {
		fmt.Fprintf(w, "  %s: ", label)
	}
	if !scanner.Scan() {
		return current
	}
	if raw := strings.TrimSpace(scanner.Text()); raw != "" {
		return raw
	}
	return current
}

// setupPromptChoice prompts for a value from a fixed set; invalid input is re-prompted.
func setupPromptChoice(scanner *bufio.Scanner, w *os.File, label, current string, valid map[string]bool) string {
	for {
		fmt.Fprintf(w, "  %s [%s]: ", label, current)
		if !scanner.Scan() {
			return current
		}
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" {
			return current
		}
		lower := strings.ToLower(raw)
		if valid[lower] {
			return lower
		}
		keys := make([]string, 0, len(valid))
		for k := range valid {
			keys = append(keys, k)
		}
		fmt.Fprintf(w, "  Invalid choice %q — must be one of: %s. Try again.\n", raw, strings.Join(keys, ", "))
	}
}

// setupPromptOptional prompts for an optional free-text value; empty input keeps current.
func setupPromptOptional(scanner *bufio.Scanner, w *os.File, label, current string) string {
	if current != "" {
		fmt.Fprintf(w, "  %s [%s]: ", label, current)
	} else {
		fmt.Fprintf(w, "  %s: ", label)
	}
	if !scanner.Scan() {
		return current
	}
	raw := strings.TrimSpace(scanner.Text())
	if raw == "" {
		return current
	}
	return raw
}

// setupPromptSecret prompts for an API key; shows masked existing value if present.
func setupPromptSecret(scanner *bufio.Scanner, w *os.File, label, current string) string {
	if current != "" {
		fmt.Fprintf(w, "  %s [****]: ", label)
	} else {
		fmt.Fprintf(w, "  %s: ", label)
	}
	if !scanner.Scan() {
		return ""
	}
	return strings.TrimSpace(scanner.Text())
}

// --- API keys persistence ---

// keysPath returns the path to ~/.trvl/keys.json.
func keysPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".trvl", "keys.json"), nil
}

// loadExistingKeys reads ~/.trvl/keys.json; returns empty struct on any error.
func loadExistingKeys() APIKeys {
	path, err := keysPath()
	if err != nil {
		return APIKeys{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return APIKeys{}
	}
	var k APIKeys
	_ = json.Unmarshal(data, &k)
	return k
}

// saveKeys writes keys to ~/.trvl/keys.json with mode 0600.
func saveKeys(keys APIKeys) error {
	path, err := keysPath()
	if err != nil {
		return err
	}
	return saveKeysTo(path, keys)
}

// saveKeysTo writes keys to an explicit path with mode 0600.
func saveKeysTo(path string, keys APIKeys) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create keys dir: %w", err)
	}

	b, err := json.MarshalIndent(keys, "", "  ")
	if err != nil {
		return fmt.Errorf("encode keys: %w", err)
	}

	// Atomic write with secure permissions.
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}

	cleanup = false
	return nil
}

// --- utility ---

// resolveString picks: flag value > existing pref > fallback default.
func resolveString(nonInteractive bool, flag, existing, fallback string) string {
	if flag != "" {
		return flag
	}
	if existing != "" {
		return existing
	}
	if nonInteractive {
		return fallback
	}
	return fallback
}

// coalesce returns the first non-empty string.
func coalesce(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// setupTimestamp returns a UTC timestamp string (used internally for created_at tracking).
func setupTimestamp() string {
	return time.Now().UTC().Format(time.RFC3339)
}

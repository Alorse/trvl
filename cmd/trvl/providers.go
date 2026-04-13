package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/MikkoParkkola/trvl/internal/models"
	"github.com/MikkoParkkola/trvl/internal/providers"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func providersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "providers",
		Short: "Manage external data providers",
		Long: `View, enable, disable, and monitor external data providers.

Providers are optional integrations that extend trvl with additional data
sources (e.g. Kiwi.com flights, Booking.com hotels). Each provider config
is stored in ~/.trvl/providers/<id>.json.

Examples:
  trvl providers list
  trvl providers enable kiwi --accept-tos
  trvl providers disable kiwi
  trvl providers status`,
	}

	cmd.AddCommand(providersListCmd())
	cmd.AddCommand(providersEnableCmd())
	cmd.AddCommand(providersDisableCmd())
	cmd.AddCommand(providersStatusCmd())

	return cmd
}

func init() {
	rootCmd.AddCommand(providersCmd())
}

// --- list ---

func providersListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all configured providers",
		Long: `List all configured providers from ~/.trvl/providers/*.json.
Shows ID, name, category, domain, status, and last success time.

Examples:
  trvl providers list
  trvl providers list --format json`,
		RunE: runProvidersList,
	}
}

func runProvidersList(cmd *cobra.Command, _ []string) error {
	reg, err := providers.NewRegistry()
	if err != nil {
		return fmt.Errorf("list providers: %w", err)
	}
	configs := reg.List()

	f, _ := cmd.Flags().GetString("format")
	if f == "json" {
		return models.FormatJSON(os.Stdout, configs)
	}

	if len(configs) == 0 {
		fmt.Println("No providers configured.")
		fmt.Println("Run 'trvl providers enable <id>' to add one.")
		return nil
	}

	headers := []string{"ID", "Name", "Category", "Domain", "Status", "Last Success"}
	rows := make([][]string, 0, len(configs))
	for _, cfg := range configs {
		status := cfg.Status()
		switch status {
		case "ok":
			status = models.Green(status)
		case "error":
			status = models.Red(status)
		default:
			status = models.Dim(status)
		}

		lastSuccess := ""
		if !cfg.LastSuccess.IsZero() {
			lastSuccess = cfg.LastSuccess.Format("2006-01-02 15:04")
		}

		rows = append(rows, []string{
			cfg.ID,
			cfg.Name,
			cfg.Category,
			cfg.EndpointDomain(),
			status,
			lastSuccess,
		})
	}

	models.FormatTable(os.Stdout, headers, rows)
	return nil
}

// --- enable ---

func providersEnableCmd() *cobra.Command {
	var acceptTOS bool

	cmd := &cobra.Command{
		Use:   "enable <id>",
		Short: "Enable an external data provider",
		Long: `Enable an external data provider by reading its configuration from stdin (JSON)
or interactively confirming terms of service.

The --accept-tos flag bypasses the interactive confirmation prompt, which is
useful for scripted/non-interactive environments.

Examples:
  echo '{"id":"kiwi","name":"Kiwi.com","endpoint":"https://api.tequila.kiwi.com"}' | trvl providers enable kiwi --accept-tos
  trvl providers enable kiwi`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProvidersEnable(args[0], acceptTOS)
		},
	}

	cmd.Flags().BoolVar(&acceptTOS, "accept-tos", false, "Accept terms of service without interactive prompt")

	return cmd
}

func runProvidersEnable(id string, acceptTOS bool) error {
	reg, err := providers.NewRegistry()
	if err != nil {
		return fmt.Errorf("enable provider: %w", err)
	}

	// Try to read provider config from stdin if piped.
	var cfg providers.ProviderConfig
	cfg.ID = id

	if !term.IsTerminal(int(os.Stdin.Fd())) {
		// Stdin is piped — read JSON config.
		dec := json.NewDecoder(os.Stdin)
		if err := dec.Decode(&cfg); err != nil {
			return fmt.Errorf("parse provider config from stdin: %w", err)
		}
		// Ensure ID matches the argument.
		if cfg.ID == "" {
			cfg.ID = id
		}
	} else {
		// Interactive mode — check if provider already exists.
		if existing := reg.Get(id); existing != nil {
			cfg = *existing
		} else {
			// No existing config; prompt for basic info.
			scanner := bufio.NewScanner(os.Stdin)
			fmt.Fprintf(os.Stderr, "  Name: ")
			if scanner.Scan() {
				cfg.Name = strings.TrimSpace(scanner.Text())
			}
			fmt.Fprintf(os.Stderr, "  Endpoint URL: ")
			if scanner.Scan() {
				cfg.Endpoint = strings.TrimSpace(scanner.Text())
			}
			fmt.Fprintf(os.Stderr, "  Category (flights/hotels/ground): ")
			if scanner.Scan() {
				cfg.Category = strings.TrimSpace(scanner.Text())
			}
		}
	}

	if cfg.Name == "" {
		cfg.Name = id
	}

	// Show consent prompt unless --accept-tos is set.
	if !acceptTOS {
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			return fmt.Errorf("interactive confirmation required; use --accept-tos for non-interactive mode")
		}

		domain := cfg.EndpointDomain()
		if domain == "" {
			domain = "(unknown)"
		}

		fmt.Fprintf(os.Stderr, "\nProvider: %s\n", cfg.Name)
		fmt.Fprintf(os.Stderr, "Accesses: %s\n", domain)
		fmt.Fprintf(os.Stderr, "\nThis service may restrict automated access.\n")
		fmt.Fprintf(os.Stderr, "By enabling, you accept responsibility for ToS compliance.\n\n")
		fmt.Fprintf(os.Stderr, "Enable? [y/N]: ")

		scanner := bufio.NewScanner(os.Stdin)
		if !scanner.Scan() {
			return fmt.Errorf("enable cancelled")
		}
		answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
		if answer != "y" && answer != "yes" {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	cfg.Consent = &providers.ConsentRecord{
		Granted:   true,
		Timestamp: time.Now(),
		Domain:    cfg.EndpointDomain(),
	}
	if err := reg.Save(&cfg); err != nil {
		return fmt.Errorf("save provider: %w", err)
	}

	fmt.Printf("Provider %q enabled.\n", cfg.Name)
	return nil
}

// --- disable ---

func providersDisableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "disable <id>",
		Short: "Disable and remove a provider",
		Long: `Disable a provider by removing its configuration file.
Prompts for confirmation unless stdin is not a terminal.

Examples:
  trvl providers disable kiwi`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProvidersDisable(args[0])
		},
	}
}

func runProvidersDisable(id string) error {
	reg, err := providers.NewRegistry()
	if err != nil {
		return fmt.Errorf("disable provider: %w", err)
	}

	cfg := reg.Get(id)
	if cfg == nil {
		return fmt.Errorf("provider %q not found", id)
	}

	// Confirm removal interactively.
	if term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprintf(os.Stderr, "Remove provider %q? [y/N]: ", cfg.Name)
		scanner := bufio.NewScanner(os.Stdin)
		if !scanner.Scan() {
			return fmt.Errorf("disable cancelled")
		}
		answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
		if answer != "y" && answer != "yes" {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	if err := reg.Delete(id); err != nil {
		return fmt.Errorf("remove provider: %w", err)
	}

	fmt.Printf("Provider %q disabled.\n", cfg.Name)
	return nil
}

// --- status ---

func providersStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show health status of all providers",
		Long: `Show the health status of all configured providers.
Displays last success time, error count, and last error message.
Highlights stale providers (no success in 24 hours with errors).

Examples:
  trvl providers status
  trvl providers status --format json`,
		RunE: runProvidersStatus,
	}
}

func runProvidersStatus(cmd *cobra.Command, _ []string) error {
	reg, err := providers.NewRegistry()
	if err != nil {
		return fmt.Errorf("list providers: %w", err)
	}
	configs := reg.List()

	f, _ := cmd.Flags().GetString("format")
	if f == "json" {
		return models.FormatJSON(os.Stdout, configs)
	}

	if len(configs) == 0 {
		fmt.Println("No providers configured.")
		return nil
	}

	headers := []string{"ID", "Status", "Last Success", "Errors", "Last Error"}
	rows := make([][]string, 0, len(configs))
	for _, cfg := range configs {
		status := cfg.Status()
		stale := cfg.IsStale()

		switch {
		case stale:
			status = models.Red("stale")
		case status == "ok":
			status = models.Green(status)
		case status == "error":
			status = models.Red(status)
		default:
			status = models.Dim(status)
		}

		lastSuccess := "-"
		if !cfg.LastSuccess.IsZero() {
			age := time.Since(cfg.LastSuccess).Truncate(time.Minute)
			lastSuccess = fmt.Sprintf("%s (%s ago)", cfg.LastSuccess.Format("2006-01-02 15:04"), age)
		}

		errCount := "-"
		if cfg.ErrorCount > 0 {
			errCount = fmt.Sprintf("%d", cfg.ErrorCount)
		}

		lastErr := "-"
		if cfg.LastError != "" {
			lastErr = cfg.LastError
			if len(lastErr) > 50 {
				lastErr = lastErr[:47] + "..."
			}
		}

		rows = append(rows, []string{cfg.ID, status, lastSuccess, errCount, lastErr})
	}

	models.Banner(os.Stdout, "🔌", "Providers", fmt.Sprintf("%d configured", len(configs)))
	fmt.Println()
	models.FormatTable(os.Stdout, headers, rows)

	// Show stale warning.
	var staleNames []string
	for _, cfg := range configs {
		if cfg.IsStale() {
			staleNames = append(staleNames, cfg.ID)
		}
	}
	if len(staleNames) > 0 {
		fmt.Println()
		fmt.Fprintf(os.Stderr, "  %s Stale: %s (no success in 24h with errors)\n",
			models.Yellow("!"), strings.Join(staleNames, ", "))
	}

	return nil
}

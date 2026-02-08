package sandbox

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// SandboxGetCommand returns the sandbox get subcommand.
func SandboxGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	testerID := fs.String("id", "", "Sandbox tester ID")
	email := fs.String("email", "", "Tester email address")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc sandbox get [flags]",
		ShortHelp:  "Get sandbox tester details.",
		LongHelp: `Get sandbox tester details by ID or email.

Examples:
  asc sandbox get --id "SANDBOX_TESTER_ID"
  asc sandbox get --email "tester@example.com"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if strings.TrimSpace(*testerID) == "" && strings.TrimSpace(*email) == "" {
				fmt.Fprintln(os.Stderr, "Error: --id or --email is required")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*email) != "" {
				if err := validateSandboxEmail(*email); err != nil {
					return fmt.Errorf("sandbox get: %w", err)
				}
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("sandbox get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			var response *asc.SandboxTesterResponse
			if strings.TrimSpace(*testerID) != "" {
				response, err = client.GetSandboxTester(requestCtx, strings.TrimSpace(*testerID))
			} else {
				response, err = findSandboxTesterByEmail(requestCtx, client, strings.TrimSpace(*email))
			}
			if err != nil {
				return fmt.Errorf("sandbox get: %w", err)
			}

			return shared.PrintOutput(response, *output, *pretty)
		},
	}
}

// SandboxUpdateCommand returns the sandbox update subcommand.
func SandboxUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	testerID := fs.String("id", "", "Sandbox tester ID")
	email := fs.String("email", "", "Tester email address")
	territory := fs.String("territory", "", "App Store territory code (e.g., USA, JPN)")
	subscriptionRenewalRate := fs.String("subscription-renewal-rate", "", "Subscription renewal rate (MONTHLY_RENEWAL_EVERY_ONE_HOUR, MONTHLY_RENEWAL_EVERY_THIRTY_MINUTES, MONTHLY_RENEWAL_EVERY_FIFTEEN_MINUTES, MONTHLY_RENEWAL_EVERY_FIVE_MINUTES, MONTHLY_RENEWAL_EVERY_THREE_MINUTES)")
	var interruptPurchases shared.OptionalBool
	fs.Var(&interruptPurchases, "interrupt-purchases", "Interrupt purchases (true/false)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc sandbox update [flags]",
		ShortHelp:  "Update a sandbox tester.",
		LongHelp: `Update sandbox tester settings (v2 API).

Examples:
  asc sandbox update --id "SANDBOX_TESTER_ID" --territory "USA"
  asc sandbox update --email "tester@example.com" --interrupt-purchases
  asc sandbox update --id "SANDBOX_TESTER_ID" --subscription-renewal-rate "MONTHLY_RENEWAL_EVERY_ONE_HOUR"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if strings.TrimSpace(*testerID) == "" && strings.TrimSpace(*email) == "" {
				fmt.Fprintln(os.Stderr, "Error: --id or --email is required")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*email) != "" {
				if err := validateSandboxEmail(*email); err != nil {
					return fmt.Errorf("sandbox update: %w", err)
				}
			}

			normalizedTerritory, err := normalizeSandboxTerritoryFilter(*territory)
			if err != nil {
				return fmt.Errorf("sandbox update: %w", err)
			}
			normalizedRate, err := normalizeSandboxRenewalRate(*subscriptionRenewalRate)
			if err != nil {
				return fmt.Errorf("sandbox update: %w", err)
			}

			if !interruptPurchases.IsSet() && normalizedTerritory == "" && normalizedRate == "" {
				fmt.Fprintln(os.Stderr, "Error: --territory, --interrupt-purchases, or --subscription-renewal-rate is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return err
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resolvedID := strings.TrimSpace(*testerID)
			if resolvedID == "" {
				resolvedID, err = findSandboxTesterIDByEmail(requestCtx, client, strings.TrimSpace(*email))
				if err != nil {
					return fmt.Errorf("sandbox update: %w", err)
				}
			}

			attrs := asc.SandboxTesterUpdateAttributes{}
			if normalizedTerritory != "" {
				territoryValue := normalizedTerritory
				attrs.Territory = &territoryValue
			}
			if interruptPurchases.IsSet() {
				interruptValue := interruptPurchases.Value()
				attrs.InterruptPurchases = &interruptValue
			}
			if normalizedRate != "" {
				rateValue := normalizedRate
				attrs.SubscriptionRenewalRate = &rateValue
			}

			resp, err := client.UpdateSandboxTester(requestCtx, resolvedID, attrs)
			if err != nil {
				if asc.IsNotFound(err) {
					return fmt.Errorf("sandbox update: sandbox tester update is not available via the App Store Connect API for this account")
				}
				return fmt.Errorf("sandbox update: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// SandboxClearHistoryCommand returns the sandbox clear-history subcommand.
func SandboxClearHistoryCommand() *ffcli.Command {
	fs := flag.NewFlagSet("clear-history", flag.ExitOnError)

	testerID := fs.String("id", "", "Sandbox tester ID")
	email := fs.String("email", "", "Tester email address")
	confirm := fs.Bool("confirm", false, "Confirm clearing purchase history")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "clear-history",
		ShortUsage: "asc sandbox clear-history [flags]",
		ShortHelp:  "Clear sandbox tester purchase history.",
		LongHelp: `Clear purchase history for a sandbox tester (v2 API).

Examples:
  asc sandbox clear-history --id "SANDBOX_TESTER_ID" --confirm
  asc sandbox clear-history --email "tester@example.com" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*testerID) == "" && strings.TrimSpace(*email) == "" {
				fmt.Fprintln(os.Stderr, "Error: --id or --email is required")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*email) != "" {
				if err := validateSandboxEmail(*email); err != nil {
					return fmt.Errorf("sandbox clear-history: %w", err)
				}
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return err
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resolvedID := strings.TrimSpace(*testerID)
			if resolvedID == "" {
				resolvedID, err = findSandboxTesterIDByEmail(requestCtx, client, strings.TrimSpace(*email))
				if err != nil {
					return fmt.Errorf("sandbox clear-history: %w", err)
				}
			}

			resp, err := client.ClearSandboxTesterPurchaseHistory(requestCtx, resolvedID)
			if err != nil {
				if asc.IsNotFound(err) {
					return fmt.Errorf("sandbox clear-history: sandbox clear history is not available via the App Store Connect API for this account")
				}
				return fmt.Errorf("sandbox clear-history: %w", err)
			}

			result := &asc.SandboxTesterClearHistoryResult{
				RequestID: resp.Data.ID,
				TesterID:  resolvedID,
				Cleared:   true,
			}

			return shared.PrintOutput(result, *output, *pretty)
		},
	}
}

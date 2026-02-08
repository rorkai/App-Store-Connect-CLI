package iap

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// IAPAvailabilityCommand returns the availability command group.
func IAPAvailabilityCommand() *ffcli.Command {
	fs := flag.NewFlagSet("availability", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "availability",
		ShortUsage: "asc iap availability <subcommand> [flags]",
		ShortHelp:  "Manage in-app purchase availability.",
		LongHelp: `Manage in-app purchase availability.

Examples:
  asc iap availability get --iap-id "IAP_ID"
  asc iap availability set --iap-id "IAP_ID" --territories "USA,CAN"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			IAPAvailabilityGetCommand(),
			IAPAvailabilitySetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// IAPAvailabilityGetCommand returns the availability get subcommand.
func IAPAvailabilityGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("availability get", flag.ExitOnError)

	iapID := fs.String("iap-id", "", "In-app purchase ID")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc iap availability get --iap-id \"IAP_ID\"",
		ShortHelp:  "Get in-app purchase availability.",
		LongHelp: `Get in-app purchase availability.

Examples:
  asc iap availability get --iap-id "IAP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			iapValue := strings.TrimSpace(*iapID)
			if iapValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --iap-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("iap availability get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetInAppPurchaseAvailability(requestCtx, iapValue)
			if err != nil {
				return fmt.Errorf("iap availability get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// IAPAvailabilitySetCommand returns the availability set subcommand.
func IAPAvailabilitySetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("availability set", flag.ExitOnError)

	iapID := fs.String("iap-id", "", "In-app purchase ID")
	territories := fs.String("territories", "", "Territory IDs (comma-separated)")
	availableInNew := fs.Bool("available-in-new-territories", false, "Include new territories automatically")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "set",
		ShortUsage: "asc iap availability set --iap-id \"IAP_ID\" --territories \"USA,CAN\"",
		ShortHelp:  "Set in-app purchase availability in territories.",
		LongHelp: `Set in-app purchase availability in territories.

Examples:
  asc iap availability set --iap-id "IAP_ID" --territories "USA,CAN"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			iapValue := strings.TrimSpace(*iapID)
			if iapValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --iap-id is required")
				return flag.ErrHelp
			}

			territoryIDs := shared.SplitCSVUpper(*territories)
			if len(territoryIDs) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --territories is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("iap availability set: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateInAppPurchaseAvailability(requestCtx, iapValue, *availableInNew, territoryIDs)
			if err != nil {
				return fmt.Errorf("iap availability set: failed to set: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

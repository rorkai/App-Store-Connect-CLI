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

// IAPAvailabilityCommand returns the canonical pricing availability command group.
func IAPAvailabilityCommand() *ffcli.Command {
	fs := flag.NewFlagSet("availability", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "availability",
		ShortUsage: "asc iap pricing availability <subcommand> [flags]",
		ShortHelp:  "Manage in-app purchase availability.",
		LongHelp: `Manage in-app purchase availability.

Examples:
  asc iap pricing availability view --iap-id "IAP_ID"
  asc iap pricing availability set --iap-id "IAP_ID" --territories "US,Canada"`,
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
	fs := flag.NewFlagSet("pricing availability view", flag.ExitOnError)

	appID := addIAPLookupAppFlag(fs)
	iapID := fs.String("iap-id", "", "In-app purchase ID, product ID, or exact current name")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc iap pricing availability view --iap-id \"IAP_ID\"",
		ShortHelp:  "View in-app purchase availability.",
		LongHelp: `View in-app purchase availability.

Examples:
  asc iap pricing availability view --iap-id "IAP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			iapValue := strings.TrimSpace(*iapID)
			if iapValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --iap-id is required")
				return shared.MissingRequiredUsageError()
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("iap pricing availability view: %w", err)
			}

			iapValue, err = resolveIAPLookupIDWithTimeout(ctx, client, *appID, iapValue)
			if err != nil {
				return err
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetInAppPurchaseAvailability(requestCtx, iapValue)
			if err != nil {
				return fmt.Errorf("iap pricing availability view: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// IAPAvailabilitySetCommand returns the availability set subcommand.
func IAPAvailabilitySetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("pricing availability set", flag.ExitOnError)

	appID := addIAPLookupAppFlag(fs)
	iapID := fs.String("iap-id", "", "In-app purchase ID, product ID, or exact current name")
	territories := fs.String("territories", "", "Territory inputs (comma-separated; accepts alpha-2, alpha-3, or exact English country names)")
	availableInNew := fs.Bool("available-in-new-territories", false, "Include new territories automatically")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "set",
		ShortUsage: "asc iap pricing availability set --iap-id \"IAP_ID\" --territories \"US,Canada\"",
		ShortHelp:  "Set in-app purchase availability in territories.",
		LongHelp: `Set in-app purchase availability in territories.

Examples:
  asc iap pricing availability set --iap-id "IAP_ID" --territories "US,Canada"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := shared.RecoverBoolFlagTailArgs(fs, args, availableInNew); err != nil {
				return err
			}

			iapValue := strings.TrimSpace(*iapID)
			if iapValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --iap-id is required")
				return shared.MissingRequiredUsageError()
			}

			territoryIDs, err := shared.NormalizeASCTerritoryCSV(*territories)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			if len(territoryIDs) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --territories is required")
				return shared.MissingRequiredUsageError()
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("iap availability set: %w", err)
			}

			iapValue, err = resolveIAPLookupIDWithTimeout(ctx, client, *appID, iapValue)
			if err != nil {
				return err
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateInAppPurchaseAvailability(requestCtx, iapValue, *availableInNew, territoryIDs)
			if err != nil {
				return fmt.Errorf("iap availability set: failed to set: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

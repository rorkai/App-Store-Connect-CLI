package iap

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newIAPAvailabilityCommand returns the canonical pricing availability command group.
func newIAPAvailabilityCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("availability", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "availability",
		ShortUsage: "asc iap pricing availability <subcommand> [flags]",
		ShortHelp:  "Manage in-app purchase availability.",
		LongHelp: `Manage in-app purchase availability.

Examples:
  asc iap pricing availability view --iap-id "IAP_ID"
  asc iap pricing availability set --iap-id "IAP_ID" --territories "US,Canada"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newIAPAvailabilityGetCommand(d),
			newIAPAvailabilitySetCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newIAPAvailabilityGetCommand returns the availability get subcommand.
func newIAPAvailabilityGetCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("pricing availability view", flag.ExitOnError)

	appID := addIAPLookupAppFlag(fs)
	iapID := shared.BindResourceIDFlag(fs, "iap-id", "inAppPurchases", "In-app purchase ID, product ID, or exact current name")
	include := fs.String("include", "", "Include related resources in JSON: availableTerritories")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc iap pricing availability view --iap-id \"IAP_ID\"",
		ShortHelp:  "View in-app purchase availability.",
		LongHelp: `View in-app purchase availability.

Use --include availableTerritories --output json to see the territories Apple
includes with the availability. For the complete list, pass the availability
ID from data.id to asc iap pricing availabilities available-territories with
--paginate.

Examples:
  asc iap pricing availability view --iap-id "IAP_ID"
  asc iap pricing availability view --iap-id "IAP_ID" --include availableTerritories --output json`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			iapValue := strings.TrimSpace(*iapID)
			if iapValue == "" {
				return d.RequiredFlag("--iap-id")
			}
			if err := shared.ValidateInclude(*include, "availableTerritories"); err != nil {
				return d.UsageError(err.Error())
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("iap pricing availability view: %w", err)
			}

			iapValue, err = resolveIAPLookupIDWithTimeout(ctx, d, client, *appID, iapValue)
			if err != nil {
				return err
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.GetInAppPurchaseAvailability(requestCtx, iapValue,
				asc.WithIAPAvailabilityInclude(shared.SplitUniqueCSV(*include)),
			)
			if err != nil {
				if asc.IsMissingResourceOfType(err, "inAppPurchaseAvailabilities") {
					safeIAPID := asc.SanitizeTerminalText(iapValue)
					fmt.Fprintf(d.IO.ErrOut, "In-app purchase %s has no availability configured yet; create it with: asc iap pricing availability set --iap-id %s --territories \"USA\"\n", safeIAPID, safeIAPID)
					return shared.NewNotConfiguredReportedError(fmt.Errorf("iap pricing availability view: in-app purchase %q has no availability configured", iapValue))
				}
				return fmt.Errorf("iap pricing availability view: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newIAPAvailabilitySetCommand returns the availability set subcommand.
func newIAPAvailabilitySetCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("pricing availability set", flag.ExitOnError)

	appID := addIAPLookupAppFlag(fs)
	iapID := shared.BindResourceIDFlag(fs, "iap-id", "inAppPurchases", "In-app purchase ID, product ID, or exact current name")
	territories := shared.BindOnceCSVFlag(fs, "territories", "Territory inputs (comma-separated; accepts alpha-2, alpha-3, or exact English country names)")
	availableInNew := fs.Bool("available-in-new-territories", false, "Include new territories automatically")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "set",
		ShortUsage: "asc iap pricing availability set --iap-id \"IAP_ID\" --territories \"US,Canada\"",
		ShortHelp:  "Set in-app purchase availability in territories.",
		LongHelp: `Set in-app purchase availability in territories.

Examples:
  asc iap pricing availability set --iap-id "IAP_ID" --territories "US,Canada"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.RecoverBoolFlagTailArgs(fs, args, availableInNew); err != nil {
				return err
			}

			iapValue := strings.TrimSpace(*iapID)
			if iapValue == "" {
				return d.RequiredFlag("--iap-id")
			}

			territoryIDs, err := shared.NormalizeASCTerritoryCSV(territories.String())
			if err != nil {
				return d.UsageError(err.Error())
			}
			if len(territoryIDs) == 0 {
				return d.RequiredFlag("--territories")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("iap availability set: %w", err)
			}

			iapValue, err = resolveIAPLookupIDWithTimeout(ctx, d, client, *appID, iapValue)
			if err != nil {
				return err
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.CreateInAppPurchaseAvailability(requestCtx, iapValue, *availableInNew, territoryIDs)
			if err != nil {
				return fmt.Errorf("iap availability set: failed to set: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

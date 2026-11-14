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

// IAPAvailabilitiesCommand returns the canonical pricing availabilities command group.
func IAPAvailabilitiesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("availabilities", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "availabilities",
		ShortUsage: "asc iap pricing availabilities <subcommand> [flags]",
		ShortHelp:  "Inspect in-app purchase availability records.",
		LongHelp: `Inspect in-app purchase availability records.

Examples:
  asc iap pricing availabilities view --id "AVAILABILITY_ID"
  asc iap pricing availabilities available-territories --id "AVAILABILITY_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			IAPAvailabilitiesGetCommand(),
			IAPAvailabilitiesAvailableTerritoriesCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// IAPAvailabilitiesGetCommand returns the availability get subcommand.
func IAPAvailabilitiesGetCommand() *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		FlagSetName: "pricing availabilities view",
		Name:        "view",
		ShortUsage:  "asc iap pricing availabilities view --id \"AVAILABILITY_ID\"",
		ShortHelp:   "View an in-app purchase availability by ID.",
		LongHelp: `View an in-app purchase availability by ID.

Examples:
  asc iap pricing availabilities view --id "AVAILABILITY_ID"`,
		IDUsage:            "Availability ID",
		IDType:             "inAppPurchaseAvailabilities",
		RequiredDiagnostic: true,
		ErrorPrefix:        "iap availabilities view",
		FetchErrorPrefix:   "iap availabilities view: failed to fetch",
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetInAppPurchaseAvailabilityByID(ctx, id)
		},
	})
}

// IAPAvailabilitiesAvailableTerritoriesCommand returns the available territories subcommand.
func IAPAvailabilitiesAvailableTerritoriesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("pricing availabilities available-territories", flag.ExitOnError)

	availabilityID := shared.BindResourceIDFlag(fs, "id", "inAppPurchaseAvailabilities", "Availability ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "available-territories",
		ShortUsage: "asc iap pricing availabilities available-territories --id \"AVAILABILITY_ID\"",
		ShortHelp:  "List available territories for an availability.",
		LongHelp: `List available territories for an in-app purchase availability.

Examples:
  asc iap pricing availabilities available-territories --id "AVAILABILITY_ID"
  asc iap pricing availabilities available-territories --id "AVAILABILITY_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return shared.UsageError("iap availabilities available-territories: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return shared.UsageErrorf("iap availabilities available-territories: %v", err)
			}

			id := strings.TrimSpace(*availabilityID)
			if id == "" && strings.TrimSpace(*next) == "" {
				return shared.RequiredFlag("--id")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("iap availabilities available-territories: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.IAPAvailabilityTerritoriesOption{
				asc.Limit[asc.IAPAvailabilityTerritoriesOption](*limit),
				asc.NextURL[asc.IAPAvailabilityTerritoriesOption](*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.Limit[asc.IAPAvailabilityTerritoriesOption](200))
				firstPage, err := client.GetInAppPurchaseAvailabilityAvailableTerritories(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("iap availabilities available-territories: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetInAppPurchaseAvailabilityAvailableTerritories(ctx, id, asc.NextURL[asc.IAPAvailabilityTerritoriesOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("iap availabilities available-territories: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetInAppPurchaseAvailabilityAvailableTerritories(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("iap availabilities available-territories: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

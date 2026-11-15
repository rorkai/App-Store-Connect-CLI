package iap

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newIAPAvailabilitiesCommand returns the canonical pricing availabilities command group.
func newIAPAvailabilitiesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("availabilities", flag.ExitOnError)

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
			newIAPAvailabilitiesGetCommand(d),
			newIAPAvailabilitiesAvailableTerritoriesCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newIAPAvailabilitiesGetCommand returns the availability get subcommand.
func newIAPAvailabilitiesGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Deps:        d,
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
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetInAppPurchaseAvailabilityByID(ctx, id)
		},
	})
}

// newIAPAvailabilitiesAvailableTerritoriesCommand returns the available territories subcommand.
func newIAPAvailabilitiesAvailableTerritoriesCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Deps:        d,
		FlagSetName: "pricing availabilities available-territories",
		Name:        "available-territories",
		ShortUsage:  "asc iap pricing availabilities available-territories --id \"AVAILABILITY_ID\"",
		ShortHelp:   "List available territories for an availability.",
		LongHelp: `List available territories for an in-app purchase availability.

Examples:
  asc iap pricing availabilities available-territories --id "AVAILABILITY_ID"
  asc iap pricing availabilities available-territories --id "AVAILABILITY_ID" --paginate`,
		ParentFlag:           "id",
		ParentUsage:          "Availability ID",
		ParentType:           "inAppPurchaseAvailabilities",
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		ErrorPrefix:          "iap availabilities available-territories",
		FetchPage: func(ctx context.Context, client *asc.Client, id string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetInAppPurchaseAvailabilityAvailableTerritories(ctx, id,
				asc.Limit[asc.IAPAvailabilityTerritoriesOption](limit),
				asc.NextURL[asc.IAPAvailabilityTerritoriesOption](next),
			)
		},
	})
}

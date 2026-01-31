package iap

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// IAPAvailabilitiesCommand returns the availabilities command group.
func IAPAvailabilitiesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("availabilities", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "availabilities",
		ShortUsage: "asc iap availabilities <subcommand> [flags]",
		ShortHelp:  "Inspect in-app purchase availability records.",
		LongHelp: `Inspect in-app purchase availability records.

Examples:
  asc iap availabilities get --id "AVAILABILITY_ID"
  asc iap availabilities available-territories --id "AVAILABILITY_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
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
	fs := flag.NewFlagSet("availabilities get", flag.ExitOnError)

	availabilityID := fs.String("id", "", "Availability ID")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc iap availabilities get --id \"AVAILABILITY_ID\"",
		ShortHelp:  "Get an in-app purchase availability by ID.",
		LongHelp: `Get an in-app purchase availability by ID.

Examples:
  asc iap availabilities get --id "AVAILABILITY_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*availabilityID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("iap availabilities get: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetInAppPurchaseAvailabilityByID(requestCtx, id)
			if err != nil {
				return fmt.Errorf("iap availabilities get: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// IAPAvailabilitiesAvailableTerritoriesCommand returns the available territories subcommand.
func IAPAvailabilitiesAvailableTerritoriesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("availabilities available-territories", flag.ExitOnError)

	availabilityID := fs.String("id", "", "Availability ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "available-territories",
		ShortUsage: "asc iap availabilities available-territories --id \"AVAILABILITY_ID\"",
		ShortHelp:  "List available territories for an availability.",
		LongHelp: `List available territories for an in-app purchase availability.

Examples:
  asc iap availabilities available-territories --id "AVAILABILITY_ID"
  asc iap availabilities available-territories --id "AVAILABILITY_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("iap availabilities available-territories: --limit must be between 1 and 200")
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("iap availabilities available-territories: %w", err)
			}

			id := strings.TrimSpace(*availabilityID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("iap availabilities available-territories: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			opts := []asc.IAPAvailabilityTerritoriesOption{
				asc.WithIAPAvailabilityTerritoriesLimit(*limit),
				asc.WithIAPAvailabilityTerritoriesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithIAPAvailabilityTerritoriesLimit(200))
				firstPage, err := client.GetInAppPurchaseAvailabilityAvailableTerritories(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("iap availabilities available-territories: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetInAppPurchaseAvailabilityAvailableTerritories(ctx, id, asc.WithIAPAvailabilityTerritoriesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("iap availabilities available-territories: %w", err)
				}

				return printOutput(resp, *output, *pretty)
			}

			resp, err := client.GetInAppPurchaseAvailabilityAvailableTerritories(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("iap availabilities available-territories: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

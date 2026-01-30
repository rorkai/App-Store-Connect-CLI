package testflight

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// TestFlightCommand returns the testflight command with subcommands.
func TestFlightCommand() *ffcli.Command {
	fs := flag.NewFlagSet("testflight", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "testflight",
		ShortUsage: "asc testflight <subcommand> [flags]",
		ShortHelp:  "Manage TestFlight resources.",
		LongHelp: `Manage TestFlight resources.

Examples:
  asc testflight apps list
  asc testflight apps get --app "APP_ID"
  asc testflight beta-groups list --app "APP_ID"
  asc testflight beta-testers list --app "APP_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			TestFlightAppsCommand(),
			BetaGroupsCommand(),
			BetaTestersCommand(),
			TestFlightReviewCommand(),
			TestFlightBetaDetailsCommand(),
			TestFlightRecruitmentCommand(),
			TestFlightMetricsCommand(),
			TestFlightSyncCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// TestFlightAppsCommand returns the testflight apps command with subcommands.
func TestFlightAppsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("apps", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "apps",
		ShortUsage: "asc testflight apps <subcommand> [flags]",
		ShortHelp:  "List or fetch apps for TestFlight.",
		LongHelp: `List or fetch apps for TestFlight.

Examples:
  asc testflight apps list --sort name
  asc testflight apps get --app "APP_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			TestFlightAppsListCommand(),
			TestFlightAppsGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// TestFlightAppsListCommand lists TestFlight apps using the Apps API.
func TestFlightAppsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	bundleID := fs.String("bundle-id", "", "Filter by bundle ID(s), comma-separated")
	name := fs.String("name", "", "Filter by app name(s), comma-separated")
	sku := fs.String("sku", "", "Filter by SKU(s), comma-separated")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")
	sort := fs.String("sort", "", "Sort by name or -name, bundleId or -bundleId")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc testflight apps list [flags]",
		ShortHelp:  "List apps in App Store Connect for TestFlight.",
		LongHelp: `List apps in App Store Connect for TestFlight.

Examples:
  asc testflight apps list
  asc testflight apps list --bundle-id "com.example.app"
  asc testflight apps list --name "Example App"
  asc testflight apps list --sku "SKU123"
  asc testflight apps list --sort name --limit 10
  asc testflight apps list --output table
  asc testflight apps list --next "<links.next>"
  asc testflight apps list --paginate`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("testflight apps list: --limit must be between 1 and 200")
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("testflight apps list: %w", err)
			}
			if err := validateSort(*sort, "name", "-name", "bundleId", "-bundleId"); err != nil {
				return fmt.Errorf("testflight apps list: %w", err)
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("testflight apps list: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			opts := []asc.AppsOption{
				asc.WithAppsBundleIDs(splitCSV(*bundleID)),
				asc.WithAppsNames(splitCSV(*name)),
				asc.WithAppsSKUs(splitCSV(*sku)),
				asc.WithAppsLimit(*limit),
				asc.WithAppsNextURL(*next),
			}
			if strings.TrimSpace(*sort) != "" {
				opts = append(opts, asc.WithAppsSort(*sort))
			}

			if *paginate {
				// Fetch first page with limit set for consistent pagination
				paginateOpts := append(opts, asc.WithAppsLimit(200))
				firstPage, err := client.GetApps(requestCtx, paginateOpts...)
				if err != nil {
					return fmt.Errorf("testflight apps list: failed to fetch: %w", err)
				}

				// Fetch all remaining pages
				apps, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetApps(ctx, asc.WithAppsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("testflight apps list: %w", err)
				}

				return printOutput(apps, *output, *pretty)
			}

			apps, err := client.GetApps(requestCtx, opts...)
			if err != nil {
				return fmt.Errorf("testflight apps list: failed to fetch: %w", err)
			}

			return printOutput(apps, *output, *pretty)
		},
	}
}

// TestFlightAppsGetCommand fetches a TestFlight app by ID.
func TestFlightAppsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (required)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc testflight apps get [flags]",
		ShortHelp:  "Fetch an app by ID for TestFlight.",
		LongHelp: `Fetch an app by ID for TestFlight.

Examples:
  asc testflight apps get --app "APP_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := strings.TrimSpace(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("testflight apps get: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			app, err := client.GetApp(requestCtx, resolvedAppID)
			if err != nil {
				return fmt.Errorf("testflight apps get: failed to fetch: %w", err)
			}

			return printOutput(app, *output, *pretty)
		},
	}
}

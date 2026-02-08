package apps

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

func appsListFlags(fs *flag.FlagSet) (output *string, pretty *bool, bundleID *string, name *string, sku *string, sort *string, limit *int, next *string, paginate *bool) {
	output = fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty = fs.Bool("pretty", false, "Pretty-print JSON output")
	bundleID = fs.String("bundle-id", "", "Filter by bundle ID(s), comma-separated")
	name = fs.String("name", "", "Filter by app name(s), comma-separated")
	sku = fs.String("sku", "", "Filter by SKU(s), comma-separated")
	sort = fs.String("sort", "", "Sort by name, -name, bundleId, or -bundleId")
	limit = fs.Int("limit", 0, "Maximum results per page (1-200)")
	next = fs.String("next", "", "Fetch next page using a links.next URL")
	paginate = fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	return
}

// AppsCommand returns the apps command factory.
func AppsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("apps", flag.ExitOnError)

	output, pretty, bundleID, name, sku, sort, limit, next, paginate := appsListFlags(fs)

	return &ffcli.Command{
		Name:       "apps",
		ShortUsage: "asc apps <subcommand> [flags]",
		ShortHelp:  "List and manage apps from App Store Connect.",
		LongHelp: `List and manage apps from App Store Connect.

Examples:
  asc apps
  asc apps list --bundle-id "com.example.app"
  asc apps get --id "APP_ID"
  asc apps ci-product get --id "APP_ID"
  asc apps update --id "APP_ID" --bundle-id "com.example.app"
  asc apps update --id "APP_ID" --primary-locale "en-US"
  asc apps subscription-grace-period get --app "APP_ID"
  asc apps --limit 10
  asc apps --sort name
  asc apps --output table
  asc apps --next "<links.next>"
  asc apps --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AppsListCommand(),
			AppsGetCommand(),
			AppsCIProductCommand(),
			AppsUpdateCommand(),
			AppsRemoveBetaTestersCommand(),
			AppsSubscriptionGracePeriodCommand(),
			AppsSearchKeywordsCommand(),
			AppEncryptionDeclarationsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return appsList(ctx, *output, *pretty, *bundleID, *name, *sku, *sort, *limit, *next, *paginate)
		},
	}
}

// AppsListCommand returns the apps list subcommand.
func AppsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("apps list", flag.ExitOnError)

	output, pretty, bundleID, name, sku, sort, limit, next, paginate := appsListFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc apps list [flags]",
		ShortHelp:  "List apps from App Store Connect.",
		LongHelp: `List apps from App Store Connect.

Examples:
  asc apps list
  asc apps list --bundle-id "com.example.app"
  asc apps list --name "My App"
  asc apps list --limit 10
  asc apps list --sort name
  asc apps list --output table
  asc apps list --next "<links.next>"
  asc apps list --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			return appsList(ctx, *output, *pretty, *bundleID, *name, *sku, *sort, *limit, *next, *paginate)
		},
	}
}

// AppsGetCommand returns the apps get subcommand.
func AppsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("apps get", flag.ExitOnError)

	id := fs.String("id", "", "App Store Connect app ID")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc apps get --id APP_ID",
		ShortHelp:  "Get app details by ID.",
		LongHelp: `Get app details by ID.

Examples:
  asc apps get --id "APP_ID"
  asc apps get --id "APP_ID" --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("apps get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			app, err := client.GetApp(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("apps get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(app, *output, *pretty)
		},
	}
}

// AppsUpdateCommand returns the apps update subcommand.
func AppsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("apps update", flag.ExitOnError)

	id := fs.String("id", "", "App Store Connect app ID")
	bundleID := fs.String("bundle-id", "", "Update bundle ID")
	primaryLocale := fs.String("primary-locale", "", "Update primary locale (e.g., en-US)")
	contentRights := fs.String("content-rights", "", "Content rights declaration: DOES_NOT_USE_THIRD_PARTY_CONTENT or USES_THIRD_PARTY_CONTENT")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc apps update --id APP_ID [--bundle-id BUNDLE_ID] [--primary-locale LOCALE] [--content-rights DECLARATION]",
		ShortHelp:  "Update an app's bundle ID, primary locale, or content rights declaration.",
		LongHelp: `Update an app's bundle ID, primary locale, or content rights declaration.

Examples:
  asc apps update --id "APP_ID" --bundle-id "com.example.app"
  asc apps update --id "APP_ID" --primary-locale "en-US"
  asc apps update --id "APP_ID" --content-rights "DOES_NOT_USE_THIRD_PARTY_CONTENT"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			attrs := asc.AppUpdateAttributes{}
			if bundleValue := strings.TrimSpace(*bundleID); bundleValue != "" {
				attrs.BundleID = &bundleValue
			}
			if localeValue := strings.TrimSpace(*primaryLocale); localeValue != "" {
				attrs.PrimaryLocale = &localeValue
			}
			if rightsValue := strings.TrimSpace(*contentRights); rightsValue != "" {
				normalizedRights := asc.ContentRightsDeclaration(strings.ToUpper(rightsValue))
				switch normalizedRights {
				case asc.ContentRightsDeclarationDoesNotUseThirdPartyContent,
					asc.ContentRightsDeclarationUsesThirdPartyContent:
					attrs.ContentRightsDeclaration = &normalizedRights
				default:
					fmt.Fprintf(os.Stderr, "Error: --content-rights must be %s or %s\n", asc.ContentRightsDeclarationDoesNotUseThirdPartyContent, asc.ContentRightsDeclarationUsesThirdPartyContent)
					return flag.ErrHelp
				}
			}
			if attrs.BundleID == nil && attrs.PrimaryLocale == nil && attrs.ContentRightsDeclaration == nil {
				fmt.Fprintln(os.Stderr, "Error: --bundle-id, --primary-locale, or --content-rights is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("apps update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			app, err := client.UpdateApp(requestCtx, idValue, attrs)
			if err != nil {
				return fmt.Errorf("apps update: failed to update: %w", err)
			}

			return shared.PrintOutput(app, *output, *pretty)
		},
	}
}

func appsList(ctx context.Context, output string, pretty bool, bundleID string, name string, sku string, sort string, limit int, next string, paginate bool) error {
	if limit != 0 && (limit < 1 || limit > 200) {
		return fmt.Errorf("apps: --limit must be between 1 and 200")
	}
	if err := shared.ValidateNextURL(next); err != nil {
		return fmt.Errorf("apps: %w", err)
	}
	if err := shared.ValidateSort(sort, "name", "-name", "bundleId", "-bundleId"); err != nil {
		return fmt.Errorf("apps: %w", err)
	}

	client, err := shared.GetASCClient()
	if err != nil {
		return fmt.Errorf("apps: %w", err)
	}

	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	opts := []asc.AppsOption{
		asc.WithAppsBundleIDs(shared.SplitCSV(bundleID)),
		asc.WithAppsNames(shared.SplitCSV(name)),
		asc.WithAppsSKUs(shared.SplitCSV(sku)),
		asc.WithAppsLimit(limit),
		asc.WithAppsNextURL(next),
	}
	if strings.TrimSpace(sort) != "" {
		opts = append(opts, asc.WithAppsSort(sort))
	}

	if paginate {
		// Fetch first page with limit set for consistent pagination
		paginateOpts := append(opts, asc.WithAppsLimit(200))
		firstPage, err := client.GetApps(requestCtx, paginateOpts...)
		if err != nil {
			return fmt.Errorf("apps: failed to fetch: %w", err)
		}

		// Fetch all remaining pages
		apps, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
			return client.GetApps(ctx, asc.WithAppsNextURL(nextURL))
		})
		if err != nil {
			return fmt.Errorf("apps: %w", err)
		}

		return shared.PrintOutput(apps, output, pretty)
	}

	apps, err := client.GetApps(requestCtx, opts...)
	if err != nil {
		return fmt.Errorf("apps: failed to fetch: %w", err)
	}

	return shared.PrintOutput(apps, output, pretty)
}

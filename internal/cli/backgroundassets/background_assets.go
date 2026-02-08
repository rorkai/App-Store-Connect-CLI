package backgroundassets

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// BackgroundAssetsCommand returns the background assets command group.
func BackgroundAssetsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("background-assets", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "background-assets",
		ShortUsage: "asc background-assets <subcommand> [flags]",
		ShortHelp:  "Manage background assets.",
		LongHelp: `Manage background assets.

Examples:
  asc background-assets list --app "APP_ID"
  asc background-assets get --id "ASSET_ID"
  asc background-assets create --app "APP_ID" --asset-pack-identifier "com.example.assetpack"
  asc background-assets update --id "ASSET_ID" --archived true
  asc background-assets versions list --background-asset-id "ASSET_ID"
  asc background-assets app-store-releases get --id "RELEASE_ID"
  asc background-assets upload-files create --version-id "VERSION_ID" --file "./asset.zip" --asset-type ASSET`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			BackgroundAssetsListCommand(),
			BackgroundAssetsGetCommand(),
			BackgroundAssetsCreateCommand(),
			BackgroundAssetsUpdateCommand(),
			BackgroundAssetsVersionsCommand(),
			BackgroundAssetsAppStoreReleasesCommand(),
			BackgroundAssetsExternalBetaReleasesCommand(),
			BackgroundAssetsInternalBetaReleasesCommand(),
			BackgroundAssetsUploadFilesCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BackgroundAssetsListCommand returns the background assets list subcommand.
func BackgroundAssetsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	archived := fs.String("archived", "", "Filter by archived state (true/false)")
	assetPackIdentifier := fs.String("asset-pack-identifier", "", "Filter by asset pack identifier(s), comma-separated")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc background-assets list --app \"APP_ID\" [flags]",
		ShortHelp:  "List background assets for an app.",
		LongHelp: `List background assets for an app.

Examples:
  asc background-assets list --app "APP_ID"
  asc background-assets list --app "APP_ID" --archived false
  asc background-assets list --app "APP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}
			if *limit != 0 && (*limit < 1 || *limit > backgroundAssetsMaxLimit) {
				return fmt.Errorf("background-assets list: --limit must be between 1 and %d", backgroundAssetsMaxLimit)
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("background-assets list: %w", err)
			}

			var archivedFilter []string
			if strings.TrimSpace(*archived) != "" {
				value, err := parseBool(*archived, "--archived")
				if err != nil {
					return fmt.Errorf("background-assets list: %w", err)
				}
				archivedFilter = []string{strconv.FormatBool(value)}
			}

			assetPackIdentifiers := shared.SplitCSV(*assetPackIdentifier)

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("background-assets list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.BackgroundAssetsOption{
				asc.WithBackgroundAssetsLimit(*limit),
				asc.WithBackgroundAssetsNextURL(*next),
			}
			if len(archivedFilter) > 0 {
				opts = append(opts, asc.WithBackgroundAssetsFilterArchived(archivedFilter))
			}
			if len(assetPackIdentifiers) > 0 {
				opts = append(opts, asc.WithBackgroundAssetsFilterAssetPackIdentifier(assetPackIdentifiers))
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithBackgroundAssetsLimit(backgroundAssetsMaxLimit))
				firstPage, err := client.GetBackgroundAssets(requestCtx, resolvedAppID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("background-assets list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetBackgroundAssets(ctx, resolvedAppID, asc.WithBackgroundAssetsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("background-assets list: %w", err)
				}

				return shared.PrintOutput(resp, *output, *pretty)
			}

			resp, err := client.GetBackgroundAssets(requestCtx, resolvedAppID, opts...)
			if err != nil {
				return fmt.Errorf("background-assets list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// BackgroundAssetsGetCommand returns the background assets get subcommand.
func BackgroundAssetsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	assetID := fs.String("id", "", "Background asset ID")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc background-assets get --id \"ASSET_ID\"",
		ShortHelp:  "Get a background asset by ID.",
		LongHelp: `Get a background asset by ID.

Examples:
  asc background-assets get --id "ASSET_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			assetIDValue := strings.TrimSpace(*assetID)
			if assetIDValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("background-assets get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetBackgroundAsset(requestCtx, assetIDValue)
			if err != nil {
				return fmt.Errorf("background-assets get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// BackgroundAssetsCreateCommand returns the background assets create subcommand.
func BackgroundAssetsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	assetPackIdentifier := fs.String("asset-pack-identifier", "", "Asset pack identifier")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc background-assets create --app \"APP_ID\" --asset-pack-identifier \"ASSET_PACK_ID\"",
		ShortHelp:  "Create a background asset.",
		LongHelp: `Create a background asset.

Examples:
  asc background-assets create --app "APP_ID" --asset-pack-identifier "com.example.assetpack"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			assetPackIdentifierValue := strings.TrimSpace(*assetPackIdentifier)
			if assetPackIdentifierValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --asset-pack-identifier is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("background-assets create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateBackgroundAsset(requestCtx, resolvedAppID, assetPackIdentifierValue)
			if err != nil {
				return fmt.Errorf("background-assets create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// BackgroundAssetsUpdateCommand returns the background assets update subcommand.
func BackgroundAssetsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	assetID := fs.String("id", "", "Background asset ID")
	archived := fs.String("archived", "", "Set archived state (true/false)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc background-assets update --id \"ASSET_ID\" --archived true",
		ShortHelp:  "Update a background asset.",
		LongHelp: `Update a background asset.

Examples:
  asc background-assets update --id "ASSET_ID" --archived true`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			assetIDValue := strings.TrimSpace(*assetID)
			if assetIDValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			if strings.TrimSpace(*archived) == "" {
				fmt.Fprintln(os.Stderr, "Error: --archived is required")
				return flag.ErrHelp
			}
			archivedValue, err := parseBool(*archived, "--archived")
			if err != nil {
				return fmt.Errorf("background-assets update: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("background-assets update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			attrs := asc.BackgroundAssetUpdateAttributes{Archived: &archivedValue}
			resp, err := client.UpdateBackgroundAsset(requestCtx, assetIDValue, attrs)
			if err != nil {
				return fmt.Errorf("background-assets update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

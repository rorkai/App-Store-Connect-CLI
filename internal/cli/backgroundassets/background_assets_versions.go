package backgroundassets

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

// BackgroundAssetsVersionsCommand returns the versions command group.
func BackgroundAssetsVersionsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "versions",
		ShortUsage: "asc background-assets versions <subcommand> [flags]",
		ShortHelp:  "Manage background asset versions.",
		LongHelp: `Manage background asset versions.

Examples:
  asc background-assets versions list --background-asset-id "ASSET_ID"
  asc background-assets versions get --version-id "VERSION_ID"
  asc background-assets versions create --background-asset-id "ASSET_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			BackgroundAssetsVersionsListCommand(),
			BackgroundAssetsVersionsGetCommand(),
			BackgroundAssetsVersionsCreateCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BackgroundAssetsVersionsListCommand returns the versions list subcommand.
func BackgroundAssetsVersionsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	assetID := fs.String("background-asset-id", "", "Background asset ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc background-assets versions list --background-asset-id \"ASSET_ID\"",
		ShortHelp:  "List versions for a background asset.",
		LongHelp: `List versions for a background asset.

Examples:
  asc background-assets versions list --background-asset-id "ASSET_ID"
  asc background-assets versions list --background-asset-id "ASSET_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			assetIDValue := strings.TrimSpace(*assetID)
			if assetIDValue == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --background-asset-id is required")
				return flag.ErrHelp
			}
			if *limit != 0 && (*limit < 1 || *limit > backgroundAssetsMaxLimit) {
				return fmt.Errorf("background-assets versions list: --limit must be between 1 and %d", backgroundAssetsMaxLimit)
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("background-assets versions list: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("background-assets versions list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.BackgroundAssetVersionsOption{
				asc.WithBackgroundAssetVersionsLimit(*limit),
				asc.WithBackgroundAssetVersionsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithBackgroundAssetVersionsLimit(backgroundAssetsMaxLimit))
				firstPage, err := client.GetBackgroundAssetVersions(requestCtx, assetIDValue, paginateOpts...)
				if err != nil {
					return fmt.Errorf("background-assets versions list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetBackgroundAssetVersions(ctx, assetIDValue, asc.WithBackgroundAssetVersionsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("background-assets versions list: %w", err)
				}

				return shared.PrintOutput(resp, *output, *pretty)
			}

			resp, err := client.GetBackgroundAssetVersions(requestCtx, assetIDValue, opts...)
			if err != nil {
				return fmt.Errorf("background-assets versions list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// BackgroundAssetsVersionsGetCommand returns the versions get subcommand.
func BackgroundAssetsVersionsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	versionID := fs.String("version-id", "", "Background asset version ID")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc background-assets versions get --version-id \"VERSION_ID\"",
		ShortHelp:  "Get a background asset version by ID.",
		LongHelp: `Get a background asset version by ID.

Examples:
  asc background-assets versions get --version-id "VERSION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			versionIDValue := strings.TrimSpace(*versionID)
			if versionIDValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("background-assets versions get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetBackgroundAssetVersion(requestCtx, versionIDValue)
			if err != nil {
				return fmt.Errorf("background-assets versions get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// BackgroundAssetsVersionsCreateCommand returns the versions create subcommand.
func BackgroundAssetsVersionsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	assetID := fs.String("background-asset-id", "", "Background asset ID")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc background-assets versions create --background-asset-id \"ASSET_ID\"",
		ShortHelp:  "Create a background asset version.",
		LongHelp: `Create a background asset version.

Examples:
  asc background-assets versions create --background-asset-id "ASSET_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			assetIDValue := strings.TrimSpace(*assetID)
			if assetIDValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --background-asset-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("background-assets versions create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateBackgroundAssetVersion(requestCtx, assetIDValue)
			if err != nil {
				return fmt.Errorf("background-assets versions create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

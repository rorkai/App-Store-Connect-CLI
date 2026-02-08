package versions

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

// VersionsExperimentsV2Command returns the experiments v2 command group.
func VersionsExperimentsV2Command() *ffcli.Command {
	fs := flag.NewFlagSet("experiments-v2", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "experiments-v2",
		ShortUsage: "asc versions experiments-v2 <subcommand> [flags]",
		ShortHelp:  "Manage App Store version experiments (v2).",
		LongHelp: `Manage App Store version experiments (v2).

Examples:
  asc versions experiments-v2 list --version-id "VERSION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			VersionsExperimentsV2ListCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// VersionsExperimentsV2ListCommand lists v2 experiments for a version.
func VersionsExperimentsV2ListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("experiments-v2 list", flag.ExitOnError)

	versionID := fs.String("version-id", "", "App Store version ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc versions experiments-v2 list --version-id \"VERSION_ID\" [flags]",
		ShortHelp:  "List v2 experiments for an app store version.",
		LongHelp: `List v2 experiments for an app store version.

Examples:
  asc versions experiments-v2 list --version-id "VERSION_ID"
  asc versions experiments-v2 list --version-id "VERSION_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("versions experiments-v2 list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("versions experiments-v2 list: %w", err)
			}

			versionValue := strings.TrimSpace(*versionID)
			if versionValue == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("versions experiments-v2 list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.AppStoreVersionExperimentsV2Option{
				asc.WithAppStoreVersionExperimentsV2Limit(*limit),
				asc.WithAppStoreVersionExperimentsV2NextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithAppStoreVersionExperimentsV2Limit(200))
				firstPage, err := client.GetAppStoreVersionExperimentsV2ForVersion(requestCtx, versionValue, paginateOpts...)
				if err != nil {
					return fmt.Errorf("versions experiments-v2 list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppStoreVersionExperimentsV2ForVersion(ctx, versionValue, asc.WithAppStoreVersionExperimentsV2NextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("versions experiments-v2 list: %w", err)
				}

				return shared.PrintOutput(resp, *output, *pretty)
			}

			resp, err := client.GetAppStoreVersionExperimentsV2ForVersion(requestCtx, versionValue, opts...)
			if err != nil {
				return fmt.Errorf("versions experiments-v2 list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

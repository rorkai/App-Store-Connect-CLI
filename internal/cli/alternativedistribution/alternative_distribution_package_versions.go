package alternativedistribution

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

// AlternativeDistributionPackageVersionsCommand returns the package versions command group.
func AlternativeDistributionPackageVersionsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "versions",
		ShortUsage: "asc alternative-distribution packages versions <subcommand> [flags]",
		ShortHelp:  "Manage alternative distribution package versions.",
		LongHelp: `Manage alternative distribution package versions.

Examples:
  asc alternative-distribution packages versions list --package-id "PACKAGE_ID"
  asc alternative-distribution packages versions get --version-id "VERSION_ID"
  asc alternative-distribution packages versions deltas --version-id "VERSION_ID"
  asc alternative-distribution packages versions variants --version-id "VERSION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AlternativeDistributionPackageVersionsListCommand(),
			AlternativeDistributionPackageVersionsGetCommand(),
			AlternativeDistributionPackageVersionsDeltasCommand(),
			AlternativeDistributionPackageVersionsVariantsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// AlternativeDistributionPackageVersionsListCommand returns the package versions list subcommand.
func AlternativeDistributionPackageVersionsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	packageID := fs.String("package-id", "", "Alternative distribution package ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc alternative-distribution packages versions list --package-id \"PACKAGE_ID\" [flags]",
		ShortHelp:  "List alternative distribution package versions.",
		LongHelp: `List alternative distribution package versions.

Examples:
  asc alternative-distribution packages versions list --package-id "PACKAGE_ID"
  asc alternative-distribution packages versions list --package-id "PACKAGE_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*packageID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --package-id is required")
				return flag.ErrHelp
			}
			if *limit != 0 && (*limit < 1 || *limit > alternativeDistributionMaxLimit) {
				return fmt.Errorf("alternative-distribution packages versions list: --limit must be between 1 and %d", alternativeDistributionMaxLimit)
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("alternative-distribution packages versions list: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("alternative-distribution packages versions list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.AlternativeDistributionPackageVersionsOption{
				asc.WithAlternativeDistributionPackageVersionsLimit(*limit),
				asc.WithAlternativeDistributionPackageVersionsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithAlternativeDistributionPackageVersionsLimit(alternativeDistributionMaxLimit))
				firstPage, err := client.GetAlternativeDistributionPackageVersions(requestCtx, trimmedID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("alternative-distribution packages versions list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAlternativeDistributionPackageVersions(ctx, trimmedID, asc.WithAlternativeDistributionPackageVersionsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("alternative-distribution packages versions list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAlternativeDistributionPackageVersions(requestCtx, trimmedID, opts...)
			if err != nil {
				return fmt.Errorf("alternative-distribution packages versions list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// AlternativeDistributionPackageVersionsGetCommand returns the package versions get subcommand.
func AlternativeDistributionPackageVersionsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	versionID := fs.String("version-id", "", "Alternative distribution package version ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc alternative-distribution packages versions get --version-id \"VERSION_ID\"",
		ShortHelp:  "Get an alternative distribution package version.",
		LongHelp: `Get an alternative distribution package version.

Examples:
  asc alternative-distribution packages versions get --version-id "VERSION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*versionID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("alternative-distribution packages versions get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAlternativeDistributionPackageVersion(requestCtx, trimmedID)
			if err != nil {
				return fmt.Errorf("alternative-distribution packages versions get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// AlternativeDistributionPackageVersionsDeltasCommand returns the package version deltas subcommand.
func AlternativeDistributionPackageVersionsDeltasCommand() *ffcli.Command {
	fs := flag.NewFlagSet("deltas", flag.ExitOnError)

	versionID := fs.String("version-id", "", "Alternative distribution package version ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "deltas",
		ShortUsage: "asc alternative-distribution packages versions deltas --version-id \"VERSION_ID\" [flags]",
		ShortHelp:  "List alternative distribution package deltas.",
		LongHelp: `List alternative distribution package deltas.

Examples:
  asc alternative-distribution packages versions deltas --version-id "VERSION_ID"
  asc alternative-distribution packages versions deltas --version-id "VERSION_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*versionID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return flag.ErrHelp
			}
			if *limit != 0 && (*limit < 1 || *limit > alternativeDistributionMaxLimit) {
				return fmt.Errorf("alternative-distribution packages versions deltas: --limit must be between 1 and %d", alternativeDistributionMaxLimit)
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("alternative-distribution packages versions deltas: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("alternative-distribution packages versions deltas: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.AlternativeDistributionPackageDeltasOption{
				asc.WithAlternativeDistributionPackageDeltasLimit(*limit),
				asc.WithAlternativeDistributionPackageDeltasNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithAlternativeDistributionPackageDeltasLimit(alternativeDistributionMaxLimit))
				firstPage, err := client.GetAlternativeDistributionPackageVersionDeltas(requestCtx, trimmedID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("alternative-distribution packages versions deltas: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAlternativeDistributionPackageVersionDeltas(ctx, trimmedID, asc.WithAlternativeDistributionPackageDeltasNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("alternative-distribution packages versions deltas: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAlternativeDistributionPackageVersionDeltas(requestCtx, trimmedID, opts...)
			if err != nil {
				return fmt.Errorf("alternative-distribution packages versions deltas: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// AlternativeDistributionPackageVersionsVariantsCommand returns the package version variants subcommand.
func AlternativeDistributionPackageVersionsVariantsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("variants", flag.ExitOnError)

	versionID := fs.String("version-id", "", "Alternative distribution package version ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "variants",
		ShortUsage: "asc alternative-distribution packages versions variants --version-id \"VERSION_ID\" [flags]",
		ShortHelp:  "List alternative distribution package variants.",
		LongHelp: `List alternative distribution package variants.

Examples:
  asc alternative-distribution packages versions variants --version-id "VERSION_ID"
  asc alternative-distribution packages versions variants --version-id "VERSION_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*versionID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return flag.ErrHelp
			}
			if *limit != 0 && (*limit < 1 || *limit > alternativeDistributionMaxLimit) {
				return fmt.Errorf("alternative-distribution packages versions variants: --limit must be between 1 and %d", alternativeDistributionMaxLimit)
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("alternative-distribution packages versions variants: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("alternative-distribution packages versions variants: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.AlternativeDistributionPackageVariantsOption{
				asc.WithAlternativeDistributionPackageVariantsLimit(*limit),
				asc.WithAlternativeDistributionPackageVariantsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithAlternativeDistributionPackageVariantsLimit(alternativeDistributionMaxLimit))
				firstPage, err := client.GetAlternativeDistributionPackageVersionVariants(requestCtx, trimmedID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("alternative-distribution packages versions variants: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAlternativeDistributionPackageVersionVariants(ctx, trimmedID, asc.WithAlternativeDistributionPackageVariantsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("alternative-distribution packages versions variants: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAlternativeDistributionPackageVersionVariants(requestCtx, trimmedID, opts...)
			if err != nil {
				return fmt.Errorf("alternative-distribution packages versions variants: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

package alternativedistribution

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newAlternativeDistributionPackageVersionsCommand returns the package versions command group.
func newAlternativeDistributionPackageVersionsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("versions", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "versions",
		ShortUsage: "asc alternative-distribution packages versions <subcommand> [flags]",
		ShortHelp:  "Manage alternative distribution package versions.",
		LongHelp: `Manage alternative distribution package versions.

Examples:
  asc alternative-distribution packages versions list --package-id "PACKAGE_ID"
  asc alternative-distribution packages versions view --version-id "VERSION_ID"
  asc alternative-distribution packages versions deltas --version-id "VERSION_ID"
  asc alternative-distribution packages versions variants --version-id "VERSION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newAlternativeDistributionPackageVersionsListCommand(d),
			newAlternativeDistributionPackageVersionsGetCommand(d),
			newAlternativeDistributionPackageVersionsDeltasCommand(d),
			newAlternativeDistributionPackageVersionsVariantsCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newAlternativeDistributionPackageVersionsListCommand returns the package versions list subcommand.
func newAlternativeDistributionPackageVersionsListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("list", flag.ExitOnError)

	packageID := shared.BindResourceIDFlag(fs, "package-id", "alternativeDistributionPackages", "Alternative distribution package ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := d.BindOutputFlags(fs)

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
				return d.RequiredFlag("--package-id")
			}
			if *limit != 0 && (*limit < 1 || *limit > alternativeDistributionMaxLimit) {
				return d.UsageErrorf("alternative-distribution packages versions list: --limit must be between 1 and %d", alternativeDistributionMaxLimit)
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return d.UsageErrorf("alternative-distribution packages versions list: %v", err)
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("alternative-distribution packages versions list: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			opts := []asc.AlternativeDistributionPackageVersionsOption{
				asc.Limit[asc.AlternativeDistributionPackageVersionsOption](*limit),
				asc.NextURL[asc.AlternativeDistributionPackageVersionsOption](*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.Limit[asc.AlternativeDistributionPackageVersionsOption](alternativeDistributionMaxLimit))
				firstPage, err := client.GetAlternativeDistributionPackageVersions(requestCtx, trimmedID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("alternative-distribution packages versions list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAlternativeDistributionPackageVersions(ctx, trimmedID, asc.NextURL[asc.AlternativeDistributionPackageVersionsOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("alternative-distribution packages versions list: %w", err)
				}

				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAlternativeDistributionPackageVersions(requestCtx, trimmedID, opts...)
			if err != nil {
				return fmt.Errorf("alternative-distribution packages versions list: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newAlternativeDistributionPackageVersionsGetCommand returns the package versions get subcommand.
func newAlternativeDistributionPackageVersionsGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Deps:        d,
		FlagSetName: "view",
		Name:        "view",
		ShortUsage:  "asc alternative-distribution packages versions view --version-id \"VERSION_ID\"",
		ShortHelp:   "View an alternative distribution package version.",
		LongHelp: `View an alternative distribution package version.

Examples:
  asc alternative-distribution packages versions view --version-id "VERSION_ID"`,
		IDFlag:             "version-id",
		IDUsage:            "Alternative distribution package version ID",
		IDType:             "alternativeDistributionPackageVersions",
		RequiredDiagnostic: true,
		ErrorPrefix:        "alternative-distribution packages versions view",
		FetchErrorPrefix:   "alternative-distribution packages versions view: failed to fetch",
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetAlternativeDistributionPackageVersion(ctx, id)
		},
	})
}

// newAlternativeDistributionPackageVersionsDeltasCommand returns the package version deltas subcommand.
func newAlternativeDistributionPackageVersionsDeltasCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("deltas", flag.ExitOnError)

	versionID := shared.BindResourceIDFlag(fs, "version-id", "alternativeDistributionPackageVersions", "Alternative distribution package version ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := d.BindOutputFlags(fs)

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
				return d.RequiredFlag("--version-id")
			}
			if *limit != 0 && (*limit < 1 || *limit > alternativeDistributionMaxLimit) {
				return d.UsageErrorf("alternative-distribution packages versions deltas: --limit must be between 1 and %d", alternativeDistributionMaxLimit)
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return d.UsageErrorf("alternative-distribution packages versions deltas: %v", err)
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("alternative-distribution packages versions deltas: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			opts := []asc.AlternativeDistributionPackageDeltasOption{
				asc.Limit[asc.AlternativeDistributionPackageDeltasOption](*limit),
				asc.NextURL[asc.AlternativeDistributionPackageDeltasOption](*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.Limit[asc.AlternativeDistributionPackageDeltasOption](alternativeDistributionMaxLimit))
				firstPage, err := client.GetAlternativeDistributionPackageVersionDeltas(requestCtx, trimmedID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("alternative-distribution packages versions deltas: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAlternativeDistributionPackageVersionDeltas(ctx, trimmedID, asc.NextURL[asc.AlternativeDistributionPackageDeltasOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("alternative-distribution packages versions deltas: %w", err)
				}

				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAlternativeDistributionPackageVersionDeltas(requestCtx, trimmedID, opts...)
			if err != nil {
				return fmt.Errorf("alternative-distribution packages versions deltas: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newAlternativeDistributionPackageVersionsVariantsCommand returns the package version variants subcommand.
func newAlternativeDistributionPackageVersionsVariantsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("variants", flag.ExitOnError)

	versionID := shared.BindResourceIDFlag(fs, "version-id", "alternativeDistributionPackageVersions", "Alternative distribution package version ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := d.BindOutputFlags(fs)

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
				return d.RequiredFlag("--version-id")
			}
			if *limit != 0 && (*limit < 1 || *limit > alternativeDistributionMaxLimit) {
				return d.UsageErrorf("alternative-distribution packages versions variants: --limit must be between 1 and %d", alternativeDistributionMaxLimit)
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return d.UsageErrorf("alternative-distribution packages versions variants: %v", err)
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("alternative-distribution packages versions variants: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			opts := []asc.AlternativeDistributionPackageVariantsOption{
				asc.Limit[asc.AlternativeDistributionPackageVariantsOption](*limit),
				asc.NextURL[asc.AlternativeDistributionPackageVariantsOption](*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.Limit[asc.AlternativeDistributionPackageVariantsOption](alternativeDistributionMaxLimit))
				firstPage, err := client.GetAlternativeDistributionPackageVersionVariants(requestCtx, trimmedID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("alternative-distribution packages versions variants: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAlternativeDistributionPackageVersionVariants(ctx, trimmedID, asc.NextURL[asc.AlternativeDistributionPackageVariantsOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("alternative-distribution packages versions variants: %w", err)
				}

				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAlternativeDistributionPackageVersionVariants(requestCtx, trimmedID, opts...)
			if err != nil {
				return fmt.Errorf("alternative-distribution packages versions variants: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

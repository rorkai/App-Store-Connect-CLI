package alternativedistribution

import (
	"context"
	"flag"

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
		UsageFunc: d.DefaultUsageFunc,
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
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "list",
		ShortUsage: "asc alternative-distribution packages versions list --package-id \"PACKAGE_ID\" [flags]",
		ShortHelp:  "List alternative distribution package versions.",
		LongHelp: `List alternative distribution package versions.

Examples:
  asc alternative-distribution packages versions list --package-id "PACKAGE_ID"
  asc alternative-distribution packages versions list --package-id "PACKAGE_ID" --paginate`,
		ParentFlag:           "package-id",
		ParentUsage:          "Alternative distribution package ID",
		ParentType:           "alternativeDistributionPackages",
		RequiredDiagnostic:   true,
		LimitMax:             alternativeDistributionMaxLimit,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		Deps:                 d,
		FetchPage: func(ctx context.Context, client *asc.Client, id string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetAlternativeDistributionPackageVersions(ctx, id,
				asc.Limit[asc.AlternativeDistributionPackageVersionsOption](limit),
				asc.NextURL[asc.AlternativeDistributionPackageVersionsOption](next),
			)
		},
	})
}

// newAlternativeDistributionPackageVersionsGetCommand returns the package versions get subcommand.
func newAlternativeDistributionPackageVersionsGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Deps:       d,
		Name:       "view",
		ShortUsage: "asc alternative-distribution packages versions view --version-id \"VERSION_ID\"",
		ShortHelp:  "View an alternative distribution package version.",
		LongHelp: `View an alternative distribution package version.

Examples:
  asc alternative-distribution packages versions view --version-id "VERSION_ID"`,
		IDFlag:             "version-id",
		IDUsage:            "Alternative distribution package version ID",
		IDType:             "alternativeDistributionPackageVersions",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetAlternativeDistributionPackageVersion(ctx, id)
		},
	})
}

// newAlternativeDistributionPackageVersionsDeltasCommand returns the package version deltas subcommand.
func newAlternativeDistributionPackageVersionsDeltasCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "deltas",
		ShortUsage: "asc alternative-distribution packages versions deltas --version-id \"VERSION_ID\" [flags]",
		ShortHelp:  "List alternative distribution package deltas.",
		LongHelp: `List alternative distribution package deltas.

Examples:
  asc alternative-distribution packages versions deltas --version-id "VERSION_ID"
  asc alternative-distribution packages versions deltas --version-id "VERSION_ID" --paginate`,
		ParentFlag:           "version-id",
		ParentUsage:          "Alternative distribution package version ID",
		ParentType:           "alternativeDistributionPackageVersions",
		RequiredDiagnostic:   true,
		LimitMax:             alternativeDistributionMaxLimit,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		Deps:                 d,
		FetchPage: func(ctx context.Context, client *asc.Client, id string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetAlternativeDistributionPackageVersionDeltas(ctx, id,
				asc.Limit[asc.AlternativeDistributionPackageDeltasOption](limit),
				asc.NextURL[asc.AlternativeDistributionPackageDeltasOption](next),
			)
		},
	})
}

// newAlternativeDistributionPackageVersionsVariantsCommand returns the package version variants subcommand.
func newAlternativeDistributionPackageVersionsVariantsCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "variants",
		ShortUsage: "asc alternative-distribution packages versions variants --version-id \"VERSION_ID\" [flags]",
		ShortHelp:  "List alternative distribution package variants.",
		LongHelp: `List alternative distribution package variants.

Examples:
  asc alternative-distribution packages versions variants --version-id "VERSION_ID"
  asc alternative-distribution packages versions variants --version-id "VERSION_ID" --paginate`,
		ParentFlag:           "version-id",
		ParentUsage:          "Alternative distribution package version ID",
		ParentType:           "alternativeDistributionPackageVersions",
		RequiredDiagnostic:   true,
		LimitMax:             alternativeDistributionMaxLimit,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		Deps:                 d,
		FetchPage: func(ctx context.Context, client *asc.Client, id string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetAlternativeDistributionPackageVersionVariants(ctx, id,
				asc.Limit[asc.AlternativeDistributionPackageVariantsOption](limit),
				asc.NextURL[asc.AlternativeDistributionPackageVariantsOption](next),
			)
		},
	})
}

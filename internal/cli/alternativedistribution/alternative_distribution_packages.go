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

// newAlternativeDistributionPackagesCommand returns the packages command group.
func newAlternativeDistributionPackagesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("packages", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "packages",
		ShortUsage: "asc alternative-distribution packages <subcommand> [flags]",
		ShortHelp:  "Manage alternative distribution packages.",
		LongHelp: `Manage alternative distribution packages.

Examples:
  asc alternative-distribution packages view --package-id "PACKAGE_ID"
  asc alternative-distribution packages create --app-store-version-id "APP_STORE_VERSION_ID"
  asc alternative-distribution packages app-store-version --app-store-version-id "APP_STORE_VERSION_ID"
  asc alternative-distribution packages versions list --package-id "PACKAGE_ID"
  asc alternative-distribution packages versions view --version-id "VERSION_ID"
  asc alternative-distribution packages versions deltas --version-id "VERSION_ID"
  asc alternative-distribution packages versions variants --version-id "VERSION_ID"
  asc alternative-distribution packages variants --variant-id "VARIANT_ID"
  asc alternative-distribution packages deltas --delta-id "DELTA_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newAlternativeDistributionPackagesGetCommand(d),
			newAlternativeDistributionPackagesCreateCommand(d),
			newAlternativeDistributionPackagesAppStoreVersionCommand(d),
			newAlternativeDistributionPackageVersionsCommand(d),
			newAlternativeDistributionPackageVariantsCommand(d),
			newAlternativeDistributionPackageDeltasCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newAlternativeDistributionPackagesGetCommand returns the packages get subcommand.
func newAlternativeDistributionPackagesGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Deps:        d,
		FlagSetName: "view",
		Name:        "view",
		ShortUsage:  "asc alternative-distribution packages view --package-id \"PACKAGE_ID\"",
		ShortHelp:   "View an alternative distribution package.",
		LongHelp: `View an alternative distribution package.

Examples:
  asc alternative-distribution packages view --package-id "PACKAGE_ID"`,
		IDFlag:             "package-id",
		IDUsage:            "Alternative distribution package ID",
		IDType:             "alternativeDistributionPackages",
		RequiredDiagnostic: true,
		ErrorPrefix:        "alternative-distribution packages view",
		FetchErrorPrefix:   "alternative-distribution packages view: failed to fetch",
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetAlternativeDistributionPackage(ctx, id)
		},
	})
}

// newAlternativeDistributionPackagesCreateCommand returns the packages create subcommand.
func newAlternativeDistributionPackagesCreateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("create", flag.ExitOnError)

	appStoreVersionID := shared.BindResourceIDFlag(fs, "app-store-version-id", "appStoreVersions", "App Store version ID for the package")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc alternative-distribution packages create --app-store-version-id \"APP_STORE_VERSION_ID\"",
		ShortHelp:  "Create an alternative distribution package.",
		LongHelp: `Create an alternative distribution package.

Examples:
  asc alternative-distribution packages create --app-store-version-id "APP_STORE_VERSION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*appStoreVersionID)
			if trimmedID == "" {
				return d.RequiredFlag("--app-store-version-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("alternative-distribution packages create: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.CreateAlternativeDistributionPackage(requestCtx, trimmedID)
			if err != nil {
				return fmt.Errorf("alternative-distribution packages create: failed to create: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newAlternativeDistributionPackagesAppStoreVersionCommand returns the app-store-version package subcommand.
func newAlternativeDistributionPackagesAppStoreVersionCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("app-store-version", flag.ExitOnError)

	appStoreVersionID := shared.BindResourceIDFlag(fs, "app-store-version-id", "appStoreVersions", "App Store version ID")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "app-store-version",
		ShortUsage: "asc alternative-distribution packages app-store-version --app-store-version-id \"APP_STORE_VERSION_ID\"",
		ShortHelp:  "Get the package for an app store version.",
		LongHelp: `Get the package for an app store version.

Examples:
  asc alternative-distribution packages app-store-version --app-store-version-id "APP_STORE_VERSION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*appStoreVersionID)
			if trimmedID == "" {
				return d.RequiredFlag("--app-store-version-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("alternative-distribution packages app-store-version: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.GetAppStoreVersionAlternativeDistributionPackage(requestCtx, trimmedID)
			if err != nil {
				return fmt.Errorf("alternative-distribution packages app-store-version: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newAlternativeDistributionPackageVariantsCommand returns the package variant get subcommand.
func newAlternativeDistributionPackageVariantsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("variants", flag.ExitOnError)

	variantID := shared.BindResourceIDFlag(fs, "variant-id", "alternativeDistributionPackageVariants", "Alternative distribution package variant ID")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "variants",
		ShortUsage: "asc alternative-distribution packages variants --variant-id \"VARIANT_ID\"",
		ShortHelp:  "Get an alternative distribution package variant.",
		LongHelp: `Get an alternative distribution package variant.

Examples:
  asc alternative-distribution packages variants --variant-id "VARIANT_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*variantID)
			if trimmedID == "" {
				return d.RequiredFlag("--variant-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("alternative-distribution packages variants: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.GetAlternativeDistributionPackageVariant(requestCtx, trimmedID)
			if err != nil {
				return fmt.Errorf("alternative-distribution packages variants: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newAlternativeDistributionPackageDeltasCommand returns the package delta get subcommand.
func newAlternativeDistributionPackageDeltasCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("deltas", flag.ExitOnError)

	deltaID := shared.BindResourceIDFlag(fs, "delta-id", "alternativeDistributionPackageDeltas", "Alternative distribution package delta ID")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "deltas",
		ShortUsage: "asc alternative-distribution packages deltas --delta-id \"DELTA_ID\"",
		ShortHelp:  "Get an alternative distribution package delta.",
		LongHelp: `Get an alternative distribution package delta.

Examples:
  asc alternative-distribution packages deltas --delta-id "DELTA_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*deltaID)
			if trimmedID == "" {
				return d.RequiredFlag("--delta-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("alternative-distribution packages deltas: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.GetAlternativeDistributionPackageDelta(requestCtx, trimmedID)
			if err != nil {
				return fmt.Errorf("alternative-distribution packages deltas: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

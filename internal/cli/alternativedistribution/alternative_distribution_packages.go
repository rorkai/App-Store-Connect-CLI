package alternativedistribution

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// AlternativeDistributionPackagesCommand returns the packages command group.
func AlternativeDistributionPackagesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("packages", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "packages",
		ShortUsage: "asc alternative-distribution packages <subcommand> [flags]",
		ShortHelp:  "Manage alternative distribution packages.",
		LongHelp: `Manage alternative distribution packages.

Examples:
  asc alternative-distribution packages get --package-id "PACKAGE_ID"
  asc alternative-distribution packages create --app-store-version-id "APP_STORE_VERSION_ID"
  asc alternative-distribution packages app-store-version --app-store-version-id "APP_STORE_VERSION_ID"
  asc alternative-distribution packages versions list --package-id "PACKAGE_ID"
  asc alternative-distribution packages versions get --version-id "VERSION_ID"
  asc alternative-distribution packages versions deltas --version-id "VERSION_ID"
  asc alternative-distribution packages versions variants --version-id "VERSION_ID"
  asc alternative-distribution packages variants --variant-id "VARIANT_ID"
  asc alternative-distribution packages deltas --delta-id "DELTA_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AlternativeDistributionPackagesGetCommand(),
			AlternativeDistributionPackagesCreateCommand(),
			AlternativeDistributionPackagesAppStoreVersionCommand(),
			AlternativeDistributionPackageVersionsCommand(),
			AlternativeDistributionPackageVariantsCommand(),
			AlternativeDistributionPackageDeltasCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// AlternativeDistributionPackagesGetCommand returns the packages get subcommand.
func AlternativeDistributionPackagesGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	packageID := fs.String("package-id", "", "Alternative distribution package ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc alternative-distribution packages get --package-id \"PACKAGE_ID\"",
		ShortHelp:  "Get an alternative distribution package.",
		LongHelp: `Get an alternative distribution package.

Examples:
  asc alternative-distribution packages get --package-id "PACKAGE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*packageID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --package-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("alternative-distribution packages get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAlternativeDistributionPackage(requestCtx, trimmedID)
			if err != nil {
				return fmt.Errorf("alternative-distribution packages get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// AlternativeDistributionPackagesCreateCommand returns the packages create subcommand.
func AlternativeDistributionPackagesCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	appStoreVersionID := fs.String("app-store-version-id", "", "App Store version ID for the package")
	output := shared.BindOutputFlags(fs)

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
				fmt.Fprintln(os.Stderr, "Error: --app-store-version-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("alternative-distribution packages create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateAlternativeDistributionPackage(requestCtx, trimmedID)
			if err != nil {
				return fmt.Errorf("alternative-distribution packages create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// AlternativeDistributionPackagesAppStoreVersionCommand returns the app-store-version package subcommand.
func AlternativeDistributionPackagesAppStoreVersionCommand() *ffcli.Command {
	fs := flag.NewFlagSet("app-store-version", flag.ExitOnError)

	appStoreVersionID := fs.String("app-store-version-id", "", "App Store version ID")
	output := shared.BindOutputFlags(fs)

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
				fmt.Fprintln(os.Stderr, "Error: --app-store-version-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("alternative-distribution packages app-store-version: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAppStoreVersionAlternativeDistributionPackage(requestCtx, trimmedID)
			if err != nil {
				return fmt.Errorf("alternative-distribution packages app-store-version: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// AlternativeDistributionPackageVariantsCommand returns the package variant get subcommand.
func AlternativeDistributionPackageVariantsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("variants", flag.ExitOnError)

	variantID := fs.String("variant-id", "", "Alternative distribution package variant ID")
	output := shared.BindOutputFlags(fs)

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
				fmt.Fprintln(os.Stderr, "Error: --variant-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("alternative-distribution packages variants: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAlternativeDistributionPackageVariant(requestCtx, trimmedID)
			if err != nil {
				return fmt.Errorf("alternative-distribution packages variants: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// AlternativeDistributionPackageDeltasCommand returns the package delta get subcommand.
func AlternativeDistributionPackageDeltasCommand() *ffcli.Command {
	fs := flag.NewFlagSet("deltas", flag.ExitOnError)

	deltaID := fs.String("delta-id", "", "Alternative distribution package delta ID")
	output := shared.BindOutputFlags(fs)

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
				fmt.Fprintln(os.Stderr, "Error: --delta-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("alternative-distribution packages deltas: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAlternativeDistributionPackageDelta(requestCtx, trimmedID)
			if err != nil {
				return fmt.Errorf("alternative-distribution packages deltas: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

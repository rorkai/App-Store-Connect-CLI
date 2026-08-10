package routingcoverage

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

// RoutingCoverageCommand returns the routing coverage command group.
func RoutingCoverageCommand() *ffcli.Command {
	return &ffcli.Command{
		Name:       "routing-coverage",
		ShortUsage: "asc routing-coverage <subcommand> [flags]",
		ShortHelp:  "Manage routing app coverage files.",
		LongHelp: `Manage routing app coverage files required for routing apps.

Examples:
  asc routing-coverage view --version-id "VERSION_ID"
  asc routing-coverage info --id "COVERAGE_ID"
  asc routing-coverage create --version-id "VERSION_ID" --file ./coverage.geojson
  asc routing-coverage delete --id "COVERAGE_ID" --confirm`,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			RoutingCoverageGetCommand(),
			RoutingCoverageInfoCommand(),
			RoutingCoverageCreateCommand(),
			RoutingCoverageDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// RoutingCoverageGetCommand returns the routing coverage get subcommand.
func RoutingCoverageGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("routing-coverage view", flag.ExitOnError)

	versionID := fs.String("version-id", "", "App Store version ID (required)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc routing-coverage view --version-id \"VERSION_ID\"",
		ShortHelp:  "View routing app coverage for a version.",
		LongHelp: `View routing app coverage for an App Store version.

Examples:
  asc routing-coverage view --version-id "VERSION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			versionValue := strings.TrimSpace(*versionID)
			if versionValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return shared.MissingRequiredUsageError()
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("routing-coverage view: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetRoutingAppCoverageForVersion(requestCtx, versionValue)
			if err != nil {
				return fmt.Errorf("routing-coverage view: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// RoutingCoverageInfoCommand returns the routing coverage info subcommand.
func RoutingCoverageInfoCommand() *ffcli.Command {
	fs := flag.NewFlagSet("routing-coverage info", flag.ExitOnError)

	coverageID := fs.String("id", "", "Routing app coverage ID (required)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "info",
		ShortUsage: "asc routing-coverage info --id \"COVERAGE_ID\"",
		ShortHelp:  "Get routing app coverage by ID.",
		LongHelp: `Get routing app coverage by ID.

Examples:
  asc routing-coverage info --id "COVERAGE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			coverageValue := strings.TrimSpace(*coverageID)
			if coverageValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError()
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("routing-coverage info: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetRoutingAppCoverage(requestCtx, coverageValue)
			if err != nil {
				return fmt.Errorf("routing-coverage info: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// RoutingCoverageCreateCommand returns the routing coverage create subcommand.
func RoutingCoverageCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("routing-coverage create", flag.ExitOnError)

	versionID := fs.String("version-id", "", "App Store version ID (required)")
	filePath := fs.String("file", "", "Path to routing coverage file (required)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc routing-coverage create --version-id \"VERSION_ID\" --file ./coverage.geojson",
		ShortHelp:  "Upload routing app coverage for a version.",
		LongHelp: `Upload routing app coverage for an App Store version.

Examples:
  asc routing-coverage create --version-id "VERSION_ID" --file ./coverage.geojson`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			versionValue := strings.TrimSpace(*versionID)
			if versionValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return shared.MissingRequiredUsageError()
			}

			pathValue := strings.TrimSpace(*filePath)
			if pathValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --file is required")
				return shared.MissingRequiredUsageError()
			}

			prepared, err := PrepareRoutingCoverageFile(pathValue)
			if err != nil {
				return fmt.Errorf("routing-coverage create: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("routing-coverage create: %w", err)
			}

			commitResp, err := UploadPreparedRoutingCoverageFile(ctx, client, versionValue, prepared)
			if err != nil {
				return fmt.Errorf("routing-coverage create: %w", err)
			}

			return shared.PrintOutput(commitResp, *output.Output, *output.Pretty)
		},
	}
}

// RoutingCoverageDeleteCommand returns the routing coverage delete subcommand.
func RoutingCoverageDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("routing-coverage delete", flag.ExitOnError)

	coverageID := fs.String("id", "", "Routing app coverage ID (required)")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc routing-coverage delete --id \"COVERAGE_ID\" --confirm",
		ShortHelp:  "Delete routing app coverage.",
		LongHelp: `Delete routing app coverage.

Examples:
  asc routing-coverage delete --id "COVERAGE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			coverageValue := strings.TrimSpace(*coverageID)
			if coverageValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError()
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return shared.MissingRequiredUsageError()
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("routing-coverage delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteRoutingAppCoverage(requestCtx, coverageValue); err != nil {
				return fmt.Errorf("routing-coverage delete: failed to delete: %w", err)
			}

			result := &asc.RoutingAppCoverageDeleteResult{
				ID:      coverageValue,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

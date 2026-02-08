package routingcoverage

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
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
  asc routing-coverage get --version-id "VERSION_ID"
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
	fs := flag.NewFlagSet("routing-coverage get", flag.ExitOnError)

	versionID := fs.String("version-id", "", "App Store version ID (required)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc routing-coverage get --version-id \"VERSION_ID\"",
		ShortHelp:  "Get routing app coverage for a version.",
		LongHelp: `Get routing app coverage for an App Store version.

Examples:
  asc routing-coverage get --version-id "VERSION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			versionValue := strings.TrimSpace(*versionID)
			if versionValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("routing-coverage get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetRoutingAppCoverageForVersion(requestCtx, versionValue)
			if err != nil {
				return fmt.Errorf("routing-coverage get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// RoutingCoverageInfoCommand returns the routing coverage info subcommand.
func RoutingCoverageInfoCommand() *ffcli.Command {
	fs := flag.NewFlagSet("routing-coverage info", flag.ExitOnError)

	coverageID := fs.String("id", "", "Routing app coverage ID (required)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

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
				return flag.ErrHelp
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

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// RoutingCoverageCreateCommand returns the routing coverage create subcommand.
func RoutingCoverageCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("routing-coverage create", flag.ExitOnError)

	versionID := fs.String("version-id", "", "App Store version ID (required)")
	filePath := fs.String("file", "", "Path to routing coverage file (required)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

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
				return flag.ErrHelp
			}

			pathValue := strings.TrimSpace(*filePath)
			if pathValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --file is required")
				return flag.ErrHelp
			}

			info, err := os.Lstat(pathValue)
			if err != nil {
				return fmt.Errorf("routing-coverage create: %w", err)
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("routing-coverage create: refusing to read symlink %q", pathValue)
			}
			if info.IsDir() {
				return fmt.Errorf("routing-coverage create: %q is a directory", pathValue)
			}
			if info.Size() <= 0 {
				return fmt.Errorf("routing-coverage create: file size must be greater than 0")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("routing-coverage create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateRoutingAppCoverage(requestCtx, versionValue, filepath.Base(pathValue), info.Size())
			if err != nil {
				return fmt.Errorf("routing-coverage create: failed to create: %w", err)
			}
			if resp == nil || len(resp.Data.Attributes.UploadOperations) == 0 {
				return fmt.Errorf("routing-coverage create: no upload operations returned")
			}

			uploadCtx, uploadCancel := shared.ContextWithUploadTimeout(ctx)
			err = asc.ExecuteUploadOperations(uploadCtx, pathValue, resp.Data.Attributes.UploadOperations)
			uploadCancel()
			if err != nil {
				return fmt.Errorf("routing-coverage create: upload failed: %w", err)
			}

			checksum, err := asc.ComputeFileChecksum(pathValue, asc.ChecksumAlgorithmMD5)
			if err != nil {
				return fmt.Errorf("routing-coverage create: checksum failed: %w", err)
			}

			uploaded := true
			updateAttrs := asc.RoutingAppCoverageUpdateAttributes{
				SourceFileChecksum: &checksum.Hash,
				Uploaded:           &uploaded,
			}

			commitCtx, commitCancel := shared.ContextWithUploadTimeout(ctx)
			commitResp, err := client.UpdateRoutingAppCoverage(commitCtx, resp.Data.ID, updateAttrs)
			commitCancel()
			if err != nil {
				return fmt.Errorf("routing-coverage create: failed to commit upload: %w", err)
			}

			return shared.PrintOutput(commitResp, *output, *pretty)
		},
	}
}

// RoutingCoverageDeleteCommand returns the routing coverage delete subcommand.
func RoutingCoverageDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("routing-coverage delete", flag.ExitOnError)

	coverageID := fs.String("id", "", "Routing app coverage ID (required)")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

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
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
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

			return shared.PrintOutput(result, *output, *pretty)
		},
	}
}

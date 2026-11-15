package routingcoverage

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// NewRoutingCoverageCommand returns the routing coverage command group, built
// on d.
func NewRoutingCoverageCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("routing-coverage", flag.ExitOnError)

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
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newRoutingCoverageGetCommand(d),
			newRoutingCoverageInfoCommand(d),
			newRoutingCoverageCreateCommand(d),
			newRoutingCoverageDeleteCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

func newRoutingCoverageGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Deps:        d,
		FlagSetName: "routing-coverage view",
		Name:        "view",
		ShortUsage:  "asc routing-coverage view --version-id \"VERSION_ID\"",
		ShortHelp:   "View routing app coverage for a version.",
		LongHelp: `View routing app coverage for an App Store version.

Examples:
  asc routing-coverage view --version-id "VERSION_ID"`,
		IDFlag:             "version-id",
		IDUsage:            "App Store version ID (required)",
		IDType:             "appStoreVersions",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetRoutingAppCoverageForVersion(ctx, id)
		},
	})
}

func newRoutingCoverageInfoCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("routing-coverage info", flag.ExitOnError)

	coverageID := shared.BindResourceIDFlag(fs, "id", "routingAppCoverages", "Routing app coverage ID (required)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "info",
		ShortUsage: "asc routing-coverage info --id \"COVERAGE_ID\"",
		ShortHelp:  "Get routing app coverage by ID.",
		LongHelp: `Get routing app coverage by ID.

Examples:
  asc routing-coverage info --id "COVERAGE_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			coverageValue := strings.TrimSpace(*coverageID)
			if coverageValue == "" {
				return d.RequiredFlag("--id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("routing-coverage info: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.GetRoutingAppCoverage(requestCtx, coverageValue)
			if err != nil {
				return fmt.Errorf("routing-coverage info: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func newRoutingCoverageCreateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("routing-coverage create", flag.ExitOnError)

	versionID := shared.BindResourceIDFlag(fs, "version-id", "appStoreVersions", "App Store version ID (required)")
	filePath := fs.String("file", "", "Path to routing coverage file (required)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc routing-coverage create --version-id \"VERSION_ID\" --file ./coverage.geojson",
		ShortHelp:  "Upload routing app coverage for a version.",
		LongHelp: `Upload routing app coverage for an App Store version.

Examples:
  asc routing-coverage create --version-id "VERSION_ID" --file ./coverage.geojson`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			versionValue := strings.TrimSpace(*versionID)
			if versionValue == "" {
				return d.RequiredFlag("--version-id")
			}

			pathValue := strings.TrimSpace(*filePath)
			if pathValue == "" {
				return d.RequiredFlag("--file")
			}

			prepared, err := prepareRoutingCoverageFile(d, pathValue, asc.ComputeChecksumFromReader)
			if err != nil {
				return fmt.Errorf("routing-coverage create: %w", err)
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("routing-coverage create: %w", err)
			}

			commitResp, err := uploadPreparedRoutingCoverageFile(ctx, d, client, versionValue, "", prepared)
			if err != nil {
				return fmt.Errorf("routing-coverage create: %w", err)
			}

			return d.PrintOutput(commitResp, *output.Output, *output.Pretty)
		},
	}
}

func newRoutingCoverageDeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Deps:        d,
		FlagSetName: "routing-coverage delete",
		Name:        "delete",
		ShortUsage:  "asc routing-coverage delete --id \"COVERAGE_ID\" --confirm",
		ShortHelp:   "Delete routing app coverage.",
		LongHelp: `Delete routing app coverage.

Examples:
  asc routing-coverage delete --id "COVERAGE_ID" --confirm`,
		IDFlag:             "id",
		IDUsage:            "Routing app coverage ID (required)",
		IDType:             "routingAppCoverages",
		RequiredDiagnostic: true,
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteRoutingAppCoverage(ctx, id)
		},
		Result: func(id string) any {
			return &asc.DeleteResult{ID: id, Deleted: true}
		},
	})
}

package performance

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func newPerformanceMetricsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("metrics", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "metrics",
		ShortUsage: "asc performance metrics <subcommand> [flags]",
		ShortHelp:  "Work with performance/power metrics.",
		LongHelp: `Work with performance/power metrics.

Examples:
  asc performance metrics list --app "APP_ID"
  asc performance metrics view --build-id "BUILD_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newPerformanceMetricsListCommand(d),
			newPerformanceMetricsGetCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

func newPerformanceMetricsListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("metrics list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	platform := fs.String("platform", "", "Platform filter (IOS)")
	metricType := fs.String("metric-type", "", "Metric types (comma-separated: "+strings.Join(perfPowerMetricTypeList(), ", ")+")")
	deviceType := fs.String("device-type", "", "Device types (comma-separated, e.g., iPhone15,2)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc performance metrics list --app \"APP_ID\"",
		ShortHelp:  "List performance/power metrics for an app.",
		LongHelp: `List performance/power metrics for an app.

Examples:
  asc performance metrics list --app "APP_ID"
  asc performance metrics list --app "APP_ID" --metric-type "MEMORY,DISK" --device-type "iPhone15,2"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("--app")
			}

			platforms, err := normalizePerfPowerMetricPlatforms(shared.SplitCSVUpper(*platform), "--platform")
			if err != nil {
				return d.UsageErrorf("performance metrics list: %v", err)
			}
			metricTypes, err := normalizePerfPowerMetricTypes(shared.SplitCSVUpper(*metricType))
			if err != nil {
				return d.UsageErrorf("performance metrics list: %v", err)
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("performance metrics list: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.GetPerfPowerMetricsForApp(
				requestCtx, resolvedAppID,
				asc.WithPerfPowerMetricsPlatforms(platforms),
				asc.WithPerfPowerMetricsMetricTypes(metricTypes),
				asc.WithPerfPowerMetricsDeviceTypes(shared.SplitCSV(*deviceType)),
			)
			if err != nil {
				return fmt.Errorf("performance metrics list: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func newPerformanceMetricsGetCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("metrics view", flag.ExitOnError)

	buildID := shared.BindResourceIDFlag(fs, "build-id", "builds", "Build ID to fetch metrics for")
	platform := fs.String("platform", "", "Platform filter (IOS)")
	metricType := fs.String("metric-type", "", "Metric types (comma-separated: "+strings.Join(perfPowerMetricTypeList(), ", ")+")")
	deviceType := fs.String("device-type", "", "Device types (comma-separated, e.g., iPhone15,2)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc performance metrics view --build-id \"BUILD_ID\"",
		ShortHelp:  "View performance/power metrics for a build.",
		LongHelp: `View performance/power metrics for a build.

Examples:
  asc performance metrics view --build-id "BUILD_ID"
  asc performance metrics view --build-id "BUILD_ID" --metric-type "MEMORY" --device-type "iPhone15,2"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedBuildID := strings.TrimSpace(*buildID)
			if trimmedBuildID == "" {
				return d.RequiredFlag("--build-id")
			}

			platforms, err := normalizePerfPowerMetricPlatforms(shared.SplitCSVUpper(*platform), "--platform")
			if err != nil {
				return d.UsageErrorf("performance metrics view: %v", err)
			}
			metricTypes, err := normalizePerfPowerMetricTypes(shared.SplitCSVUpper(*metricType))
			if err != nil {
				return d.UsageErrorf("performance metrics view: %v", err)
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("performance metrics view: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.GetPerfPowerMetricsForBuild(
				requestCtx, trimmedBuildID,
				asc.WithPerfPowerMetricsPlatforms(platforms),
				asc.WithPerfPowerMetricsMetricTypes(metricTypes),
				asc.WithPerfPowerMetricsDeviceTypes(shared.SplitCSV(*deviceType)),
			)
			if err != nil {
				return fmt.Errorf("performance metrics view: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

var perfPowerMetricTypes = map[string]struct{}{
	string(asc.PerfPowerMetricTypeDisk):        {},
	string(asc.PerfPowerMetricTypeHang):        {},
	string(asc.PerfPowerMetricTypeBattery):     {},
	string(asc.PerfPowerMetricTypeLaunch):      {},
	string(asc.PerfPowerMetricTypeMemory):      {},
	string(asc.PerfPowerMetricTypeAnimation):   {},
	string(asc.PerfPowerMetricTypeTermination): {},
	string(asc.PerfPowerMetricTypeStorage):     {},
}

func perfPowerMetricTypeList() []string {
	return []string{
		string(asc.PerfPowerMetricTypeAnimation),
		string(asc.PerfPowerMetricTypeBattery),
		string(asc.PerfPowerMetricTypeDisk),
		string(asc.PerfPowerMetricTypeHang),
		string(asc.PerfPowerMetricTypeLaunch),
		string(asc.PerfPowerMetricTypeMemory),
		string(asc.PerfPowerMetricTypeStorage),
		string(asc.PerfPowerMetricTypeTermination),
	}
}

func normalizePerfPowerMetricTypes(values []string) ([]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	for _, value := range values {
		if _, ok := perfPowerMetricTypes[value]; !ok {
			return nil, fmt.Errorf("--metric-type must be one of: %s", strings.Join(perfPowerMetricTypeList(), ", "))
		}
	}
	return values, nil
}

var perfPowerMetricPlatforms = map[string]struct{}{
	string(asc.PlatformIOS): {},
}

func perfPowerMetricPlatformList() []string {
	return []string{
		string(asc.PlatformIOS),
	}
}

func normalizePerfPowerMetricPlatforms(values []string, flagName string) ([]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	for _, value := range values {
		if _, ok := perfPowerMetricPlatforms[value]; !ok {
			return nil, fmt.Errorf("%s must be one of: %s", flagName, strings.Join(perfPowerMetricPlatformList(), ", "))
		}
	}
	return values, nil
}

package performance

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

// PerformanceMetricsCommand returns the metrics subcommand group.
func PerformanceMetricsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("metrics", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "metrics",
		ShortUsage: "asc performance metrics <subcommand> [flags]",
		ShortHelp:  "Work with performance/power metrics.",
		LongHelp: `Work with performance/power metrics.

Examples:
  asc performance metrics list --app "APP_ID"
  asc performance metrics get --build "BUILD_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			PerformanceMetricsListCommand(),
			PerformanceMetricsGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// PerformanceMetricsListCommand returns the metrics list subcommand.
func PerformanceMetricsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("metrics list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	platform := fs.String("platform", "", "Platform filter (IOS)")
	metricType := fs.String("metric-type", "", "Metric types (comma-separated: "+strings.Join(perfPowerMetricTypeList(), ", ")+")")
	deviceType := fs.String("device-type", "", "Device types (comma-separated, e.g., iPhone15,2)")
	output := shared.BindOutputFlags(fs)

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
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			platforms, err := normalizePerfPowerMetricPlatforms(shared.SplitCSVUpper(*platform), "--platform")
			if err != nil {
				return fmt.Errorf("performance metrics list: %w", err)
			}
			metricTypes, err := normalizePerfPowerMetricTypes(shared.SplitCSVUpper(*metricType))
			if err != nil {
				return fmt.Errorf("performance metrics list: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("performance metrics list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetPerfPowerMetricsForApp(requestCtx, resolvedAppID,
				asc.WithPerfPowerMetricsPlatforms(platforms),
				asc.WithPerfPowerMetricsMetricTypes(metricTypes),
				asc.WithPerfPowerMetricsDeviceTypes(shared.SplitCSV(*deviceType)),
			)
			if err != nil {
				return fmt.Errorf("performance metrics list: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// PerformanceMetricsGetCommand returns the metrics get subcommand.
func PerformanceMetricsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("metrics get", flag.ExitOnError)

	buildID := fs.String("build", "", "Build ID to fetch metrics for")
	platform := fs.String("platform", "", "Platform filter (IOS)")
	metricType := fs.String("metric-type", "", "Metric types (comma-separated: "+strings.Join(perfPowerMetricTypeList(), ", ")+")")
	deviceType := fs.String("device-type", "", "Device types (comma-separated, e.g., iPhone15,2)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc performance metrics get --build \"BUILD_ID\"",
		ShortHelp:  "Get performance/power metrics for a build.",
		LongHelp: `Get performance/power metrics for a build.

Examples:
  asc performance metrics get --build "BUILD_ID"
  asc performance metrics get --build "BUILD_ID" --metric-type "MEMORY" --device-type "iPhone15,2"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedBuildID := strings.TrimSpace(*buildID)
			if trimmedBuildID == "" {
				fmt.Fprintln(os.Stderr, "Error: --build is required")
				return flag.ErrHelp
			}

			platforms, err := normalizePerfPowerMetricPlatforms(shared.SplitCSVUpper(*platform), "--platform")
			if err != nil {
				return fmt.Errorf("performance metrics get: %w", err)
			}
			metricTypes, err := normalizePerfPowerMetricTypes(shared.SplitCSVUpper(*metricType))
			if err != nil {
				return fmt.Errorf("performance metrics get: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("performance metrics get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetPerfPowerMetricsForBuild(requestCtx, trimmedBuildID,
				asc.WithPerfPowerMetricsPlatforms(platforms),
				asc.WithPerfPowerMetricsMetricTypes(metricTypes),
				asc.WithPerfPowerMetricsDeviceTypes(shared.SplitCSV(*deviceType)),
			)
			if err != nil {
				return fmt.Errorf("performance metrics get: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
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
}

func perfPowerMetricTypeList() []string {
	return []string{
		string(asc.PerfPowerMetricTypeAnimation),
		string(asc.PerfPowerMetricTypeBattery),
		string(asc.PerfPowerMetricTypeDisk),
		string(asc.PerfPowerMetricTypeHang),
		string(asc.PerfPowerMetricTypeLaunch),
		string(asc.PerfPowerMetricTypeMemory),
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

package performance

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// PerformanceOverviewCommand returns the app performance overview command.
func PerformanceOverviewCommand() *ffcli.Command {
	fs := flag.NewFlagSet("overview", flag.ExitOnError)
	appID := shared.BindResourceIDFlag(fs, "app", "apps", "App Store Connect app ID (or ASC_APP_ID)")
	deviceType := fs.String("device-type", "", "Device types (comma-separated)")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "overview", ShortUsage: "asc performance overview --app APP_ID [flags]", ShortHelp: "Read an app's Xcode performance overview.",
		LongHelp: `Read aggregated performance data, regressions, and trends for an app.

JSON output preserves Apple's complete Xcode overview, including metric datasets
and signatures. Table and Markdown output summarize the app and insight counts.
This endpoint has no pagination.

Examples:
  asc performance overview --app "APP_ID" --output json
  asc performance overview --app "APP_ID" --device-type "iPhone15,2"`,
		FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageErrorf("performance overview: unexpected argument %q", args[0])
			}
			resolved := strings.TrimSpace(shared.ResolveAppID(*appID))
			if resolved == "" {
				return shared.UsageError("performance overview: --app is required (or set ASC_APP_ID)")
			}
			devices := shared.SplitCSV(*deviceType)
			supplied := false
			fs.Visit(func(f *flag.Flag) { supplied = supplied || f.Name == "device-type" })
			if supplied && len(devices) == 0 {
				return shared.UsageError("performance overview: --device-type must not be empty")
			}
			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("performance overview: %w", err)
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			resp, err := client.GetPerformanceOverviewForApp(requestCtx, resolved, devices)
			if err != nil {
				return fmt.Errorf("performance overview: %w", err)
			}
			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

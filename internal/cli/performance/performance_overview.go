package performance

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func newPerformanceOverviewCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("overview", flag.ExitOnError)
	appID := shared.BindResourceIDFlag(fs, "app", "apps", "App Store Connect app ID (or ASC_APP_ID)")
	deviceType := fs.String("device-type", "", "Device types (comma-separated)")
	output := d.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "overview", ShortUsage: "asc performance overview --app APP_ID [flags]", ShortHelp: "Read an app's Xcode performance overview.",
		LongHelp: `Read aggregated performance data, regressions, and trends for an app.

JSON output preserves Apple's complete Xcode overview, including metric datasets
and signatures. Table and Markdown output summarize the app and insight counts.
This endpoint has no pagination.

Examples:
  asc performance overview --app "APP_ID" --output json
  asc performance overview --app "APP_ID" --device-type "iPhone15,2"`,
		FlagSet: fs, UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolved := strings.TrimSpace(d.ResolveAppID(*appID))
			if resolved == "" {
				return d.UsageError("performance overview: --app is required (or set ASC_APP_ID)")
			}
			devices := shared.SplitCSV(*deviceType)
			supplied := false
			fs.Visit(func(f *flag.Flag) { supplied = supplied || f.Name == "device-type" })
			if supplied && len(devices) == 0 {
				return d.UsageError("performance overview: --device-type must not be empty")
			}
			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("performance overview: %w", err)
			}
			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()
			resp, err := client.GetPerformanceOverviewForApp(requestCtx, resolved, devices)
			if err != nil {
				return fmt.Errorf("performance overview: %w", err)
			}
			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

package feedback

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

// Feedback command factory
func FeedbackCommand() *ffcli.Command {
	fs := flag.NewFlagSet("feedback", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")
	includeScreenshots := fs.Bool("include-screenshots", false, "Include screenshot URLs in feedback output")
	deviceModel := fs.String("device-model", "", "Filter by device model(s), comma-separated")
	osVersion := fs.String("os-version", "", "Filter by OS version(s), comma-separated")
	appPlatform := fs.String("app-platform", "", "Filter by app platform(s), comma-separated (IOS, MAC_OS, TV_OS, VISION_OS)")
	devicePlatform := fs.String("device-platform", "", "Filter by device platform(s), comma-separated (IOS, MAC_OS, TV_OS, VISION_OS)")
	buildID := fs.String("build", "", "Filter by build ID(s), comma-separated")
	buildPreRelease := fs.String("build-pre-release-version", "", "Filter by pre-release version ID(s), comma-separated")
	tester := fs.String("tester", "", "Filter by tester ID(s), comma-separated")
	sort := fs.String("sort", "", "Sort by createdDate or -createdDate")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")

	return &ffcli.Command{
		Name:       "feedback",
		ShortUsage: "asc feedback [flags]",
		ShortHelp:  "List TestFlight feedback from beta testers.",
		LongHelp: `List TestFlight feedback from beta testers.

This command fetches beta feedback screenshot submissions and comments.

Examples:
  asc feedback --app "123456789"
  asc feedback --app "123456789" --include-screenshots
  asc feedback --app "123456789" --device-model "iPhone15,3" --os-version "17.2"
  asc feedback --app "123456789" --sort -createdDate --limit 5
  asc feedback --next "<links.next>"
  asc feedback --app "123456789" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("feedback: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("feedback: %w", err)
			}
			if err := shared.ValidateSort(*sort, "createdDate", "-createdDate"); err != nil {
				return fmt.Errorf("feedback: %w", err)
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintf(os.Stderr, "Error: --app is required (or set ASC_APP_ID)\n\n")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("feedback: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.FeedbackOption{
				asc.WithFeedbackDeviceModels(shared.SplitCSV(*deviceModel)),
				asc.WithFeedbackOSVersions(shared.SplitCSV(*osVersion)),
				asc.WithFeedbackAppPlatforms(shared.SplitCSVUpper(*appPlatform)),
				asc.WithFeedbackDevicePlatforms(shared.SplitCSVUpper(*devicePlatform)),
				asc.WithFeedbackBuildIDs(shared.SplitCSV(*buildID)),
				asc.WithFeedbackBuildPreReleaseVersionIDs(shared.SplitCSV(*buildPreRelease)),
				asc.WithFeedbackTesterIDs(shared.SplitCSV(*tester)),
				asc.WithFeedbackLimit(*limit),
				asc.WithFeedbackNextURL(*next),
			}
			if strings.TrimSpace(*sort) != "" {
				opts = append(opts, asc.WithFeedbackSort(*sort))
			}
			if *includeScreenshots {
				opts = append(opts, asc.WithFeedbackIncludeScreenshots())
			}

			if *paginate {
				// Fetch first page with limit set for consistent pagination
				paginateOpts := append(opts, asc.WithFeedbackLimit(200))
				firstPage, err := client.GetFeedback(requestCtx, resolvedAppID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("feedback: failed to fetch: %w", err)
				}

				// Fetch all remaining pages
				feedback, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetFeedback(ctx, resolvedAppID, asc.WithFeedbackNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("feedback: %w", err)
				}

				format := *output
				return shared.PrintOutput(feedback, format, *pretty)
			}

			feedback, err := client.GetFeedback(requestCtx, resolvedAppID, opts...)
			if err != nil {
				return fmt.Errorf("feedback: failed to fetch: %w", err)
			}

			format := *output

			return shared.PrintOutput(feedback, format, *pretty)
		},
	}
}

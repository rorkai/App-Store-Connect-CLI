package crashes

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

// Crashes command factory
func CrashesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("crashes", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	output := shared.BindOutputFlags(fs)
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
		Name:       "crashes",
		ShortUsage: "asc crashes [flags]",
		ShortHelp:  "List and export TestFlight crash reports.",
		LongHelp: `List and export TestFlight crash reports.

This command fetches crash reports submitted by TestFlight beta testers,
helping you identify and fix issues in your app.

Examples:
  asc crashes --app "123456789"
  asc crashes --app "123456789" > crashes.json
  asc crashes --app "123456789" --device-model "iPhone15,3" --os-version "17.2"
  asc crashes --app "123456789" --sort -createdDate --limit 5
  asc crashes --next "<links.next>"
  asc crashes --app "123456789" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("crashes: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("crashes: %w", err)
			}
			if err := shared.ValidateSort(*sort, "createdDate", "-createdDate"); err != nil {
				return fmt.Errorf("crashes: %w", err)
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintf(os.Stderr, "Error: --app is required (or set ASC_APP_ID)\n\n")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("crashes: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.CrashOption{
				asc.WithCrashDeviceModels(shared.SplitCSV(*deviceModel)),
				asc.WithCrashOSVersions(shared.SplitCSV(*osVersion)),
				asc.WithCrashAppPlatforms(shared.SplitCSVUpper(*appPlatform)),
				asc.WithCrashDevicePlatforms(shared.SplitCSVUpper(*devicePlatform)),
				asc.WithCrashBuildIDs(shared.SplitCSV(*buildID)),
				asc.WithCrashBuildPreReleaseVersionIDs(shared.SplitCSV(*buildPreRelease)),
				asc.WithCrashTesterIDs(shared.SplitCSV(*tester)),
				asc.WithCrashLimit(*limit),
				asc.WithCrashNextURL(*next),
			}
			if strings.TrimSpace(*sort) != "" {
				opts = append(opts, asc.WithCrashSort(*sort))
			}

			if *paginate {
				// Fetch first page with limit set for consistent pagination
				paginateOpts := append(opts, asc.WithCrashLimit(200))
				firstPage, err := client.GetCrashes(requestCtx, resolvedAppID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("crashes: failed to fetch: %w", err)
				}

				// Fetch all remaining pages
				crashes, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetCrashes(ctx, resolvedAppID, asc.WithCrashNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("crashes: %w", err)
				}

				return shared.PrintOutput(crashes, *output.Output, *output.Pretty)
			}

			crashes, err := client.GetCrashes(requestCtx, resolvedAppID, opts...)
			if err != nil {
				return fmt.Errorf("crashes: failed to fetch: %w", err)
			}

			return shared.PrintOutput(crashes, *output.Output, *output.Pretty)
		},
	}
}

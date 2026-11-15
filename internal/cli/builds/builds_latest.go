package builds

import (
	"context"
	"flag"
	"fmt"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type latestBuildSelectionOptions = shared.LatestBuildSelectionOptions

type nextBuildNumberOptions = shared.NextBuildNumberOptions

// newBuildsNextBuildNumberCommand returns the canonical next build number subcommand.
func newBuildsNextBuildNumberCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("next-build-number", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID, bundle ID, or exact app name (required, or ASC_APP_ID env)")
	version := fs.String("version", "", "Optional version filter for latest processed/uploaded build selection")
	platform := fs.String("platform", "", "Optional platform filter: IOS, MAC_OS, TV_OS, VISION_OS")
	processingState := fs.String("processing-state", "", "Optional processing state filter: VALID, PROCESSING, FAILED, INVALID, or all")
	initialBuildNumber := fs.Int("initial-build-number", 1, "Initial build number when none exist")
	excludeExpired := fs.Bool("exclude-expired", false, "Exclude expired builds when selecting the latest processed build")
	notExpired := fs.Bool("not-expired", false, "Alias for --exclude-expired")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "next-build-number",
		ShortUsage: "asc builds next-build-number --app APP [flags]",
		ShortHelp:  "Calculate the next build number for an app.",
		LongHelp: `Calculate the next build number for an app.

This command compares the latest processed build and in-flight build uploads,
then returns the next build number that should be safe to use.

latestProcessedBuildNumber reports the most recently uploaded matching build
when its number is positive; zero-style placeholders are reported as null.
latestObservedBuildNumber and nextBuildNumber use the highest positive numeric
build number across processed builds and build uploads. If only zero-style
placeholders remain, all latest fields are null, sourcesConsidered is empty,
and nextBuildNumber uses --initial-build-number.

Examples:
  asc builds next-build-number --app "123456789"
  asc builds next-build-number --app "123456789" --version "1.2.3" --platform IOS
  asc builds next-build-number --app "123456789" --version "1.2.3" --platform IOS --exclude-expired
  asc builds next-build-number --app "123456789" --initial-build-number 7`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			excludeExpiredValue := *excludeExpired || *notExpired
			selectionOpts, err := normalizeLatestBuildSelectionOptions(d, *appID, *version, *platform, *processingState, excludeExpiredValue)
			if err != nil {
				return err
			}
			if *initialBuildNumber < 1 {
				return d.UsageError("--initial-build-number must be >= 1")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("builds next-build-number: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			result, err := resolveNextBuildNumber(requestCtx, d, client, nextBuildNumberOptions{
				LatestBuildSelectionOptions: selectionOpts,
				InitialBuildNumber:          *initialBuildNumber,
			})
			if err != nil {
				return fmt.Errorf("builds next-build-number: %w", err)
			}
			return d.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

func normalizeLatestBuildSelectionOptions(d *shared.Deps, appID, version, platform, processingState string, excludeExpired bool) (latestBuildSelectionOptions, error) {
	opts, err := d.NormalizeLatestBuildSelectionOptions(appID, version, platform, processingState, excludeExpired)
	if err != nil {
		return latestBuildSelectionOptions{}, err
	}
	return opts, nil
}

func resolveNextBuildNumber(ctx context.Context, d *shared.Deps, client *asc.Client, opts nextBuildNumberOptions) (*asc.BuildsNextBuildNumberResult, error) {
	return d.ResolveNextBuildNumber(ctx, client, opts)
}

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

type listCommandFlags struct {
	appID              *string
	output             shared.OutputFlags
	includeScreenshots *bool
	deviceModel        *string
	osVersion          *string
	appPlatform        *string
	devicePlatform     *string
	buildID            *string
	buildPreRelease    *string
	tester             *string
	sort               *string
	limit              *int
	next               *string
	paginate           *bool
}

func bindListCommandFlags(fs *flag.FlagSet) listCommandFlags {
	return listCommandFlags{
		appID:              fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)"),
		output:             shared.BindOutputFlags(fs),
		includeScreenshots: fs.Bool("include-screenshots", false, "Include screenshot URLs in feedback output"),
		deviceModel:        fs.String("device-model", "", "Filter by device model(s), comma-separated"),
		osVersion:          fs.String("os-version", "", "Filter by OS version(s), comma-separated"),
		appPlatform:        fs.String("app-platform", "", "Filter by app platform(s), comma-separated (IOS, MAC_OS, TV_OS, VISION_OS)"),
		devicePlatform:     fs.String("device-platform", "", "Filter by device platform(s), comma-separated (IOS, MAC_OS, TV_OS, VISION_OS)"),
		buildID:            fs.String("build", "", "Filter by build ID(s), comma-separated"),
		buildPreRelease:    fs.String("build-pre-release-version", "", "Filter by pre-release version ID(s), comma-separated"),
		tester:             fs.String("tester", "", "Filter by tester ID(s), comma-separated"),
		sort:               fs.String("sort", "", "Sort by createdDate or -createdDate"),
		limit:              fs.Int("limit", 0, "Maximum results per page (1-200)"),
		next:               fs.String("next", "", "Fetch next page using a links.next URL"),
		paginate:           fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)"),
	}
}

// NewListCommand builds a feedback list command with configurable help and warnings.
func NewListCommand(config shared.ListCommandConfig) *ffcli.Command {
	name := strings.TrimSpace(config.Name)
	if name == "" {
		name = "feedback"
	}
	usageFunc := config.UsageFunc
	if usageFunc == nil {
		usageFunc = shared.DefaultUsageFunc
	}

	fs := flag.NewFlagSet(name, flag.ExitOnError)
	flags := bindListCommandFlags(fs)

	return &ffcli.Command{
		Name:       name,
		ShortUsage: config.ShortUsage,
		ShortHelp:  config.ShortHelp,
		LongHelp:   config.LongHelp,
		FlagSet:    fs,
		UsageFunc:  usageFunc,
		Exec: func(ctx context.Context, args []string) error {
			return runListCommand(ctx, config, flags)
		},
	}
}

func runListCommand(ctx context.Context, config shared.ListCommandConfig, flags listCommandFlags) error {
	prefix := strings.TrimSpace(config.ErrorPrefix)
	if prefix == "" {
		prefix = "feedback"
	}
	if strings.TrimSpace(config.DeprecatedWarning) != "" {
		fmt.Fprintln(os.Stderr, config.DeprecatedWarning)
	}

	if *flags.limit != 0 && (*flags.limit < 1 || *flags.limit > 200) {
		return fmt.Errorf("%s: --limit must be between 1 and 200", prefix)
	}
	if err := shared.ValidateNextURL(*flags.next); err != nil {
		return fmt.Errorf("%s: %w", prefix, err)
	}
	if err := shared.ValidateSort(*flags.sort, "createdDate", "-createdDate"); err != nil {
		return fmt.Errorf("%s: %w", prefix, err)
	}

	resolvedAppID := shared.ResolveAppID(*flags.appID)
	if resolvedAppID == "" && strings.TrimSpace(*flags.next) == "" {
		fmt.Fprintf(os.Stderr, "Error: --app is required (or set ASC_APP_ID)\n\n")
		return flag.ErrHelp
	}

	client, err := shared.GetASCClient()
	if err != nil {
		return fmt.Errorf("%s: %w", prefix, err)
	}

	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	opts := []asc.FeedbackOption{
		asc.WithFeedbackDeviceModels(shared.SplitCSV(*flags.deviceModel)),
		asc.WithFeedbackOSVersions(shared.SplitCSV(*flags.osVersion)),
		asc.WithFeedbackAppPlatforms(shared.SplitCSVUpper(*flags.appPlatform)),
		asc.WithFeedbackDevicePlatforms(shared.SplitCSVUpper(*flags.devicePlatform)),
		asc.WithFeedbackBuildIDs(shared.SplitCSV(*flags.buildID)),
		asc.WithFeedbackBuildPreReleaseVersionIDs(shared.SplitCSV(*flags.buildPreRelease)),
		asc.WithFeedbackTesterIDs(shared.SplitCSV(*flags.tester)),
		asc.WithFeedbackLimit(*flags.limit),
		asc.WithFeedbackNextURL(*flags.next),
	}
	if strings.TrimSpace(*flags.sort) != "" {
		opts = append(opts, asc.WithFeedbackSort(*flags.sort))
	}
	if *flags.includeScreenshots {
		opts = append(opts, asc.WithFeedbackIncludeScreenshots())
	}

	if *flags.paginate {
		paginateOpts := append(opts, asc.WithFeedbackLimit(200))
		firstPage, err := client.GetFeedback(requestCtx, resolvedAppID, paginateOpts...)
		if err != nil {
			return fmt.Errorf("%s: failed to fetch: %w", prefix, err)
		}

		feedback, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
			return client.GetFeedback(ctx, resolvedAppID, asc.WithFeedbackNextURL(nextURL))
		})
		if err != nil {
			return fmt.Errorf("%s: %w", prefix, err)
		}

		return shared.PrintOutput(feedback, *flags.output.Output, *flags.output.Pretty)
	}

	feedback, err := client.GetFeedback(requestCtx, resolvedAppID, opts...)
	if err != nil {
		return fmt.Errorf("%s: failed to fetch: %w", prefix, err)
	}

	return shared.PrintOutput(feedback, *flags.output.Output, *flags.output.Pretty)
}

// Feedback command factory
func FeedbackCommand() *ffcli.Command {
	return NewListCommand(shared.ListCommandConfig{
		Name:       "feedback",
		ShortUsage: "asc testflight feedback list [flags]",
		ShortHelp:  "DEPRECATED: use `asc testflight feedback list`.",
		LongHelp: `DEPRECATED: use ` + "`asc testflight feedback list`" + `.

This compatibility shim preserves the legacy root feedback list behavior while
the canonical TestFlight surface moves under ` + "`asc testflight feedback ...`" + `.

Examples:
  asc testflight feedback list --app "123456789"
  asc testflight feedback list --app "123456789" --include-screenshots
  asc testflight feedback list --next "<links.next>"`,
		ErrorPrefix:       "feedback",
		DeprecatedWarning: "Warning: `asc feedback` is deprecated. Use `asc testflight feedback list`.",
		UsageFunc:         shared.DeprecatedUsageFunc,
	})
}

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

type listCommandFlags struct {
	appID           *string
	output          shared.OutputFlags
	deviceModel     *string
	osVersion       *string
	appPlatform     *string
	devicePlatform  *string
	buildID         *string
	buildPreRelease *string
	tester          *string
	sort            *string
	limit           *int
	next            *string
	paginate        *bool
}

func bindListCommandFlags(fs *flag.FlagSet) listCommandFlags {
	return listCommandFlags{
		appID:           fs.String("app", "", "App Store Connect app ID, bundle ID, or exact app name (or ASC_APP_ID env)"),
		output:          shared.BindOutputFlags(fs),
		deviceModel:     fs.String("device-model", "", "Filter by device model(s), comma-separated"),
		osVersion:       fs.String("os-version", "", "Filter by OS version(s), comma-separated"),
		appPlatform:     fs.String("app-platform", "", "Filter by app platform(s), comma-separated (IOS, MAC_OS, TV_OS, VISION_OS)"),
		devicePlatform:  fs.String("device-platform", "", "Filter by device platform(s), comma-separated (IOS, MAC_OS, TV_OS, VISION_OS)"),
		buildID:         fs.String("build", "", "Filter by build ID(s), comma-separated"),
		buildPreRelease: fs.String("build-pre-release-version", "", "Filter by pre-release version ID(s), comma-separated"),
		tester:          fs.String("tester", "", "Filter by tester ID(s), comma-separated"),
		sort:            fs.String("sort", "", "Sort by createdDate or -createdDate"),
		limit:           fs.Int("limit", 0, "Maximum results per page (1-200)"),
		next:            fs.String("next", "", "Fetch next page using a links.next URL"),
		paginate:        fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)"),
	}
}

// NewListCommand builds a crashes list command with configurable help and warnings.
func NewListCommand(config shared.ListCommandConfig) *ffcli.Command {
	name := strings.TrimSpace(config.Name)
	if name == "" {
		name = "crashes"
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
		prefix = "crashes"
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

	if resolvedAppID != "" && strings.TrimSpace(*flags.next) == "" {
		resolvedAppID, err = shared.ResolveAppIDWithLookup(requestCtx, client, resolvedAppID)
		if err != nil {
			return fmt.Errorf("%s: %w", prefix, err)
		}
	}

	opts := []asc.CrashOption{
		asc.WithCrashDeviceModels(shared.SplitCSV(*flags.deviceModel)),
		asc.WithCrashOSVersions(shared.SplitCSV(*flags.osVersion)),
		asc.WithCrashAppPlatforms(shared.SplitCSVUpper(*flags.appPlatform)),
		asc.WithCrashDevicePlatforms(shared.SplitCSVUpper(*flags.devicePlatform)),
		asc.WithCrashBuildIDs(shared.SplitCSV(*flags.buildID)),
		asc.WithCrashBuildPreReleaseVersionIDs(shared.SplitCSV(*flags.buildPreRelease)),
		asc.WithCrashTesterIDs(shared.SplitCSV(*flags.tester)),
		asc.WithCrashLimit(*flags.limit),
		asc.WithCrashNextURL(*flags.next),
	}
	if strings.TrimSpace(*flags.sort) != "" {
		opts = append(opts, asc.WithCrashSort(*flags.sort))
	}

	if *flags.paginate {
		paginateOpts := append(opts, asc.WithCrashLimit(200))
		firstPage, err := client.GetCrashes(requestCtx, resolvedAppID, paginateOpts...)
		if err != nil {
			return fmt.Errorf("%s: failed to fetch: %w", prefix, err)
		}

		crashes, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
			return client.GetCrashes(ctx, resolvedAppID, asc.WithCrashNextURL(nextURL))
		})
		if err != nil {
			return fmt.Errorf("%s: %w", prefix, err)
		}

		return shared.PrintOutput(crashes, *flags.output.Output, *flags.output.Pretty)
	}

	crashes, err := client.GetCrashes(requestCtx, resolvedAppID, opts...)
	if err != nil {
		return fmt.Errorf("%s: failed to fetch: %w", prefix, err)
	}

	return shared.PrintOutput(crashes, *flags.output.Output, *flags.output.Pretty)
}

// Crashes command factory
func CrashesCommand() *ffcli.Command {
	return NewListCommand(shared.ListCommandConfig{
		Name:       "crashes",
		ShortUsage: "asc testflight crashes list [flags]",
		ShortHelp:  "DEPRECATED: use `asc testflight crashes list`.",
		LongHelp: `DEPRECATED: use ` + "`asc testflight crashes list`" + `.

This compatibility shim preserves the legacy root crash list behavior while
the canonical TestFlight surface moves under ` + "`asc testflight crashes ...`" + `.

Examples:
  asc testflight crashes list --app "123456789"
  asc testflight crashes list --app "123456789" --device-model "iPhone15,3" --os-version "17.2"
  asc testflight crashes list --next "<links.next>"`,
		ErrorPrefix:       "crashes",
		DeprecatedWarning: "Warning: `asc crashes` is deprecated. Use `asc testflight crashes list`.",
		UsageFunc:         shared.DeprecatedUsageFunc,
	})
}

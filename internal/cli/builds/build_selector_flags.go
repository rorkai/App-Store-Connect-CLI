package builds

import (
	"context"
	"flag"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type buildSelectorFlags struct {
	buildID        *string
	appID          *string
	latest         *bool
	version        *string
	buildNumber    *string
	platform       *string
	excludeExpired *bool
	notExpired     *bool
}

type buildSelectorFlagOptions struct {
	buildIDUsage     string
	appUsage         string
	latestUsage      string
	versionUsage     string
	buildNumberUsage string
	platformUsage    string
}

func bindBuildSelectorFlags(fs *flag.FlagSet, opts buildSelectorFlagOptions) buildSelectorFlags {
	buildIDUsage := strings.TrimSpace(opts.buildIDUsage)
	if buildIDUsage == "" {
		buildIDUsage = "Build ID"
	}
	appUsage := strings.TrimSpace(opts.appUsage)
	if appUsage == "" {
		appUsage = "App Store Connect app ID, bundle ID, or exact app name (required when --build-id is not provided)"
	}
	latestUsage := strings.TrimSpace(opts.latestUsage)
	if latestUsage == "" {
		latestUsage = "Select the latest matching build for --app context"
	}
	versionUsage := strings.TrimSpace(opts.versionUsage)
	if versionUsage == "" {
		versionUsage = "Optional marketing version filter (CFBundleShortVersionString) for --app selectors"
	}
	buildNumberUsage := strings.TrimSpace(opts.buildNumberUsage)
	if buildNumberUsage == "" {
		buildNumberUsage = "Select a unique build by build number (CFBundleVersion) for --app context"
	}
	platformUsage := strings.TrimSpace(opts.platformUsage)
	if platformUsage == "" {
		platformUsage = "Platform filter for --app selectors (required with --build-number): IOS, MAC_OS, TV_OS, VISION_OS"
	}

	return buildSelectorFlags{
		buildID:        shared.BindResourceIDFlag(fs, "build-id", "builds", buildIDUsage),
		appID:          shared.BindResourceIDFlag(fs, "app", "apps", appUsage),
		latest:         fs.Bool("latest", false, latestUsage),
		version:        fs.String("version", "", versionUsage),
		buildNumber:    fs.String("build-number", "", buildNumberUsage),
		platform:       fs.String("platform", "", platformUsage),
		excludeExpired: fs.Bool("exclude-expired", false, "Exclude expired builds when selecting --latest"),
		notExpired:     fs.Bool("not-expired", false, "Alias for --exclude-expired"),
	}
}

func (s buildSelectorFlags) resolveOptions() ResolveBuildOptions {
	return ResolveBuildOptions{
		BuildID:     strings.TrimSpace(s.value(s.buildID)),
		AppID:       strings.TrimSpace(s.value(s.appID)),
		Version:     strings.TrimSpace(s.value(s.version)),
		BuildNumber: strings.TrimSpace(s.value(s.buildNumber)),
		Platform:    strings.TrimSpace(s.value(s.platform)),
		Latest:      s.latest != nil && *s.latest,
		ExcludeExpired: (s.excludeExpired != nil && *s.excludeExpired) ||
			(s.notExpired != nil && *s.notExpired),
	}
}

func (s buildSelectorFlags) validate() error {
	return validateResolveBuildOptions(s.resolveOptions())
}

func (s buildSelectorFlags) validateNextPageSelectorFlags() error {
	opts := s.resolveOptions()
	if opts.ExcludeExpired && !opts.Latest {
		return shared.UsageError("--exclude-expired and --not-expired require --latest")
	}
	return nil
}

func (s buildSelectorFlags) resolveBuild(ctx context.Context, client *asc.Client) (*asc.BuildResponse, error) {
	return ResolveBuild(ctx, client, s.resolveOptions())
}

func (s buildSelectorFlags) resolveBuildID(ctx context.Context, client *asc.Client) (string, error) {
	buildID := strings.TrimSpace(s.value(s.buildID))
	if buildID != "" {
		return buildID, nil
	}

	buildResp, err := s.resolveBuild(ctx, client)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(buildResp.Data.ID), nil
}

func (s buildSelectorFlags) value(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}

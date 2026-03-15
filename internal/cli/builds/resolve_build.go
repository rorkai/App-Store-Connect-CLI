package builds

import (
	"context"
	"fmt"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// ResolveBuildOptions configures how a build is resolved.
type ResolveBuildOptions struct {
	BuildID     string
	AppID       string
	Version     string
	BuildNumber string
	Platform    string
	Latest      bool
}

// ResolveBuild finds a build by ID, by app+build-number+platform, or by latest.
// Returns the build response or an error. Callers use this to avoid duplicating
// build lookup logic across commands (dsyms, wait, find, etc.).
func ResolveBuild(ctx context.Context, client *asc.Client, opts ResolveBuildOptions) (*asc.BuildResponse, error) {
	if err := validateResolveBuildOptions(opts); err != nil {
		return nil, err
	}
	if client == nil {
		return nil, fmt.Errorf("build client is required")
	}

	buildNumber := strings.TrimSpace(opts.BuildNumber)
	appID := shared.ResolveAppID(strings.TrimSpace(opts.AppID))

	// Direct build ID.
	if opts.BuildID != "" {
		resp, err := client.GetBuild(ctx, opts.BuildID)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch build %s: %w", opts.BuildID, err)
		}
		return resp, nil
	}

	platform := strings.TrimSpace(opts.Platform)
	if platform != "" {
		normalized, err := shared.NormalizeAppStoreVersionPlatform(platform)
		if err != nil {
			return nil, shared.UsageError(err.Error())
		}
		platform = normalized
	}

	resolvedAppID, err := shared.ResolveAppIDWithLookup(ctx, client, appID)
	if err != nil {
		return nil, err
	}

	version := strings.TrimSpace(opts.Version)

	// Latest mode: find the most recently uploaded build.
	if opts.Latest {
		buildOpts := []asc.BuildsOption{
			asc.WithBuildsSort("-uploadedDate"),
			asc.WithBuildsLimit(1),
		}
		if platform != "" {
			buildOpts = append(buildOpts, asc.WithBuildsPreReleaseVersionPlatforms([]string{platform}))
		}
		if version != "" {
			buildOpts = append(buildOpts, asc.WithBuildsPreReleaseVersionVersion(version))
		}

		buildsResp, err := client.GetBuilds(ctx, resolvedAppID, buildOpts...)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch latest build: %w", err)
		}
		if len(buildsResp.Data) == 0 {
			return nil, fmt.Errorf("no builds found for app %s", resolvedAppID)
		}
		return &asc.BuildResponse{Data: buildsResp.Data[0], Links: buildsResp.Links}, nil
	}

	// Build number mode: find by app + build number + platform.
	buildOpts := []asc.BuildsOption{
		asc.WithBuildsBuildNumber(buildNumber),
		asc.WithBuildsSort("-uploadedDate"),
		asc.WithBuildsLimit(1),
	}
	if platform != "" {
		buildOpts = append(buildOpts, asc.WithBuildsPreReleaseVersionPlatforms([]string{platform}))
	}
	if version != "" {
		buildOpts = append(buildOpts, asc.WithBuildsPreReleaseVersionVersion(version))
	}

	buildsResp, err := client.GetBuilds(ctx, resolvedAppID, buildOpts...)
	if err != nil {
		return nil, err
	}
	if len(buildsResp.Data) == 0 {
		return nil, fmt.Errorf("no build found for app %s with build number %q", resolvedAppID, buildNumber)
	}

	return &asc.BuildResponse{Data: buildsResp.Data[0], Links: buildsResp.Links}, nil
}

func validateResolveBuildOptions(opts ResolveBuildOptions) error {
	buildID := strings.TrimSpace(opts.BuildID)
	buildNumber := strings.TrimSpace(opts.BuildNumber)
	version := strings.TrimSpace(opts.Version)
	platform := strings.TrimSpace(opts.Platform)
	appInput := strings.TrimSpace(opts.AppID)
	hasExplicitAppSelectors := appInput != "" || opts.Latest || buildNumber != "" || version != "" || platform != ""

	if buildID != "" && hasExplicitAppSelectors {
		return shared.UsageError("--build cannot be combined with --app, --latest, --build-number, --version, or --platform")
	}
	if opts.Latest && buildNumber != "" {
		return shared.UsageError("--latest and --build-number are mutually exclusive")
	}
	if buildID != "" {
		return nil
	}

	if shared.ResolveAppID(appInput) == "" {
		return shared.UsageError("--build or --app is required (or set ASC_APP_ID)")
	}
	if !opts.Latest && buildNumber == "" {
		return shared.UsageError("--build, --latest, or --build-number is required")
	}
	if platform != "" {
		if _, err := shared.NormalizeAppStoreVersionPlatform(platform); err != nil {
			return shared.UsageError(err.Error())
		}
	}
	return nil
}

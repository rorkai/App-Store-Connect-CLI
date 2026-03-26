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
	BuildID               string
	AppID                 string
	Version               string
	BuildNumber           string
	Platform              string
	Latest                bool
	ProcessingStateValues []string
	ExcludeExpired        bool
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

	// Direct build ID.
	if opts.BuildID != "" {
		resp, err := client.GetBuild(ctx, opts.BuildID)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch build %s: %w", opts.BuildID, err)
		}
		return resp, nil
	}

	// Latest mode: find the most recently uploaded build.
	if opts.Latest {
		platform := strings.TrimSpace(opts.Platform)
		if platform != "" {
			normalized, err := shared.NormalizeAppStoreVersionPlatform(platform)
			if err != nil {
				return nil, shared.UsageError(err.Error())
			}
			platform = normalized
		}
		selection, err := resolveLatestBuildSelection(ctx, client, latestBuildSelectionOptions{
			AppID:                 strings.TrimSpace(opts.AppID),
			Version:               strings.TrimSpace(opts.Version),
			Platform:              platform,
			ProcessingStateValues: opts.ProcessingStateValues,
			ExcludeExpired:        opts.ExcludeExpired,
		}, false)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch latest build: %w", err)
		}
		return selection.LatestBuild, nil
	}

	appID := shared.ResolveAppID(strings.TrimSpace(opts.AppID))
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
	hasExplicitAppSelectors := appInput != "" || opts.Latest || buildNumber != "" || version != "" || platform != "" || len(opts.ProcessingStateValues) > 0 || opts.ExcludeExpired

	if buildID != "" && hasExplicitAppSelectors {
		return shared.UsageError("--build-id cannot be combined with --app, --latest, --build-number, --version, --platform, --processing-state, --exclude-expired, or --not-expired")
	}
	if opts.Latest && buildNumber != "" {
		return shared.UsageError("--latest and --build-number are mutually exclusive")
	}
	if buildID != "" {
		return nil
	}

	if shared.ResolveAppID(appInput) == "" {
		return shared.UsageError("--build-id or --app is required (or set ASC_APP_ID)")
	}
	if len(opts.ProcessingStateValues) > 0 && !opts.Latest {
		return shared.UsageError("--processing-state requires --latest")
	}
	if opts.ExcludeExpired && !opts.Latest {
		return shared.UsageError("--exclude-expired and --not-expired require --latest")
	}
	if !opts.Latest && buildNumber == "" {
		return shared.UsageError("--build-id, --latest, or --build-number is required")
	}
	if platform != "" {
		if _, err := shared.NormalizeAppStoreVersionPlatform(platform); err != nil {
			return shared.UsageError(err.Error())
		}
	}
	return nil
}

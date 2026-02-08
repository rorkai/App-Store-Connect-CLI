package builds

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

// BuildsLatestCommand returns the builds latest subcommand.
func BuildsLatestCommand() *ffcli.Command {
	fs := flag.NewFlagSet("latest", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (required, or ASC_APP_ID env)")
	version := fs.String("version", "", "Filter by version string (e.g., 1.2.3); requires --platform for deterministic results")
	platform := fs.String("platform", "", "Filter by platform: IOS, MAC_OS, TV_OS, VISION_OS")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "latest",
		ShortUsage: "asc builds latest [flags]",
		ShortHelp:  "Get the latest build for an app.",
		LongHelp: `Get the latest build for an app.

Returns the most recently uploaded build with full metadata including
build number, version, processing state, and upload date.

This command is useful for CI/CD scripts and AI agents that need to
query the current build state before uploading a new build.

Platform and version filtering:
  --platform alone    Returns latest build for the specified platform
  --version alone     Returns latest build for that version (may be any platform)
  --platform + --version  Returns latest build matching both (recommended)

Examples:
  # Get latest build (JSON output for AI agents)
  asc builds latest --app "123456789"

  # Get latest build for a specific version and platform (recommended)
  asc builds latest --app "123456789" --version "1.2.3" --platform IOS

  # Get latest build for a platform (any version)
  asc builds latest --app "123456789" --platform IOS

  # Get latest build for a version (any platform - nondeterministic if multi-platform)
  asc builds latest --app "123456789" --version "1.2.3"

  # Human-readable output
  asc builds latest --app "123456789" --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintf(os.Stderr, "Error: --app is required (or set ASC_APP_ID)\n\n")
				return flag.ErrHelp
			}

			// Normalize and validate platform if provided
			normalizedPlatform := ""
			if strings.TrimSpace(*platform) != "" {
				validPlatforms := []string{"IOS", "MAC_OS", "TV_OS", "VISION_OS"}
				normalizedPlatform = strings.ToUpper(strings.TrimSpace(*platform))
				valid := false
				for _, p := range validPlatforms {
					if normalizedPlatform == p {
						valid = true
						break
					}
				}
				if !valid {
					fmt.Fprintf(os.Stderr, "Error: --platform must be one of: IOS, MAC_OS, TV_OS, VISION_OS\n\n")
					return flag.ErrHelp
				}
			}

			normalizedVersion := strings.TrimSpace(*version)

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("builds latest: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			// Determine which preReleaseVersion(s) to filter by
			var preReleaseVersionIDs []string

			if normalizedVersion != "" || normalizedPlatform != "" {
				// Need to look up preReleaseVersions with the specified filters
				preReleaseVersionIDs, err = findPreReleaseVersionIDs(requestCtx, client, resolvedAppID, normalizedVersion, normalizedPlatform)
				if err != nil {
					return fmt.Errorf("builds latest: %w", err)
				}
				if len(preReleaseVersionIDs) == 0 {
					if normalizedVersion != "" && normalizedPlatform != "" {
						return fmt.Errorf("builds latest: no pre-release version found for version %q on platform %s", normalizedVersion, normalizedPlatform)
					} else if normalizedVersion != "" {
						return fmt.Errorf("builds latest: no pre-release version found for version %q", normalizedVersion)
					} else {
						return fmt.Errorf("builds latest: no pre-release version found for platform %s", normalizedPlatform)
					}
				}
			}

			// Get latest build with sort by uploadedDate descending
			// If we have preReleaseVersion filter(s), we need to find the latest across them
			var latestBuild *asc.BuildResponse

			if len(preReleaseVersionIDs) == 0 {
				// No filters - just get the latest build for the app
				opts := []asc.BuildsOption{
					asc.WithBuildsSort("-uploadedDate"),
					asc.WithBuildsLimit(1),
				}
				builds, err := client.GetBuilds(requestCtx, resolvedAppID, opts...)
				if err != nil {
					return fmt.Errorf("builds latest: failed to fetch: %w", err)
				}
				if len(builds.Data) == 0 {
					return fmt.Errorf("builds latest: no builds found for app %s", resolvedAppID)
				}
				latestBuild = &asc.BuildResponse{
					Data:  builds.Data[0],
					Links: builds.Links,
				}
			} else if len(preReleaseVersionIDs) == 1 {
				// Single preReleaseVersion - straightforward query
				opts := []asc.BuildsOption{
					asc.WithBuildsSort("-uploadedDate"),
					asc.WithBuildsLimit(1),
					asc.WithBuildsPreReleaseVersion(preReleaseVersionIDs[0]),
				}
				builds, err := client.GetBuilds(requestCtx, resolvedAppID, opts...)
				if err != nil {
					return fmt.Errorf("builds latest: failed to fetch: %w", err)
				}
				if len(builds.Data) == 0 {
					return fmt.Errorf("builds latest: no builds found matching filters")
				}
				latestBuild = &asc.BuildResponse{
					Data:  builds.Data[0],
					Links: builds.Links,
				}
			} else {
				// Multiple preReleaseVersions (platform filter without version filter)
				// Query each and find the one with the most recent uploadedDate
				var newestBuild *asc.Resource[asc.BuildAttributes]
				var newestDate string

				for _, prvID := range preReleaseVersionIDs {
					opts := []asc.BuildsOption{
						asc.WithBuildsSort("-uploadedDate"),
						asc.WithBuildsLimit(1),
						asc.WithBuildsPreReleaseVersion(prvID),
					}
					builds, err := client.GetBuilds(requestCtx, resolvedAppID, opts...)
					if err != nil {
						return fmt.Errorf("builds latest: failed to fetch: %w", err)
					}
					if len(builds.Data) > 0 {
						if newestBuild == nil || builds.Data[0].Attributes.UploadedDate > newestDate {
							newestBuild = &builds.Data[0]
							newestDate = builds.Data[0].Attributes.UploadedDate
						}
					}
				}

				if newestBuild == nil {
					return fmt.Errorf("builds latest: no builds found matching filters")
				}
				latestBuild = &asc.BuildResponse{
					Data: *newestBuild,
				}
			}

			return shared.PrintOutput(latestBuild, *output, *pretty)
		},
	}
}

// findPreReleaseVersionIDs looks up preReleaseVersion IDs for given filters.
// Returns all matching IDs when only platform is specified (paginates to get all),
// or a single ID when version is specified.
func findPreReleaseVersionIDs(ctx context.Context, client *asc.Client, appID, version, platform string) ([]string, error) {
	opts := []asc.PreReleaseVersionsOption{}

	if version != "" {
		opts = append(opts, asc.WithPreReleaseVersionsVersion(version))
		// When version is specified, we only need one result (platform narrows it further)
		opts = append(opts, asc.WithPreReleaseVersionsLimit(1))
	} else {
		// When only platform is specified, use max limit for pagination
		opts = append(opts, asc.WithPreReleaseVersionsLimit(200))
	}

	if platform != "" {
		opts = append(opts, asc.WithPreReleaseVersionsPlatform(platform))
	}

	// Get first page
	firstPage, err := client.GetPreReleaseVersions(ctx, appID, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup pre-release versions: %w", err)
	}

	// If version is specified, we only need the first result
	if version != "" {
		if len(firstPage.Data) == 0 {
			return nil, nil
		}
		return []string{firstPage.Data[0].ID}, nil
	}

	// For platform-only filtering, paginate to get ALL preReleaseVersions
	allVersions, err := asc.PaginateAll(ctx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return client.GetPreReleaseVersions(ctx, appID, asc.WithPreReleaseVersionsNextURL(nextURL))
	})
	if err != nil {
		return nil, fmt.Errorf("failed to paginate pre-release versions: %w", err)
	}

	// Extract IDs from paginated results
	versionsResp := allVersions.(*asc.PreReleaseVersionsResponse)
	ids := make([]string, len(versionsResp.Data))
	for i, v := range versionsResp.Data {
		ids[i] = v.ID
	}

	return ids, nil
}

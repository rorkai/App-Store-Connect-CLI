package builds

import (
	"context"
	"flag"
	"fmt"
	"math"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// BuildsLatestCommand returns the builds latest subcommand.
func BuildsLatestCommand() *ffcli.Command {
	fs := flag.NewFlagSet("latest", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID, bundle ID, or exact app name (required, or ASC_APP_ID env)")
	version := fs.String("version", "", "Filter by version string (e.g., 1.2.3); requires --platform for deterministic results")
	platform := fs.String("platform", "", "Filter by platform: IOS, MAC_OS, TV_OS, VISION_OS")
	output := shared.BindOutputFlags(fs)
	next := fs.Bool("next", false, "Return next build number using processed builds and in-flight uploads")
	initialBuildNumber := fs.Int("initial-build-number", 1, "Initial build number when none exist (used with --next)")
	excludeExpired := fs.Bool("exclude-expired", false, "Exclude expired builds when selecting latest build")
	notExpired := fs.Bool("not-expired", false, "Alias for --exclude-expired")

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

Next build number mode:
  --next              Returns the next build number (latest + 1) using
                      processed builds and in-flight uploads
  --initial-build-number  Starting build number when no history exists (default: 1)
  --exclude-expired   Ignore expired builds when selecting the latest processed build
  --not-expired       Alias for --exclude-expired

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
  asc builds latest --app "123456789" --output table

  # Collision-safe next build number for CI
  asc builds latest --app "123456789" --version "1.2.3" --platform IOS --next

  # Exclude expired builds when resolving latest or next
  asc builds latest --app "123456789" --version "1.2.3" --platform IOS --next --exclude-expired
  asc builds latest --app "123456789" --version "1.2.3" --platform IOS --not-expired`,
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
				valid := slices.Contains(validPlatforms, normalizedPlatform)
				if !valid {
					fmt.Fprintf(os.Stderr, "Error: --platform must be one of: IOS, MAC_OS, TV_OS, VISION_OS\n\n")
					return flag.ErrHelp
				}
			}

			normalizedVersion := strings.TrimSpace(*version)
			if *initialBuildNumber < 1 {
				fmt.Fprintf(os.Stderr, "Error: --initial-build-number must be >= 1\n\n")
				return flag.ErrHelp
			}
			excludeExpiredValue := *excludeExpired || *notExpired

			hasPreReleaseFilters := normalizedVersion != "" || normalizedPlatform != ""

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("builds latest: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resolvedAppID, err = shared.ResolveAppIDWithLookup(requestCtx, client, resolvedAppID)
			if err != nil {
				return fmt.Errorf("builds latest: %w", err)
			}

			// Determine which preReleaseVersion(s) to filter by
			var preReleaseVersionIDs []string

			if hasPreReleaseFilters {
				// Need to look up preReleaseVersions with the specified filters
				preReleaseVersionIDs, err = findPreReleaseVersionIDs(requestCtx, client, resolvedAppID, normalizedVersion, normalizedPlatform)
				if err != nil {
					return fmt.Errorf("builds latest: %w", err)
				}
				if len(preReleaseVersionIDs) == 0 {
					if !*next {
						if normalizedVersion != "" && normalizedPlatform != "" {
							return fmt.Errorf("builds latest: no pre-release version found for version %q on platform %s", normalizedVersion, normalizedPlatform)
						} else if normalizedVersion != "" {
							return fmt.Errorf("builds latest: no pre-release version found for version %q", normalizedVersion)
						} else {
							return fmt.Errorf("builds latest: no pre-release version found for platform %s", normalizedPlatform)
						}
					}
				}
			}

			// Get latest build with sort by uploadedDate descending
			// If we have preReleaseVersion filter(s), we need to find the latest across them
			var latestBuild *asc.BuildResponse

			if !hasPreReleaseFilters {
				// No filters - just get the latest build for the app
				opts := []asc.BuildsOption{
					asc.WithBuildsSort("-uploadedDate"),
					asc.WithBuildsLimit(1),
				}
				if excludeExpiredValue {
					opts = append(opts, asc.WithBuildsExpired(false))
				}
				builds, err := client.GetBuilds(requestCtx, resolvedAppID, opts...)
				if err != nil {
					return fmt.Errorf("builds latest: failed to fetch: %w", err)
				}
				if len(builds.Data) == 0 {
					if !*next {
						return fmt.Errorf("builds latest: no builds found for app %s", resolvedAppID)
					}
				} else {
					latestBuild = &asc.BuildResponse{
						Data:  builds.Data[0],
						Links: builds.Links,
					}
				}
			} else if len(preReleaseVersionIDs) == 1 {
				// Single preReleaseVersion - straightforward query
				opts := []asc.BuildsOption{
					asc.WithBuildsSort("-uploadedDate"),
					asc.WithBuildsLimit(1),
					asc.WithBuildsPreReleaseVersion(preReleaseVersionIDs[0]),
				}
				if excludeExpiredValue {
					opts = append(opts, asc.WithBuildsExpired(false))
				}
				builds, err := client.GetBuilds(requestCtx, resolvedAppID, opts...)
				if err != nil {
					return fmt.Errorf("builds latest: failed to fetch: %w", err)
				}
				if len(builds.Data) == 0 {
					if !*next {
						return fmt.Errorf("builds latest: no builds found matching filters")
					}
				} else {
					latestBuild = &asc.BuildResponse{
						Data:  builds.Data[0],
						Links: builds.Links,
					}
				}
			} else if len(preReleaseVersionIDs) > 1 {
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
					if excludeExpiredValue {
						opts = append(opts, asc.WithBuildsExpired(false))
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
					if !*next {
						return fmt.Errorf("builds latest: no builds found matching filters")
					}
				} else {
					latestBuild = &asc.BuildResponse{
						Data: *newestBuild,
					}
				}
			}

			if !*next {
				return shared.PrintOutput(latestBuild, *output.Output, *output.Pretty)
			}

			var latestProcessedNumber *string
			var latestUploadNumber *string
			var latestObservedNumber *string
			sourcesConsidered := make([]string, 0, 2)

			var latestProcessedValue buildNumber
			hasProcessed := false
			if latestBuild != nil {
				parsed, err := parseBuildNumber(latestBuild.Data.Attributes.Version, fmt.Sprintf("processed build %s", latestBuild.Data.ID))
				if err != nil {
					return fmt.Errorf("builds latest: %w", err)
				}
				latestProcessedValue = parsed
				value := parsed.String()
				latestProcessedNumber = &value
				hasProcessed = true
				sourcesConsidered = append(sourcesConsidered, "processed_builds")
			}

			latestUploadValue, latestUploadNumber, hasUpload, err := findLatestBuildUploadNumber(
				requestCtx,
				client,
				resolvedAppID,
				normalizedVersion,
				normalizedPlatform,
			)
			if err != nil {
				return fmt.Errorf("builds latest: %w", err)
			}
			if hasUpload {
				sourcesConsidered = append(sourcesConsidered, "build_uploads")
			}

			var latestObservedValue buildNumber
			hasObserved := false
			if hasProcessed {
				latestObservedValue = latestProcessedValue
				hasObserved = true
				latestObservedNumber = latestProcessedNumber
			}
			if hasUpload && (!hasObserved || latestUploadValue.Compare(latestObservedValue) > 0) {
				latestObservedValue = latestUploadValue
				hasObserved = true
				latestObservedNumber = latestUploadNumber
			}

			nextBuildNumberValue := strconv.FormatInt(int64(*initialBuildNumber), 10)
			if hasObserved {
				nextValue, err := latestObservedValue.Next()
				if err != nil {
					return fmt.Errorf("builds latest: %w", err)
				}
				nextBuildNumberValue = nextValue.String()
			}

			result := &asc.BuildsLatestNextResult{
				LatestProcessedBuildNumber: latestProcessedNumber,
				LatestUploadBuildNumber:    latestUploadNumber,
				LatestObservedBuildNumber:  latestObservedNumber,
				NextBuildNumber:            nextBuildNumberValue,
				SourcesConsidered:          sourcesConsidered,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
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

	// For platform-only filtering, stream pages and keep only IDs.
	ids := make([]string, 0, len(firstPage.Data))
	appendIDs := func(page *asc.PreReleaseVersionsResponse) {
		for _, preReleaseVersion := range page.Data {
			ids = append(ids, preReleaseVersion.ID)
		}
	}

	err = asc.PaginateEach(ctx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return client.GetPreReleaseVersions(ctx, appID, asc.WithPreReleaseVersionsNextURL(nextURL))
	}, func(page asc.PaginatedResponse) error {
		resp, ok := page.(*asc.PreReleaseVersionsResponse)
		if !ok {
			return fmt.Errorf("unexpected pre-release versions page type %T", page)
		}
		appendIDs(resp)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to paginate pre-release versions: %w", err)
	}

	return ids, nil
}

func findLatestBuildUploadNumber(
	ctx context.Context,
	client *asc.Client,
	appID, version, platform string,
) (buildNumber, *string, bool, error) {
	opts := []asc.BuildUploadsOption{
		asc.WithBuildUploadsStates([]string{"AWAITING_UPLOAD", "PROCESSING", "COMPLETE"}),
		asc.WithBuildUploadsLimit(200),
	}
	if strings.TrimSpace(version) != "" {
		opts = append(opts, asc.WithBuildUploadsCFBundleShortVersionStrings([]string{version}))
	}
	if strings.TrimSpace(platform) != "" {
		opts = append(opts, asc.WithBuildUploadsPlatforms([]string{platform}))
	}

	uploads, err := client.GetBuildUploads(ctx, appID, opts...)
	if err != nil {
		return buildNumber{}, nil, false, fmt.Errorf("failed to fetch build uploads: %w", err)
	}

	var latestUploadValue buildNumber
	var latestUploadNumber *string
	hasUpload := false

	processPage := func(page *asc.BuildUploadsResponse) error {
		for _, upload := range page.Data {
			parsed, err := parseBuildNumber(upload.Attributes.CFBundleVersion, fmt.Sprintf("build upload %s", upload.ID))
			if err != nil {
				return err
			}
			if !hasUpload || parsed.Compare(latestUploadValue) > 0 {
				latestUploadValue = parsed
				value := parsed.String()
				latestUploadNumber = &value
				hasUpload = true
			}
		}
		return nil
	}

	err = asc.PaginateEach(ctx, uploads, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return client.GetBuildUploads(ctx, appID, asc.WithBuildUploadsNextURL(nextURL))
	}, func(page asc.PaginatedResponse) error {
		resp, ok := page.(*asc.BuildUploadsResponse)
		if !ok {
			return fmt.Errorf("unexpected build uploads page type %T", page)
		}
		return processPage(resp)
	})
	if err != nil {
		return buildNumber{}, nil, false, fmt.Errorf("failed to paginate build uploads: %w", err)
	}

	return latestUploadValue, latestUploadNumber, hasUpload, nil
}

type buildNumber struct {
	components []int64
}

func (n buildNumber) String() string {
	if len(n.components) == 0 {
		return ""
	}
	parts := make([]string, len(n.components))
	for i, component := range n.components {
		parts[i] = strconv.FormatInt(component, 10)
	}
	return strings.Join(parts, ".")
}

func (n buildNumber) Compare(other buildNumber) int {
	maxLen := len(n.components)
	if len(other.components) > maxLen {
		maxLen = len(other.components)
	}
	for i := 0; i < maxLen; i++ {
		var left int64
		if i < len(n.components) {
			left = n.components[i]
		}
		var right int64
		if i < len(other.components) {
			right = other.components[i]
		}
		if left > right {
			return 1
		}
		if left < right {
			return -1
		}
	}
	return 0
}

func (n buildNumber) Next() (buildNumber, error) {
	if len(n.components) == 0 {
		return buildNumber{}, fmt.Errorf("build number is missing (expected a positive integer)")
	}
	nextComponents := make([]int64, len(n.components))
	copy(nextComponents, n.components)
	last := len(nextComponents) - 1
	if nextComponents[last] == math.MaxInt64 {
		return buildNumber{}, fmt.Errorf("build number %q is too large to increment", n.String())
	}
	nextComponents[last]++
	return buildNumber{components: nextComponents}, nil
}

func parseBuildNumber(raw, source string) (buildNumber, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return buildNumber{}, fmt.Errorf("%s build number is missing (expected a positive integer)", source)
	}

	segments := strings.Split(trimmed, ".")
	components := make([]int64, 0, len(segments))
	for _, segment := range segments {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			return buildNumber{}, fmt.Errorf("%s build number %q is not numeric (expected a positive integer)", source, raw)
		}
		for _, ch := range segment {
			if ch < '0' || ch > '9' {
				return buildNumber{}, fmt.Errorf("%s build number %q is not numeric (expected a positive integer)", source, raw)
			}
		}
		value, err := strconv.ParseInt(segment, 10, 64)
		if err != nil {
			return buildNumber{}, fmt.Errorf("%s build number %q is not numeric (expected a positive integer)", source, raw)
		}
		components = append(components, value)
	}

	if len(components) == 0 || components[0] < 1 {
		return buildNumber{}, fmt.Errorf("%s build number %q must be >= 1", source, raw)
	}

	return buildNumber{components: components}, nil
}

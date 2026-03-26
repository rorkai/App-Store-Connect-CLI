package builds

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const (
	deprecatedBuildsLatestFetchWarning = "Warning: `asc builds latest` is deprecated. Use `asc builds info --latest`."
	deprecatedBuildsLatestNextWarning  = "Warning: `asc builds latest --next` is deprecated. Use `asc builds next-build-number`."
)

type latestBuildSelectionOptions struct {
	AppID                 string
	Version               string
	Platform              string
	ProcessingStateValues []string
	ExcludeExpired        bool
}

type latestBuildSelectionResult struct {
	ResolvedAppID      string
	NormalizedVersion  string
	NormalizedPlatform string
	LatestBuild        *asc.BuildResponse
}

type nextBuildNumberOptions struct {
	LatestBuildSelectionOptions latestBuildSelectionOptions
	InitialBuildNumber          int
}

// BuildsLatestCommand returns a deprecated compatibility wrapper for the old
// latest-build command surface.
func BuildsLatestCommand() *ffcli.Command {
	fs := flag.NewFlagSet("latest", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID, bundle ID, or exact app name (required, or ASC_APP_ID env)")
	version := fs.String("version", "", "Filter by version string (e.g., 1.2.3); requires --platform for deterministic results")
	platform := fs.String("platform", "", "Filter by platform: IOS, MAC_OS, TV_OS, VISION_OS")
	processingState := fs.String("processing-state", "", "Filter by processing state: VALID, PROCESSING, FAILED, INVALID, or all")
	output := shared.BindOutputFlags(fs)
	next := fs.Bool("next", false, "Return next build number using processed builds and in-flight uploads")
	initialBuildNumber := fs.Int("initial-build-number", 1, "Initial build number when none exist (used with --next)")
	excludeExpired := fs.Bool("exclude-expired", false, "Exclude expired builds when selecting latest build")
	notExpired := fs.Bool("not-expired", false, "Alias for --exclude-expired")

	return &ffcli.Command{
		Name:       "latest",
		ShortUsage: "asc builds latest [flags]",
		ShortHelp:  "DEPRECATED: use `asc builds info --latest` or `asc builds next-build-number`.",
		LongHelp:   "Deprecated compatibility command for `asc builds info --latest` and `asc builds next-build-number`.",
		FlagSet:    fs,
		UsageFunc:  shared.DeprecatedUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *next {
				fmt.Fprintln(os.Stderr, deprecatedBuildsLatestNextWarning)
			} else {
				fmt.Fprintln(os.Stderr, deprecatedBuildsLatestFetchWarning)
			}

			excludeExpiredValue := *excludeExpired || *notExpired
			selectionOpts, err := normalizeLatestBuildSelectionOptions(*appID, *version, *platform, *processingState, excludeExpiredValue)
			if err != nil {
				return err
			}
			if *next && *initialBuildNumber < 1 {
				return shared.UsageError("--initial-build-number must be >= 1")
			}
			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("builds latest: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if *next {
				result, err := resolveNextBuildNumber(requestCtx, client, nextBuildNumberOptions{
					LatestBuildSelectionOptions: selectionOpts,
					InitialBuildNumber:          *initialBuildNumber,
				})
				if err != nil {
					return fmt.Errorf("builds latest: %w", err)
				}
				return shared.PrintOutput(result, *output.Output, *output.Pretty)
			}

			build, err := resolveLatestBuild(requestCtx, client, selectionOpts)
			if err != nil {
				return fmt.Errorf("builds latest: %w", err)
			}
			return shared.PrintOutput(build, *output.Output, *output.Pretty)
		},
	}
}

// BuildsNextBuildNumberCommand returns the canonical next build number subcommand.
func BuildsNextBuildNumberCommand() *ffcli.Command {
	fs := flag.NewFlagSet("next-build-number", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID, bundle ID, or exact app name (required, or ASC_APP_ID env)")
	version := fs.String("version", "", "Optional version filter for latest processed/uploaded build selection")
	platform := fs.String("platform", "", "Optional platform filter: IOS, MAC_OS, TV_OS, VISION_OS")
	processingState := fs.String("processing-state", "", "Optional processing state filter: VALID, PROCESSING, FAILED, INVALID, or all")
	initialBuildNumber := fs.Int("initial-build-number", 1, "Initial build number when none exist")
	excludeExpired := fs.Bool("exclude-expired", false, "Exclude expired builds when selecting the latest processed build")
	notExpired := fs.Bool("not-expired", false, "Alias for --exclude-expired")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "next-build-number",
		ShortUsage: "asc builds next-build-number --app APP [flags]",
		ShortHelp:  "Calculate the next build number for an app.",
		LongHelp: `Calculate the next build number for an app.

This command compares the latest processed build and in-flight build uploads,
then returns the next build number that should be safe to use.

Examples:
  asc builds next-build-number --app "123456789"
  asc builds next-build-number --app "123456789" --version "1.2.3" --platform IOS
  asc builds next-build-number --app "123456789" --version "1.2.3" --platform IOS --exclude-expired
  asc builds next-build-number --app "123456789" --initial-build-number 7`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			excludeExpiredValue := *excludeExpired || *notExpired
			selectionOpts, err := normalizeLatestBuildSelectionOptions(*appID, *version, *platform, *processingState, excludeExpiredValue)
			if err != nil {
				return err
			}
			if *initialBuildNumber < 1 {
				return shared.UsageError("--initial-build-number must be >= 1")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("builds next-build-number: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			result, err := resolveNextBuildNumber(requestCtx, client, nextBuildNumberOptions{
				LatestBuildSelectionOptions: selectionOpts,
				InitialBuildNumber:          *initialBuildNumber,
			})
			if err != nil {
				return fmt.Errorf("builds next-build-number: %w", err)
			}
			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

func normalizeLatestBuildSelectionOptions(appID, version, platform, processingState string, excludeExpired bool) (latestBuildSelectionOptions, error) {
	if shared.ResolveAppID(strings.TrimSpace(appID)) == "" {
		return latestBuildSelectionOptions{}, shared.UsageError("--app is required (or set ASC_APP_ID)")
	}

	processingStateValues, err := normalizeBuildProcessingStateFilter(processingState)
	if err != nil {
		return latestBuildSelectionOptions{}, err
	}

	normalizedPlatform := strings.TrimSpace(platform)
	if normalizedPlatform != "" {
		normalizedPlatform, err = shared.NormalizeAppStoreVersionPlatform(normalizedPlatform)
		if err != nil {
			return latestBuildSelectionOptions{}, shared.UsageError(err.Error())
		}
	}

	return latestBuildSelectionOptions{
		AppID:                 strings.TrimSpace(appID),
		Version:               strings.TrimSpace(version),
		Platform:              normalizedPlatform,
		ProcessingStateValues: processingStateValues,
		ExcludeExpired:        excludeExpired,
	}, nil
}

func resolveLatestBuild(ctx context.Context, client *asc.Client, opts latestBuildSelectionOptions) (*asc.BuildResponse, error) {
	selection, err := resolveLatestBuildSelection(ctx, client, opts, false)
	if err != nil {
		return nil, err
	}
	return selection.LatestBuild, nil
}

func resolveNextBuildNumber(ctx context.Context, client *asc.Client, opts nextBuildNumberOptions) (*asc.BuildsNextBuildNumberResult, error) {
	if opts.InitialBuildNumber < 1 {
		return nil, shared.UsageError("--initial-build-number must be >= 1")
	}

	selection, err := resolveLatestBuildSelection(ctx, client, opts.LatestBuildSelectionOptions, true)
	if err != nil {
		return nil, err
	}

	var latestProcessedNumber *string
	var latestUploadNumber *string
	var latestObservedNumber *string
	sourcesConsidered := make([]string, 0, 2)

	var latestProcessedValue buildNumber
	hasProcessed := false
	if selection.LatestBuild != nil {
		parsed, err := parseBuildNumber(selection.LatestBuild.Data.Attributes.Version, fmt.Sprintf("processed build %s", selection.LatestBuild.Data.ID))
		if err != nil {
			return nil, err
		}
		latestProcessedValue = parsed
		value := parsed.String()
		latestProcessedNumber = &value
		hasProcessed = true
		sourcesConsidered = append(sourcesConsidered, "processed_builds")
	}

	latestUploadValue, latestUploadNumber, hasUpload, err := findLatestBuildUploadNumber(
		ctx,
		client,
		selection.ResolvedAppID,
		selection.NormalizedVersion,
		selection.NormalizedPlatform,
	)
	if err != nil {
		return nil, err
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

	nextBuildNumberValue := strconv.FormatInt(int64(opts.InitialBuildNumber), 10)
	if hasObserved {
		nextValue, err := latestObservedValue.Next()
		if err != nil {
			return nil, err
		}
		nextBuildNumberValue = nextValue.String()
	}

	return &asc.BuildsNextBuildNumberResult{
		LatestProcessedBuildNumber: latestProcessedNumber,
		LatestUploadBuildNumber:    latestUploadNumber,
		LatestObservedBuildNumber:  latestObservedNumber,
		NextBuildNumber:            nextBuildNumberValue,
		SourcesConsidered:          sourcesConsidered,
	}, nil
}

func resolveLatestBuildSelection(ctx context.Context, client *asc.Client, opts latestBuildSelectionOptions, allowEmpty bool) (*latestBuildSelectionResult, error) {
	if client == nil {
		return nil, fmt.Errorf("build client is required")
	}

	resolvedAppID := shared.ResolveAppID(opts.AppID)
	if resolvedAppID == "" {
		return nil, shared.UsageError("--app is required (or set ASC_APP_ID)")
	}

	resolvedAppID, err := shared.ResolveAppIDWithLookup(ctx, client, resolvedAppID)
	if err != nil {
		return nil, err
	}

	hasPreReleaseFilters := opts.Version != "" || opts.Platform != ""

	var preReleaseVersionIDs []string
	if hasPreReleaseFilters {
		preReleaseVersionIDs, err = findPreReleaseVersionIDs(ctx, client, resolvedAppID, opts.Version, opts.Platform)
		if err != nil {
			return nil, err
		}
		if len(preReleaseVersionIDs) == 0 && !allowEmpty {
			if opts.Version != "" && opts.Platform != "" {
				return nil, fmt.Errorf("no pre-release version found for version %q on platform %s", opts.Version, opts.Platform)
			}
			if opts.Version != "" {
				return nil, fmt.Errorf("no pre-release version found for version %q", opts.Version)
			}
			return nil, fmt.Errorf("no pre-release version found for platform %s", opts.Platform)
		}
	}

	var latestBuild *asc.BuildResponse
	if !hasPreReleaseFilters {
		buildOpts := []asc.BuildsOption{
			asc.WithBuildsSort("-uploadedDate"),
			asc.WithBuildsLimit(200),
		}
		if len(opts.ProcessingStateValues) > 0 {
			buildOpts = append(buildOpts, asc.WithBuildsProcessingStates(opts.ProcessingStateValues))
		}
		if opts.ExcludeExpired {
			buildOpts = append(buildOpts, asc.WithBuildsExpired(false))
		}

		latestBuild, err = findMostRecentlyUploadedBuild(ctx, client, resolvedAppID, buildOpts...)
		if err != nil {
			return nil, err
		}
		if latestBuild == nil && !allowEmpty {
			return nil, fmt.Errorf("no builds found for app %s", resolvedAppID)
		}
	} else if len(preReleaseVersionIDs) == 1 {
		buildOpts := []asc.BuildsOption{
			asc.WithBuildsSort("-uploadedDate"),
			asc.WithBuildsLimit(1),
			asc.WithBuildsPreReleaseVersion(preReleaseVersionIDs[0]),
		}
		if len(opts.ProcessingStateValues) > 0 {
			buildOpts = append(buildOpts, asc.WithBuildsProcessingStates(opts.ProcessingStateValues))
		}
		if opts.ExcludeExpired {
			buildOpts = append(buildOpts, asc.WithBuildsExpired(false))
		}
		builds, err := client.GetBuilds(ctx, resolvedAppID, buildOpts...)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch: %w", err)
		}
		if len(builds.Data) == 0 {
			if !allowEmpty {
				return nil, fmt.Errorf("no builds found matching filters")
			}
		} else {
			latestBuild = &asc.BuildResponse{
				Data:  builds.Data[0],
				Links: builds.Links,
			}
		}
	} else if len(preReleaseVersionIDs) > 1 {
		var newestBuild *asc.Resource[asc.BuildAttributes]

		for _, prvID := range preReleaseVersionIDs {
			buildOpts := []asc.BuildsOption{
				asc.WithBuildsSort("-uploadedDate"),
				asc.WithBuildsLimit(1),
				asc.WithBuildsPreReleaseVersion(prvID),
			}
			if len(opts.ProcessingStateValues) > 0 {
				buildOpts = append(buildOpts, asc.WithBuildsProcessingStates(opts.ProcessingStateValues))
			}
			if opts.ExcludeExpired {
				buildOpts = append(buildOpts, asc.WithBuildsExpired(false))
			}
			builds, err := client.GetBuilds(ctx, resolvedAppID, buildOpts...)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch: %w", err)
			}
			if len(builds.Data) > 0 {
				candidate := builds.Data[0]
				if newestBuild == nil || isMoreRecentUploadedBuild(candidate, *newestBuild) {
					selected := candidate
					newestBuild = &selected
				}
			}
		}

		if newestBuild == nil {
			if !allowEmpty {
				return nil, fmt.Errorf("no builds found matching filters")
			}
		} else {
			latestBuild = &asc.BuildResponse{
				Data: *newestBuild,
			}
		}
	}

	return &latestBuildSelectionResult{
		ResolvedAppID:      resolvedAppID,
		NormalizedVersion:  opts.Version,
		NormalizedPlatform: opts.Platform,
		LatestBuild:        latestBuild,
	}, nil
}

// findPreReleaseVersionIDs looks up preReleaseVersion IDs for given filters.
// Returns all exact-matching IDs. Version+platform usually narrows to a single
// ID, but the lookup still paginates because ASC can return near-match version
// results (for example 1.1.0 for a requested 1.1) ahead of the exact match.
func findPreReleaseVersionIDs(ctx context.Context, client *asc.Client, appID, version, platform string) ([]string, error) {
	opts := []asc.PreReleaseVersionsOption{}
	exactVersion := strings.TrimSpace(version)

	if version != "" {
		opts = append(opts, asc.WithPreReleaseVersionsVersion(version))
		opts = append(opts, asc.WithPreReleaseVersionsLimit(200))
	} else {
		// Platform-only lookups can span multiple versions.
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

	matchesRequestedVersion := func(preReleaseVersion asc.PreReleaseVersion) bool {
		if exactVersion == "" {
			return true
		}
		// ASC's version filter can return dotted-version near-matches like
		// 1.1 and 1.1.0 together, so enforce exact matching client-side when
		// the response includes attributes.version. If ASC omits the attribute
		// entirely, trust the server-side filter instead.
		versionAttr := strings.TrimSpace(preReleaseVersion.Attributes.Version)
		if versionAttr == "" {
			return true
		}
		return versionAttr == exactVersion
	}

	// Stream pages and keep only exact-matching IDs.
	ids := make([]string, 0, len(firstPage.Data))
	appendIDs := func(page *asc.PreReleaseVersionsResponse) {
		for _, preReleaseVersion := range page.Data {
			if matchesRequestedVersion(preReleaseVersion) {
				ids = append(ids, preReleaseVersion.ID)
			}
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

func findMostRecentlyUploadedBuild(
	ctx context.Context,
	client *asc.Client,
	appID string,
	opts ...asc.BuildsOption,
) (*asc.BuildResponse, error) {
	const buildsLatestScanPageLimit = 10

	firstPage, err := client.GetBuilds(ctx, appID, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch builds: %w", err)
	}

	var latest *asc.Resource[asc.BuildAttributes]
	latestLinks := asc.Links{}
	consumePage := func(page *asc.BuildsResponse) bool {
		pageHadStrictlyNewer := false
		pageLinks := page.GetLinks()
		for i := range page.Data {
			candidate := page.Data[i]
			if latest != nil && isStrictlyMoreRecentUploadedBuild(candidate, *latest) {
				pageHadStrictlyNewer = true
			}
			if latest == nil || isMoreRecentUploadedBuild(candidate, *latest) {
				selected := candidate
				latest = &selected
				if pageLinks != nil {
					latestLinks = *pageLinks
				} else {
					latestLinks = asc.Links{}
				}
			}
		}
		return pageHadStrictlyNewer
	}
	consumePage(firstPage)

	if latest == nil {
		return nil, nil
	}

	links := firstPage.GetLinks()
	if links == nil || links.Next == "" {
		return &asc.BuildResponse{
			Data:  *latest,
			Links: latestLinks,
		}, nil
	}

	nextURL := links.Next
	pagesScanned := 1
	anomalyDetected := false
	seenProbeURLs := map[string]struct{}{}
	for nextURL != "" && pagesScanned < buildsLatestScanPageLimit {
		if _, seen := seenProbeURLs[nextURL]; seen {
			return nil, fmt.Errorf("failed to paginate builds: %w: %s", asc.ErrRepeatedPaginationURL, nextURL)
		}
		seenProbeURLs[nextURL] = struct{}{}

		nextPage, err := client.GetBuilds(ctx, appID, asc.WithBuildsNextURL(nextURL))
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil, fmt.Errorf("failed to paginate builds: page %d: %w", pagesScanned+1, err)
			}
			if ctxErr := ctx.Err(); errors.Is(ctxErr, context.Canceled) || errors.Is(ctxErr, context.DeadlineExceeded) {
				return nil, fmt.Errorf("failed to paginate builds: page %d: %w", pagesScanned+1, ctxErr)
			}
			// Probing additional pages is best-effort. If a probe page fails, keep
			// the best candidate gathered so far instead of failing the command.
			break
		}
		pagesScanned++

		pageHadNewer := consumePage(nextPage)
		pageLinks := nextPage.GetLinks()
		if pageLinks != nil && pageLinks.Next != "" {
			if _, seen := seenProbeURLs[pageLinks.Next]; seen {
				return nil, fmt.Errorf("failed to paginate builds: %w: %s", asc.ErrRepeatedPaginationURL, pageLinks.Next)
			}
		}

		if !anomalyDetected {
			// Normal case: page 1 already contains the latest item.
			// Stop immediately once a later page fails to produce a newer build.
			if !pageHadNewer {
				break
			}
			// If a later page is newer than page 1, ordering is inconsistent.
			// Continue scanning remaining pages until pagination exhausts or
			// we hit the safety cap so non-monotonic sequences are handled.
			anomalyDetected = true
		}

		if pageLinks == nil || pageLinks.Next == "" {
			nextURL = ""
			break
		}
		nextURL = pageLinks.Next
	}
	if nextURL != "" && pagesScanned >= buildsLatestScanPageLimit {
		return nil, fmt.Errorf("failed to paginate builds: reached scan cap of %d pages with additional pages remaining", buildsLatestScanPageLimit)
	}

	return &asc.BuildResponse{
		Data:  *latest,
		Links: latestLinks,
	}, nil
}

func isStrictlyMoreRecentUploadedBuild(candidate, current asc.Resource[asc.BuildAttributes]) bool {
	comparison := compareUploadedDate(candidate.Attributes.UploadedDate, current.Attributes.UploadedDate)
	return comparison > 0
}

func isMoreRecentUploadedBuild(candidate, current asc.Resource[asc.BuildAttributes]) bool {
	comparison := compareUploadedDate(candidate.Attributes.UploadedDate, current.Attributes.UploadedDate)
	if comparison != 0 {
		return comparison > 0
	}

	// Break ties deterministically to avoid unstable output for identical timestamps.
	return candidate.ID > current.ID
}

func compareUploadedDate(left, right string) int {
	leftParsed, leftErr := parseBuildTimestamp(left)
	rightParsed, rightErr := parseBuildTimestamp(right)

	switch {
	case leftErr == nil && rightErr == nil:
		if leftParsed.After(rightParsed) {
			return 1
		}
		if leftParsed.Before(rightParsed) {
			return -1
		}
		return 0
	case leftErr == nil && rightErr != nil:
		return 1
	case leftErr != nil && rightErr == nil:
		return -1
	default:
		// Fallback for unexpected timestamp formats.
		return strings.Compare(strings.TrimSpace(left), strings.TrimSpace(right))
	}
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

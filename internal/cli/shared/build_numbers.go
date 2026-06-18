package shared

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// LatestBuildSelectionOptions controls how latest/next build helpers select
// build records and in-flight uploads.
type LatestBuildSelectionOptions struct {
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

// NextBuildNumberOptions configures next build number calculation.
type NextBuildNumberOptions struct {
	LatestBuildSelectionOptions LatestBuildSelectionOptions
	InitialBuildNumber          int
}

// ResolveLatestBuild finds the latest processed build matching the provided
// app/version/platform filters. When allowEmpty is true, nil is returned when
// no matching build exists.
func ResolveLatestBuild(ctx context.Context, client *asc.Client, opts LatestBuildSelectionOptions, allowEmpty bool) (*asc.BuildResponse, error) {
	selection, err := resolveLatestBuildSelection(ctx, client, opts, allowEmpty)
	if err != nil {
		return nil, err
	}
	return selection.LatestBuild, nil
}

// NormalizeLatestBuildSelectionOptions validates and normalizes the common
// version/platform filters shared by latest-build selection workflows.
func NormalizeLatestBuildSelectionOptions(appID, version, platform, processingState string, excludeExpired bool) (LatestBuildSelectionOptions, error) {
	if ResolveAppID(strings.TrimSpace(appID)) == "" {
		return LatestBuildSelectionOptions{}, UsageError("--app is required (or set ASC_APP_ID)")
	}

	processingStateValues, err := NormalizeBuildProcessingStateFilter(processingState, BuildProcessingStateFilterOptions{})
	if err != nil {
		return LatestBuildSelectionOptions{}, err
	}

	normalizedPlatform := strings.TrimSpace(platform)
	if normalizedPlatform != "" {
		normalizedPlatform, err = NormalizeAppStoreVersionPlatform(normalizedPlatform)
		if err != nil {
			return LatestBuildSelectionOptions{}, UsageError(err.Error())
		}
	}

	return LatestBuildSelectionOptions{
		AppID:                 strings.TrimSpace(appID),
		Version:               strings.TrimSpace(version),
		Platform:              normalizedPlatform,
		ProcessingStateValues: processingStateValues,
		ExcludeExpired:        excludeExpired,
	}, nil
}

// ResolveNextBuildNumber compares the latest processed build and the latest
// in-flight build upload, then returns the next safe build number.
func ResolveNextBuildNumber(ctx context.Context, client *asc.Client, opts NextBuildNumberOptions) (*asc.BuildsNextBuildNumberResult, error) {
	if opts.InitialBuildNumber < 1 {
		return nil, UsageError("--initial-build-number must be >= 1")
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

func resolveLatestBuildSelection(ctx context.Context, client *asc.Client, opts LatestBuildSelectionOptions, allowEmpty bool) (*latestBuildSelectionResult, error) {
	if client == nil {
		return nil, fmt.Errorf("build client is required")
	}

	resolvedAppID := ResolveAppID(opts.AppID)
	if resolvedAppID == "" {
		return nil, UsageError("--app is required (or set ASC_APP_ID)")
	}

	resolvedAppID, err := ResolveAppIDWithLookup(ctx, client, resolvedAppID)
	if err != nil {
		return nil, err
	}

	hasPreReleaseFilters := opts.Version != "" || opts.Platform != ""

	var preReleaseVersionIDs []string
	if hasPreReleaseFilters {
		preReleaseVersionIDs, err = FindPreReleaseVersionIDs(ctx, client, resolvedAppID, opts.Version, opts.Platform)
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

// FindPreReleaseVersionIDs returns the exact-matching pre-release version IDs
// for the provided app/version/platform filters.
func FindPreReleaseVersionIDs(ctx context.Context, client *asc.Client, appID, version, platform string) ([]string, error) {
	opts := []asc.PreReleaseVersionsOption{}
	exactVersion := strings.TrimSpace(version)

	if version != "" {
		opts = append(opts, asc.WithPreReleaseVersionsVersion(version))
		opts = append(opts, asc.WithPreReleaseVersionsLimit(200))
	} else {
		opts = append(opts, asc.WithPreReleaseVersionsLimit(200))
	}

	if platform != "" {
		opts = append(opts, asc.WithPreReleaseVersionsPlatform(platform))
	}

	firstPage, err := client.GetPreReleaseVersions(ctx, appID, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup pre-release versions: %w", err)
	}

	matchesRequestedVersion := func(preReleaseVersion asc.PreReleaseVersion) bool {
		if exactVersion == "" {
			return true
		}
		versionAttr := strings.TrimSpace(preReleaseVersion.Attributes.Version)
		if versionAttr == "" {
			return true
		}
		return versionAttr == exactVersion
	}

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

func findMostRecentlyUploadedBuild(ctx context.Context, client *asc.Client, appID string, opts ...asc.BuildsOption) (*asc.BuildResponse, error) {
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
			if !pageHadNewer {
				break
			}
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
	return compareUploadedDate(candidate.Attributes.UploadedDate, current.Attributes.UploadedDate) > 0
}

func isMoreRecentUploadedBuild(candidate, current asc.Resource[asc.BuildAttributes]) bool {
	comparison := compareUploadedDate(candidate.Attributes.UploadedDate, current.Attributes.UploadedDate)
	if comparison != 0 {
		return comparison > 0
	}
	return candidate.ID > current.ID
}

func compareUploadedDate(left, right string) int {
	leftParsed, leftErr := ParseBuildTimestamp(left)
	rightParsed, rightErr := ParseBuildTimestamp(right)

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
		return strings.Compare(strings.TrimSpace(left), strings.TrimSpace(right))
	}
}

// ParseBuildTimestamp parses ASC uploadedDate values used across build helpers.
func ParseBuildTimestamp(value string) (time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, fmt.Errorf("uploadedDate is empty")
	}
	if parsed, err := time.Parse(time.RFC3339, trimmed); err == nil {
		return parsed, nil
	}
	if parsed, err := time.Parse(time.RFC3339Nano, trimmed); err == nil {
		return parsed, nil
	}
	return time.Time{}, fmt.Errorf("invalid time %q", trimmed)
}

func findLatestBuildUploadNumber(ctx context.Context, client *asc.Client, appID, version, platform string) (buildNumber, *string, bool, error) {
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
			if isNonPositiveNumericBuildNumber(upload.Attributes.CFBundleVersion) {
				continue
			}
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

func isNonPositiveNumericBuildNumber(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return false
	}

	segments := strings.Split(trimmed, ".")
	if len(segments) == 0 {
		return false
	}
	for i, segment := range segments {
		segment = strings.TrimSpace(segment)
		segments[i] = segment
		if segment == "" {
			return false
		}
		for _, ch := range segment {
			if ch < '0' || ch > '9' {
				return false
			}
		}
	}

	value, err := strconv.ParseInt(segments[0], 10, 64)
	return err == nil && value < 1
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

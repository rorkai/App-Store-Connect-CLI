package shared

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
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
	ResolvedAppID        string
	BuildUploadVersions  []string
	NormalizedPlatform   string
	HasPreReleaseFilters bool
	PreReleaseVersionIDs []string
	LatestBuild          *asc.BuildResponse
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

// ResolveNextBuildNumber compares the highest observed processed build and
// in-flight build upload numbers, then returns the next safe build number.
func ResolveNextBuildNumber(ctx context.Context, client *asc.Client, opts NextBuildNumberOptions) (*asc.BuildsNextBuildNumberResult, error) {
	if opts.InitialBuildNumber < 1 {
		return nil, UsageError("--initial-build-number must be >= 1")
	}

	// Resolving the next build number walks the full processed build history
	// and the full build upload history. A single caller-supplied request
	// deadline cannot bound that many sequential pages, so drop the innermost
	// request deadline and give every outbound request its own fresh one.
	scanCtx := contextWithoutCurrentTimeout(ctx)

	selection, err := resolveLatestBuildSelection(scanCtx, client, opts.LatestBuildSelectionOptions, true)
	if err != nil {
		return nil, err
	}

	var latestProcessedNumber *string
	var latestUploadNumber *string
	var latestObservedNumber *string
	sourcesConsidered := make([]string, 0, 2)

	skipped := &skippedBuildNumberReporter{}

	var latestProcessedValue buildNumber
	hasLatestProcessed := false
	if selection.LatestBuild != nil {
		latestVersion := selection.LatestBuild.Data.Attributes.Version
		if !isNonPositiveNumericBuildNumber(latestVersion) {
			parsed, ok := parseProcessedBuildNumber(latestVersion)
			if !ok {
				skipped.warn(selection.LatestBuild.Data.ID, latestVersion)
			} else {
				latestProcessedValue = parsed
				value := parsed.String()
				latestProcessedNumber = &value
				hasLatestProcessed = true
			}
		}
	}

	highestProcessedValue := latestProcessedValue
	highestProcessedNumber := latestProcessedNumber
	hasProcessed := hasLatestProcessed
	scannedProcessedValue, scannedProcessedNumber, hasScannedProcessed, err := findHighestProcessedBuildNumber(
		scanCtx,
		client,
		selection,
		opts.LatestBuildSelectionOptions,
		skipped,
	)
	if err != nil {
		return nil, err
	}
	if hasScannedProcessed && (!hasProcessed || scannedProcessedValue.Compare(highestProcessedValue) > 0) {
		highestProcessedValue = scannedProcessedValue
		highestProcessedNumber = scannedProcessedNumber
		hasProcessed = true
	}
	if hasProcessed {
		sourcesConsidered = append(sourcesConsidered, "processed_builds")
	}

	latestUploadValue, latestUploadNumber, hasUpload, err := findLatestBuildUploadNumber(
		scanCtx,
		client,
		selection.ResolvedAppID,
		selection.BuildUploadVersions,
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
		latestObservedValue = highestProcessedValue
		hasObserved = true
		latestObservedNumber = highestProcessedNumber
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

	lookupCtx, lookupCancel := contextWithTimeout(ctx)
	resolvedAppID, err := ResolveAppIDWithLookup(lookupCtx, client, resolvedAppID)
	lookupCancel()
	if err != nil {
		return nil, err
	}

	hasPreReleaseFilters := opts.Version != "" || opts.Platform != ""
	buildUploadVersions := versionQueryVariants(opts.Version)

	var preReleaseVersionIDs []string
	if hasPreReleaseFilters {
		preReleaseVersionIDs, err = FindPreReleaseVersionIDs(ctx, client, resolvedAppID, opts.Version, opts.Platform)
		if err != nil {
			return nil, err
		}
		if len(preReleaseVersionIDs) == 0 && !allowEmpty {
			return nil, noPreReleaseVersionFound(opts.Version, opts.Platform)
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
			return nil, noLatestBuildFound(resolvedAppID, opts)
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
		requestCtx, cancel := contextWithTimeout(ctx)
		builds, err := client.GetBuilds(requestCtx, resolvedAppID, buildOpts...)
		cancel()
		if err != nil {
			return nil, fmt.Errorf("failed to fetch: %w", err)
		}
		if len(builds.Data) == 0 {
			if !allowEmpty {
				return nil, noLatestBuildFound(resolvedAppID, opts)
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
			requestCtx, cancel := contextWithTimeout(ctx)
			builds, err := client.GetBuilds(requestCtx, resolvedAppID, buildOpts...)
			cancel()
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
				return nil, noLatestBuildFound(resolvedAppID, opts)
			}
		} else {
			latestBuild = &asc.BuildResponse{
				Data: *newestBuild,
			}
		}
	}

	return &latestBuildSelectionResult{
		ResolvedAppID:        resolvedAppID,
		BuildUploadVersions:  append([]string(nil), buildUploadVersions...),
		NormalizedPlatform:   opts.Platform,
		HasPreReleaseFilters: hasPreReleaseFilters,
		PreReleaseVersionIDs: append([]string(nil), preReleaseVersionIDs...),
		LatestBuild:          latestBuild,
	}, nil
}

// noPreReleaseVersionFound reports that no pre-release version matched the
// caller's version/platform filters. The error carries asc.ErrNotFound so the
// CLI exits with the not-found code instead of a generic failure.
func noPreReleaseVersionFound(version, platform string) error {
	version = strings.TrimSpace(version)
	platform = strings.TrimSpace(platform)
	var message string
	switch {
	case version != "" && platform != "":
		message = fmt.Sprintf("no pre-release version found for version %q on platform %s; check --version and --platform", version, platform)
	case version != "":
		message = fmt.Sprintf("no pre-release version found for version %q; check --version", version)
	default:
		message = fmt.Sprintf("no pre-release version found for platform %s; check --platform", platform)
	}
	return NewErrorWithCause(errors.New(message), asc.ErrNotFound)
}

// noLatestBuildFound reports that no build matched the latest-build selection
// filters. The error carries asc.ErrNotFound so the CLI exits with the
// not-found code instead of a generic failure.
func noLatestBuildFound(appID string, opts LatestBuildSelectionOptions) error {
	filters := describeLatestBuildSelectionFilters(opts)
	if len(filters) == 0 {
		return NewErrorWithCause(
			fmt.Errorf("no builds found for app %s; a new upload may still be processing, or check --app", appID),
			asc.ErrNotFound,
		)
	}
	return NewErrorWithCause(
		fmt.Errorf(
			"no builds found for app %s matching %s; adjust the filters, or use --build-id",
			appID,
			strings.Join(filters, " and "),
		),
		asc.ErrNotFound,
	)
}

func describeLatestBuildSelectionFilters(opts LatestBuildSelectionOptions) []string {
	var filters []string
	if version := strings.TrimSpace(opts.Version); version != "" {
		filters = append(filters, fmt.Sprintf("--version %q", version))
	}
	if platform := strings.TrimSpace(opts.Platform); platform != "" {
		filters = append(filters, "--platform "+platform)
	}
	if len(opts.ProcessingStateValues) > 0 {
		filters = append(filters, "--processing-state "+strings.Join(opts.ProcessingStateValues, ","))
	}
	if opts.ExcludeExpired {
		filters = append(filters, "--exclude-expired")
	}
	return filters
}

func findHighestProcessedBuildNumber(
	ctx context.Context,
	client *asc.Client,
	selection *latestBuildSelectionResult,
	opts LatestBuildSelectionOptions,
	skipped *skippedBuildNumberReporter,
) (buildNumber, *string, bool, error) {
	if selection.HasPreReleaseFilters && len(selection.PreReleaseVersionIDs) == 0 {
		return buildNumber{}, nil, false, nil
	}

	buildOpts := []asc.BuildsOption{
		asc.WithBuildsSort("-uploadedDate"),
		asc.WithBuildsLimit(200),
	}
	if len(selection.PreReleaseVersionIDs) > 0 {
		buildOpts = append(buildOpts, asc.WithBuildsPreReleaseVersions(selection.PreReleaseVersionIDs))
	}
	if len(opts.ProcessingStateValues) > 0 {
		buildOpts = append(buildOpts, asc.WithBuildsProcessingStates(opts.ProcessingStateValues))
	}
	if opts.ExcludeExpired {
		buildOpts = append(buildOpts, asc.WithBuildsExpired(false))
	}

	firstPageCtx, firstPageCancel := contextWithTimeout(ctx)
	builds, err := client.GetBuilds(firstPageCtx, selection.ResolvedAppID, buildOpts...)
	firstPageCancel()
	if err != nil {
		return buildNumber{}, nil, false, fmt.Errorf("failed to fetch processed build history: %w", err)
	}

	var highestValue buildNumber
	var highestNumber *string
	hasBuild := false
	processPage := func(page *asc.BuildsResponse) {
		for _, build := range page.Data {
			if isNonPositiveNumericBuildNumber(build.Attributes.Version) {
				continue
			}
			parsed, ok := parseProcessedBuildNumber(build.Attributes.Version)
			if !ok {
				skipped.warn(build.ID, build.Attributes.Version)
				continue
			}
			if !hasBuild || parsed.Compare(highestValue) > 0 {
				highestValue = parsed
				value := parsed.String()
				highestNumber = &value
				hasBuild = true
			}
		}
	}

	err = asc.PaginateEach(ctx, builds, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		requestCtx, cancel := contextWithTimeout(ctx)
		defer cancel()
		return client.GetBuilds(requestCtx, selection.ResolvedAppID, asc.WithBuildsNextURL(nextURL))
	}, func(page asc.PaginatedResponse) error {
		resp, ok := page.(*asc.BuildsResponse)
		if !ok {
			return fmt.Errorf("unexpected builds page type %T", page)
		}
		processPage(resp)
		return nil
	})
	if err != nil {
		return buildNumber{}, nil, false, fmt.Errorf("failed to paginate processed build history: %w", err)
	}

	return highestValue, highestNumber, hasBuild, nil
}

// FindPreReleaseVersionIDs returns the exact-matching pre-release version IDs
// for the provided app/version/platform filters. App Store Connect treats
// "1.2" and "1.2.0" as the same version, so when the requested format matches
// nothing the equivalent format is tried before reporting no match.
func FindPreReleaseVersionIDs(ctx context.Context, client *asc.Client, appID, version, platform string) ([]string, error) {
	return findPreReleaseVersionIDs(ctx, client, appID, version, platform, 0)
}

// FindPreReleaseVersionIDsWithMaxPages is the bounded variant used by
// selectors that must keep their complete lookup within a finite pagination
// budget. The unbounded FindPreReleaseVersionIDs function remains available
// for callers whose existing pagination semantics must not change.
func FindPreReleaseVersionIDsWithMaxPages(ctx context.Context, client *asc.Client, appID, version, platform string, maxPages int) ([]string, error) {
	if maxPages <= 0 {
		return nil, fmt.Errorf("max pages must be greater than zero")
	}
	return findPreReleaseVersionIDs(ctx, client, appID, version, platform, maxPages)
}

func findPreReleaseVersionIDs(ctx context.Context, client *asc.Client, appID, version, platform string, maxPages int) ([]string, error) {
	requestedVersion := strings.TrimSpace(version)

	variants := versionQueryVariants(requestedVersion)
	if len(variants) == 0 {
		ids, _, err := findPreReleaseVersionIDsForVersions(ctx, client, appID, nil, platform, maxPages)
		return ids, err
	}

	// A platform-scoped lookup can stop at the caller's exact spelling. Without
	// a platform filter, equivalent spellings can legitimately belong to
	// different platform trains, so every variant is requested together.
	if strings.TrimSpace(platform) == "" {
		ids, matchedVersions, err := findPreReleaseVersionIDsForVersions(ctx, client, appID, variants, "", maxPages)
		if err != nil {
			return nil, err
		}
		if _, exactMatched := matchedVersions[requestedVersion]; !exactMatched {
			for _, variant := range variants {
				if variant == requestedVersion {
					continue
				}
				if _, ok := matchedVersions[variant]; ok {
					noteEquivalentVersionMatch(requestedVersion, variant)
					break
				}
			}
		}
		return ids, nil
	}

	// The requested format is queried first and wins outright, so an app that
	// really does have a train under the caller's exact version string keeps
	// resolving exactly as before.
	for _, variant := range variants {
		ids, _, err := findPreReleaseVersionIDsForVersions(ctx, client, appID, []string{variant}, platform, maxPages)
		if err != nil {
			return nil, err
		}
		if len(ids) == 0 {
			continue
		}
		noteEquivalentVersionMatch(requestedVersion, variant)
		return ids, nil
	}

	return nil, nil
}

func findPreReleaseVersionIDsForVersions(ctx context.Context, client *asc.Client, appID string, versions []string, platform string, maxPages int) ([]string, map[string]struct{}, error) {
	opts := []asc.PreReleaseVersionsOption{}
	acceptedVersions := make(map[string]struct{}, len(versions))
	for _, version := range versions {
		if normalized := strings.TrimSpace(version); normalized != "" {
			acceptedVersions[normalized] = struct{}{}
		}
	}
	versionFilter := strings.Join(versions, ",")

	if versionFilter != "" {
		opts = append(opts, asc.WithPreReleaseVersionsVersion(versionFilter))
		opts = append(opts, asc.WithPreReleaseVersionsLimit(200))
	} else {
		opts = append(opts, asc.WithPreReleaseVersionsLimit(200))
	}

	if platform != "" {
		opts = append(opts, asc.WithPreReleaseVersionsPlatform(platform))
	}

	firstPageCtx, firstPageCancel := contextWithTimeout(ctx)
	firstPage, err := client.GetPreReleaseVersions(firstPageCtx, appID, opts...)
	firstPageCancel()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to lookup pre-release versions: %w", err)
	}

	matchedVersions := make(map[string]struct{}, len(acceptedVersions))
	matchesRequestedVersion := func(preReleaseVersion asc.PreReleaseVersion) bool {
		if len(acceptedVersions) == 0 {
			return true
		}
		versionAttr := strings.TrimSpace(preReleaseVersion.Attributes.Version)
		if versionAttr == "" {
			return true
		}
		if _, ok := acceptedVersions[versionAttr]; ok {
			matchedVersions[versionAttr] = struct{}{}
			return true
		}
		return false
	}

	ids := make([]string, 0, len(firstPage.Data))
	seen := make(map[string]struct{}, len(firstPage.Data))
	appendIDs := func(page *asc.PreReleaseVersionsResponse) {
		for _, preReleaseVersion := range page.Data {
			if !matchesRequestedVersion(preReleaseVersion) {
				continue
			}
			id := strings.TrimSpace(preReleaseVersion.ID)
			if id == "" {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
	}

	fetchNext := func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		requestCtx, cancel := contextWithTimeout(ctx)
		defer cancel()
		return client.GetPreReleaseVersions(requestCtx, appID, asc.WithPreReleaseVersionsNextURL(nextURL))
	}
	consumePage := func(page asc.PaginatedResponse) error {
		resp, ok := page.(*asc.PreReleaseVersionsResponse)
		if !ok {
			return fmt.Errorf("unexpected pre-release versions page type %T", page)
		}
		appendIDs(resp)
		return nil
	}
	if maxPages > 0 {
		err = asc.PaginateEachWithMaxPages(ctx, firstPage, fetchNext, consumePage, maxPages)
	} else {
		err = asc.PaginateEach(ctx, firstPage, fetchNext, consumePage)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("failed to paginate pre-release versions: %w", err)
	}

	return ids, matchedVersions, nil
}

// versionQueryVariants returns the marketing version strings worth querying for
// a caller-supplied version, most specific first. App Store Connect treats
// "1.2" and "1.2.0" as the same version but only exposes the format that was
// uploaded first through filter[version], so a lookup that finds nothing under
// the requested format has to retry the equivalent one before concluding the
// version does not exist. Only the trailing ".0" equivalence is inferred;
// other spellings, including leading-zero variants, are preserved. Versions
// that are not purely numeric are returned unchanged.
func versionQueryVariants(version string) []string {
	trimmed := strings.TrimSpace(version)
	if trimmed == "" {
		return nil
	}

	variants := []string{trimmed}
	appendVariant := func(candidate string) {
		if candidate == "" {
			return
		}
		for _, existing := range variants {
			if existing == candidate {
				return
			}
		}
		variants = append(variants, candidate)
	}

	segments := strings.Split(trimmed, ".")
	for _, segment := range segments {
		if segment == "" {
			return variants
		}
		for _, ch := range segment {
			if ch < '0' || ch > '9' {
				return variants
			}
		}
	}

	switch {
	case len(segments) == 2:
		appendVariant(trimmed + ".0")
	case len(segments) == 3 && segments[2] == "0":
		appendVariant(strings.Join(segments[:2], "."))
	}

	return variants
}

var (
	equivalentVersionNoteMu sync.Mutex
	equivalentVersionNotes  = map[string]struct{}{}
)

// noteEquivalentVersionMatch reports that a lookup succeeded through an
// equivalent version format instead of the exact string the caller asked for.
// Build waits repeat these lookups on every poll, so each requested/matched
// pair is reported only once.
func noteEquivalentVersionMatch(requested, matched string) {
	if requested == "" || matched == "" || requested == matched {
		return
	}

	key := requested + "\x00" + matched
	equivalentVersionNoteMu.Lock()
	_, seen := equivalentVersionNotes[key]
	if !seen {
		equivalentVersionNotes[key] = struct{}{}
	}
	equivalentVersionNoteMu.Unlock()
	if seen {
		return
	}

	fmt.Fprintf(os.Stderr, "note: matched version %q for requested %q\n", matched, requested)
}

func findMostRecentlyUploadedBuild(ctx context.Context, client *asc.Client, appID string, opts ...asc.BuildsOption) (*asc.BuildResponse, error) {
	const buildsLatestScanPageLimit = 10

	firstPageCtx, firstPageCancel := contextWithTimeout(ctx)
	firstPage, err := client.GetBuilds(firstPageCtx, appID, opts...)
	firstPageCancel()
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

		nextPageCtx, nextPageCancel := contextWithTimeout(ctx)
		nextPage, err := client.GetBuilds(nextPageCtx, appID, asc.WithBuildsNextURL(nextURL))
		nextPageCancel()
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil, fmt.Errorf("failed to paginate builds: page %d: %w", pagesScanned+1, err)
			}
			if ctxErr := ctx.Err(); errors.Is(ctxErr, context.Canceled) || errors.Is(ctxErr, context.DeadlineExceeded) {
				return nil, fmt.Errorf("failed to paginate builds: page %d: %w", pagesScanned+1, ctxErr)
			}
			if anomalyDetected {
				return nil, fmt.Errorf("failed to paginate builds: page %d: %w", pagesScanned+1, err)
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

func findLatestBuildUploadNumber(ctx context.Context, client *asc.Client, appID string, versions []string, platform string) (buildNumber, *string, bool, error) {
	opts := []asc.BuildUploadsOption{
		asc.WithBuildUploadsStates([]string{"AWAITING_UPLOAD", "PROCESSING", "COMPLETE"}),
		asc.WithBuildUploadsLimit(200),
	}
	if len(versions) > 0 {
		opts = append(opts, asc.WithBuildUploadsCFBundleShortVersionStrings(versions))
	}
	if strings.TrimSpace(platform) != "" {
		opts = append(opts, asc.WithBuildUploadsPlatforms([]string{platform}))
	}

	firstPageCtx, firstPageCancel := contextWithTimeout(ctx)
	uploads, err := client.GetBuildUploads(firstPageCtx, appID, opts...)
	firstPageCancel()
	if err != nil {
		return buildNumber{}, nil, false, buildUploadHistoryError(appID, "failed to fetch build uploads", err)
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
		requestCtx, cancel := contextWithTimeout(ctx)
		defer cancel()
		return client.GetBuildUploads(requestCtx, appID, asc.WithBuildUploadsNextURL(nextURL))
	}, func(page asc.PaginatedResponse) error {
		resp, ok := page.(*asc.BuildUploadsResponse)
		if !ok {
			return fmt.Errorf("unexpected build uploads page type %T", page)
		}
		return processPage(resp)
	})
	if err != nil {
		return buildNumber{}, nil, false, buildUploadHistoryError(appID, "failed to paginate build uploads", err)
	}

	return latestUploadValue, latestUploadNumber, hasUpload, nil
}

func buildUploadHistoryError(appID, operation string, err error) error {
	if !errors.Is(err, asc.ErrForbidden) && !asc.IsNotFound(err) {
		return fmt.Errorf("%s: %w", operation, err)
	}

	return fmt.Errorf(
		"build upload history is unavailable for app %q; refusing to guess because an in-flight upload may already use the next number. Verify access with asc builds uploads list --app %q --paginate: %w",
		appID,
		appID,
		err,
	)
}

// skippedBuildNumberReporter warns once per processed build whose build number
// cannot be interpreted as a positive integer, so a single legacy
// CFBundleVersion anywhere in an app's history never aborts build number
// resolution.
type skippedBuildNumberReporter struct {
	warned map[string]struct{}
}

func (r *skippedBuildNumberReporter) warn(buildID, rawBuildNumber string) {
	if r == nil {
		return
	}
	if r.warned == nil {
		r.warned = make(map[string]struct{})
	}
	if _, seen := r.warned[buildID]; seen {
		return
	}
	r.warned[buildID] = struct{}{}
	fmt.Fprintf(
		os.Stderr,
		"Warning: skipping processed build %s: build number %q is not a positive integer\n",
		buildID,
		rawBuildNumber,
	)
}

// parseProcessedBuildNumber parses a processed build's number and reports
// whether it is usable. Callers skip unusable values instead of failing.
func parseProcessedBuildNumber(raw string) (buildNumber, bool) {
	parsed, err := parseBuildNumber(raw, "processed build")
	if err != nil {
		return buildNumber{}, false
	}
	return parsed, true
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

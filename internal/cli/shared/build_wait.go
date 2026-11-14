package shared

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/xcode"
)

// PublishDefaultPollInterval is the default polling interval for build discovery.
const PublishDefaultPollInterval = 30 * time.Second

type buildUploadFailureDiagnosticsFunc func(context.Context, *asc.Client, string, *asc.BuildUploadResponse) (string, error)

var (
	buildUploadFailureDiagnosticsFn buildUploadFailureDiagnosticsFunc = diagnoseBuildUploadFailure
	buildStatusBundleIDSupportedFn                                    = xcode.SupportsBuildStatusBundleID
)

// ContextWithTimeoutDuration creates a context with a specific timeout.
func ContextWithTimeoutDuration(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return withTimeoutContext(ctx, timeout)
}

// WaitForBuildByNumberOrUploadFailure waits for a build matching version/build
// number to appear while also watching the originating build upload for early
// failure states. This prevents long hangs when App Store Connect rejects the
// uploaded artifact before a build record is created.
func WaitForBuildByNumberOrUploadFailure(ctx context.Context, client *asc.Client, appID, uploadID, version, buildNumber, platform string, pollInterval time.Duration) (*asc.BuildResponse, error) {
	if pollInterval <= 0 {
		pollInterval = PublishDefaultPollInterval
	}
	buildNumber = strings.TrimSpace(buildNumber)
	if buildNumber == "" {
		return nil, fmt.Errorf("build number is required to resolve build")
	}
	uploadID = strings.TrimSpace(uploadID)

	preReleaseID := ""
	return asc.PollUntilTolerant(ctx, pollInterval, func(ctx context.Context) (*asc.BuildResponse, bool, error) {
		if uploadID != "" {
			upload, err := getBuildUploadWithLinkedBuild(ctx, client, uploadID)
			if err != nil {
				if !shouldIgnoreBuildWaitLookupError(err) {
					return nil, false, err
				}
			} else {
				if err := BuildUploadFailureError(upload); err != nil {
					return nil, false, enrichBuildUploadFailure(ctx, client, appID, upload, err)
				}
				buildID, err := buildIDForUpload(upload)
				if err != nil {
					return nil, false, err
				}
				if buildID != "" {
					// Keep upload-status probing best-effort only for linked-build
					// lookups that legitimately have not materialized yet.
					build, err := client.GetBuild(ctx, buildID)
					if err != nil {
						if !shouldIgnoreBuildWaitLookupError(err) {
							return nil, false, err
						}
					} else {
						return build, true, nil
					}
				}
			}
		}
		if preReleaseID == "" {
			id, err := findPreReleaseVersionIDForBuildWait(ctx, client, appID, version, platform)
			if err != nil {
				return nil, false, err
			}
			if id == "" {
				return nil, false, nil
			}
			preReleaseID = id
		}
		build, err := findBuildByNumber(ctx, client, appID, preReleaseID, buildNumber, uploadID)
		if err != nil {
			return nil, false, err
		}
		if build != nil {
			return build, true, nil
		}
		return nil, false, nil
	}, asc.PollOptions{Tolerate: isTransientBuildWaitError})
}

// getBuildUploadWithLinkedBuild reads a build upload including its build
// relationship. App Store Connect omits the buildUpload -> build linkage unless
// include=build is requested, so plain reads never reveal the created build.
func getBuildUploadWithLinkedBuild(ctx context.Context, client *asc.Client, uploadID string) (*asc.BuildUploadResponse, error) {
	return client.GetBuildUpload(ctx, uploadID, asc.WithBuildUploadInclude([]string{"build"}))
}

// BuildUploadWaitSelector identifies the build uploads a build wait reports on
// while no build matching its selector is visible yet. Empty fields do not
// filter.
type BuildUploadWaitSelector struct {
	AppID       string
	Version     string
	BuildNumber string
	Platform    string
	Since       *time.Time
}

// buildUploadWaitLookupLimit bounds the single page a wait reads. The filters
// already narrow it to the selector, and only the newest upload is reported.
const buildUploadWaitLookupLimit = 20

// LatestBuildUploadForWait returns the most recently uploaded build upload
// that matches selector, or nil when none is visible. The build number and
// platform filters are exact; the marketing version also matches its
// equivalent "1.2"/"1.2.0" spelling like build discovery does. Uploads whose
// upload (or, before the upload finishes, creation) time is before Since are
// skipped. Callers use the result only to describe a wait that has not found
// its build yet, so they may treat an error as "no upload information".
func LatestBuildUploadForWait(ctx context.Context, client *asc.Client, selector BuildUploadWaitSelector) (*asc.BuildUploadResponse, error) {
	appID := strings.TrimSpace(selector.AppID)
	if client == nil || appID == "" {
		return nil, nil
	}

	opts := []asc.BuildUploadsOption{
		asc.WithBuildUploadsSort("-uploadedDate"),
		asc.Limit[asc.BuildUploadsOption](buildUploadWaitLookupLimit),
	}
	if buildNumber := strings.TrimSpace(selector.BuildNumber); buildNumber != "" {
		opts = append(opts, asc.WithBuildUploadsCFBundleVersions([]string{buildNumber}))
	}
	if versions := versionQueryVariants(selector.Version); len(versions) > 0 {
		opts = append(opts, asc.WithBuildUploadsCFBundleShortVersionStrings(versions))
	}
	if platform := strings.TrimSpace(selector.Platform); platform != "" {
		opts = append(opts, asc.WithBuildUploadsPlatforms([]string{platform}))
	}

	uploads, err := client.GetBuildUploads(ctx, appID, opts...)
	if err != nil {
		return nil, err
	}

	var newest *asc.Resource[asc.BuildUploadAttributes]
	var newestAt time.Time
	for index := range uploads.Data {
		upload := &uploads.Data[index]
		at, known := buildUploadActivityTime(upload.Attributes)
		if selector.Since != nil && (!known || at.Before(selector.Since.UTC())) {
			continue
		}
		if newest == nil || (known && at.After(newestAt)) {
			newest = upload
			newestAt = at
		}
	}
	if newest == nil {
		return nil, nil
	}
	return &asc.BuildUploadResponse{Data: *newest}, nil
}

// buildUploadActivityTime is when an upload last moved: its upload time, or
// its creation time while the upload has not finished.
func buildUploadActivityTime(attributes asc.BuildUploadAttributes) (time.Time, bool) {
	for _, value := range []*string{attributes.UploadedDate, attributes.CreatedDate} {
		if value == nil || strings.TrimSpace(*value) == "" {
			continue
		}
		parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(*value))
		if err == nil {
			return parsed.UTC(), true
		}
	}
	return time.Time{}, false
}

// VerifyBuildUploadAfterCommit briefly watches a newly committed upload for
// immediate App Store Connect failures. It returns the ID of the build the
// upload created once App Store Connect links it, which ends the watch early.
// It returns an empty build ID and nil error on timeout so the caller can keep
// the default asynchronous success behavior when no failure is observed during
// the bounded verification window.
func VerifyBuildUploadAfterCommit(ctx context.Context, client *asc.Client, appID, uploadID string, pollInterval, verifyTimeout time.Duration) (string, error) {
	if client == nil {
		return "", nil
	}
	uploadID = strings.TrimSpace(uploadID)
	if uploadID == "" || verifyTimeout <= 0 {
		return "", nil
	}

	verifyCtx, cancel := ContextWithTimeoutDuration(ctx, verifyTimeout)
	defer cancel()

	effectiveInterval := pollInterval
	switch {
	case effectiveInterval <= 0:
		effectiveInterval = 5 * time.Second
	case effectiveInterval > 5*time.Second:
		effectiveInterval = 5 * time.Second
	}
	if effectiveInterval > verifyTimeout {
		effectiveInterval = verifyTimeout
	}
	if effectiveInterval <= 0 {
		effectiveInterval = time.Millisecond
	}

	callerCtx := ctx
	linkedBuildID := ""
	_, err := asc.PollUntil(verifyCtx, effectiveInterval, func(ctx context.Context) (*asc.BuildUploadResponse, bool, error) {
		upload, err := getBuildUploadWithLinkedBuild(ctx, client, uploadID)
		if err != nil {
			if isTerminalBuildWaitNotFound(err) {
				if callerCtx.Err() != nil {
					return nil, false, callerCtx.Err()
				}
				// Verification is best effort, but the request-level retry
				// budget has already been consumed. Stop instead of replaying
				// the same permanent lookup for the rest of this window.
				return nil, true, nil
			}
			// A retry delay that cannot fit in this bounded verification window
			// is already a terminal best-effort outcome: stop probing now and
			// preserve the caller's asynchronous-success behavior.
			if asc.IsRetryDelayExceeded(err) {
				return nil, true, nil
			}
			// Transient lookup errors stay ignorable for the whole verification
			// window: expiry of the bounded window itself is reported by the
			// poll context, not by this predicate.
			if shouldIgnoreBuildWaitLookupError(err) || asc.IsRetryDelayExceeded(err) || asc.IsTransientWaitError(callerCtx, err) {
				return nil, false, nil
			}
			return nil, false, err
		}
		if err := BuildUploadFailureError(upload); err != nil {
			return nil, false, enrichBuildUploadFailure(ctx, client, appID, upload, err)
		}
		buildID, err := buildIDForUpload(upload)
		if err != nil {
			return nil, false, err
		}
		if buildID == "" {
			// The builds collection exposes a new build shortly before the
			// upload links it. The lookup is best effort: a failure only
			// means this poll has not seen the build yet.
			buildID, _ = findBuildIDForUpload(ctx, client, appID, uploadID, upload.Data.Attributes.CFBundleVersion)
		}
		if buildID != "" {
			linkedBuildID = buildID
			return upload, true, nil
		}
		return nil, false, nil
	})
	if err == nil {
		return linkedBuildID, nil
	}
	if errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil {
		return "", nil
	}
	return "", err
}

// findBuildIDForUpload returns the ID of the build App Store Connect created
// from uploadID, matched by the build's own buildUpload linkage among the app's
// builds with the upload's build number. An empty ID means no such build is
// visible yet.
func findBuildIDForUpload(ctx context.Context, client *asc.Client, appID, uploadID, buildNumber string) (string, error) {
	appID = strings.TrimSpace(appID)
	uploadID = strings.TrimSpace(uploadID)
	buildNumber = strings.TrimSpace(buildNumber)
	if client == nil || appID == "" || uploadID == "" || buildNumber == "" {
		return "", nil
	}

	builds, err := client.GetBuilds(
		ctx, appID,
		asc.WithBuildsVersion(buildNumber),
		asc.WithBuildsInclude([]string{"buildUpload"}),
		asc.Limit[asc.BuildsOption](200),
	)
	if err != nil {
		return "", err
	}
	for _, build := range builds.Data {
		buildUploadID, err := buildUploadIDForBuild(build)
		if err != nil {
			return "", err
		}
		if buildUploadID == uploadID {
			return strings.TrimSpace(build.ID), nil
		}
	}
	return "", nil
}

func findBuildByNumber(ctx context.Context, client *asc.Client, appID, preReleaseID, buildNumber, uploadID string) (*asc.BuildResponse, error) {
	buildOpts := []asc.BuildsOption{
		asc.WithBuildsPreReleaseVersion(preReleaseID),
		asc.WithBuildsSort("-uploadedDate"),
		asc.Limit[asc.BuildsOption](200),
	}
	if uploadID != "" {
		buildOpts = append(buildOpts, asc.WithBuildsInclude([]string{"buildUpload"}))
	}
	buildsResp, err := client.GetBuilds(ctx, appID, buildOpts...)
	if err != nil {
		return nil, err
	}
	for _, build := range buildsResp.Data {
		if strings.TrimSpace(build.Attributes.Version) != buildNumber {
			continue
		}
		if uploadID != "" {
			buildUploadID, err := buildUploadIDForBuild(build)
			if err != nil {
				return nil, err
			}
			if buildUploadID != uploadID {
				continue
			}
		}
		return &asc.BuildResponse{Data: build}, nil
	}
	return nil, nil
}

// findPreReleaseVersionIDForBuildWait resolves the pre-release version train a
// build wait should watch. App Store Connect treats "1.2" and "1.2.0" as the
// same version but only makes the first-uploaded format queryable, so the
// requested format is tried first and the equivalent format only when it
// matches nothing. An empty ID means no matching train exists yet.
func findPreReleaseVersionIDForBuildWait(ctx context.Context, client *asc.Client, appID, version, platform string) (string, error) {
	requestedVersion := strings.TrimSpace(version)

	variants := versionQueryVariants(requestedVersion)
	if len(variants) == 0 {
		variants = []string{""}
	}

	for _, variant := range variants {
		ids, _, err := findPreReleaseVersionIDsForVersions(ctx, client, appID, []string{variant}, platform, 0)
		if err != nil {
			return "", err
		}
		if len(ids) == 0 {
			continue
		}
		if len(ids) > 1 {
			candidates := make([]AmbiguousCandidate, 0, len(ids))
			for _, id := range ids {
				candidates = append(candidates, AmbiguousCandidate{ID: id})
			}
			return "", &AmbiguousSelectionError{
				Kind:        "pre-release version",
				Description: fmt.Sprintf("version %q on platform %q", version, platform),
				Candidates:  candidates,
				Hint:        "Pass --build-id to wait for a specific build.",
			}
		}
		noteEquivalentVersionMatch(requestedVersion, variant)
		return ids[0], nil
	}

	return "", nil
}

type buildRelationships struct {
	BuildUpload *asc.Relationship `json:"buildUpload,omitempty"`
}

func buildUploadIDForBuild(build asc.Resource[asc.BuildAttributes]) (string, error) {
	if len(build.Relationships) == 0 {
		return "", nil
	}

	var relationships buildRelationships
	if err := json.Unmarshal(build.Relationships, &relationships); err != nil {
		return "", fmt.Errorf("parse build %q relationships: %w", strings.TrimSpace(build.ID), err)
	}
	if relationships.BuildUpload == nil {
		return "", nil
	}
	return strings.TrimSpace(relationships.BuildUpload.Data.ID), nil
}

type buildUploadRelationships struct {
	Build *asc.Relationship `json:"build,omitempty"`
}

func buildIDForUpload(upload *asc.BuildUploadResponse) (string, error) {
	if upload == nil || len(upload.Data.Relationships) == 0 {
		return "", nil
	}

	var relationships buildUploadRelationships
	if err := json.Unmarshal(upload.Data.Relationships, &relationships); err != nil {
		return "", fmt.Errorf("parse build upload %q relationships: %w", strings.TrimSpace(upload.Data.ID), err)
	}
	if relationships.Build == nil {
		return "", nil
	}
	return strings.TrimSpace(relationships.Build.Data.ID), nil
}

// BuildUploadFailureError describes a FAILED build upload with Apple's error
// details and recovery guidance. It returns nil for any other upload state.
func BuildUploadFailureError(upload *asc.BuildUploadResponse) error {
	if upload == nil || upload.Data.Attributes.State == nil || upload.Data.Attributes.State.State == nil {
		return nil
	}

	state := strings.ToUpper(strings.TrimSpace(*upload.Data.Attributes.State.State))
	if state != "FAILED" {
		return nil
	}

	message := fmt.Sprintf("build upload %q failed with state %s", upload.Data.ID, state)
	if details := buildUploadStateDetails(upload.Data.Attributes.State.Errors); details != "" {
		message += ": " + details
	}
	if recovery := buildUploadRecoveryGuidance(upload.Data.Attributes.State.Errors); recovery != "" {
		message += "; recovery: " + recovery
	}
	return buildStateFailure(errors.New(message))
}

// buildStateFailure classifies App Store Connect rejecting an uploaded build
// as an expected negative instead of a CLI defect.
func buildStateFailure(err error) error {
	return WithDiagnostic(NewValidationError(err), DiagnosticStateNotReady, "")
}

var usageDescriptionKeyPattern = regexp.MustCompile(`\b[A-Za-z0-9_]+UsageDescription\b`)

func buildUploadRecoveryGuidance(details []asc.StateDetail) string {
	if recovery, ok := buildUploadVersionRecoveryGuidance(details); ok {
		return recovery
	}

	switch {
	case allStateDetailCodesIn(details, "90062", "90186", "90478"):
		return "increase the marketing version (CFBundleShortVersionString), rebuild, and upload again"
	case allStateDetailCodesIn(details, "90189"):
		return "increase the build number (CFBundleVersion), rebuild, and upload again"
	case allStateDetailCodesIn(details, "90683"):
		keys := missingUsageDescriptionKeys(details)
		if len(keys) > 0 {
			return fmt.Sprintf("add the missing privacy purpose strings to Info.plist (%s), rebuild, and upload again", strings.Join(keys, ", "))
		}
		return "add the missing privacy purpose strings to Info.plist, rebuild, and upload again"
	case allStateDetailCodesIn(details, "90725"):
		return "rebuild with a currently supported SDK and toolchain, then upload again"
	case allStateDetailCodesIn(details, "90771"):
		return "add BGTaskSchedulerPermittedIdentifiers to Info.plist with every scheduled background task identifier, rebuild, and upload again"
	case allStateDetailCodesIn(details, "90391", "90713"):
		return "add the required app icons and icon metadata (such as CFBundleIconName or CFBundleIconFiles) to every failing bundle, rebuild, and upload again"
	default:
		return ""
	}
}

func buildUploadVersionRecoveryGuidance(details []asc.StateDetail) (string, bool) {
	if !allStateDetailCodesIn(details, "90054", "90055") {
		return "", false
	}

	invalidBuildNumber := false
	bundleIdentifierMismatch := false
	for _, detail := range details {
		switch {
		case strings.TrimSpace(detail.Code) == "90054" && stateDetailTextContains([]asc.StateDetail{detail}, "cfbundleversion"):
			invalidBuildNumber = true
		case stateDetailTextContains([]asc.StateDetail{detail}, "bundle identifier"):
			bundleIdentifierMismatch = true
		default:
			return "", false
		}
	}

	recoveries := make([]string, 0, 2)
	if invalidBuildNumber {
		recoveries = append(recoveries, "format CFBundleVersion as a period-separated list of at most three non-negative integers, rebuild, and upload again")
	}
	if bundleIdentifierMismatch {
		recoveries = append(recoveries, "verify that the artifact's bundle identifier matches the selected app; rebuild with the correct identifier or select the intended app")
	}
	return strings.Join(recoveries, "; "), len(recoveries) > 0
}

// allStateDetailCodesIn reports whether every received error belongs to one
// known code family. Codes in a family are alternatives and need not all occur.
func allStateDetailCodesIn(details []asc.StateDetail, allowed ...string) bool {
	if len(details) == 0 {
		return false
	}

	allowedCodes := make(map[string]struct{}, len(allowed))
	for _, code := range allowed {
		allowedCodes[code] = struct{}{}
	}
	for _, detail := range details {
		code := strings.TrimSpace(detail.Code)
		if _, ok := allowedCodes[code]; !ok {
			return false
		}
	}
	return true
}

func stateDetailTextContains(details []asc.StateDetail, fragment string) bool {
	fragment = strings.ToLower(strings.TrimSpace(fragment))
	if fragment == "" {
		return false
	}
	for _, detail := range details {
		for _, text := range []string{detail.Message, detail.Description} {
			if strings.Contains(strings.ToLower(text), fragment) {
				return true
			}
		}
	}
	return false
}

func missingUsageDescriptionKeys(details []asc.StateDetail) []string {
	keys := make(map[string]struct{})
	for _, detail := range details {
		for _, text := range []string{detail.Message, detail.Description} {
			for _, key := range usageDescriptionKeyPattern.FindAllString(strings.TrimSpace(text), -1) {
				keys[key] = struct{}{}
			}
		}
	}

	result := make([]string, 0, len(keys))
	for key := range keys {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}

// BuildProcessingFailureContext identifies the artifact behind a build that
// finished processing in a failure state, so the upload it came from can be
// matched even when the caller selected the build by ID alone.
type BuildProcessingFailureContext struct {
	AppID         string
	BuildID       string
	BundleVersion string
	ShortVersion  string
	Platform      string
}

// WaitForBuildProcessingWithDetails waits for a build to finish processing
// like asc.Client.WaitForBuildProcessing, and appends the App Store Connect
// processing details of the originating upload when the build ends FAILED or
// INVALID. appID may be empty; it is then resolved from the build. The state
// error is returned unchanged when no details can be resolved.
func WaitForBuildProcessingWithDetails(ctx context.Context, client *asc.Client, appID, buildID string, pollInterval time.Duration) (*asc.BuildResponse, error) {
	build, err := client.WaitForBuildProcessing(ctx, buildID, pollInterval)
	if err == nil || !asc.IsBuildProcessingFailure(err) {
		return build, err
	}

	failure := BuildProcessingFailureContext{AppID: appID, BuildID: buildID}
	if build != nil {
		failure.BundleVersion = build.Data.Attributes.Version
	}
	return build, buildStateFailure(EnrichBuildProcessingFailure(ctx, client, failure, err))
}

// EnrichBuildProcessingFailure appends the App Store Connect processing
// details of the originating build upload to a terminal build-processing
// error. The base error is returned unchanged when no upload matches or no
// details are available, so a failed lookup never masks the state error.
func EnrichBuildProcessingFailure(ctx context.Context, client *asc.Client, failure BuildProcessingFailureContext, baseErr error) error {
	if baseErr == nil {
		return nil
	}

	failure = normalizeBuildProcessingFailureContext(failure)
	if client == nil || failure.BundleVersion == "" {
		return baseErr
	}
	resolveBuildProcessingFailureContext(ctx, client, &failure)

	upload, err := findBuildUploadForProcessingFailure(ctx, client, failure)
	if err != nil || upload == nil {
		return baseErr
	}
	return enrichBuildUploadFailure(ctx, client, failure.AppID, upload, baseErr)
}

func normalizeBuildProcessingFailureContext(failure BuildProcessingFailureContext) BuildProcessingFailureContext {
	failure.AppID = strings.TrimSpace(failure.AppID)
	failure.BuildID = strings.TrimSpace(failure.BuildID)
	failure.BundleVersion = strings.TrimSpace(failure.BundleVersion)
	failure.ShortVersion = strings.TrimSpace(failure.ShortVersion)
	failure.Platform = strings.TrimSpace(failure.Platform)
	return failure
}

// resolveBuildProcessingFailureContext describes the failed build from App
// Store Connect itself, so waits by build ID can match the upload the build
// came from and selectors cannot mismatch it. Every lookup is best effort: an
// unresolved field only means the failure is reported without added details.
func resolveBuildProcessingFailureContext(ctx context.Context, client *asc.Client, failure *BuildProcessingFailureContext) {
	if failure.BuildID == "" {
		return
	}

	if failure.AppID == "" {
		if app, err := client.GetBuildApp(ctx, failure.BuildID); err == nil && app != nil {
			failure.AppID = strings.TrimSpace(app.Data.ID)
		}
	}

	// App Store Connect treats spellings such as "1.2" and "1.2.0" as the same
	// train but stores only the uploaded one, so the build's own pre-release
	// version wins over the spelling the selector asked for.
	preRelease, err := client.GetBuildPreReleaseVersion(ctx, failure.BuildID)
	if err != nil || preRelease == nil {
		return
	}
	if version := strings.TrimSpace(preRelease.Data.Attributes.Version); version != "" {
		failure.ShortVersion = version
	}
	if platform := strings.TrimSpace(string(preRelease.Data.Attributes.Platform)); platform != "" {
		failure.Platform = platform
	}
}

const buildProcessingFailureLookupLimit = 200

// findBuildUploadForProcessingFailure resolves the upload that produced a
// failed build, preferring the upload App Store Connect links to the build
// itself so a later retry of the same build number cannot be reported instead.
func findBuildUploadForProcessingFailure(ctx context.Context, client *asc.Client, failure BuildProcessingFailureContext) (*asc.BuildUploadResponse, error) {
	if client == nil {
		return nil, nil
	}

	appID := strings.TrimSpace(failure.AppID)
	bundleVersion := strings.TrimSpace(failure.BundleVersion)
	if appID == "" || bundleVersion == "" {
		return nil, nil
	}

	if uploadID := linkedBuildUploadID(ctx, client, appID, strings.TrimSpace(failure.BuildID), bundleVersion); uploadID != "" {
		if upload, err := client.GetBuildUpload(ctx, uploadID); err == nil && upload != nil {
			return upload, nil
		}
	}
	return findBuildUploadByVersion(ctx, client, appID, bundleVersion, strings.TrimSpace(failure.ShortVersion), strings.TrimSpace(failure.Platform))
}

// linkedBuildUploadID resolves the upload a build was created from. It returns
// an empty ID when the linkage is unavailable, leaving the version-matched
// lookup as the fallback.
func linkedBuildUploadID(ctx context.Context, client *asc.Client, appID, buildID, bundleVersion string) string {
	if buildID == "" {
		return ""
	}

	builds, err := client.GetBuilds(
		ctx, appID,
		asc.WithBuildsVersion(bundleVersion),
		asc.WithBuildsInclude([]string{"buildUpload"}),
		asc.Limit[asc.BuildsOption](buildProcessingFailureLookupLimit),
	)
	if err != nil {
		return ""
	}
	for _, build := range builds.Data {
		if strings.TrimSpace(build.ID) != buildID {
			continue
		}
		uploadID, err := buildUploadIDForBuild(build)
		if err != nil {
			return ""
		}
		return uploadID
	}
	return ""
}

// findBuildUploadByVersion matches an upload by the artifact identity it was
// uploaded with. Processing details are reported per app, build number,
// marketing version, and platform, but retries can reuse that identity, so a
// fallback is safe only when exactly one matching upload is visible. An
// incomplete or paginated result is left unmatched rather than guessed.
func findBuildUploadByVersion(ctx context.Context, client *asc.Client, appID, bundleVersion, shortVersion, platform string) (*asc.BuildUploadResponse, error) {
	if shortVersion == "" || platform == "" {
		return nil, nil
	}

	uploads, err := client.GetBuildUploads(
		ctx, appID,
		asc.WithBuildUploadsCFBundleVersions([]string{bundleVersion}),
		asc.WithBuildUploadsCFBundleShortVersionStrings([]string{shortVersion}),
		asc.WithBuildUploadsPlatforms([]string{platform}),
		asc.WithBuildUploadsSort("-uploadedDate"),
		asc.Limit[asc.BuildUploadsOption](buildProcessingFailureLookupLimit),
	)
	if err != nil {
		return nil, err
	}
	var match asc.Resource[asc.BuildUploadAttributes]
	matches := 0
	for _, upload := range uploads.Data {
		if strings.TrimSpace(upload.Attributes.CFBundleVersion) != bundleVersion {
			continue
		}
		if strings.TrimSpace(upload.Attributes.CFBundleShortVersionString) != shortVersion {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(string(upload.Attributes.Platform)), platform) {
			continue
		}
		matches++
		if matches > 1 {
			return nil, nil
		}
		match = upload
	}
	if matches != 1 || strings.TrimSpace(match.ID) == "" {
		return nil, nil
	}
	if strings.TrimSpace(uploads.Links.Next) != "" {
		return nil, nil
	}
	if total, ok := asc.ParsePagingTotalOK(uploads.Meta); ok && total != 1 {
		return nil, nil
	}
	return &asc.BuildUploadResponse{Data: match}, nil
}

func enrichBuildUploadFailure(ctx context.Context, client *asc.Client, appID string, upload *asc.BuildUploadResponse, baseErr error) error {
	if baseErr == nil {
		return nil
	}
	details, err := buildUploadFailureDiagnosticsFn(ctx, client, appID, upload)
	if err != nil {
		return baseErr
	}
	details = strings.TrimSpace(details)
	if details == "" || strings.Contains(baseErr.Error(), details) {
		return baseErr
	}
	return fmt.Errorf("%w; App Store Connect processing details: %s", baseErr, details)
}

func diagnoseBuildUploadFailure(ctx context.Context, client *asc.Client, appID string, upload *asc.BuildUploadResponse) (string, error) {
	if upload == nil {
		return "", nil
	}

	appID = strings.TrimSpace(appID)
	buildNumber := strings.TrimSpace(upload.Data.Attributes.CFBundleVersion)
	if appID == "" || buildNumber == "" {
		return "", nil
	}

	creds, err := ResolveAuthCredentials("")
	if err != nil {
		return "", err
	}
	keyPath, err := AltoolPrivateKeyPath(creds)
	if err != nil {
		return "", err
	}

	bundleID := resolveBuildStatusBundleID(ctx, client, appID)
	result, err := xcode.BuildStatus(ctx, xcode.BuildStatusOptions{
		AppleID:            appID,
		BundleID:           bundleID,
		BundleVersion:      buildNumber,
		BundleShortVersion: strings.TrimSpace(upload.Data.Attributes.CFBundleShortVersionString),
		Platform:           string(upload.Data.Attributes.Platform),
		APIKey:             strings.TrimSpace(creds.KeyID),
		APIIssuer:          strings.TrimSpace(creds.IssuerID),
		P8FilePath:         keyPath,
	})
	if err != nil {
		return "", err
	}
	return joinDiagnosticDetails(result.ProcessingErrors), nil
}

func resolveBuildStatusBundleID(ctx context.Context, client *asc.Client, appID string) string {
	if client == nil || !buildStatusBundleIDSupportedFn(ctx) {
		return ""
	}

	appID = strings.TrimSpace(appID)
	if appID == "" {
		return ""
	}

	app, err := client.GetApp(ctx, appID)
	if err != nil || app == nil {
		return ""
	}
	return strings.TrimSpace(app.Data.Attributes.BundleID)
}

// AltoolPrivateKeyPath returns a .p8 path for altool's --p8-file-path, writing
// PEM material loaded from the keychain or environment to a private temp file.
// It returns "" when creds carry no usable key.
func AltoolPrivateKeyPath(creds ResolvedAuthCredentials) (string, error) {
	if pem := strings.TrimSpace(creds.KeyPEM); pem != "" {
		if decoded, cacheKey, ok := decodeBuildStatusPrivateKeyPEMBase64(pem); ok {
			if path := cachedTempPrivateKeyPath(cacheKey); path != "" {
				return path, nil
			}
			return writeTempPrivateKey(decoded, cacheKey)
		}
		normalized := normalizePrivateKeyValue(pem)
		cacheKey := tempPrivateKeyCacheKey("raw", normalized)
		if path := cachedTempPrivateKeyPath(cacheKey); path != "" {
			return path, nil
		}
		return writeTempPrivateKey([]byte(normalized), cacheKey)
	}
	if path := strings.TrimSpace(creds.KeyPath); path != "" {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, nil
		}
	}
	return "", nil
}

func decodeBuildStatusPrivateKeyPEMBase64(value string) ([]byte, string, bool) {
	compact := strings.Join(strings.Fields(value), "")
	if compact == "" {
		return nil, "", false
	}
	decoded, err := decodeBase64Secret(value)
	if err != nil {
		return nil, "", false
	}
	normalized := normalizePrivateKeyValue(string(decoded))
	if !looksLikePrivateKeyPEM(normalized) {
		return nil, "", false
	}
	return []byte(normalized), tempPrivateKeyCacheKey("b64", compact), true
}

func looksLikePrivateKeyPEM(value string) bool {
	normalized := normalizePrivateKeyValue(value)
	return strings.Contains(normalized, "BEGIN ") && strings.Contains(normalized, "PRIVATE KEY")
}

func joinDiagnosticDetails(values []string) string {
	return strings.Join(xcode.UniqueDiagnosticDetails(values), "; ")
}

func shouldIgnoreBuildWaitLookupError(err error) bool {
	return asc.IsNotFound(err) && !isTerminalBuildWaitNotFound(err)
}

func isTransientBuildWaitError(ctx context.Context, err error) bool {
	if isTerminalBuildWaitNotFound(err) {
		return false
	}
	return asc.IsTransientWaitError(ctx, err)
}

func isTerminalBuildWaitNotFound(err error) bool {
	if !asc.IsRetryBudgetExhausted(err) && !asc.IsRetryDelayExceeded(err) {
		return false
	}

	var apiErr *asc.APIError
	if !errors.As(err, &apiErr) || apiErr == nil {
		return false
	}
	return apiErr.StatusCode == http.StatusNotFound && strings.EqualFold(apiErr.Code, "NOT_FOUND")
}

// SetBuildUploadFailureDiagnosticsForTesting overrides build failure enrichment.
// Tests only.
func SetBuildUploadFailureDiagnosticsForTesting(fn func(context.Context, *asc.Client, string, *asc.BuildUploadResponse) (string, error)) func() {
	previous := buildUploadFailureDiagnosticsFn
	if fn == nil {
		buildUploadFailureDiagnosticsFn = diagnoseBuildUploadFailure
	} else {
		buildUploadFailureDiagnosticsFn = fn
	}
	return func() {
		buildUploadFailureDiagnosticsFn = previous
	}
}

func buildUploadStateDetails(details []asc.StateDetail) string {
	if len(details) == 0 {
		return ""
	}

	parts := make([]string, 0, len(details))
	for _, detail := range details {
		code := strings.TrimSpace(detail.Code)
		message := strings.TrimSpace(detail.Description)
		if message == "" {
			message = strings.TrimSpace(detail.Message)
		}
		switch {
		case code != "" && message != "":
			parts = append(parts, fmt.Sprintf("%s (%s)", code, message))
		case code != "":
			parts = append(parts, code)
		case message != "":
			parts = append(parts, message)
		}
	}

	return strings.Join(parts, ", ")
}

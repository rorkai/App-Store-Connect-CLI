package submit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// BuildAttachmentResult captures the resolved state of ensuring a build is
// attached to an App Store version.
type BuildAttachmentResult struct {
	VersionID       string `json:"versionId"`
	BuildID         string `json:"buildId"`
	CurrentBuildID  string `json:"currentBuildId,omitempty"`
	Attached        bool   `json:"attached,omitempty"`
	AlreadyAttached bool   `json:"alreadyAttached,omitempty"`
	WouldAttach     bool   `json:"wouldAttach,omitempty"`
}

// SubmitResolvedVersionOptions configures the shared App Store submission flow
// used by submit, release, and publish surfaces.
type SubmitResolvedVersionOptions struct {
	AppID                    string
	VersionID                string
	BuildID                  string
	Platform                 string
	RequestTimeout           time.Duration
	EnsureBuildAttached      bool
	LookupExistingSubmission bool
	DryRun                   bool
	Emit                     func(string)
}

// SubmitResolvedVersionResult captures the outcome of creating/submitting a
// review submission for an already-resolved version.
type SubmitResolvedVersionResult struct {
	SubmissionID     string                 `json:"submissionId,omitempty"`
	SubmittedDate    string                 `json:"submittedDate,omitempty"`
	AlreadySubmitted bool                   `json:"alreadySubmitted,omitempty"`
	WouldSubmit      bool                   `json:"wouldSubmit,omitempty"`
	BuildAttachment  *BuildAttachmentResult `json:"buildAttachment,omitempty"`
	Messages         []string               `json:"messages,omitempty"`
}

// SubmissionLocalizationPreflightWith runs the submission-blocking
// localization preflight used by submit-style App Store review flows on d.
func SubmissionLocalizationPreflightWith(ctx context.Context, d *shared.Deps, client *asc.Client, appID, versionID, platform, retryCommand string) error {
	return runSubmissionLocalizationPreflight(ctx, d, client, appID, versionID, platform, 0, "", retryCommand)
}

// SubmissionLocalizationPreflightWithTimeout runs localization preflight with
// an explicit request budget when the caller needs a per-phase timeout.
func SubmissionLocalizationPreflightWithTimeout(
	ctx context.Context,
	d *shared.Deps,
	client *asc.Client,
	appID, versionID, platform string,
	requestTimeout time.Duration,
	retryCommand string,
) error {
	return runSubmissionLocalizationPreflight(ctx, d, client, appID, versionID, platform, requestTimeout, "", retryCommand)
}

// SubmissionSubscriptionPreflightWith runs the advisory subscription preflight
// used by submit-style App Store review flows on d.
func SubmissionSubscriptionPreflightWith(ctx context.Context, d *shared.Deps, client *asc.Client, appID, retryCommand string) {
	runSubmissionSubscriptionPreflight(ctx, d, client, appID, 0, retryCommand)
}

// SubmissionAgeRatingPreflightWith runs the age rating declaration check on d.
func SubmissionAgeRatingPreflightWith(ctx context.Context, d *shared.Deps, client *asc.Client, appID, versionID string, requestTimeout time.Duration, retryCommand string) error {
	return runSubmissionAgeRatingPreflight(ctx, d, client, appID, versionID, requestTimeout, retryCommand)
}

// SubmissionSubscriptionPreflightWithTimeout runs subscription preflight with
// an explicit request budget when the caller needs a per-phase timeout.
func SubmissionSubscriptionPreflightWithTimeout(
	ctx context.Context,
	d *shared.Deps,
	client *asc.Client,
	appID string,
	requestTimeout time.Duration,
	retryCommand string,
) {
	runSubmissionSubscriptionPreflight(ctx, d, client, appID, requestTimeout, retryCommand)
}

// LookupExistingSubmissionForVersion returns the existing legacy submission ID
// for an App Store version, if one already exists.
func LookupExistingSubmissionForVersion(ctx context.Context, client *asc.Client, versionID string, requestTimeout time.Duration) (string, error) {
	resolvedVersionID := strings.TrimSpace(versionID)
	if resolvedVersionID == "" {
		return "", fmt.Errorf("resolved version ID is empty")
	}

	lookupCtx, lookupCancel := submitResolvedVersionPhaseContext(ctx, requestTimeout)
	legacySubmission, err := client.GetAppStoreVersionSubmissionForVersion(lookupCtx, resolvedVersionID)
	lookupCancel()
	if err != nil {
		if asc.IsNotFound(err) {
			return "", nil
		}
		return "", err
	}

	return strings.TrimSpace(legacySubmission.Data.ID), nil
}

// EnsureBuildAttachedWith ensures the target build is attached to the resolved
// App Store version, on d. In dry-run mode it reports the planned change
// without mutating.
func EnsureBuildAttachedWith(ctx context.Context, d *shared.Deps, client *asc.Client, versionID, buildID string, dryRun bool) (BuildAttachmentResult, error) {
	return ensureBuildAttached(ctx, d, client, versionID, buildID, dryRun)
}

func ensureBuildAttached(ctx context.Context, d *shared.Deps, client *asc.Client, versionID, buildID string, dryRun bool) (BuildAttachmentResult, error) {
	result := BuildAttachmentResult{
		VersionID: strings.TrimSpace(versionID),
		BuildID:   strings.TrimSpace(buildID),
	}
	if result.VersionID == "" {
		return result, fmt.Errorf("attach build: resolved version ID is empty")
	}
	if result.BuildID == "" {
		return result, fmt.Errorf("attach build: build ID is required")
	}

	buildResp, err := shared.RetryReadWithFreshTimeoutWith(ctx, d, func(requestCtx context.Context) (*asc.BuildResponse, error) {
		return client.GetAppStoreVersionBuild(requestCtx, result.VersionID)
	})
	if err != nil {
		if !asc.IsNotFound(err) {
			return result, fmt.Errorf("attach build: failed to fetch current build: %w", err)
		}
	} else {
		result.CurrentBuildID = strings.TrimSpace(buildResp.Data.ID)
	}

	if result.CurrentBuildID == result.BuildID {
		result.AlreadyAttached = true
		return result, nil
	}

	if dryRun {
		result.WouldAttach = true
		return result, nil
	}

	_, _, err = shared.RunReconciledMutationWith(
		ctx,
		d,
		func(requestCtx context.Context) (string, error) {
			attachErr := client.AttachBuildToVersion(requestCtx, result.VersionID, result.BuildID)
			return result.BuildID, attachErr
		},
		func(readbackCtx context.Context) (string, bool, error) {
			current, readErr := shared.RetryReadWithFreshTimeoutWith(readbackCtx, d, func(requestCtx context.Context) (*asc.BuildResponse, error) {
				return client.GetAppStoreVersionBuild(requestCtx, result.VersionID)
			})
			if readErr != nil {
				if asc.IsNotFound(readErr) {
					return "", false, nil
				}
				return "", false, readErr
			}
			currentID := strings.TrimSpace(current.Data.ID)
			return currentID, currentID == result.BuildID, nil
		},
	)
	if err != nil {
		return result, fmt.Errorf("attach build: %w", err)
	}
	result.Attached = true
	return result, nil
}

// SubmitResolvedVersionWith runs the shared modern review-submission flow for
// an already-resolved version ID, on d.
func SubmitResolvedVersionWith(ctx context.Context, d *shared.Deps, client *asc.Client, opts SubmitResolvedVersionOptions) (SubmitResolvedVersionResult, error) {
	return submitResolvedVersion(ctx, d, client, opts)
}

func submitResolvedVersion(ctx context.Context, d *shared.Deps, client *asc.Client, opts SubmitResolvedVersionOptions) (SubmitResolvedVersionResult, error) {
	result := SubmitResolvedVersionResult{
		Messages: make([]string, 0),
	}

	emit := func(message string) {
		trimmed := strings.TrimSpace(message)
		if trimmed == "" {
			return
		}
		result.Messages = append(result.Messages, trimmed)
		if opts.Emit != nil {
			opts.Emit(trimmed)
		}
	}

	versionID := strings.TrimSpace(opts.VersionID)
	if versionID == "" {
		return result, fmt.Errorf("submit review: resolved version ID is empty")
	}
	appID := strings.TrimSpace(opts.AppID)
	if appID == "" {
		return result, fmt.Errorf("submit review: app ID is required")
	}
	platform := strings.TrimSpace(opts.Platform)
	if platform == "" {
		return result, fmt.Errorf("submit review: platform is required")
	}

	if opts.LookupExistingSubmission {
		existingSubmissionID, err := LookupExistingSubmissionForVersion(ctx, client, versionID, opts.RequestTimeout)
		if err != nil {
			return result, fmt.Errorf("submit review: failed to lookup existing submission: %w", err)
		}
		if existingSubmissionID != "" {
			result.AlreadySubmitted = true
			result.SubmissionID = existingSubmissionID
			return result, nil
		}
	}

	var (
		preparedSubmission submitCreateReviewSubmissionPreparation
		err                error
	)
	// Read-only, so --dry-run runs it too and refuses what the real run refuses.
	unresolvedCtx, unresolvedCancel := submitResolvedVersionPhaseContext(ctx, opts.RequestTimeout)
	unresolvedSubmissionID, unresolvedErr := findUnresolvedSubmissionForVersion(unresolvedCtx, client, appID, platform, versionID)
	unresolvedCancel()
	if unresolvedErr != nil {
		return result, fmt.Errorf("submit review: query unresolved review submissions: %w", unresolvedErr)
	}
	if unresolvedSubmissionID != "" {
		buildToAttach := ""
		if opts.EnsureBuildAttached {
			buildToAttach = strings.TrimSpace(opts.BuildID)
		}
		return result, unresolvedSubmissionError(unresolvedSubmissionID, versionID, buildToAttach)
	}

	if !opts.DryRun {
		preparationCtx, preparationCancel := submitResolvedVersionPhaseContext(ctx, opts.RequestTimeout)
		preparedSubmission, err = prepareReviewSubmissionForCreate(preparationCtx, d.IO.ErrOut, client, appID, platform, versionID, emit)
		preparationCancel()
		if err != nil {
			return result, fmt.Errorf("submit review: prepare review submission: %w", err)
		}
	}

	if opts.EnsureBuildAttached {
		attachmentCtx, attachmentCancel := submitResolvedVersionPhaseContext(ctx, opts.RequestTimeout)
		attachment, attachmentErr := ensureBuildAttached(attachmentCtx, d, client, versionID, opts.BuildID, opts.DryRun)
		attachmentCancel()
		result.BuildAttachment = &attachment
		if attachmentErr != nil {
			return result, attachmentErr
		}
	}

	if opts.DryRun {
		result.WouldSubmit = true
		return result, nil
	}

	submitCtx, submitCancel := submitResolvedVersionPhaseContext(ctx, opts.RequestTimeout)
	defer submitCancel()

	submissionIDToSubmit := strings.TrimSpace(preparedSubmission.reuseSubmissionID)
	createdSubmissionID := ""
	if submissionIDToSubmit == "" {
		reviewSubmission, createErr := client.CreateReviewSubmission(submitCtx, appID, asc.Platform(platform))
		if createErr != nil {
			var partialErr *asc.ReviewSubmissionCreatePartialError
			if errors.As(createErr, &partialErr) && partialErr.Response != nil &&
				partialErr.Response.Data.Type == asc.ResourceTypeReviewSubmissions {
				createdSubmissionID = strings.TrimSpace(partialErr.Response.Data.ID)
				preserveCreatedReviewSubmission(d.IO.ErrOut, createdSubmissionID, emit)
			}
			return result, fmt.Errorf("submit review: create review submission: %w", createErr)
		}
		if reviewSubmission != nil {
			createdSubmissionID = strings.TrimSpace(reviewSubmission.Data.ID)
		}
		if receiptErr := validateReviewSubmissionCreateReceipt(reviewSubmission, appID, platform); receiptErr != nil {
			if createdSubmissionID != "" {
				preserveCreatedReviewSubmission(d.IO.ErrOut, createdSubmissionID, emit)
			}
			return result, fmt.Errorf("submit review: create review submission receipt: %w", receiptErr)
		}
		submissionIDToSubmit = createdSubmissionID
	}

	if !preparedSubmission.reuseSubmissionHasVersion {
		submissionIDToSubmit, err = addVersionToSubmissionOrRecover(
			submitCtx,
			d.IO.ErrOut,
			client,
			submissionIDToSubmit,
			versionID,
			appID,
			platform,
			emit,
		)
		if err != nil {
			if createdSubmissionID != "" {
				preserveCreatedReviewSubmission(d.IO.ErrOut, createdSubmissionID, emit)
			}
			return result, fmt.Errorf("submit review: add version to submission: %w", err)
		}
		if createdSubmissionID != "" && submissionIDToSubmit != createdSubmissionID {
			preserveCreatedReviewSubmission(d.IO.ErrOut, createdSubmissionID, emit)
		}
	}

	if err := verifyReviewSubmissionForSubmit(
		submitCtx,
		client,
		submissionIDToSubmit,
		appID,
		platform,
		versionID,
	); err != nil {
		if submissionIDToSubmit == createdSubmissionID {
			preserveCreatedReviewSubmission(d.IO.ErrOut, createdSubmissionID, emit)
		}
		return result, fmt.Errorf("submit review: final submission validation: %w", err)
	}

	submitResp, err := client.SubmitReviewSubmission(submitCtx, submissionIDToSubmit)
	if err != nil {
		if submissionIDToSubmit == createdSubmissionID {
			preserveCreatedReviewSubmission(d.IO.ErrOut, createdSubmissionID, emit)
		}
		printSubmissionErrorHints(d.IO.ErrOut, err, submissionErrorHintContext{AppID: appID, Platform: platform, VersionID: versionID})
		return result, fmt.Errorf("submit review: submit for review: %w", err)
	}

	result.SubmissionID = strings.TrimSpace(submitResp.Data.ID)
	result.SubmittedDate = strings.TrimSpace(submitResp.Data.Attributes.SubmittedDate)
	return result, nil
}

func submitResolvedVersionPhaseContext(ctx context.Context, requestTimeout time.Duration) (context.Context, context.CancelFunc) {
	if requestTimeout <= 0 {
		if ctx == nil {
			return context.Background(), func() {}
		}
		return ctx, func() {}
	}

	return shared.ContextWithTimeoutDuration(shared.ContextWithoutTimeout(ctx), requestTimeout)
}

// findUnresolvedSubmissionForVersion returns the UNRESOLVED_ISSUES review
// submission that still holds versionID, if any. App Review rejected that
// submission, and the version stays in it until the developer resolves the
// rejected items and resubmits it, or cancels it.
//
// appStoreVersionForReview alone is not enough: App Store Connect keeps that
// link after the version's item is removed from the submission. A linked
// submission counts only when one of its items is the version and is not
// REMOVED.
func findUnresolvedSubmissionForVersion(ctx context.Context, client *asc.Client, appID, platform, versionID string) (string, error) {
	submissions, err := shared.FetchAllReviewSubmissions(
		ctx,
		client,
		appID,
		asc.WithReviewSubmissionsStates([]string{string(asc.ReviewSubmissionStateUnresolvedIssues)}),
		asc.WithReviewSubmissionsPlatforms([]string{platform}),
		asc.WithReviewSubmissionsInclude([]string{"appStoreVersionForReview"}),
		asc.Limit[asc.ReviewSubmissionsOption](200),
	)
	if err != nil {
		return "", err
	}
	targetVersionID := strings.TrimSpace(versionID)
	for _, submission := range submissions {
		if submission.Attributes.SubmissionState != asc.ReviewSubmissionStateUnresolvedIssues {
			continue
		}
		if submission.Relationships == nil || submission.Relationships.AppStoreVersionForReview == nil {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(submission.Relationships.AppStoreVersionForReview.Data.ID), targetVersionID) {
			continue
		}
		submissionID := strings.TrimSpace(submission.ID)
		holdsVersion, err := reviewSubmissionHoldsActiveVersion(ctx, client, submissionID, targetVersionID)
		if err != nil {
			return "", fmt.Errorf("list items of review submission %s: %w", submissionID, err)
		}
		if holdsVersion {
			return submissionID, nil
		}
	}
	return "", nil
}

// reviewSubmissionHoldsActiveVersion reports whether submissionID has an item
// for versionID that is not REMOVED.
func reviewSubmissionHoldsActiveVersion(ctx context.Context, client *asc.Client, submissionID, versionID string) (bool, error) {
	resp, err := client.GetReviewSubmissionItemsStrict(
		ctx,
		submissionID,
		asc.WithReviewSubmissionItemsInclude([]string{"appStoreVersion"}),
		asc.Limit[asc.ReviewSubmissionItemsOption](200),
	)
	if err != nil {
		return false, err
	}
	if resp == nil || resp.Data == nil {
		return false, fmt.Errorf("review submission items response data is required")
	}

	holdsVersion := false
	consume := func(pageResponse asc.PaginatedResponse) error {
		page, ok := pageResponse.(*asc.ReviewSubmissionItemsResponse)
		if !ok || page == nil {
			return fmt.Errorf("unexpected review submission items response type %T", pageResponse)
		}
		for _, item := range page.Data {
			if reviewSubmissionItemVersionID(item) == versionID && !strings.EqualFold(strings.TrimSpace(item.Attributes.State), "REMOVED") {
				holdsVersion = true
			}
		}
		return nil
	}
	fetchNext := func(fetchCtx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return client.GetReviewSubmissionItemsStrict(fetchCtx, submissionID, asc.NextURL[asc.ReviewSubmissionItemsOption](strings.TrimSpace(nextURL)))
	}
	if err := asc.PaginateEachWithMaxPages(ctx, resp, fetchNext, consume, reviewSubmissionItemsMaxPages); err != nil {
		return false, err
	}
	return holdsVersion, nil
}

// unresolvedSubmissionError stops a submit before it creates a review
// submission that cannot take the version, and names the way forward: resolve
// the REJECTED items and resubmit the same submission. It does not suggest
// canceling: App Store Connect refuses to cancel an UNRESOLVED_ISSUES
// submission, and the rejected version stays editable without it.
// The check runs before the requested build is attached, so when buildID is
// set the hint attaches it first; otherwise the resubmit would send the build
// App Review rejected.
func unresolvedSubmissionError(submissionID, versionID, buildID string) error {
	attachStep := ""
	if buildID != "" {
		attachStep = fmt.Sprintf(
			"attach build %q with `asc versions attach-build --version-id %q --build-id %q` (it was not attached), ",
			buildID,
			strings.TrimSpace(versionID),
			buildID,
		)
	}
	return &shared.HintedError{
		Err: shared.WithDiagnostic(
			fmt.Errorf(
				"submit review: version %s is still in review submission %s, which App Review returned with unresolved issues; no new submission was created",
				strings.TrimSpace(versionID),
				submissionID,
			),
			shared.DiagnosticResourceConflict,
			"",
		),
		Hint: fmt.Sprintf(
			"To resubmit it, edit the version if the fix needs it, %slist its items with `asc review items list --submission %q`, mark each REJECTED item resolved with `asc review items update --id \"ITEM_ID\" --resolved true`, then run `asc review submissions-submit --id %q --confirm`.",
			attachStep,
			submissionID,
			submissionID,
		),
	}
}

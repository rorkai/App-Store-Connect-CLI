package submit

import (
	"context"
	"fmt"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
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
	EnsureBuildAttached      bool
	LookupExistingSubmission bool
	DryRun                   bool
	Emit                     func(string)
}

// SubmitResolvedVersionResult captures the outcome of creating/submitting a
// review submission for an already-resolved version.
type SubmitResolvedVersionResult struct {
	SubmissionID     string                `json:"submissionId,omitempty"`
	SubmittedDate    string                `json:"submittedDate,omitempty"`
	AlreadySubmitted bool                  `json:"alreadySubmitted,omitempty"`
	WouldSubmit      bool                  `json:"wouldSubmit,omitempty"`
	BuildAttachment  BuildAttachmentResult `json:"buildAttachment,omitempty"`
	Messages         []string              `json:"messages,omitempty"`
}

// SubmissionLocalizationPreflight runs the submission-blocking localization
// preflight used by submit-style App Store review flows.
func SubmissionLocalizationPreflight(ctx context.Context, client *asc.Client, appID, versionID, platform string) error {
	return runSubmitCreateLocalizationPreflight(ctx, client, appID, versionID, platform)
}

// SubmissionSubscriptionPreflight runs the advisory subscription preflight used
// by submit-style App Store review flows.
func SubmissionSubscriptionPreflight(ctx context.Context, client *asc.Client, appID string) {
	runSubmitCreateSubscriptionPreflight(ctx, client, appID)
}

// EnsureBuildAttached ensures the target build is attached to the resolved App
// Store version. In dry-run mode it reports the planned change without mutating.
func EnsureBuildAttached(ctx context.Context, client *asc.Client, versionID, buildID string, dryRun bool) (BuildAttachmentResult, error) {
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

	buildResp, err := client.GetAppStoreVersionBuild(ctx, result.VersionID)
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

	if err := client.AttachBuildToVersion(ctx, result.VersionID, result.BuildID); err != nil {
		return result, fmt.Errorf("attach build: %w", err)
	}
	result.Attached = true
	return result, nil
}

// SubmitResolvedVersion runs the shared modern review-submission flow for an
// already-resolved version ID.
func SubmitResolvedVersion(ctx context.Context, client *asc.Client, opts SubmitResolvedVersionOptions) (SubmitResolvedVersionResult, error) {
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

	if opts.EnsureBuildAttached {
		attachment, err := EnsureBuildAttached(ctx, client, versionID, opts.BuildID, opts.DryRun)
		result.BuildAttachment = attachment
		if err != nil {
			return result, err
		}
	}

	if opts.LookupExistingSubmission {
		legacySubmission, err := client.GetAppStoreVersionSubmissionForVersion(ctx, versionID)
		if err != nil && !asc.IsNotFound(err) {
			return result, fmt.Errorf("submit review: failed to lookup existing submission: %w", err)
		}
		if err == nil && strings.TrimSpace(legacySubmission.Data.ID) != "" {
			result.AlreadySubmitted = true
			result.SubmissionID = strings.TrimSpace(legacySubmission.Data.ID)
			return result, nil
		}
	}

	if opts.DryRun {
		result.WouldSubmit = true
		return result, nil
	}

	canceledStaleSubmissionIDs := cancelStaleReviewSubmissions(ctx, client, appID, platform, emit)

	reviewSubmission, err := client.CreateReviewSubmission(ctx, appID, asc.Platform(platform))
	if err != nil {
		return result, fmt.Errorf("submit review: create review submission: %w", err)
	}

	submissionIDToSubmit, err := addVersionToSubmissionOrRecover(
		ctx,
		client,
		reviewSubmission.Data.ID,
		versionID,
		canceledStaleSubmissionIDs,
		emit,
	)
	if err != nil {
		cleanupEmptyReviewSubmission(ctx, client, reviewSubmission.Data.ID, emit)
		return result, fmt.Errorf("submit review: add version to submission: %w", err)
	}
	if submissionIDToSubmit != reviewSubmission.Data.ID {
		cleanupEmptyReviewSubmission(ctx, client, reviewSubmission.Data.ID, emit)
	}

	submitResp, err := client.SubmitReviewSubmission(ctx, submissionIDToSubmit)
	if err != nil {
		return result, fmt.Errorf("submit review: submit for review: %w", err)
	}

	result.SubmissionID = strings.TrimSpace(submitResp.Data.ID)
	result.SubmittedDate = strings.TrimSpace(submitResp.Data.Attributes.SubmittedDate)
	return result, nil
}

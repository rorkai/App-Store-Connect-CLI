package reviews

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// reviewSubmissionNeedsVersionText is in Apple's 409 on submit when the app
// has never had a version approved on the platform and the submission holds
// no App Store version: in-app purchases, subscriptions and other items of a
// first release must be submitted together with the version.
const reviewSubmissionNeedsVersionText = "must have an approved appStoreVersions"

// reviewSubmissionEditableVersionStates are the appVersionState values of a
// version that can still be added to a submission.
var reviewSubmissionEditableVersionStates = []string{
	"PREPARE_FOR_SUBMISSION",
	"DEVELOPER_REJECTED",
	"REJECTED",
	"METADATA_REJECTED",
	"INVALID_BINARY",
}

// withSubmissionNeedsVersionHint names the version to add to the submission
// when Apple refuses to submit it without one. Reads are best effort; when one
// fails, or the 409 is another one, submitErr is returned unchanged.
func withSubmissionNeedsVersionHint(ctx context.Context, client *asc.Client, submissionID string, submitErr error) error {
	apiErr, ok := errors.AsType[*asc.APIError](submitErr)
	if !ok || apiErr.StatusCode != http.StatusConflict || !strings.Contains(apiErr.Detail, reviewSubmissionNeedsVersionText) {
		return submitErr
	}
	submission, err := client.GetReviewSubmission(ctx, submissionID, asc.WithReviewSubmissionInclude([]string{"app"}))
	if err != nil || submission.Data.Relationships == nil || submission.Data.Relationships.App == nil {
		return submitErr
	}
	appID := strings.TrimSpace(submission.Data.Relationships.App.Data.ID)
	platform := strings.TrimSpace(string(submission.Data.Attributes.Platform))
	if appID == "" || platform == "" {
		return submitErr
	}
	versions, err := client.GetAppStoreVersions(ctx, appID, asc.WithAppStoreVersionsPlatforms([]string{platform}), asc.Limit[asc.AppStoreVersionsOption](200))
	if err != nil {
		return submitErr
	}

	hint := fmt.Sprintf("App %s has no approved %s version yet, so App Store Connect reviews this submission only together with an App Store version.", appID, platform)
	editable := slices.IndexFunc(versions.Data, func(version asc.Resource[asc.AppStoreVersionAttributes]) bool {
		return slices.Contains(reviewSubmissionEditableVersionStates, submissionVersionState(version.Attributes))
	})
	if editable >= 0 {
		version := versions.Data[editable]
		hint += fmt.Sprintf(" Add version %s (%s) with `asc review items add --submission %s --item-type appStoreVersions --item-id %s`, then submit again.",
			version.Attributes.VersionString, submissionVersionState(version.Attributes), submissionID, version.ID)
	} else {
		hint += fmt.Sprintf(" Create one with `asc versions create --app %s --version VERSION --platform %s`, add it with `asc review items add --submission %s --item-type appStoreVersions --item-id VERSION_ID`, then submit again.",
			appID, platform, submissionID)
	}
	return &shared.HintedError{Err: submitErr, Hint: hint}
}

// submissionVersionState prefers appVersionState, which replaces the
// deprecated appStoreState that some versions still report alone.
func submissionVersionState(attrs asc.AppStoreVersionAttributes) string {
	if state := strings.TrimSpace(attrs.AppVersionState); state != "" {
		return state
	}
	return strings.TrimSpace(attrs.AppStoreState)
}

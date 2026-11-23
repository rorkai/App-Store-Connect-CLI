package submit

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func newSubmitStatusCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("submit status", flag.ExitOnError)

	submissionID := shared.BindResourceIDFlag(fs, "id", "reviewSubmissions", "Submission ID")
	versionID := shared.BindResourceIDFlag(fs, "version-id", "appStoreVersions", "App Store version ID")
	appID := fs.String("app", "", "App Store Connect app ID, bundle ID, or exact name: check its most relevant review submission, or require --id/--version-id to be this app's")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "status",
		ShortUsage: "asc submit status [flags]",
		ShortHelp:  "Check submission status.",
		LongHelp: `Check submission status.

--app alone checks the app's most relevant review submission: one in review
or waiting for review first, then the most recently submitted. An app with no
review submission reports its editable (else live) App Store version. Next to
--id or --version-id, --app checks that the submission or version belongs to
the app.

Examples:
  asc submit status --id "SUBMISSION_ID"
  asc submit status --version-id "VERSION_ID"
  asc submit status --app "APP_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			appValue := strings.TrimSpace(*appID)
			if strings.TrimSpace(*submissionID) == "" && strings.TrimSpace(*versionID) == "" && appValue == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --id, --version-id, or --app is required")
				return shared.MissingRequiredUsageError("")
			}
			if strings.TrimSpace(*submissionID) != "" && strings.TrimSpace(*versionID) != "" {
				return d.UsageError("--id and --version-id are mutually exclusive")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("submit status: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer func() {
				if cancel != nil {
					cancel()
				}
			}()

			resolvedVersionID := strings.TrimSpace(*versionID)
			resolvedSubmissionID := strings.TrimSpace(*submissionID)
			scopeAppID := ""
			if appValue != "" {
				scopeAppID, err = shared.ResolveAppIDWithLookup(requestCtx, client, appValue)
				if err != nil {
					return fmt.Errorf("submit status: %w", err)
				}
				if resolvedSubmissionID == "" && resolvedVersionID == "" {
					resolvedSubmissionID, resolvedVersionID, err = selectAppSubmissionStatusTarget(requestCtx, d, client, scopeAppID)
					if err != nil {
						return fmt.Errorf("submit status: %w", err)
					}
				}
			}
			result := &asc.AppStoreVersionSubmissionStatusResult{}
			if resolvedSubmissionID != "" {
				var submissionOpts []asc.ReviewSubmissionOption
				if scopeAppID != "" {
					submissionOpts = append(submissionOpts, asc.WithReviewSubmissionInclude([]string{"app"}))
				}
				reviewSubmission, reviewErr := client.GetReviewSubmission(requestCtx, resolvedSubmissionID, submissionOpts...)
				if reviewErr != nil {
					if asc.IsNotFound(reviewErr) {
						return fmt.Errorf(
							"submit status: no review submission found for ID %q; retry with --version-id to inspect the App Store version state",
							resolvedSubmissionID,
						)
					}
					return fmt.Errorf("submit status: failed to fetch review submission %q: %w", resolvedSubmissionID, reviewErr)
				}
				if scopeAppID != "" {
					if err := requireSubmissionApp(&reviewSubmission.Data, scopeAppID); err != nil {
						return fmt.Errorf("submit status: %w", err)
					}
				}

				applyReviewSubmissionStatus(result, &reviewSubmission.Data)
				resolvedVersionID, err = resolveReviewSubmissionVersionID(requestCtx, client, &reviewSubmission.Data)
				if err != nil {
					if !shouldIgnoreReviewSubmissionVersionLookupError(err) {
						return fmt.Errorf("submit status: %w", err)
					}
					resolvedVersionID = ""
				}
			} else {
				versionResp, versionErr := client.GetAppStoreVersion(requestCtx, resolvedVersionID, asc.WithAppStoreVersionInclude([]string{"app"}))
				if versionErr != nil {
					if asc.IsNotFound(versionErr) {
						return fmt.Errorf("submit status: no version found for ID %q", resolvedVersionID)
					}
					return fmt.Errorf("submit status: %w", versionErr)
				}
				if scopeAppID != "" {
					owner, ownerErr := resolveAppIDFromVersionResponse(versionResp)
					if ownerErr != nil {
						return fmt.Errorf("submit status: %w", ownerErr)
					}
					if owner != scopeAppID {
						mismatch := fmt.Errorf("version %q belongs to app %q, not %q", resolvedVersionID, owner, scopeAppID)
						return fmt.Errorf("submit status: %w", shared.WithDiagnostic(shared.NewValidationError(mismatch), shared.DiagnosticInvalidInput, "--version-id"))
					}
				}
				applyVersionStatus(result, versionResp)

				if appID, appErr := resolveAppIDFromVersionResponse(versionResp); appErr == nil {
					reviewSubmission, reviewErr := findReviewSubmissionForVersion(requestCtx, client, appID, resolvedVersionID)
					if reviewErr != nil {
						if !shouldIgnoreReviewSubmissionVersionLookupError(reviewErr) {
							return fmt.Errorf("submit status: %w", reviewErr)
						}
					} else if reviewSubmission != nil {
						applyReviewSubmissionStatus(result, reviewSubmission)
						return d.PrintOutput(result, *output.Output, *output.Pretty)
					}
				}

				legacySubmission, legacyErr := client.GetAppStoreVersionSubmissionForVersion(requestCtx, resolvedVersionID)
				if legacyErr == nil {
					applyLegacySubmissionStatus(result, legacySubmission)
				} else if !asc.IsNotFound(legacyErr) {
					return fmt.Errorf("submit status: %w", legacyErr)
				}

				return d.PrintOutput(result, *output.Output, *output.Pretty)
			}

			if resolvedVersionID != "" {
				versionResp, err := client.GetAppStoreVersion(requestCtx, resolvedVersionID)
				if err != nil {
					return fmt.Errorf("submit status: %w", err)
				}
				applyVersionStatus(result, versionResp)
			}

			return d.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// selectAppSubmissionStatusTarget picks what submit status --app reports: the
// app's most relevant review submission, or, when it has none, its default App
// Store version, announced on stderr.
func selectAppSubmissionStatusTarget(ctx context.Context, d *shared.Deps, client *asc.Client, appID string) (string, string, error) {
	resp, err := client.GetReviewSubmissions(ctx, appID, asc.Limit[asc.ReviewSubmissionsOption](200))
	if err != nil {
		return "", "", err
	}
	var candidates []asc.ReviewSubmissionResource
	for {
		candidates = append(candidates, resp.Data...)
		nextURL := strings.TrimSpace(resp.Links.Next)
		if nextURL == "" {
			break
		}
		resp, err = client.GetReviewSubmissions(ctx, appID, asc.NextURL[asc.ReviewSubmissionsOption](nextURL))
		if err != nil {
			return "", "", err
		}
	}
	if len(candidates) > 0 {
		sort.SliceStable(candidates, func(i, j int) bool {
			return reviewSubmissionSortKey(candidates[i]).less(reviewSubmissionSortKey(candidates[j]))
		})
		return strings.TrimSpace(candidates[0].ID), "", nil
	}

	version, err := shared.ResolveDefaultAppStoreVersion(ctx, client, appID, "")
	if err != nil {
		return "", "", fmt.Errorf("app %q has no review submissions: %w", appID, err)
	}
	fmt.Fprintf(d.IO.ErrOut, "App %s has no review submissions. %s\n", asc.SanitizeTerminalText(appID), version.Note("--version-id"))
	return "", version.ID, nil
}

// requireSubmissionApp reports an error unless submission, read with its app
// included, belongs to appID.
func requireSubmissionApp(submission *asc.ReviewSubmissionResource, appID string) error {
	if submission.Relationships == nil || submission.Relationships.App == nil || strings.TrimSpace(submission.Relationships.App.Data.ID) == "" {
		return fmt.Errorf("review submission %s did not prove its app relationship", strings.TrimSpace(submission.ID))
	}
	if owner := strings.TrimSpace(submission.Relationships.App.Data.ID); owner != appID {
		mismatch := fmt.Errorf("review submission %q belongs to app %q, not %q", strings.TrimSpace(submission.ID), owner, appID)
		return shared.WithDiagnostic(shared.NewValidationError(mismatch), shared.DiagnosticInvalidInput, "--id")
	}
	return nil
}

type submitStatusVersionRelationships struct {
	App *asc.Relationship `json:"app"`
}

func applyReviewSubmissionStatus(result *asc.AppStoreVersionSubmissionStatusResult, submission *asc.ReviewSubmissionResource) {
	if result == nil || submission == nil {
		return
	}
	result.ID = strings.TrimSpace(submission.ID)
	if submittedDate := strings.TrimSpace(submission.Attributes.SubmittedDate); submittedDate != "" {
		result.CreatedDate = &submittedDate
	}
	if state := strings.TrimSpace(string(submission.Attributes.SubmissionState)); state != "" {
		result.State = state
	}
}

func applyLegacySubmissionStatus(result *asc.AppStoreVersionSubmissionStatusResult, submission *asc.AppStoreVersionSubmissionResourceResponse) {
	if result == nil || submission == nil {
		return
	}
	result.ID = strings.TrimSpace(submission.Data.ID)
	result.CreatedDate = submission.Data.Attributes.CreatedDate
	if result.VersionID == "" && submission.Data.Relationships.AppStoreVersion != nil {
		result.VersionID = strings.TrimSpace(submission.Data.Relationships.AppStoreVersion.Data.ID)
	}
}

func applyVersionStatus(result *asc.AppStoreVersionSubmissionStatusResult, versionResp *asc.AppStoreVersionResponse) {
	if result == nil || versionResp == nil {
		return
	}
	result.VersionID = strings.TrimSpace(versionResp.Data.ID)
	result.VersionString = strings.TrimSpace(versionResp.Data.Attributes.VersionString)
	result.Platform = strings.TrimSpace(string(versionResp.Data.Attributes.Platform))
	if result.State == "" {
		result.State = shared.ResolveAppStoreVersionState(versionResp.Data.Attributes)
	}
}

func resolveAppIDFromVersionResponse(versionResp *asc.AppStoreVersionResponse) (string, error) {
	if versionResp == nil {
		return "", fmt.Errorf("version response is required")
	}
	if len(versionResp.Data.Relationships) == 0 {
		return "", fmt.Errorf("app relationship missing for app store version %q", versionResp.Data.ID)
	}
	var relationships submitStatusVersionRelationships
	if err := json.Unmarshal(versionResp.Data.Relationships, &relationships); err != nil {
		return "", fmt.Errorf("failed to parse app store version relationships: %w", err)
	}
	if relationships.App == nil {
		return "", fmt.Errorf("app relationship missing for app store version %q", versionResp.Data.ID)
	}
	appID := strings.TrimSpace(relationships.App.Data.ID)
	if appID == "" {
		return "", fmt.Errorf("app relationship missing for app store version %q", versionResp.Data.ID)
	}
	return appID, nil
}

func resolveReviewSubmissionVersionID(ctx context.Context, client *asc.Client, submission *asc.ReviewSubmissionResource) (string, error) {
	if submission == nil {
		return "", nil
	}
	if submission.Relationships != nil && submission.Relationships.AppStoreVersionForReview != nil {
		if versionID := strings.TrimSpace(submission.Relationships.AppStoreVersionForReview.Data.ID); versionID != "" {
			return versionID, nil
		}
	}
	return resolveReviewSubmissionVersionIDFromItems(ctx, client, strings.TrimSpace(submission.ID))
}

func resolveReviewSubmissionVersionIDFromItems(ctx context.Context, client *asc.Client, submissionID string) (string, error) {
	submissionID = strings.TrimSpace(submissionID)
	if submissionID == "" || client == nil {
		return "", nil
	}

	// include=, not just fields[]: the API only materialises relationship linkage for an included
	// relationship, so without it every item arrives with no relationships and this resolver reports
	// "no version" for a submission that plainly has one.
	opts := []asc.ReviewSubmissionItemsOption{
		asc.WithReviewSubmissionItemsInclude([]string{"appStoreVersion"}),
		asc.WithReviewSubmissionItemsFields([]string{"appStoreVersion"}),
		asc.Limit[asc.ReviewSubmissionItemsOption](200),
	}
	resp, err := client.GetReviewSubmissionItems(ctx, submissionID, opts...)
	if err != nil {
		return "", err
	}

	for {
		if versionID := reviewSubmissionVersionIDFromItems(resp.Data); versionID != "" {
			return versionID, nil
		}
		nextURL := strings.TrimSpace(resp.Links.Next)
		if nextURL == "" {
			return "", nil
		}
		resp, err = client.GetReviewSubmissionItems(ctx, submissionID, asc.NextURL[asc.ReviewSubmissionItemsOption](nextURL))
		if err != nil {
			return "", err
		}
	}
}

func reviewSubmissionVersionIDFromItems(items []asc.ReviewSubmissionItemResource) string {
	for _, item := range items {
		if item.Relationships == nil || item.Relationships.AppStoreVersion == nil {
			continue
		}
		if versionID := strings.TrimSpace(item.Relationships.AppStoreVersion.Data.ID); versionID != "" {
			return versionID
		}
	}
	return ""
}

func findReviewSubmissionForVersion(ctx context.Context, client *asc.Client, appID, versionID string) (*asc.ReviewSubmissionResource, error) {
	appID = strings.TrimSpace(appID)
	versionID = strings.TrimSpace(versionID)
	if appID == "" || versionID == "" || client == nil {
		return nil, nil
	}

	resp, err := client.GetReviewSubmissions(
		ctx,
		appID,
		asc.WithReviewSubmissionsInclude([]string{"appStoreVersionForReview"}),
		asc.Limit[asc.ReviewSubmissionsOption](200),
	)
	if err != nil {
		return nil, err
	}

	var candidates []asc.ReviewSubmissionResource
	for {
		for i := range resp.Data {
			submission := resp.Data[i]
			submissionVersionID, err := resolveReviewSubmissionVersionID(ctx, client, &submission)
			if err != nil {
				if !shouldIgnoreReviewSubmissionVersionLookupError(err) {
					return nil, err
				}
				continue
			}
			if submissionVersionID == versionID {
				candidates = append(candidates, submission)
			}
		}

		nextURL := strings.TrimSpace(resp.Links.Next)
		if nextURL == "" {
			break
		}
		resp, err = client.GetReviewSubmissions(ctx, appID, asc.NextURL[asc.ReviewSubmissionsOption](nextURL))
		if err != nil {
			return nil, err
		}
	}

	if len(candidates) == 0 {
		return nil, nil
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		return reviewSubmissionSortKey(candidates[i]).less(reviewSubmissionSortKey(candidates[j]))
	})
	best := candidates[0]
	return &best, nil
}

func shouldIgnoreReviewSubmissionVersionLookupError(err error) bool {
	return asc.IsNotFound(err) || errors.Is(err, asc.ErrForbidden)
}

type reviewSubmissionCandidateKey struct {
	statePriority int
	submittedAt   time.Time
	hasSubmitted  bool
	id            string
}

func reviewSubmissionSortKey(submission asc.ReviewSubmissionResource) reviewSubmissionCandidateKey {
	submittedAt, hasSubmitted := parseReviewSubmissionSubmittedDate(submission.Attributes.SubmittedDate)
	return reviewSubmissionCandidateKey{
		statePriority: reviewSubmissionStatePriority(submission.Attributes.SubmissionState),
		submittedAt:   submittedAt,
		hasSubmitted:  hasSubmitted,
		id:            strings.TrimSpace(submission.ID),
	}
}

func (k reviewSubmissionCandidateKey) less(other reviewSubmissionCandidateKey) bool {
	if k.statePriority != other.statePriority {
		return k.statePriority > other.statePriority
	}
	if k.hasSubmitted != other.hasSubmitted {
		return k.hasSubmitted
	}
	if !k.submittedAt.Equal(other.submittedAt) {
		return k.submittedAt.After(other.submittedAt)
	}
	return k.id > other.id
}

func reviewSubmissionStatePriority(state asc.ReviewSubmissionState) int {
	switch state {
	case asc.ReviewSubmissionStateInReview:
		return 70
	case asc.ReviewSubmissionStateWaitingForReview:
		return 60
	case asc.ReviewSubmissionStateUnresolvedIssues:
		return 50
	case asc.ReviewSubmissionStateReadyForReview:
		return 40
	case asc.ReviewSubmissionStateCompleting:
		return 30
	case asc.ReviewSubmissionStateCanceling:
		return 20
	case asc.ReviewSubmissionStateComplete:
		return 10
	default:
		return 0
	}
}

func isPotentiallyCancellableReviewSubmissionState(state asc.ReviewSubmissionState) bool {
	switch state {
	case asc.ReviewSubmissionStateInReview,
		asc.ReviewSubmissionStateWaitingForReview,
		asc.ReviewSubmissionStateUnresolvedIssues,
		asc.ReviewSubmissionStateReadyForReview:
		return true
	default:
		return false
	}
}

func parseReviewSubmissionSubmittedDate(value string) (time.Time, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		parsed, err := time.Parse(layout, trimmed)
		if err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

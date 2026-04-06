package reviews

import (
	"context"
	"flag"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	validatecli "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/validate"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

var reviewReadinessReportBuilder = validatecli.BuildReadinessReport

type reviewVersionContext struct {
	ID          string `json:"id"`
	Version     string `json:"version"`
	Platform    string `json:"platform"`
	State       string `json:"state"`
	CreatedDate string `json:"createdDate,omitempty"`
}

type reviewSubmissionContext struct {
	ID            string `json:"id"`
	State         string `json:"state"`
	Platform      string `json:"platform,omitempty"`
	SubmittedDate string `json:"submittedDate,omitempty"`
}

type reviewSubmissionItemsContext struct {
	TotalCount   int
	RemovedCount int
	ActiveCount  int
}

type reviewSnapshot struct {
	AppID            string
	Version          *reviewVersionContext
	ReviewDetailID   string
	LatestSubmission *reviewSubmissionContext
	SubmissionItems  *reviewSubmissionItemsContext
}

type reviewStatusResult struct {
	AppID            string                   `json:"appId"`
	Version          *reviewVersionContext    `json:"version,omitempty"`
	ReviewDetailID   string                   `json:"reviewDetailId,omitempty"`
	LatestSubmission *reviewSubmissionContext `json:"latestSubmission,omitempty"`
	ReviewState      string                   `json:"reviewState"`
	NextAction       string                   `json:"nextAction"`
	Blockers         []string                 `json:"blockers,omitempty"`
}

type reviewDoctorResult struct {
	AppID            string                   `json:"appId"`
	Version          *reviewVersionContext    `json:"version,omitempty"`
	ReviewDetailID   string                   `json:"reviewDetailId,omitempty"`
	LatestSubmission *reviewSubmissionContext `json:"latestSubmission,omitempty"`
	ReviewState      string                   `json:"reviewState"`
	Summary          validation.Summary       `json:"summary"`
	NextAction       string                   `json:"nextAction"`
	BlockingChecks   []validation.CheckResult `json:"blockingChecks,omitempty"`
	WarningChecks    []validation.CheckResult `json:"warningChecks,omitempty"`
}

// ReviewStatusCommand returns an app-scoped review status command.
func ReviewStatusCommand() *ffcli.Command {
	fs := flag.NewFlagSet("status", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID, bundle ID, or exact app name (required, or ASC_APP_ID)")
	version := fs.String("version", "", "App Store version string to inspect")
	versionID := fs.String("version-id", "", "App Store version ID to inspect")
	platform := fs.String("platform", "", "Platform filter: IOS, MAC_OS, TV_OS, VISION_OS")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "status",
		ShortUsage: "asc review status --app \"APP_ID\" [flags]",
		ShortHelp:  "Show app-scoped App Review status and next action.",
		LongHelp: `Show app-scoped App Review status without requiring submission IDs.

Examples:
  asc review status --app "123456789"
  asc review status --app "123456789" --version "1.2.3"
  asc review status --app "123456789" --version-id "VERSION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("review status does not accept positional arguments")
			}

			resolvedAppID, versionValue, versionIDValue, platformValue, err := resolveReviewOverviewFlags(*appID, *version, *versionID, *platform)
			if err != nil {
				return err
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("review status: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resolvedAppID, err = shared.ResolveAppIDWithLookup(requestCtx, client, resolvedAppID)
			if err != nil {
				return fmt.Errorf("review status: %w", err)
			}

			snapshot, err := buildReviewSnapshot(requestCtx, client, resolvedAppID, versionValue, versionIDValue, platformValue)
			if err != nil {
				return fmt.Errorf("review status: %w", err)
			}
			result := buildReviewStatusResult(snapshot)

			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { renderReviewStatus(result, false); return nil },
				func() error { renderReviewStatus(result, true); return nil },
			)
		},
	}
}

// ReviewDoctorCommand returns an app-scoped review blocker diagnosis command.
func ReviewDoctorCommand() *ffcli.Command {
	fs := flag.NewFlagSet("doctor", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID, bundle ID, or exact app name (required, or ASC_APP_ID)")
	version := fs.String("version", "", "App Store version string to diagnose")
	versionID := fs.String("version-id", "", "App Store version ID to diagnose")
	platform := fs.String("platform", "", "Platform filter: IOS, MAC_OS, TV_OS, VISION_OS")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "doctor",
		ShortUsage: "asc review doctor --app \"APP_ID\" [flags]",
		ShortHelp:  "Explain why an app cannot be submitted for review.",
		LongHelp: `Diagnose App Review blockers for the relevant App Store version.

Examples:
  asc review doctor --app "123456789"
  asc review doctor --app "123456789" --version "1.2.3"
  asc review doctor --app "123456789" --version-id "VERSION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("review doctor does not accept positional arguments")
			}

			resolvedAppID, versionValue, versionIDValue, platformValue, err := resolveReviewOverviewFlags(*appID, *version, *versionID, *platform)
			if err != nil {
				return err
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("review doctor: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resolvedAppID, err = shared.ResolveAppIDWithLookup(requestCtx, client, resolvedAppID)
			if err != nil {
				return fmt.Errorf("review doctor: %w", err)
			}

			snapshot, err := buildReviewSnapshot(requestCtx, client, resolvedAppID, versionValue, versionIDValue, platformValue)
			if err != nil {
				return fmt.Errorf("review doctor: %w", err)
			}

			var report validation.Report
			if snapshot.Version != nil {
				report, err = reviewReadinessReportBuilder(requestCtx, validatecli.ReadinessOptions{
					AppID:     resolvedAppID,
					VersionID: snapshot.Version.ID,
					Platform:  snapshot.Version.Platform,
					Strict:    false,
				})
				if err != nil {
					return fmt.Errorf("review doctor: %w", err)
				}
			}

			result := buildReviewDoctorResult(snapshot, report)
			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { renderReviewDoctor(result, false); return nil },
				func() error { renderReviewDoctor(result, true); return nil },
			)
		},
	}
}

func resolveReviewOverviewFlags(appID, version, versionID, platform string) (string, string, string, string, error) {
	resolvedAppID := shared.ResolveAppID(appID)
	if strings.TrimSpace(resolvedAppID) == "" {
		fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
		return "", "", "", "", flag.ErrHelp
	}

	versionValue := strings.TrimSpace(version)
	versionIDValue := strings.TrimSpace(versionID)
	if versionValue != "" && versionIDValue != "" {
		return "", "", "", "", shared.UsageError("--version and --version-id are mutually exclusive")
	}

	platformValue := strings.TrimSpace(platform)
	if platformValue != "" {
		normalizedPlatform, err := shared.NormalizeAppStoreVersionPlatform(platformValue)
		if err != nil {
			return "", "", "", "", shared.UsageError(err.Error())
		}
		platformValue = normalizedPlatform
	}

	return resolvedAppID, versionValue, versionIDValue, platformValue, nil
}

func buildReviewSnapshot(ctx context.Context, client *asc.Client, appID, version, versionID, platform string) (reviewSnapshot, error) {
	snapshot := reviewSnapshot{AppID: appID}

	versionContext, err := resolveReviewVersion(ctx, client, appID, version, versionID, platform)
	if err != nil {
		return snapshot, err
	}
	snapshot.Version = versionContext

	if versionContext != nil {
		reviewDetailResp, err := client.GetAppStoreReviewDetailForVersion(ctx, versionContext.ID)
		if err != nil {
			if !asc.IsNotFound(err) {
				return snapshot, fmt.Errorf("fetch review detail for version: %w", err)
			}
		} else {
			snapshot.ReviewDetailID = strings.TrimSpace(reviewDetailResp.Data.ID)
		}
	}

	submissionOpts := []asc.ReviewSubmissionsOption{asc.WithReviewSubmissionsLimit(200)}
	if versionContext != nil && strings.TrimSpace(versionContext.Platform) != "" {
		submissionOpts = append(submissionOpts, asc.WithReviewSubmissionsPlatforms([]string{versionContext.Platform}))
	} else if strings.TrimSpace(platform) != "" {
		submissionOpts = append(submissionOpts, asc.WithReviewSubmissionsPlatforms([]string{platform}))
	}
	if versionContext != nil && strings.TrimSpace(versionContext.ID) != "" {
		submissionOpts = append(submissionOpts, asc.WithReviewSubmissionsInclude([]string{"appStoreVersionForReview"}))
	}

	reviewSubmissions, err := shared.FetchAllReviewSubmissions(ctx, client, appID, submissionOpts...)
	if err != nil {
		return snapshot, fmt.Errorf("fetch review submissions: %w", err)
	}
	selectedVersionID := ""
	if versionContext != nil {
		selectedVersionID = strings.TrimSpace(versionContext.ID)
	}
	snapshot.LatestSubmission = selectRelevantReviewSubmission(reviewSubmissions, selectedVersionID)
	if selectedVersionID != "" && shouldInspectReviewSubmissionItems(snapshot.LatestSubmission) {
		submissionItems, err := summarizeReviewSubmissionItems(ctx, client, snapshot.LatestSubmission.ID, selectedVersionID)
		if err != nil {
			return snapshot, fmt.Errorf("fetch review submission items for %q: %w", snapshot.LatestSubmission.ID, err)
		}
		snapshot.SubmissionItems = &submissionItems
	}

	return snapshot, nil
}

func resolveReviewVersion(ctx context.Context, client *asc.Client, appID, version, versionID, platform string) (*reviewVersionContext, error) {
	if strings.TrimSpace(versionID) != "" {
		versionData, err := shared.ResolveOwnedAppStoreVersionByID(ctx, client, appID, versionID, platform)
		if err != nil {
			return nil, fmt.Errorf("fetch app store version %q: %w", strings.TrimSpace(versionID), err)
		}
		versionContext := mapReviewVersion(versionData)
		return &versionContext, nil
	}

	opts := []asc.AppStoreVersionsOption{asc.WithAppStoreVersionsLimit(200)}
	if strings.TrimSpace(version) != "" {
		opts = append(opts, asc.WithAppStoreVersionsVersionStrings([]string{strings.TrimSpace(version)}))
	}
	if strings.TrimSpace(platform) != "" {
		opts = append(opts, asc.WithAppStoreVersionsPlatforms([]string{platform}))
	}

	versions, err := shared.FetchAllAppStoreVersions(ctx, client, appID, opts...)
	if err != nil {
		return nil, fmt.Errorf("fetch app store versions: %w", err)
	}
	if len(versions) == 0 {
		if strings.TrimSpace(version) != "" {
			return nil, fmt.Errorf("no app store version found for version %q", strings.TrimSpace(version))
		}
		return nil, nil
	}
	if strings.TrimSpace(version) != "" {
		if len(versions) > 1 {
			return nil, fmt.Errorf("multiple app store versions found for version %q", strings.TrimSpace(version))
		}
		versionContext := mapReviewVersion(versions[0])
		return &versionContext, nil
	}

	best := mapReviewVersion(versions[0])
	for _, item := range versions[1:] {
		current := mapReviewVersion(item)
		cmp := shared.CompareRFC3339DateStrings(current.CreatedDate, best.CreatedDate)
		if cmp > 0 || (cmp == 0 && current.ID > best.ID) {
			best = current
		}
	}
	return &best, nil
}

func mapReviewVersion(item asc.Resource[asc.AppStoreVersionAttributes]) reviewVersionContext {
	return reviewVersionContext{
		ID:          strings.TrimSpace(item.ID),
		Version:     strings.TrimSpace(item.Attributes.VersionString),
		Platform:    strings.TrimSpace(string(item.Attributes.Platform)),
		State:       shared.ResolveAppStoreVersionState(item.Attributes),
		CreatedDate: strings.TrimSpace(item.Attributes.CreatedDate),
	}
}

func selectRelevantReviewSubmission(submissions []asc.ReviewSubmissionResource, versionID string) *reviewSubmissionContext {
	filtered := submissions
	if normalizedVersionID := strings.TrimSpace(versionID); normalizedVersionID != "" {
		filtered = make([]asc.ReviewSubmissionResource, 0, len(submissions))
		for _, submission := range submissions {
			if strings.EqualFold(reviewSubmissionVersionID(submission), normalizedVersionID) {
				filtered = append(filtered, submission)
			}
		}
	}
	if len(filtered) == 0 {
		return nil
	}

	best := filtered[0]
	for _, current := range filtered[1:] {
		if shared.ShouldPreferLatestReviewSubmission(current, best) {
			best = current
		}
	}

	context := reviewSubmissionContext{
		ID:            strings.TrimSpace(best.ID),
		State:         strings.TrimSpace(string(best.Attributes.SubmissionState)),
		Platform:      strings.TrimSpace(string(best.Attributes.Platform)),
		SubmittedDate: strings.TrimSpace(best.Attributes.SubmittedDate),
	}
	return &context
}

func reviewSubmissionVersionID(submission asc.ReviewSubmissionResource) string {
	if submission.Relationships == nil || submission.Relationships.AppStoreVersionForReview == nil {
		return ""
	}
	return strings.TrimSpace(submission.Relationships.AppStoreVersionForReview.Data.ID)
}

func shouldInspectReviewSubmissionItems(submission *reviewSubmissionContext) bool {
	if submission == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(submission.State), string(asc.ReviewSubmissionStateComplete))
}

func summarizeReviewSubmissionItems(ctx context.Context, client *asc.Client, submissionID, versionID string) (reviewSubmissionItemsContext, error) {
	var summary reviewSubmissionItemsContext

	submissionID = strings.TrimSpace(submissionID)
	versionID = strings.TrimSpace(versionID)
	if submissionID == "" || client == nil {
		return summary, nil
	}

	resp, err := client.GetReviewSubmissionItems(
		ctx,
		submissionID,
		asc.WithReviewSubmissionItemsLimit(200),
		asc.WithReviewSubmissionItemsFields([]string{"state", "appStoreVersion"}),
	)
	if err != nil {
		return summary, err
	}

	for {
		accumulateReviewSubmissionItems(&summary, resp.Data, versionID)

		nextURL := strings.TrimSpace(resp.Links.Next)
		if nextURL == "" {
			return summary, nil
		}

		resp, err = client.GetReviewSubmissionItems(ctx, submissionID, asc.WithReviewSubmissionItemsNextURL(nextURL))
		if err != nil {
			return summary, err
		}
	}
}

func accumulateReviewSubmissionItems(summary *reviewSubmissionItemsContext, items []asc.ReviewSubmissionItemResource, versionID string) {
	if summary == nil {
		return
	}

	for _, item := range items {
		if !reviewSubmissionItemTargetsVersion(item, versionID) {
			continue
		}
		summary.TotalCount++
		if strings.EqualFold(strings.TrimSpace(item.Attributes.State), "REMOVED") {
			summary.RemovedCount++
			continue
		}
		summary.ActiveCount++
	}
}

func reviewSubmissionItemTargetsVersion(item asc.ReviewSubmissionItemResource, versionID string) bool {
	if strings.TrimSpace(versionID) == "" {
		return true
	}
	if item.Relationships == nil || item.Relationships.AppStoreVersion == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(item.Relationships.AppStoreVersion.Data.ID), strings.TrimSpace(versionID))
}

func reviewSnapshotHasOnlyRemovedItems(snapshot reviewSnapshot) bool {
	if snapshot.LatestSubmission == nil || snapshot.SubmissionItems == nil {
		return false
	}
	return snapshot.SubmissionItems.TotalCount > 0 &&
		snapshot.SubmissionItems.RemovedCount == snapshot.SubmissionItems.TotalCount &&
		snapshot.SubmissionItems.ActiveCount == 0
}

func staleReviewSubmissionBlocker() string {
	return "Latest completed review submission no longer contains active review items because all items were removed."
}

func staleReviewSubmissionNextAction() string {
	return "Create a fresh review submission for the current version."
}

func buildReviewStatusResult(snapshot reviewSnapshot) reviewStatusResult {
	result := reviewStatusResult{
		AppID:            snapshot.AppID,
		Version:          snapshot.Version,
		ReviewDetailID:   snapshot.ReviewDetailID,
		LatestSubmission: snapshot.LatestSubmission,
		ReviewState:      "NO_VERSION",
		NextAction:       "Create or select an App Store version before preparing review.",
		Blockers:         make([]string, 0),
	}

	if snapshot.Version == nil {
		result.Blockers = append(result.Blockers, "No App Store version found for this app")
		return result
	}

	if snapshot.LatestSubmission != nil {
		result.ReviewState = snapshot.LatestSubmission.State
	} else {
		result.ReviewState = "NOT_SUBMITTED"
	}

	if strings.TrimSpace(snapshot.ReviewDetailID) == "" {
		result.Blockers = append(result.Blockers, "No App Store review detail is configured for the current version")
	}
	hasOnlyRemovedItems := reviewSnapshotHasOnlyRemovedItems(snapshot)
	if hasOnlyRemovedItems {
		result.Blockers = append(result.Blockers, staleReviewSubmissionBlocker())
	}

	versionState := strings.ToUpper(strings.TrimSpace(snapshot.Version.State))
	switch versionState {
	case "DEVELOPER_REJECTED", "REJECTED", "METADATA_REJECTED", "INVALID_BINARY":
		result.Blockers = append(result.Blockers, fmt.Sprintf("App Store version is in blocking state %s", versionState))
	}

	if snapshot.LatestSubmission != nil {
		switch strings.ToUpper(strings.TrimSpace(snapshot.LatestSubmission.State)) {
		case "UNRESOLVED_ISSUES":
			result.Blockers = append(result.Blockers, "Latest review submission has unresolved issues")
		case "WAITING_FOR_REVIEW", "IN_REVIEW":
			result.NextAction = "Wait for App Store review outcome."
		case "READY_FOR_REVIEW":
			result.NextAction = "Submit the prepared review submission."
		case "COMPLETE":
			result.NextAction = reviewPostCompleteAction(snapshot.Version.State)
		default:
			result.NextAction = "Review the latest submission state in App Store Connect."
		}
	}

	if len(result.Blockers) > 0 {
		switch {
		case hasOnlyRemovedItems:
			result.NextAction = staleReviewSubmissionNextAction()
		case strings.Contains(result.Blockers[0], "review detail"):
			result.NextAction = "Create or update the App Store review detail for the current version."
		case strings.Contains(strings.ToLower(result.Blockers[0]), "unresolved issues"):
			result.NextAction = "Run `asc review doctor --app \"" + snapshot.AppID + "\"` and fix the reported blockers."
		default:
			result.NextAction = "Run `asc review doctor --app \"" + snapshot.AppID + "\"` to inspect the blocking issues."
		}
		return result
	}

	if snapshot.LatestSubmission == nil {
		switch versionState {
		case "READY_FOR_SALE":
			result.NextAction = "No action needed."
		case "PENDING_DEVELOPER_RELEASE":
			result.NextAction = "Release the approved version when ready."
		default:
			result.NextAction = "Submit the version for App Review when ready."
		}
	}

	return result
}

func buildReviewDoctorResult(snapshot reviewSnapshot, report validation.Report) reviewDoctorResult {
	result := reviewDoctorResult{
		AppID:            snapshot.AppID,
		Version:          snapshot.Version,
		ReviewDetailID:   snapshot.ReviewDetailID,
		LatestSubmission: snapshot.LatestSubmission,
		ReviewState:      "NO_VERSION",
		NextAction:       "Create or select an App Store version before diagnosing review blockers.",
		BlockingChecks:   make([]validation.CheckResult, 0),
		WarningChecks:    make([]validation.CheckResult, 0),
	}

	if snapshot.Version == nil {
		result.Summary = validation.Summary{Errors: 1, Blocking: 1}
		result.BlockingChecks = append(result.BlockingChecks, validation.CheckResult{
			ID:          "review.version.missing",
			Severity:    validation.SeverityError,
			Message:     "No App Store version found for this app",
			Remediation: "Create or select an App Store version, then run `asc review doctor` again.",
		})
		result.NextAction = result.BlockingChecks[0].Remediation
		return result
	}

	if snapshot.LatestSubmission != nil {
		result.ReviewState = snapshot.LatestSubmission.State
	} else {
		result.ReviewState = "NOT_SUBMITTED"
	}

	result.Summary = report.Summary
	for _, check := range report.Checks {
		switch check.Severity {
		case validation.SeverityError:
			result.BlockingChecks = append(result.BlockingChecks, check)
		case validation.SeverityWarning:
			result.WarningChecks = append(result.WarningChecks, check)
		}
	}

	if snapshot.LatestSubmission != nil && strings.EqualFold(snapshot.LatestSubmission.State, string(asc.ReviewSubmissionStateUnresolvedIssues)) {
		synthetic := validation.CheckResult{
			ID:          "review.submission.unresolved_issues",
			Severity:    validation.SeverityError,
			Message:     "Latest review submission has unresolved issues in App Review",
			Remediation: "Resolve the outstanding App Review issues in App Store Connect, then resubmit if needed.",
		}
		result.BlockingChecks = append([]validation.CheckResult{synthetic}, result.BlockingChecks...)
		result.Summary.Errors++
		result.Summary.Blocking++
	}
	if reviewSnapshotHasOnlyRemovedItems(snapshot) {
		synthetic := validation.CheckResult{
			ID:          "review.submission.removed_items_only",
			Severity:    validation.SeverityError,
			Message:     staleReviewSubmissionBlocker(),
			Remediation: staleReviewSubmissionNextAction(),
		}
		result.BlockingChecks = append(result.BlockingChecks, synthetic)
		result.Summary.Errors++
		result.Summary.Blocking++
	}

	slices.SortFunc(result.BlockingChecks, func(a, b validation.CheckResult) int {
		return strings.Compare(a.ID, b.ID)
	})
	slices.SortFunc(result.WarningChecks, func(a, b validation.CheckResult) int {
		return strings.Compare(a.ID, b.ID)
	})

	switch {
	case reviewSnapshotHasOnlyRemovedItems(snapshot):
		result.NextAction = staleReviewSubmissionNextAction()
	case len(result.BlockingChecks) > 0 && strings.TrimSpace(result.BlockingChecks[0].Remediation) != "":
		result.NextAction = result.BlockingChecks[0].Remediation
	case len(result.BlockingChecks) > 0:
		result.NextAction = result.BlockingChecks[0].Message
	case strings.EqualFold(result.ReviewState, string(asc.ReviewSubmissionStateWaitingForReview)) || strings.EqualFold(result.ReviewState, string(asc.ReviewSubmissionStateInReview)):
		result.NextAction = "No submission blockers detected. Wait for App Store review outcome."
	case strings.EqualFold(strings.TrimSpace(snapshot.Version.State), "READY_FOR_SALE"):
		result.NextAction = "No action needed."
	default:
		result.NextAction = "No submission blockers detected. Submit the version when ready."
	}

	return result
}

func reviewPostCompleteAction(versionState string) string {
	switch strings.ToUpper(strings.TrimSpace(versionState)) {
	case "READY_FOR_SALE":
		return "No action needed."
	case "PENDING_DEVELOPER_RELEASE":
		return "Release the approved version when ready."
	default:
		return "Review the completed App Review outcome."
	}
}

func renderReviewStatus(result reviewStatusResult, markdown bool) {
	rows := [][]string{
		{"appId", result.AppID},
		{"reviewState", shared.OrNA(result.ReviewState)},
		{"nextAction", shared.OrNA(result.NextAction)},
		{"blockerCount", fmt.Sprintf("%d", len(result.Blockers))},
	}
	shared.RenderSection("Summary", []string{"field", "value"}, rows, markdown)

	contextRows := [][]string{
		{"versionId", shared.OrNA(reviewVersionField(result.Version, func(v *reviewVersionContext) string { return v.ID }))},
		{"version", shared.OrNA(reviewVersionField(result.Version, func(v *reviewVersionContext) string { return v.Version }))},
		{"platform", shared.OrNA(reviewVersionField(result.Version, func(v *reviewVersionContext) string { return v.Platform }))},
		{"versionState", shared.OrNA(reviewVersionField(result.Version, func(v *reviewVersionContext) string { return v.State }))},
		{"reviewDetailId", shared.OrNA(result.ReviewDetailID)},
		{"latestSubmissionId", shared.OrNA(reviewSubmissionField(result.LatestSubmission, func(s *reviewSubmissionContext) string { return s.ID }))},
		{"submissionState", shared.OrNA(reviewSubmissionField(result.LatestSubmission, func(s *reviewSubmissionContext) string { return s.State }))},
	}
	shared.RenderSection("Current Review", []string{"field", "value"}, contextRows, markdown)

	if len(result.Blockers) == 0 {
		return
	}

	blockerRows := make([][]string, 0, len(result.Blockers))
	for idx, blocker := range result.Blockers {
		blockerRows = append(blockerRows, []string{fmt.Sprintf("blocker_%d", idx+1), blocker})
	}
	shared.RenderSection("Blocking Issues", []string{"id", "detail"}, blockerRows, markdown)
}

func renderReviewDoctor(result reviewDoctorResult, markdown bool) {
	rows := [][]string{
		{"appId", result.AppID},
		{"reviewState", shared.OrNA(result.ReviewState)},
		{"blockingCount", fmt.Sprintf("%d", len(result.BlockingChecks))},
		{"warningCount", fmt.Sprintf("%d", len(result.WarningChecks))},
		{"nextAction", shared.OrNA(result.NextAction)},
	}
	shared.RenderSection("Summary", []string{"field", "value"}, rows, markdown)

	contextRows := [][]string{
		{"versionId", shared.OrNA(reviewVersionField(result.Version, func(v *reviewVersionContext) string { return v.ID }))},
		{"version", shared.OrNA(reviewVersionField(result.Version, func(v *reviewVersionContext) string { return v.Version }))},
		{"platform", shared.OrNA(reviewVersionField(result.Version, func(v *reviewVersionContext) string { return v.Platform }))},
		{"versionState", shared.OrNA(reviewVersionField(result.Version, func(v *reviewVersionContext) string { return v.State }))},
		{"reviewDetailId", shared.OrNA(result.ReviewDetailID)},
		{"latestSubmissionId", shared.OrNA(reviewSubmissionField(result.LatestSubmission, func(s *reviewSubmissionContext) string { return s.ID }))},
	}
	shared.RenderSection("Current Review", []string{"field", "value"}, contextRows, markdown)

	if len(result.BlockingChecks) > 0 {
		blockerRows := make([][]string, 0, len(result.BlockingChecks))
		for _, check := range result.BlockingChecks {
			blockerRows = append(blockerRows, []string{check.ID, check.Message, shared.OrNA(check.Remediation)})
		}
		shared.RenderSection("Blocking Issues", []string{"id", "message", "remediation"}, blockerRows, markdown)
	}

	if len(result.WarningChecks) > 0 {
		warningRows := make([][]string, 0, len(result.WarningChecks))
		for _, check := range result.WarningChecks {
			warningRows = append(warningRows, []string{check.ID, check.Message, shared.OrNA(check.Remediation)})
		}
		shared.RenderSection("Warnings", []string{"id", "message", "remediation"}, warningRows, markdown)
	}
}

func reviewVersionField(version *reviewVersionContext, field func(*reviewVersionContext) string) string {
	if version == nil {
		return ""
	}
	return strings.TrimSpace(field(version))
}

func reviewSubmissionField(submission *reviewSubmissionContext, field func(*reviewSubmissionContext) string) string {
	if submission == nil {
		return ""
	}
	return strings.TrimSpace(field(submission))
}

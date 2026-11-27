package asc

import (
	"fmt"
	"strings"
)

// TestNotesRecovery describes a shell-neutral retry after a build exists but
// setting its What to Test notes fails.
type TestNotesRecovery struct {
	BuildID        string   `json:"buildId"`
	Locale         string   `json:"locale"`
	SubmittedNotes string   `json:"submittedNotes"`
	Command        string   `json:"command"`
	Arguments      []string `json:"arguments"`
}

// TestFlightNotificationRecovery reports an unresolved notification outcome.
type TestFlightNotificationRecovery struct {
	BuildID string `json:"buildId"`
	State   string `json:"state"`
	Message string `json:"message"`
}

func testFlightPublishResultRows(result *TestFlightPublishResult) ([]string, [][]string) {
	headers := []string{"Build ID", "Version", "Build Number", "Processing", "Groups", "Uploaded", "Notified", "Notification Action", "Beta Review Submitted", "Beta Review Submission ID"}
	notified := ""
	if result.Notified != nil {
		notified = fmt.Sprintf("%t", *result.Notified)
	}
	betaReviewSubmitted := ""
	if result.BetaReviewSubmitted != nil {
		betaReviewSubmitted = fmt.Sprintf("%t", *result.BetaReviewSubmitted)
	}
	row := []string{
		result.BuildID,
		result.BuildVersion,
		result.BuildNumber,
		result.ProcessingState,
		strings.Join(result.GroupIDs, ", "),
		fmt.Sprintf("%t", result.Uploaded),
		notified,
		string(result.NotificationAction),
		betaReviewSubmitted,
		result.BetaReviewSubmissionID,
	}
	rows := [][]string{row}
	if result.HTTPStatus != 0 {
		headers = append(headers, "HTTP Status")
		rows[0] = append(rows[0], fmt.Sprint(result.HTTPStatus))
	}
	if result.NotificationRecovery != nil {
		recovery := result.NotificationRecovery
		headers = append(headers, "Notification State", "Notification Recovery")
		rows[0] = append(rows[0], recovery.State, recovery.Message)
	}
	if result.UploadOnly {
		headers = append(headers, "Upload Only")
		rows[0] = append(rows[0], "true")
	}
	if strings.TrimSpace(result.Status) != "" {
		headers = append(headers, "Status", "Failure Stage", "Failure Code", "Completed Stages", "Failure")
		rows[0] = append(rows[0], result.Status, result.FailureStage, result.FailureCode, strings.Join(result.CompletedStages, ", "), result.Failure)
	}
	return headers, rows
}

func appStorePublishResultRows(result *AppStorePublishResult) ([]string, [][]string) {
	if result.DryRun {
		headers := []string{"Dry Run", "Mode", "Version", "Build Number", "Will Wait", "Will Submit"}
		rows := [][]string{{
			fmt.Sprintf("%t", result.DryRun),
			string(result.Mode),
			result.BuildVersion,
			result.BuildNumber,
			fmt.Sprintf("%t", publishPlanContainsStep(result.Plan, "wait_for_build_processing")),
			fmt.Sprintf("%t", publishPlanContainsStep(result.Plan, "submit_review")),
		}}
		return headers, rows
	}

	headers := []string{"Build ID", "Version", "Build Number", "Version ID", "Submission ID", "Uploaded", "Attached", "Submitted"}
	rows := [][]string{{
		result.BuildID,
		result.BuildVersion,
		result.BuildNumber,
		result.VersionID,
		result.SubmissionID,
		fmt.Sprintf("%t", result.Uploaded),
		fmt.Sprintf("%t", result.Attached),
		fmt.Sprintf("%t", result.Submitted),
	}}
	return headers, rows
}

func publishPlanRows(plan []PublishPlanStep) ([]string, [][]string) {
	if len(plan) == 0 {
		return []string{"Step", "Status", "Message"}, nil
	}

	rows := make([][]string, 0, len(plan))
	for _, step := range plan {
		rows = append(rows, []string{step.Name, step.Status, step.Message})
	}
	return []string{"Step", "Status", "Message"}, rows
}

func publishPlanContainsStep(plan []PublishPlanStep, name string) bool {
	for _, step := range plan {
		if step.Name == name {
			return true
		}
	}
	return false
}

func publishArchiveStageRows(stage *PublishArchiveStageResult) ([]string, [][]string) {
	if stage == nil {
		return []string{"Field", "Value"}, nil
	}
	rows := [][]string{
		{"archive_path", stage.ArchivePath},
		{"bundle_id", stage.BundleID},
		{"version", stage.Version},
		{"build_number", stage.BuildNumber},
		{"scheme", stage.Scheme},
	}
	if strings.TrimSpace(stage.Configuration) != "" {
		rows = append(rows, []string{"configuration", stage.Configuration})
	}
	if stage.DisplayName != "" {
		rows = append(rows, []string{"display_name", stage.DisplayName})
	}
	if stage.ProjectCheckout != nil {
		rows = append(rows, []string{"project_checkout_revision", stage.ProjectCheckout.Revision}, []string{"project_checkout_dirty", fmt.Sprint(stage.ProjectCheckout.Dirty)}, []string{"project_checkout_observed_at", stage.ProjectCheckout.ObservedAt})
	}
	return []string{"Field", "Value"}, rows
}

func publishExportStageRows(stage *PublishExportStageResult) ([]string, [][]string) {
	if stage == nil {
		return []string{"Field", "Value"}, nil
	}
	rows := [][]string{
		{"archive_path", stage.ArchivePath},
	}
	if stage.DisplayName != "" {
		rows = append(rows, []string{"display_name", stage.DisplayName})
	}
	if strings.TrimSpace(stage.PKGPath) != "" {
		rows = append(rows, []string{"pkg_path", stage.PKGPath})
	} else {
		ipaPath := stage.IPAPath
		if strings.TrimSpace(ipaPath) == "" {
			ipaPath = "(direct upload - no local artifact)"
		}
		rows = append(rows, []string{"ipa_path", ipaPath})
	}
	rows = append(
		rows,
		[]string{"bundle_id", stage.BundleID},
		[]string{"version", stage.Version},
		[]string{"build_number", stage.BuildNumber},
		[]string{"export_options_path", stage.ExportOptionsPath},
		[]string{"direct_upload", fmt.Sprintf("%t", stage.DirectUpload)},
	)
	return []string{"Field", "Value"}, rows
}

package asc

import (
	"fmt"
	"strings"
)

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
	rows := [][]string{{
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
	}}
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
	return []string{"Field", "Value"}, rows
}

func publishExportStageRows(stage *PublishExportStageResult) ([]string, [][]string) {
	if stage == nil {
		return []string{"Field", "Value"}, nil
	}
	ipaPath := stage.IPAPath
	if strings.TrimSpace(ipaPath) == "" {
		ipaPath = "(direct upload - no local artifact)"
	}
	rows := [][]string{
		{"archive_path", stage.ArchivePath},
		{"ipa_path", ipaPath},
		{"bundle_id", stage.BundleID},
		{"version", stage.Version},
		{"build_number", stage.BuildNumber},
		{"export_options_path", stage.ExportOptionsPath},
		{"direct_upload", fmt.Sprintf("%t", stage.DirectUpload)},
	}
	return []string{"Field", "Value"}, rows
}

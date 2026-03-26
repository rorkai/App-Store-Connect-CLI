package asc

import (
	"fmt"
	"strings"
)

func testFlightPublishResultRows(result *TestFlightPublishResult) ([]string, [][]string) {
	headers := []string{"Build ID", "Version", "Build Number", "Processing", "Groups", "Uploaded", "Notified", "Notification Action"}
	notified := ""
	if result.Notified != nil {
		notified = fmt.Sprintf("%t", *result.Notified)
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
	}}
	return headers, rows
}

func appStorePublishResultRows(result *AppStorePublishResult) ([]string, [][]string) {
	headers := []string{"Build ID", "Version ID", "Submission ID", "Uploaded", "Attached", "Submitted"}
	rows := [][]string{{
		result.BuildID,
		result.VersionID,
		result.SubmissionID,
		fmt.Sprintf("%t", result.Uploaded),
		fmt.Sprintf("%t", result.Attached),
		fmt.Sprintf("%t", result.Submitted),
	}}
	return headers, rows
}

package asc

import "fmt"

// AppStoreVersionSubmissionResult represents CLI output for submissions.
type AppStoreVersionSubmissionResult struct {
	SubmissionID string  `json:"submissionId"`
	CreatedDate  *string `json:"createdDate,omitempty"`
}

// AppStoreVersionSubmissionCreateResult represents CLI output for submission creation.
type AppStoreVersionSubmissionCreateResult struct {
	SubmissionID string  `json:"submissionId"`
	VersionID    string  `json:"versionId"`
	BuildID      string  `json:"buildId"`
	CreatedDate  *string `json:"createdDate,omitempty"`
}

// AppStoreVersionSubmissionStatusResult represents CLI output for submission status.
type AppStoreVersionSubmissionStatusResult struct {
	ID            string  `json:"id"`
	VersionID     string  `json:"versionId,omitempty"`
	VersionString string  `json:"versionString,omitempty"`
	Platform      string  `json:"platform,omitempty"`
	State         string  `json:"state,omitempty"`
	CreatedDate   *string `json:"createdDate,omitempty"`
}

// AppStoreVersionSubmissionCancelResult represents CLI output for submission cancellation.
type AppStoreVersionSubmissionCancelResult struct {
	ID        string `json:"id"`
	Cancelled bool   `json:"cancelled"`
}

// AppStoreVersionDetailResult represents CLI output for version details.
type AppStoreVersionDetailResult struct {
	ID            string `json:"id"`
	VersionString string `json:"versionString,omitempty"`
	Platform      string `json:"platform,omitempty"`
	State         string `json:"state,omitempty"`
	BuildID       string `json:"buildId,omitempty"`
	BuildVersion  string `json:"buildVersion,omitempty"`
	SubmissionID  string `json:"submissionId,omitempty"`
}

// AppStoreVersionAttachBuildResult represents CLI output for build attachment.
type AppStoreVersionAttachBuildResult struct {
	VersionID string `json:"versionId"`
	BuildID   string `json:"buildId"`
	Attached  bool   `json:"attached"`
}

// AppStoreVersionReleaseRequestResult represents CLI output for release requests.
type AppStoreVersionReleaseRequestResult struct {
	ReleaseRequestID string `json:"releaseRequestId"`
	VersionID        string `json:"versionId"`
}

func appStoreVersionsRows(resp *AppStoreVersionsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Version", "Platform", "State", "Created"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		state := item.Attributes.AppVersionState
		if state == "" {
			state = item.Attributes.AppStoreState
		}
		rows = append(rows, []string{
			item.ID,
			item.Attributes.VersionString,
			string(item.Attributes.Platform),
			state,
			item.Attributes.CreatedDate,
		})
	}
	return headers, rows
}

func preReleaseVersionsRows(resp *PreReleaseVersionsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Version", "Platform"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.Version),
			string(item.Attributes.Platform),
		})
	}
	return headers, rows
}

func appStoreVersionSubmissionRows(result *AppStoreVersionSubmissionResult) ([]string, [][]string) {
	headers := []string{"Submission ID", "Created Date"}
	createdDate := ""
	if result.CreatedDate != nil {
		createdDate = *result.CreatedDate
	}
	rows := [][]string{{result.SubmissionID, createdDate}}
	return headers, rows
}

func appStoreVersionSubmissionCreateRows(result *AppStoreVersionSubmissionCreateResult) ([]string, [][]string) {
	headers := []string{"Submission ID", "Version ID", "Build ID", "Created Date"}
	createdDate := ""
	if result.CreatedDate != nil {
		createdDate = *result.CreatedDate
	}
	rows := [][]string{{result.SubmissionID, result.VersionID, result.BuildID, createdDate}}
	return headers, rows
}

func appStoreVersionSubmissionStatusRows(result *AppStoreVersionSubmissionStatusResult) ([]string, [][]string) {
	headers := []string{"Submission ID", "Version ID", "Version", "Platform", "State", "Created Date"}
	createdDate := ""
	if result.CreatedDate != nil {
		createdDate = *result.CreatedDate
	}
	rows := [][]string{{result.ID, result.VersionID, result.VersionString, result.Platform, result.State, createdDate}}
	return headers, rows
}

func appStoreVersionSubmissionCancelRows(result *AppStoreVersionSubmissionCancelResult) ([]string, [][]string) {
	headers := []string{"Submission ID", "Cancelled"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Cancelled)}}
	return headers, rows
}

func appStoreVersionDetailRows(result *AppStoreVersionDetailResult) ([]string, [][]string) {
	headers := []string{"Version ID", "Version", "Platform", "State", "Build ID", "Build Version", "Submission ID"}
	rows := [][]string{{result.ID, result.VersionString, result.Platform, result.State, result.BuildID, result.BuildVersion, result.SubmissionID}}
	return headers, rows
}

func appStoreVersionPhasedReleaseRows(resp *AppStoreVersionPhasedReleaseResponse) ([]string, [][]string) {
	headers := []string{"Phased Release ID", "State", "Start Date", "Current Day", "Total Pause Duration"}
	attrs := resp.Data.Attributes
	rows := [][]string{{
		resp.Data.ID,
		string(attrs.PhasedReleaseState),
		attrs.StartDate,
		fmt.Sprintf("%d", attrs.CurrentDayNumber),
		fmt.Sprintf("%d", attrs.TotalPauseDuration),
	}}
	return headers, rows
}

func appStoreVersionPhasedReleaseDeleteResultRows(result *AppStoreVersionPhasedReleaseDeleteResult) ([]string, [][]string) {
	headers := []string{"Phased Release ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func appStoreVersionAttachBuildRows(result *AppStoreVersionAttachBuildResult) ([]string, [][]string) {
	headers := []string{"Version ID", "Build ID", "Attached"}
	rows := [][]string{{result.VersionID, result.BuildID, fmt.Sprintf("%t", result.Attached)}}
	return headers, rows
}

func appStoreVersionReleaseRequestRows(result *AppStoreVersionReleaseRequestResult) ([]string, [][]string) {
	headers := []string{"Release Request ID", "Version ID"}
	rows := [][]string{{result.ReleaseRequestID, result.VersionID}}
	return headers, rows
}

package asc

import (
	"fmt"
	"os"
)

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

func printAppStoreVersionsTable(resp *AppStoreVersionsResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printPreReleaseVersionsTable(resp *PreReleaseVersionsResponse) error {
	headers := []string{"ID", "Version", "Platform"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.Version),
			string(item.Attributes.Platform),
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printAppStoreVersionsMarkdown(resp *AppStoreVersionsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Version | Platform | State | Created |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		state := item.Attributes.AppVersionState
		if state == "" {
			state = item.Attributes.AppStoreState
		}
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.VersionString),
			escapeMarkdown(string(item.Attributes.Platform)),
			escapeMarkdown(state),
			escapeMarkdown(item.Attributes.CreatedDate),
		)
	}
	return nil
}

func printPreReleaseVersionsMarkdown(resp *PreReleaseVersionsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Version | Platform |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.Version),
			escapeMarkdown(string(item.Attributes.Platform)),
		)
	}
	return nil
}

func printAppStoreVersionSubmissionTable(result *AppStoreVersionSubmissionResult) error {
	headers := []string{"Submission ID", "Created Date"}
	createdDate := ""
	if result.CreatedDate != nil {
		createdDate = *result.CreatedDate
	}
	rows := [][]string{{result.SubmissionID, createdDate}}
	RenderTable(headers, rows)
	return nil
}

func printAppStoreVersionSubmissionCreateTable(result *AppStoreVersionSubmissionCreateResult) error {
	headers := []string{"Submission ID", "Version ID", "Build ID", "Created Date"}
	createdDate := ""
	if result.CreatedDate != nil {
		createdDate = *result.CreatedDate
	}
	rows := [][]string{{result.SubmissionID, result.VersionID, result.BuildID, createdDate}}
	RenderTable(headers, rows)
	return nil
}

func printAppStoreVersionSubmissionStatusTable(result *AppStoreVersionSubmissionStatusResult) error {
	headers := []string{"Submission ID", "Version ID", "Version", "Platform", "State", "Created Date"}
	createdDate := ""
	if result.CreatedDate != nil {
		createdDate = *result.CreatedDate
	}
	rows := [][]string{{result.ID, result.VersionID, result.VersionString, result.Platform, result.State, createdDate}}
	RenderTable(headers, rows)
	return nil
}

func printAppStoreVersionSubmissionCancelTable(result *AppStoreVersionSubmissionCancelResult) error {
	headers := []string{"Submission ID", "Cancelled"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Cancelled)}}
	RenderTable(headers, rows)
	return nil
}

func printAppStoreVersionDetailTable(result *AppStoreVersionDetailResult) error {
	headers := []string{"Version ID", "Version", "Platform", "State", "Build ID", "Build Version", "Submission ID"}
	rows := [][]string{{result.ID, result.VersionString, result.Platform, result.State, result.BuildID, result.BuildVersion, result.SubmissionID}}
	RenderTable(headers, rows)
	return nil
}

func printAppStoreVersionPhasedReleaseTable(resp *AppStoreVersionPhasedReleaseResponse) error {
	headers := []string{"Phased Release ID", "State", "Start Date", "Current Day", "Total Pause Duration"}
	attrs := resp.Data.Attributes
	rows := [][]string{{
		resp.Data.ID,
		string(attrs.PhasedReleaseState),
		attrs.StartDate,
		fmt.Sprintf("%d", attrs.CurrentDayNumber),
		fmt.Sprintf("%d", attrs.TotalPauseDuration),
	}}
	RenderTable(headers, rows)
	return nil
}

func printAppStoreVersionPhasedReleaseDeleteResultTable(result *AppStoreVersionPhasedReleaseDeleteResult) error {
	headers := []string{"Phased Release ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printAppStoreVersionAttachBuildTable(result *AppStoreVersionAttachBuildResult) error {
	headers := []string{"Version ID", "Build ID", "Attached"}
	rows := [][]string{{result.VersionID, result.BuildID, fmt.Sprintf("%t", result.Attached)}}
	RenderTable(headers, rows)
	return nil
}

func printAppStoreVersionReleaseRequestTable(result *AppStoreVersionReleaseRequestResult) error {
	headers := []string{"Release Request ID", "Version ID"}
	rows := [][]string{{result.ReleaseRequestID, result.VersionID}}
	RenderTable(headers, rows)
	return nil
}

func printAppStoreVersionSubmissionMarkdown(result *AppStoreVersionSubmissionResult) error {
	fmt.Fprintln(os.Stdout, "| Submission ID | Created Date |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	createdDate := ""
	if result.CreatedDate != nil {
		createdDate = *result.CreatedDate
	}
	fmt.Fprintf(os.Stdout, "| %s | %s |\n",
		escapeMarkdown(result.SubmissionID),
		escapeMarkdown(createdDate),
	)
	return nil
}

func printAppStoreVersionSubmissionCreateMarkdown(result *AppStoreVersionSubmissionCreateResult) error {
	fmt.Fprintln(os.Stdout, "| Submission ID | Version ID | Build ID | Created Date |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- |")
	createdDate := ""
	if result.CreatedDate != nil {
		createdDate = *result.CreatedDate
	}
	fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s |\n",
		escapeMarkdown(result.SubmissionID),
		escapeMarkdown(result.VersionID),
		escapeMarkdown(result.BuildID),
		escapeMarkdown(createdDate),
	)
	return nil
}

func printAppStoreVersionSubmissionStatusMarkdown(result *AppStoreVersionSubmissionStatusResult) error {
	fmt.Fprintln(os.Stdout, "| Submission ID | Version ID | Version | Platform | State | Created Date |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- | --- |")
	createdDate := ""
	if result.CreatedDate != nil {
		createdDate = *result.CreatedDate
	}
	fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %s | %s |\n",
		escapeMarkdown(result.ID),
		escapeMarkdown(result.VersionID),
		escapeMarkdown(result.VersionString),
		escapeMarkdown(result.Platform),
		escapeMarkdown(result.State),
		escapeMarkdown(createdDate),
	)
	return nil
}

func printAppStoreVersionSubmissionCancelMarkdown(result *AppStoreVersionSubmissionCancelResult) error {
	fmt.Fprintln(os.Stdout, "| Submission ID | Cancelled |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n",
		escapeMarkdown(result.ID),
		result.Cancelled,
	)
	return nil
}

func printAppStoreVersionDetailMarkdown(result *AppStoreVersionDetailResult) error {
	fmt.Fprintln(os.Stdout, "| Version ID | Version | Platform | State | Build ID | Build Version | Submission ID |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- | --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %s | %s | %s |\n",
		escapeMarkdown(result.ID),
		escapeMarkdown(result.VersionString),
		escapeMarkdown(result.Platform),
		escapeMarkdown(result.State),
		escapeMarkdown(result.BuildID),
		escapeMarkdown(result.BuildVersion),
		escapeMarkdown(result.SubmissionID),
	)
	return nil
}

func printAppStoreVersionPhasedReleaseMarkdown(resp *AppStoreVersionPhasedReleaseResponse) error {
	fmt.Fprintln(os.Stdout, "| Phased Release ID | State | Start Date | Current Day | Total Pause Duration |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- |")
	attrs := resp.Data.Attributes
	fmt.Fprintf(os.Stdout, "| %s | %s | %s | %d | %d |\n",
		escapeMarkdown(resp.Data.ID),
		escapeMarkdown(string(attrs.PhasedReleaseState)),
		escapeMarkdown(attrs.StartDate),
		attrs.CurrentDayNumber,
		attrs.TotalPauseDuration,
	)
	return nil
}

func printAppStoreVersionPhasedReleaseDeleteResultMarkdown(result *AppStoreVersionPhasedReleaseDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| Phased Release ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n",
		escapeMarkdown(result.ID),
		result.Deleted,
	)
	return nil
}

func printAppStoreVersionAttachBuildMarkdown(result *AppStoreVersionAttachBuildResult) error {
	fmt.Fprintln(os.Stdout, "| Version ID | Build ID | Attached |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %s | %t |\n",
		escapeMarkdown(result.VersionID),
		escapeMarkdown(result.BuildID),
		result.Attached,
	)
	return nil
}

func printAppStoreVersionReleaseRequestMarkdown(result *AppStoreVersionReleaseRequestResult) error {
	fmt.Fprintln(os.Stdout, "| Release Request ID | Version ID |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %s |\n",
		escapeMarkdown(result.ReleaseRequestID),
		escapeMarkdown(result.VersionID),
	)
	return nil
}

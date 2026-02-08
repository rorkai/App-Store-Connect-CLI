package asc

import (
	"fmt"
	"os"
	"strings"
)

// CiArtifactDownloadResult represents CLI output for artifact downloads.
type CiArtifactDownloadResult struct {
	ID           string `json:"id"`
	FileName     string `json:"fileName,omitempty"`
	FileType     string `json:"fileType,omitempty"`
	FileSize     int    `json:"fileSize,omitempty"`
	OutputPath   string `json:"outputPath"`
	BytesWritten int64  `json:"bytesWritten,omitempty"`
}

// CiWorkflowDeleteResult represents CLI output for workflow deletions.
type CiWorkflowDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// CiProductDeleteResult represents CLI output for product deletions.
type CiProductDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

func printXcodeCloudRunResultTable(result *XcodeCloudRunResult) error {
	headers := []string{"Build Run ID", "Build #", "Workflow ID", "Workflow Name", "Git Ref ID", "Git Ref Name", "Progress", "Status", "Start Reason", "Created"}
	rows := [][]string{{
		result.BuildRunID,
		fmt.Sprintf("%d", result.BuildNumber),
		result.WorkflowID,
		result.WorkflowName,
		result.GitReferenceID,
		result.GitReferenceName,
		result.ExecutionProgress,
		result.CompletionStatus,
		result.StartReason,
		result.CreatedDate,
	}}
	RenderTable(headers, rows)
	return nil
}

func printXcodeCloudRunResultMarkdown(result *XcodeCloudRunResult) error {
	fmt.Fprintln(os.Stdout, "| Build Run ID | Build # | Workflow ID | Workflow Name | Git Ref ID | Git Ref Name | Progress | Status | Start Reason | Created |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %d | %s | %s | %s | %s | %s | %s | %s | %s |\n",
		escapeMarkdown(result.BuildRunID),
		result.BuildNumber,
		escapeMarkdown(result.WorkflowID),
		escapeMarkdown(result.WorkflowName),
		escapeMarkdown(result.GitReferenceID),
		escapeMarkdown(result.GitReferenceName),
		escapeMarkdown(result.ExecutionProgress),
		escapeMarkdown(result.CompletionStatus),
		escapeMarkdown(result.StartReason),
		escapeMarkdown(result.CreatedDate),
	)
	return nil
}

func printXcodeCloudStatusResultTable(result *XcodeCloudStatusResult) error {
	headers := []string{"Build Run ID", "Build #", "Workflow ID", "Progress", "Status", "Start Reason", "Cancel Reason", "Created", "Started", "Finished"}
	rows := [][]string{{
		result.BuildRunID,
		fmt.Sprintf("%d", result.BuildNumber),
		result.WorkflowID,
		result.ExecutionProgress,
		result.CompletionStatus,
		result.StartReason,
		result.CancelReason,
		result.CreatedDate,
		result.StartedDate,
		result.FinishedDate,
	}}
	RenderTable(headers, rows)
	return nil
}

func printXcodeCloudStatusResultMarkdown(result *XcodeCloudStatusResult) error {
	fmt.Fprintln(os.Stdout, "| Build Run ID | Build # | Workflow ID | Progress | Status | Start Reason | Cancel Reason | Created | Started | Finished |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %d | %s | %s | %s | %s | %s | %s | %s | %s |\n",
		escapeMarkdown(result.BuildRunID),
		result.BuildNumber,
		escapeMarkdown(result.WorkflowID),
		escapeMarkdown(result.ExecutionProgress),
		escapeMarkdown(result.CompletionStatus),
		escapeMarkdown(result.StartReason),
		escapeMarkdown(result.CancelReason),
		escapeMarkdown(result.CreatedDate),
		escapeMarkdown(result.StartedDate),
		escapeMarkdown(result.FinishedDate),
	)
	return nil
}

func printCiProductsTable(resp *CiProductsResponse) error {
	headers := []string{"ID", "Name", "Bundle ID", "Type", "Created"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			item.Attributes.Name,
			item.Attributes.BundleID,
			item.Attributes.ProductType,
			item.Attributes.CreatedDate,
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printCiProductsMarkdown(resp *CiProductsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Name | Bundle ID | Type | Created |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.Name),
			escapeMarkdown(item.Attributes.BundleID),
			escapeMarkdown(item.Attributes.ProductType),
			escapeMarkdown(item.Attributes.CreatedDate),
		)
	}
	return nil
}

func printCiWorkflowsTable(resp *CiWorkflowsResponse) error {
	headers := []string{"ID", "Name", "Enabled", "Last Modified"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			item.Attributes.Name,
			fmt.Sprintf("%t", item.Attributes.IsEnabled),
			item.Attributes.LastModifiedDate,
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printCiWorkflowsMarkdown(resp *CiWorkflowsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Name | Enabled | Last Modified |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %t | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.Name),
			item.Attributes.IsEnabled,
			escapeMarkdown(item.Attributes.LastModifiedDate),
		)
	}
	return nil
}

func printScmRepositoriesTable(resp *ScmRepositoriesResponse) error {
	headers := []string{"ID", "Owner", "Repository", "HTTP URL", "SSH URL", "Last Accessed"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			item.Attributes.OwnerName,
			item.Attributes.RepositoryName,
			item.Attributes.HTTPCloneURL,
			item.Attributes.SSHCloneURL,
			item.Attributes.LastAccessedDate,
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printScmRepositoriesMarkdown(resp *ScmRepositoriesResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Owner | Repository | HTTP URL | SSH URL | Last Accessed |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.OwnerName),
			escapeMarkdown(item.Attributes.RepositoryName),
			escapeMarkdown(item.Attributes.HTTPCloneURL),
			escapeMarkdown(item.Attributes.SSHCloneURL),
			escapeMarkdown(item.Attributes.LastAccessedDate),
		)
	}
	return nil
}

func printScmProvidersTable(resp *ScmProvidersResponse) error {
	headers := []string{"ID", "Provider Type", "URL"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			formatScmProviderType(item.Attributes.ScmProviderType),
			item.Attributes.URL,
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printScmProvidersMarkdown(resp *ScmProvidersResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Provider Type | URL |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(formatScmProviderType(item.Attributes.ScmProviderType)),
			escapeMarkdown(item.Attributes.URL),
		)
	}
	return nil
}

func formatScmProviderType(providerType *ScmProviderType) string {
	if providerType == nil {
		return ""
	}
	if strings.TrimSpace(providerType.DisplayName) != "" {
		return providerType.DisplayName
	}
	return strings.TrimSpace(providerType.Kind)
}

func printScmGitReferencesTable(resp *ScmGitReferencesResponse) error {
	headers := []string{"ID", "Name", "Canonical Name", "Kind", "Deleted"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			item.Attributes.Name,
			item.Attributes.CanonicalName,
			item.Attributes.Kind,
			fmt.Sprintf("%t", item.Attributes.IsDeleted),
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printScmGitReferencesMarkdown(resp *ScmGitReferencesResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Name | Canonical Name | Kind | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %t |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.Name),
			escapeMarkdown(item.Attributes.CanonicalName),
			escapeMarkdown(item.Attributes.Kind),
			item.Attributes.IsDeleted,
		)
	}
	return nil
}

func printScmPullRequestsTable(resp *ScmPullRequestsResponse) error {
	headers := []string{"ID", "Number", "Title", "Source", "Destination", "Closed", "Cross Repo", "Web URL"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			fmt.Sprintf("%d", item.Attributes.Number),
			item.Attributes.Title,
			formatScmRef(item.Attributes.SourceRepositoryOwner, item.Attributes.SourceRepositoryName, item.Attributes.SourceBranchName),
			formatScmRef(item.Attributes.DestinationRepositoryOwner, item.Attributes.DestinationRepositoryName, item.Attributes.DestinationBranchName),
			fmt.Sprintf("%t", item.Attributes.IsClosed),
			fmt.Sprintf("%t", item.Attributes.IsCrossRepository),
			item.Attributes.WebURL,
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printScmPullRequestsMarkdown(resp *ScmPullRequestsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Number | Title | Source | Destination | Closed | Cross Repo | Web URL |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %d | %s | %s | %s | %t | %t | %s |\n",
			escapeMarkdown(item.ID),
			item.Attributes.Number,
			escapeMarkdown(item.Attributes.Title),
			escapeMarkdown(formatScmRef(item.Attributes.SourceRepositoryOwner, item.Attributes.SourceRepositoryName, item.Attributes.SourceBranchName)),
			escapeMarkdown(formatScmRef(item.Attributes.DestinationRepositoryOwner, item.Attributes.DestinationRepositoryName, item.Attributes.DestinationBranchName)),
			item.Attributes.IsClosed,
			item.Attributes.IsCrossRepository,
			escapeMarkdown(item.Attributes.WebURL),
		)
	}
	return nil
}

func printCiMacOsVersionsTable(resp *CiMacOsVersionsResponse) error {
	headers := []string{"ID", "Version", "Name"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			item.Attributes.Version,
			item.Attributes.Name,
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printCiMacOsVersionsMarkdown(resp *CiMacOsVersionsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Version | Name |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.Version),
			escapeMarkdown(item.Attributes.Name),
		)
	}
	return nil
}

func printCiXcodeVersionsTable(resp *CiXcodeVersionsResponse) error {
	headers := []string{"ID", "Version", "Name"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			item.Attributes.Version,
			item.Attributes.Name,
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printCiXcodeVersionsMarkdown(resp *CiXcodeVersionsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Version | Name |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.Version),
			escapeMarkdown(item.Attributes.Name),
		)
	}
	return nil
}

func formatScmRef(owner, repo, branch string) string {
	repoValue := formatScmRepo(owner, repo)
	if branch == "" {
		return repoValue
	}
	if repoValue == "" {
		return branch
	}
	return fmt.Sprintf("%s:%s", repoValue, branch)
}

func formatScmRepo(owner, repo string) string {
	if owner == "" {
		return repo
	}
	if repo == "" {
		return owner
	}
	return fmt.Sprintf("%s/%s", owner, repo)
}

func printCiBuildRunsTable(resp *CiBuildRunsResponse) error {
	headers := []string{"ID", "Build #", "Progress", "Status", "Start Reason", "Created", "Started", "Finished"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			fmt.Sprintf("%d", item.Attributes.Number),
			string(item.Attributes.ExecutionProgress),
			string(item.Attributes.CompletionStatus),
			item.Attributes.StartReason,
			item.Attributes.CreatedDate,
			item.Attributes.StartedDate,
			item.Attributes.FinishedDate,
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printCiBuildRunsMarkdown(resp *CiBuildRunsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Build # | Progress | Status | Start Reason | Created | Started | Finished |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %d | %s | %s | %s | %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			item.Attributes.Number,
			escapeMarkdown(string(item.Attributes.ExecutionProgress)),
			escapeMarkdown(string(item.Attributes.CompletionStatus)),
			escapeMarkdown(item.Attributes.StartReason),
			escapeMarkdown(item.Attributes.CreatedDate),
			escapeMarkdown(item.Attributes.StartedDate),
			escapeMarkdown(item.Attributes.FinishedDate),
		)
	}
	return nil
}

func printCiBuildActionsTable(resp *CiBuildActionsResponse) error {
	headers := []string{"Name", "Type", "Progress", "Status", "Errors", "Warnings", "Started", "Finished"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		errors := 0
		warnings := 0
		if item.Attributes.IssueCounts != nil {
			errors = item.Attributes.IssueCounts.Errors
			warnings = item.Attributes.IssueCounts.Warnings
		}
		rows = append(rows, []string{
			item.Attributes.Name,
			item.Attributes.ActionType,
			string(item.Attributes.ExecutionProgress),
			string(item.Attributes.CompletionStatus),
			fmt.Sprintf("%d", errors),
			fmt.Sprintf("%d", warnings),
			item.Attributes.StartedDate,
			item.Attributes.FinishedDate,
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printCiBuildActionsMarkdown(resp *CiBuildActionsResponse) error {
	fmt.Fprintln(os.Stdout, "| Name | Type | Progress | Status | Errors | Warnings | Started | Finished |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		errors := 0
		warnings := 0
		if item.Attributes.IssueCounts != nil {
			errors = item.Attributes.IssueCounts.Errors
			warnings = item.Attributes.IssueCounts.Warnings
		}
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %d | %d | %s | %s |\n",
			escapeMarkdown(item.Attributes.Name),
			escapeMarkdown(item.Attributes.ActionType),
			escapeMarkdown(string(item.Attributes.ExecutionProgress)),
			escapeMarkdown(string(item.Attributes.CompletionStatus)),
			errors,
			warnings,
			escapeMarkdown(item.Attributes.StartedDate),
			escapeMarkdown(item.Attributes.FinishedDate),
		)
	}
	return nil
}

func printCiArtifactsTable(resp *CiArtifactsResponse) error {
	headers := []string{"ID", "Name", "Type", "Size", "Download URL"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			item.Attributes.FileName,
			item.Attributes.FileType,
			fmt.Sprintf("%d", item.Attributes.FileSize),
			item.Attributes.DownloadURL,
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printCiArtifactsMarkdown(resp *CiArtifactsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Name | Type | Size | Download URL |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %d | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.FileName),
			escapeMarkdown(item.Attributes.FileType),
			item.Attributes.FileSize,
			escapeMarkdown(item.Attributes.DownloadURL),
		)
	}
	return nil
}

func printCiArtifactTable(resp *CiArtifactResponse) error {
	return printCiArtifactsTable(&CiArtifactsResponse{Data: []CiArtifactResource{resp.Data}})
}

func printCiArtifactMarkdown(resp *CiArtifactResponse) error {
	return printCiArtifactsMarkdown(&CiArtifactsResponse{Data: []CiArtifactResource{resp.Data}})
}

func printCiTestResultsTable(resp *CiTestResultsResponse) error {
	headers := []string{"ID", "Class", "Name", "Status", "Duration"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			item.Attributes.ClassName,
			item.Attributes.Name,
			string(item.Attributes.Status),
			formatTestDuration(item),
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printCiTestResultsMarkdown(resp *CiTestResultsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Class | Name | Status | Duration |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.ClassName),
			escapeMarkdown(item.Attributes.Name),
			escapeMarkdown(string(item.Attributes.Status)),
			escapeMarkdown(formatTestDuration(item)),
		)
	}
	return nil
}

func printCiTestResultTable(resp *CiTestResultResponse) error {
	return printCiTestResultsTable(&CiTestResultsResponse{Data: []CiTestResultResource{resp.Data}})
}

func printCiTestResultMarkdown(resp *CiTestResultResponse) error {
	return printCiTestResultsMarkdown(&CiTestResultsResponse{Data: []CiTestResultResource{resp.Data}})
}

func printCiIssuesTable(resp *CiIssuesResponse) error {
	headers := []string{"ID", "Type", "File", "Line", "Message"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		filePath, lineNumber := formatFileLocation(item.Attributes.FileSource)
		rows = append(rows, []string{
			item.ID,
			item.Attributes.IssueType,
			filePath,
			lineNumber,
			item.Attributes.Message,
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printCiIssuesMarkdown(resp *CiIssuesResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Type | File | Line | Message |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		filePath, lineNumber := formatFileLocation(item.Attributes.FileSource)
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.IssueType),
			escapeMarkdown(filePath),
			escapeMarkdown(lineNumber),
			escapeMarkdown(item.Attributes.Message),
		)
	}
	return nil
}

func printCiIssueTable(resp *CiIssueResponse) error {
	return printCiIssuesTable(&CiIssuesResponse{Data: []CiIssueResource{resp.Data}})
}

func printCiIssueMarkdown(resp *CiIssueResponse) error {
	return printCiIssuesMarkdown(&CiIssuesResponse{Data: []CiIssueResource{resp.Data}})
}

func printCiArtifactDownloadResultTable(result *CiArtifactDownloadResult) error {
	headers := []string{"ID", "Name", "Type", "Size", "Bytes Written", "Output Path"}
	rows := [][]string{{
		result.ID,
		result.FileName,
		result.FileType,
		fmt.Sprintf("%d", result.FileSize),
		fmt.Sprintf("%d", result.BytesWritten),
		result.OutputPath,
	}}
	RenderTable(headers, rows)
	return nil
}

func printCiArtifactDownloadResultMarkdown(result *CiArtifactDownloadResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Name | Type | Size | Bytes Written | Output Path |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %s | %s | %d | %d | %s |\n",
		escapeMarkdown(result.ID),
		escapeMarkdown(result.FileName),
		escapeMarkdown(result.FileType),
		result.FileSize,
		result.BytesWritten,
		escapeMarkdown(result.OutputPath),
	)
	return nil
}

func printCiWorkflowDeleteResultTable(result *CiWorkflowDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printCiWorkflowDeleteResultMarkdown(result *CiWorkflowDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n", escapeMarkdown(result.ID), result.Deleted)
	return nil
}

func printCiProductDeleteResultTable(result *CiProductDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printCiProductDeleteResultMarkdown(result *CiProductDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n", escapeMarkdown(result.ID), result.Deleted)
	return nil
}

func formatTestDuration(result CiTestResultResource) string {
	if len(result.Attributes.DestinationTestResults) == 0 {
		return ""
	}
	duration := result.Attributes.DestinationTestResults[0].Duration
	if duration <= 0 {
		return ""
	}
	return fmt.Sprintf("%.2fs", duration)
}

func formatFileLocation(location *FileLocation) (string, string) {
	if location == nil {
		return "", ""
	}
	line := ""
	if location.LineNumber > 0 {
		line = fmt.Sprintf("%d", location.LineNumber)
	}
	return location.Path, line
}

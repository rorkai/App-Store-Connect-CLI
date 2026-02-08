package asc

import (
	"fmt"
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

func xcodeCloudRunResultRows(result *XcodeCloudRunResult) ([]string, [][]string) {
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
	return headers, rows
}

func xcodeCloudStatusResultRows(result *XcodeCloudStatusResult) ([]string, [][]string) {
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
	return headers, rows
}

func ciProductsRows(resp *CiProductsResponse) ([]string, [][]string) {
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
	return headers, rows
}

func ciWorkflowsRows(resp *CiWorkflowsResponse) ([]string, [][]string) {
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
	return headers, rows
}

func scmRepositoriesRows(resp *ScmRepositoriesResponse) ([]string, [][]string) {
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
	return headers, rows
}

func scmProvidersRows(resp *ScmProvidersResponse) ([]string, [][]string) {
	headers := []string{"ID", "Provider Type", "URL"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			formatScmProviderType(item.Attributes.ScmProviderType),
			item.Attributes.URL,
		})
	}
	return headers, rows
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

func scmGitReferencesRows(resp *ScmGitReferencesResponse) ([]string, [][]string) {
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
	return headers, rows
}

func scmPullRequestsRows(resp *ScmPullRequestsResponse) ([]string, [][]string) {
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
	return headers, rows
}

func ciMacOsVersionsRows(resp *CiMacOsVersionsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Version", "Name"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			item.Attributes.Version,
			item.Attributes.Name,
		})
	}
	return headers, rows
}

func ciXcodeVersionsRows(resp *CiXcodeVersionsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Version", "Name"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			item.Attributes.Version,
			item.Attributes.Name,
		})
	}
	return headers, rows
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

func ciBuildRunsRows(resp *CiBuildRunsResponse) ([]string, [][]string) {
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
	return headers, rows
}

func ciBuildActionsRows(resp *CiBuildActionsResponse) ([]string, [][]string) {
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
	return headers, rows
}

func ciArtifactsRows(resp *CiArtifactsResponse) ([]string, [][]string) {
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
	return headers, rows
}

func ciTestResultsRows(resp *CiTestResultsResponse) ([]string, [][]string) {
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
	return headers, rows
}

func ciIssuesRows(resp *CiIssuesResponse) ([]string, [][]string) {
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
	return headers, rows
}

func ciArtifactDownloadResultRows(result *CiArtifactDownloadResult) ([]string, [][]string) {
	headers := []string{"ID", "Name", "Type", "Size", "Bytes Written", "Output Path"}
	rows := [][]string{{
		result.ID,
		result.FileName,
		result.FileType,
		fmt.Sprintf("%d", result.FileSize),
		fmt.Sprintf("%d", result.BytesWritten),
		result.OutputPath,
	}}
	return headers, rows
}

func ciWorkflowDeleteResultRows(result *CiWorkflowDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func ciProductDeleteResultRows(result *CiProductDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
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

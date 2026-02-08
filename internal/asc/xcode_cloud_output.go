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

func printXcodeCloudRunResultTable(result *XcodeCloudRunResult) error {
	h, r := xcodeCloudRunResultRows(result)
	RenderTable(h, r)
	return nil
}

func printXcodeCloudRunResultMarkdown(result *XcodeCloudRunResult) error {
	h, r := xcodeCloudRunResultRows(result)
	RenderMarkdown(h, r)
	return nil
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

func printXcodeCloudStatusResultTable(result *XcodeCloudStatusResult) error {
	h, r := xcodeCloudStatusResultRows(result)
	RenderTable(h, r)
	return nil
}

func printXcodeCloudStatusResultMarkdown(result *XcodeCloudStatusResult) error {
	h, r := xcodeCloudStatusResultRows(result)
	RenderMarkdown(h, r)
	return nil
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

func printCiProductsTable(resp *CiProductsResponse) error {
	h, r := ciProductsRows(resp)
	RenderTable(h, r)
	return nil
}

func printCiProductsMarkdown(resp *CiProductsResponse) error {
	h, r := ciProductsRows(resp)
	RenderMarkdown(h, r)
	return nil
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

func printCiWorkflowsTable(resp *CiWorkflowsResponse) error {
	h, r := ciWorkflowsRows(resp)
	RenderTable(h, r)
	return nil
}

func printCiWorkflowsMarkdown(resp *CiWorkflowsResponse) error {
	h, r := ciWorkflowsRows(resp)
	RenderMarkdown(h, r)
	return nil
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

func printScmRepositoriesTable(resp *ScmRepositoriesResponse) error {
	h, r := scmRepositoriesRows(resp)
	RenderTable(h, r)
	return nil
}

func printScmRepositoriesMarkdown(resp *ScmRepositoriesResponse) error {
	h, r := scmRepositoriesRows(resp)
	RenderMarkdown(h, r)
	return nil
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

func printScmProvidersTable(resp *ScmProvidersResponse) error {
	h, r := scmProvidersRows(resp)
	RenderTable(h, r)
	return nil
}

func printScmProvidersMarkdown(resp *ScmProvidersResponse) error {
	h, r := scmProvidersRows(resp)
	RenderMarkdown(h, r)
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

func printScmGitReferencesTable(resp *ScmGitReferencesResponse) error {
	h, r := scmGitReferencesRows(resp)
	RenderTable(h, r)
	return nil
}

func printScmGitReferencesMarkdown(resp *ScmGitReferencesResponse) error {
	h, r := scmGitReferencesRows(resp)
	RenderMarkdown(h, r)
	return nil
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

func printScmPullRequestsTable(resp *ScmPullRequestsResponse) error {
	h, r := scmPullRequestsRows(resp)
	RenderTable(h, r)
	return nil
}

func printScmPullRequestsMarkdown(resp *ScmPullRequestsResponse) error {
	h, r := scmPullRequestsRows(resp)
	RenderMarkdown(h, r)
	return nil
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

func printCiMacOsVersionsTable(resp *CiMacOsVersionsResponse) error {
	h, r := ciMacOsVersionsRows(resp)
	RenderTable(h, r)
	return nil
}

func printCiMacOsVersionsMarkdown(resp *CiMacOsVersionsResponse) error {
	h, r := ciMacOsVersionsRows(resp)
	RenderMarkdown(h, r)
	return nil
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

func printCiXcodeVersionsTable(resp *CiXcodeVersionsResponse) error {
	h, r := ciXcodeVersionsRows(resp)
	RenderTable(h, r)
	return nil
}

func printCiXcodeVersionsMarkdown(resp *CiXcodeVersionsResponse) error {
	h, r := ciXcodeVersionsRows(resp)
	RenderMarkdown(h, r)
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

func printCiBuildRunsTable(resp *CiBuildRunsResponse) error {
	h, r := ciBuildRunsRows(resp)
	RenderTable(h, r)
	return nil
}

func printCiBuildRunsMarkdown(resp *CiBuildRunsResponse) error {
	h, r := ciBuildRunsRows(resp)
	RenderMarkdown(h, r)
	return nil
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

func printCiBuildActionsTable(resp *CiBuildActionsResponse) error {
	h, r := ciBuildActionsRows(resp)
	RenderTable(h, r)
	return nil
}

func printCiBuildActionsMarkdown(resp *CiBuildActionsResponse) error {
	h, r := ciBuildActionsRows(resp)
	RenderMarkdown(h, r)
	return nil
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

func printCiArtifactsTable(resp *CiArtifactsResponse) error {
	h, r := ciArtifactsRows(resp)
	RenderTable(h, r)
	return nil
}

func printCiArtifactsMarkdown(resp *CiArtifactsResponse) error {
	h, r := ciArtifactsRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func printCiArtifactTable(resp *CiArtifactResponse) error {
	return printCiArtifactsTable(&CiArtifactsResponse{Data: []CiArtifactResource{resp.Data}})
}

func printCiArtifactMarkdown(resp *CiArtifactResponse) error {
	return printCiArtifactsMarkdown(&CiArtifactsResponse{Data: []CiArtifactResource{resp.Data}})
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

func printCiTestResultsTable(resp *CiTestResultsResponse) error {
	h, r := ciTestResultsRows(resp)
	RenderTable(h, r)
	return nil
}

func printCiTestResultsMarkdown(resp *CiTestResultsResponse) error {
	h, r := ciTestResultsRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func printCiTestResultTable(resp *CiTestResultResponse) error {
	return printCiTestResultsTable(&CiTestResultsResponse{Data: []CiTestResultResource{resp.Data}})
}

func printCiTestResultMarkdown(resp *CiTestResultResponse) error {
	return printCiTestResultsMarkdown(&CiTestResultsResponse{Data: []CiTestResultResource{resp.Data}})
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

func printCiIssuesTable(resp *CiIssuesResponse) error {
	h, r := ciIssuesRows(resp)
	RenderTable(h, r)
	return nil
}

func printCiIssuesMarkdown(resp *CiIssuesResponse) error {
	h, r := ciIssuesRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func printCiIssueTable(resp *CiIssueResponse) error {
	return printCiIssuesTable(&CiIssuesResponse{Data: []CiIssueResource{resp.Data}})
}

func printCiIssueMarkdown(resp *CiIssueResponse) error {
	return printCiIssuesMarkdown(&CiIssuesResponse{Data: []CiIssueResource{resp.Data}})
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

func printCiArtifactDownloadResultTable(result *CiArtifactDownloadResult) error {
	h, r := ciArtifactDownloadResultRows(result)
	RenderTable(h, r)
	return nil
}

func printCiArtifactDownloadResultMarkdown(result *CiArtifactDownloadResult) error {
	h, r := ciArtifactDownloadResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func ciWorkflowDeleteResultRows(result *CiWorkflowDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printCiWorkflowDeleteResultTable(result *CiWorkflowDeleteResult) error {
	h, r := ciWorkflowDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printCiWorkflowDeleteResultMarkdown(result *CiWorkflowDeleteResult) error {
	h, r := ciWorkflowDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func ciProductDeleteResultRows(result *CiProductDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printCiProductDeleteResultTable(result *CiProductDeleteResult) error {
	h, r := ciProductDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printCiProductDeleteResultMarkdown(result *CiProductDeleteResult) error {
	h, r := ciProductDeleteResultRows(result)
	RenderMarkdown(h, r)
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

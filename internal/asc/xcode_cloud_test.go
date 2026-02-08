package asc

import (
	"bytes"
	"io"
	"net/url"
	"os"
	"strings"
	"testing"
)

func captureXcodeCloudStdout(t *testing.T, fn func() error) string {
	t.Helper()

	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe error: %v", err)
	}
	os.Stdout = w

	err = fn()

	if closeErr := w.Close(); closeErr != nil {
		t.Fatalf("close error: %v", closeErr)
	}
	os.Stdout = orig

	var buf bytes.Buffer
	if _, readErr := io.Copy(&buf, r); readErr != nil {
		t.Fatalf("read error: %v", readErr)
	}
	if err != nil {
		t.Fatalf("function error: %v", err)
	}

	return buf.String()
}

func TestPrintTable_XcodeCloudRunResult(t *testing.T) {
	result := &XcodeCloudRunResult{
		BuildRunID:        "run-123",
		BuildNumber:       42,
		WorkflowID:        "wf-456",
		WorkflowName:      "CI Build",
		GitReferenceID:    "ref-789",
		GitReferenceName:  "main",
		ExecutionProgress: "PENDING",
		CompletionStatus:  "",
		StartReason:       "MANUAL",
		CreatedDate:       "2026-01-22T10:00:00Z",
	}

	output := captureXcodeCloudStdout(t, func() error {
		return PrintTable(result)
	})

	if !strings.Contains(output, "Build Run ID") {
		t.Fatalf("expected header in output, got: %s", output)
	}
	if !strings.Contains(output, "run-123") {
		t.Fatalf("expected build run ID in output, got: %s", output)
	}
	if !strings.Contains(output, "CI Build") {
		t.Fatalf("expected workflow name in output, got: %s", output)
	}
	if !strings.Contains(output, "PENDING") {
		t.Fatalf("expected execution progress in output, got: %s", output)
	}
}

func TestPrintMarkdown_XcodeCloudRunResult(t *testing.T) {
	result := &XcodeCloudRunResult{
		BuildRunID:        "run-123",
		BuildNumber:       42,
		WorkflowID:        "wf-456",
		WorkflowName:      "CI Build",
		GitReferenceID:    "ref-789",
		GitReferenceName:  "main",
		ExecutionProgress: "RUNNING",
		CompletionStatus:  "",
		StartReason:       "MANUAL",
		CreatedDate:       "2026-01-22T10:00:00Z",
	}

	output := captureXcodeCloudStdout(t, func() error {
		return PrintMarkdown(result)
	})

	if !strings.Contains(output, "Build Run ID") {
		t.Fatalf("expected markdown header in output, got: %s", output)
	}
	if !strings.Contains(output, "run-123") {
		t.Fatalf("expected build run ID in output, got: %s", output)
	}
	if !strings.Contains(output, "RUNNING") {
		t.Fatalf("expected execution progress in output, got: %s", output)
	}
}

func TestPrintTable_XcodeCloudStatusResult(t *testing.T) {
	result := &XcodeCloudStatusResult{
		BuildRunID:        "run-123",
		BuildNumber:       42,
		WorkflowID:        "wf-456",
		ExecutionProgress: "COMPLETE",
		CompletionStatus:  "SUCCEEDED",
		StartReason:       "MANUAL",
		CancelReason:      "",
		CreatedDate:       "2026-01-22T10:00:00Z",
		StartedDate:       "2026-01-22T10:01:00Z",
		FinishedDate:      "2026-01-22T10:05:00Z",
	}

	output := captureXcodeCloudStdout(t, func() error {
		return PrintTable(result)
	})

	if !strings.Contains(output, "Build Run ID") {
		t.Fatalf("expected header in output, got: %s", output)
	}
	if !strings.Contains(output, "COMPLETE") {
		t.Fatalf("expected execution progress in output, got: %s", output)
	}
	if !strings.Contains(output, "SUCCEEDED") {
		t.Fatalf("expected completion status in output, got: %s", output)
	}
}

func TestPrintMarkdown_XcodeCloudStatusResult(t *testing.T) {
	result := &XcodeCloudStatusResult{
		BuildRunID:        "run-123",
		BuildNumber:       42,
		WorkflowID:        "wf-456",
		ExecutionProgress: "COMPLETE",
		CompletionStatus:  "FAILED",
		StartReason:       "MANUAL",
		CancelReason:      "",
		CreatedDate:       "2026-01-22T10:00:00Z",
		StartedDate:       "2026-01-22T10:01:00Z",
		FinishedDate:      "2026-01-22T10:05:00Z",
	}

	output := captureXcodeCloudStdout(t, func() error {
		return PrintMarkdown(result)
	})

	if !strings.Contains(output, "Build Run ID") {
		t.Fatalf("expected markdown header in output, got: %s", output)
	}
	if !strings.Contains(output, "FAILED") {
		t.Fatalf("expected completion status in output, got: %s", output)
	}
}

func TestPrintTable_CiProducts(t *testing.T) {
	resp := &CiProductsResponse{
		Data: []CiProductResource{
			{
				ID: "prod-1",
				Attributes: CiProductAttributes{
					Name:        "MyApp",
					BundleID:    "com.example.myapp",
					ProductType: "APP",
					CreatedDate: "2026-01-22T10:00:00Z",
				},
			},
		},
	}

	output := captureXcodeCloudStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Bundle ID") {
		t.Fatalf("expected header in output, got: %s", output)
	}
	if !strings.Contains(output, "com.example.myapp") {
		t.Fatalf("expected bundle ID in output, got: %s", output)
	}
}

func TestPrintMarkdown_CiProducts(t *testing.T) {
	resp := &CiProductsResponse{
		Data: []CiProductResource{
			{
				ID: "prod-1",
				Attributes: CiProductAttributes{
					Name:        "MyApp",
					BundleID:    "com.example.myapp",
					ProductType: "APP",
					CreatedDate: "2026-01-22T10:00:00Z",
				},
			},
		},
	}

	output := captureXcodeCloudStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Bundle ID") {
		t.Fatalf("expected markdown header in output, got: %s", output)
	}
	if !strings.Contains(output, "MyApp") {
		t.Fatalf("expected app name in output, got: %s", output)
	}
}

func TestPrintTable_CiWorkflows(t *testing.T) {
	resp := &CiWorkflowsResponse{
		Data: []CiWorkflowResource{
			{
				ID: "wf-1",
				Attributes: CiWorkflowAttributes{
					Name:             "CI Build",
					IsEnabled:        true,
					LastModifiedDate: "2026-01-22T10:00:00Z",
				},
			},
		},
	}

	output := captureXcodeCloudStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Enabled") {
		t.Fatalf("expected header in output, got: %s", output)
	}
	if !strings.Contains(output, "CI Build") {
		t.Fatalf("expected workflow name in output, got: %s", output)
	}
}

func TestPrintMarkdown_CiWorkflows(t *testing.T) {
	resp := &CiWorkflowsResponse{
		Data: []CiWorkflowResource{
			{
				ID: "wf-1",
				Attributes: CiWorkflowAttributes{
					Name:             "Deploy",
					IsEnabled:        false,
					LastModifiedDate: "2026-01-22T10:00:00Z",
				},
			},
		},
	}

	output := captureXcodeCloudStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Enabled") {
		t.Fatalf("expected markdown header in output, got: %s", output)
	}
	if !strings.Contains(output, "Deploy") {
		t.Fatalf("expected workflow name in output, got: %s", output)
	}
}

func TestPrintTable_CiBuildRuns(t *testing.T) {
	resp := &CiBuildRunsResponse{
		Data: []CiBuildRunResource{
			{
				ID: "run-1",
				Attributes: CiBuildRunAttributes{
					Number:            1,
					ExecutionProgress: CiBuildRunExecutionProgressComplete,
					CompletionStatus:  CiBuildRunCompletionStatusSucceeded,
					StartReason:       "MANUAL",
					CreatedDate:       "2026-01-22T10:00:00Z",
					StartedDate:       "2026-01-22T10:01:00Z",
					FinishedDate:      "2026-01-22T10:05:00Z",
				},
			},
		},
	}

	output := captureXcodeCloudStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Progress") {
		t.Fatalf("expected header in output, got: %s", output)
	}
	if !strings.Contains(output, "COMPLETE") {
		t.Fatalf("expected execution progress in output, got: %s", output)
	}
}

func TestPrintMarkdown_CiBuildRuns(t *testing.T) {
	resp := &CiBuildRunsResponse{
		Data: []CiBuildRunResource{
			{
				ID: "run-1",
				Attributes: CiBuildRunAttributes{
					Number:            1,
					ExecutionProgress: CiBuildRunExecutionProgressRunning,
					CompletionStatus:  "",
					StartReason:       "MANUAL",
					CreatedDate:       "2026-01-22T10:00:00Z",
					StartedDate:       "2026-01-22T10:01:00Z",
				},
			},
		},
	}

	output := captureXcodeCloudStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Build #") {
		t.Fatalf("expected markdown header in output, got: %s", output)
	}
	if !strings.Contains(output, "RUNNING") {
		t.Fatalf("expected execution progress in output, got: %s", output)
	}
}

func TestPrintTable_CiArtifacts(t *testing.T) {
	resp := &CiArtifactsResponse{
		Data: []CiArtifactResource{
			{
				ID: "art-1",
				Attributes: CiArtifactAttributes{
					FileName:    "Build.zip",
					FileType:    "ARCHIVE",
					FileSize:    2048,
					DownloadURL: "https://example.com/artifact.zip",
				},
			},
		},
	}

	output := captureXcodeCloudStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Download URL") {
		t.Fatalf("expected header in output, got: %s", output)
	}
	if !strings.Contains(output, "Build.zip") {
		t.Fatalf("expected file name in output, got: %s", output)
	}
}

func TestPrintMarkdown_CiArtifacts(t *testing.T) {
	resp := &CiArtifactsResponse{
		Data: []CiArtifactResource{
			{
				ID: "art-1",
				Attributes: CiArtifactAttributes{
					FileName: "Logs.zip",
					FileType: "LOG_BUNDLE",
					FileSize: 512,
				},
			},
		},
	}

	output := captureXcodeCloudStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Name") || !strings.Contains(output, "Type") {
		t.Fatalf("expected markdown header in output, got: %s", output)
	}
	if !strings.Contains(output, "Logs.zip") {
		t.Fatalf("expected file name in output, got: %s", output)
	}
}

func TestPrintTable_CiTestResults(t *testing.T) {
	resp := &CiTestResultsResponse{
		Data: []CiTestResultResource{
			{
				ID: "test-1",
				Attributes: CiTestResultAttributes{
					ClassName: "Tests",
					Name:      "testExample",
					Status:    CiTestStatusSuccess,
					DestinationTestResults: []CiTestDestinationResult{
						{Duration: 1.234},
					},
				},
			},
		},
	}

	output := captureXcodeCloudStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Duration") {
		t.Fatalf("expected header in output, got: %s", output)
	}
	if !strings.Contains(output, "1.23s") {
		t.Fatalf("expected duration in output, got: %s", output)
	}
}

func TestPrintMarkdown_CiTestResults(t *testing.T) {
	resp := &CiTestResultsResponse{
		Data: []CiTestResultResource{
			{
				ID: "test-1",
				Attributes: CiTestResultAttributes{
					ClassName: "Tests",
					Name:      "testExample",
					Status:    CiTestStatusFailure,
				},
			},
		},
	}

	output := captureXcodeCloudStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Class") {
		t.Fatalf("expected markdown header in output, got: %s", output)
	}
	if !strings.Contains(output, "FAILURE") {
		t.Fatalf("expected status in output, got: %s", output)
	}
}

func TestPrintTable_CiIssues(t *testing.T) {
	resp := &CiIssuesResponse{
		Data: []CiIssueResource{
			{
				ID: "issue-1",
				Attributes: CiIssueAttributes{
					IssueType: "ERROR",
					Message:   "Something broke",
					FileSource: &FileLocation{
						Path:       "Sources/App.swift",
						LineNumber: 42,
					},
				},
			},
		},
	}

	output := captureXcodeCloudStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Line") {
		t.Fatalf("expected header in output, got: %s", output)
	}
	if !strings.Contains(output, "Sources/App.swift") {
		t.Fatalf("expected file path in output, got: %s", output)
	}
}

func TestPrintMarkdown_CiIssues(t *testing.T) {
	resp := &CiIssuesResponse{
		Data: []CiIssueResource{
			{
				ID: "issue-1",
				Attributes: CiIssueAttributes{
					IssueType: "WARNING",
					Message:   "Check this",
				},
			},
		},
	}

	output := captureXcodeCloudStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Type") {
		t.Fatalf("expected markdown header in output, got: %s", output)
	}
	if !strings.Contains(output, "WARNING") {
		t.Fatalf("expected issue type in output, got: %s", output)
	}
}

func TestPrintTable_CiArtifactDownloadResult(t *testing.T) {
	result := &CiArtifactDownloadResult{
		ID:           "art-1",
		FileName:     "Build.zip",
		FileType:     "ARCHIVE",
		FileSize:     2048,
		OutputPath:   "/tmp/Build.zip",
		BytesWritten: 2048,
	}

	output := captureXcodeCloudStdout(t, func() error {
		return PrintTable(result)
	})

	if !strings.Contains(output, "Bytes Written") {
		t.Fatalf("expected header in output, got: %s", output)
	}
	if !strings.Contains(output, "/tmp/Build.zip") {
		t.Fatalf("expected output path in output, got: %s", output)
	}
}

func TestIsBuildRunComplete(t *testing.T) {
	tests := []struct {
		progress CiBuildRunExecutionProgress
		want     bool
	}{
		{CiBuildRunExecutionProgressPending, false},
		{CiBuildRunExecutionProgressRunning, false},
		{CiBuildRunExecutionProgressComplete, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.progress), func(t *testing.T) {
			got := IsBuildRunComplete(tt.progress)
			if got != tt.want {
				t.Errorf("IsBuildRunComplete(%s) = %v, want %v", tt.progress, got, tt.want)
			}
		})
	}
}

func TestIsBuildRunSuccessful(t *testing.T) {
	tests := []struct {
		status CiBuildRunCompletionStatus
		want   bool
	}{
		{CiBuildRunCompletionStatusSucceeded, true},
		{CiBuildRunCompletionStatusFailed, false},
		{CiBuildRunCompletionStatusErrored, false},
		{CiBuildRunCompletionStatusCanceled, false},
		{CiBuildRunCompletionStatusSkipped, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			got := IsBuildRunSuccessful(tt.status)
			if got != tt.want {
				t.Errorf("IsBuildRunSuccessful(%s) = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

func TestPrintTable_ScmRepositories(t *testing.T) {
	resp := &ScmRepositoriesResponse{
		Data: []ScmRepositoryResource{
			{
				ID: "repo-1",
				Attributes: ScmRepositoryAttributes{
					OwnerName:      "example",
					RepositoryName: "demo",
				},
			},
		},
	}

	output := captureXcodeCloudStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Repository") {
		t.Fatalf("expected header in output, got: %s", output)
	}
	if !strings.Contains(output, "demo") {
		t.Fatalf("expected repository name in output, got: %s", output)
	}
}

func TestPrintMarkdown_ScmRepositories(t *testing.T) {
	resp := &ScmRepositoriesResponse{
		Data: []ScmRepositoryResource{
			{
				ID: "repo-2",
				Attributes: ScmRepositoryAttributes{
					OwnerName:      "example",
					RepositoryName: "demo",
				},
			},
		},
	}

	output := captureXcodeCloudStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Repository") {
		t.Fatalf("expected markdown header in output, got: %s", output)
	}
	if !strings.Contains(output, "example") {
		t.Fatalf("expected owner name in output, got: %s", output)
	}
}

func TestPrintTable_ScmProviders(t *testing.T) {
	resp := &ScmProvidersResponse{
		Data: []ScmProviderResource{
			{
				ID: "provider-1",
				Attributes: ScmProviderAttributes{
					ScmProviderType: &ScmProviderType{
						Kind:        "GITHUB_CLOUD",
						DisplayName: "GitHub",
					},
					URL: "https://github.com",
				},
			},
		},
	}

	output := captureXcodeCloudStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Provider Type") {
		t.Fatalf("expected header in output, got: %s", output)
	}
	if !strings.Contains(output, "GitHub") {
		t.Fatalf("expected provider type in output, got: %s", output)
	}
}

func TestPrintMarkdown_ScmProviders(t *testing.T) {
	resp := &ScmProvidersResponse{
		Data: []ScmProviderResource{
			{
				ID: "provider-2",
				Attributes: ScmProviderAttributes{
					ScmProviderType: &ScmProviderType{
						Kind:        "GITLAB_CLOUD",
						DisplayName: "GitLab",
					},
					URL: "https://gitlab.com",
				},
			},
		},
	}

	output := captureXcodeCloudStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Provider Type") {
		t.Fatalf("expected markdown header in output, got: %s", output)
	}
	if !strings.Contains(output, "gitlab.com") {
		t.Fatalf("expected provider URL in output, got: %s", output)
	}
}

func TestPrintTable_ScmGitReferences(t *testing.T) {
	resp := &ScmGitReferencesResponse{
		Data: []ScmGitReferenceResource{
			{
				ID: "ref-1",
				Attributes: ScmGitReferenceAttributes{
					Name:          "main",
					CanonicalName: "refs/heads/main",
					Kind:          "BRANCH",
				},
			},
		},
	}

	output := captureXcodeCloudStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Canonical Name") {
		t.Fatalf("expected header in output, got: %s", output)
	}
	if !strings.Contains(output, "refs/heads/main") {
		t.Fatalf("expected canonical name in output, got: %s", output)
	}
}

func TestPrintMarkdown_ScmGitReferences(t *testing.T) {
	resp := &ScmGitReferencesResponse{
		Data: []ScmGitReferenceResource{
			{
				ID: "ref-2",
				Attributes: ScmGitReferenceAttributes{
					Name:          "release",
					CanonicalName: "refs/tags/release",
					Kind:          "TAG",
					IsDeleted:     true,
				},
			},
		},
	}

	output := captureXcodeCloudStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Canonical Name") {
		t.Fatalf("expected markdown header in output, got: %s", output)
	}
	if !strings.Contains(output, "refs/tags/release") {
		t.Fatalf("expected canonical name in output, got: %s", output)
	}
}

func TestPrintTable_ScmPullRequests(t *testing.T) {
	resp := &ScmPullRequestsResponse{
		Data: []ScmPullRequestResource{
			{
				ID: "pr-1",
				Attributes: ScmPullRequestAttributes{
					Title:                      "Add feature",
					Number:                     42,
					SourceRepositoryOwner:      "org",
					SourceRepositoryName:       "repo",
					SourceBranchName:           "feature",
					DestinationRepositoryOwner: "org",
					DestinationRepositoryName:  "repo",
					DestinationBranchName:      "main",
					IsClosed:                   false,
				},
			},
		},
	}

	output := captureXcodeCloudStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Pull") && !strings.Contains(output, "Source") {
		t.Fatalf("expected header in output, got: %s", output)
	}
	if !strings.Contains(output, "feature") {
		t.Fatalf("expected source branch in output, got: %s", output)
	}
}

func TestPrintMarkdown_ScmPullRequests(t *testing.T) {
	resp := &ScmPullRequestsResponse{
		Data: []ScmPullRequestResource{
			{
				ID: "pr-2",
				Attributes: ScmPullRequestAttributes{
					Title:                      "Fix bug",
					Number:                     7,
					SourceRepositoryOwner:      "org",
					SourceRepositoryName:       "repo",
					SourceBranchName:           "bugfix",
					DestinationRepositoryOwner: "org",
					DestinationRepositoryName:  "repo",
					DestinationBranchName:      "main",
					IsClosed:                   true,
				},
			},
		},
	}

	output := captureXcodeCloudStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Title") {
		t.Fatalf("expected markdown header in output, got: %s", output)
	}
	if !strings.Contains(output, "Fix bug") {
		t.Fatalf("expected title in output, got: %s", output)
	}
}

func TestPrintTable_CiMacOsVersions(t *testing.T) {
	resp := &CiMacOsVersionsResponse{
		Data: []CiMacOsVersionResource{
			{
				ID: "macos-1",
				Attributes: CiMacOsVersionAttributes{
					Version: "14.0",
					Name:    "Sonoma",
				},
			},
		},
	}

	output := captureXcodeCloudStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Version") {
		t.Fatalf("expected header in output, got: %s", output)
	}
	if !strings.Contains(output, "Sonoma") {
		t.Fatalf("expected name in output, got: %s", output)
	}
}

func TestPrintMarkdown_CiMacOsVersions(t *testing.T) {
	resp := &CiMacOsVersionsResponse{
		Data: []CiMacOsVersionResource{
			{
				ID: "macos-2",
				Attributes: CiMacOsVersionAttributes{
					Version: "13.0",
					Name:    "Ventura",
				},
			},
		},
	}

	output := captureXcodeCloudStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Version") {
		t.Fatalf("expected markdown header in output, got: %s", output)
	}
	if !strings.Contains(output, "Ventura") {
		t.Fatalf("expected name in output, got: %s", output)
	}
}

func TestPrintTable_CiXcodeVersions(t *testing.T) {
	resp := &CiXcodeVersionsResponse{
		Data: []CiXcodeVersionResource{
			{
				ID: "xcode-1",
				Attributes: CiXcodeVersionAttributes{
					Version: "15.0",
					Name:    "Xcode 15",
				},
			},
		},
	}

	output := captureXcodeCloudStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Version") {
		t.Fatalf("expected header in output, got: %s", output)
	}
	if !strings.Contains(output, "Xcode 15") {
		t.Fatalf("expected name in output, got: %s", output)
	}
}

func TestPrintMarkdown_CiXcodeVersions(t *testing.T) {
	resp := &CiXcodeVersionsResponse{
		Data: []CiXcodeVersionResource{
			{
				ID: "xcode-2",
				Attributes: CiXcodeVersionAttributes{
					Version: "14.3",
					Name:    "Xcode 14.3",
				},
			},
		},
	}

	output := captureXcodeCloudStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Version") {
		t.Fatalf("expected markdown header in output, got: %s", output)
	}
	if !strings.Contains(output, "Xcode 14.3") {
		t.Fatalf("expected name in output, got: %s", output)
	}
}

func TestPrintTable_CiWorkflowDeleteResult(t *testing.T) {
	result := &CiWorkflowDeleteResult{ID: "wf-1", Deleted: true}

	output := captureXcodeCloudStdout(t, func() error {
		return PrintTable(result)
	})

	if !strings.Contains(output, "Deleted") {
		t.Fatalf("expected header in output, got: %s", output)
	}
	if !strings.Contains(output, "wf-1") {
		t.Fatalf("expected ID in output, got: %s", output)
	}
}

func TestPrintMarkdown_CiWorkflowDeleteResult(t *testing.T) {
	result := &CiWorkflowDeleteResult{ID: "wf-2", Deleted: true}

	output := captureXcodeCloudStdout(t, func() error {
		return PrintMarkdown(result)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Deleted") {
		t.Fatalf("expected markdown header in output, got: %s", output)
	}
	if !strings.Contains(output, "wf-2") {
		t.Fatalf("expected ID in output, got: %s", output)
	}
}

func TestPrintTable_CiProductDeleteResult(t *testing.T) {
	result := &CiProductDeleteResult{ID: "prod-1", Deleted: true}

	output := captureXcodeCloudStdout(t, func() error {
		return PrintTable(result)
	})

	if !strings.Contains(output, "Deleted") {
		t.Fatalf("expected header in output, got: %s", output)
	}
	if !strings.Contains(output, "prod-1") {
		t.Fatalf("expected ID in output, got: %s", output)
	}
}

func TestPrintMarkdown_CiProductDeleteResult(t *testing.T) {
	result := &CiProductDeleteResult{ID: "prod-2", Deleted: true}

	output := captureXcodeCloudStdout(t, func() error {
		return PrintMarkdown(result)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Deleted") {
		t.Fatalf("expected markdown header in output, got: %s", output)
	}
	if !strings.Contains(output, "prod-2") {
		t.Fatalf("expected ID in output, got: %s", output)
	}
}

func TestBuildCiProductsQuery(t *testing.T) {
	query := &ciProductsQuery{}
	WithCiProductsAppID("app-1")(query)
	WithCiProductsLimit(25)(query)

	values, err := url.ParseQuery(buildCiProductsQuery(query))
	if err != nil {
		t.Fatalf("failed to parse query: %v", err)
	}
	if got := values.Get("filter[app]"); got != "app-1" {
		t.Fatalf("expected filter[app]=app-1, got %q", got)
	}
	if got := values.Get("limit"); got != "25" {
		t.Fatalf("expected limit=25, got %q", got)
	}
}

func TestBuildCiWorkflowsQuery(t *testing.T) {
	query := &ciWorkflowsQuery{}
	WithCiWorkflowsLimit(50)(query)

	values, err := url.ParseQuery(buildCiWorkflowsQuery(query))
	if err != nil {
		t.Fatalf("failed to parse query: %v", err)
	}
	if got := values.Get("limit"); got != "50" {
		t.Fatalf("expected limit=50, got %q", got)
	}
}

func TestBuildScmGitReferencesQuery(t *testing.T) {
	query := &scmGitReferencesQuery{}
	WithScmGitReferencesLimit(100)(query)

	values, err := url.ParseQuery(buildScmGitReferencesQuery(query))
	if err != nil {
		t.Fatalf("failed to parse query: %v", err)
	}
	if got := values.Get("limit"); got != "100" {
		t.Fatalf("expected limit=100, got %q", got)
	}
}

func TestBuildCiBuildRunsQuery(t *testing.T) {
	query := &ciBuildRunsQuery{}
	WithCiBuildRunsLimit(10)(query)

	values, err := url.ParseQuery(buildCiBuildRunsQuery(query))
	if err != nil {
		t.Fatalf("failed to parse query: %v", err)
	}
	if got := values.Get("limit"); got != "10" {
		t.Fatalf("expected limit=10, got %q", got)
	}
}

func TestBuildCiArtifactsQuery(t *testing.T) {
	query := &ciArtifactsQuery{}
	WithCiArtifactsLimit(25)(query)

	values, err := url.ParseQuery(buildCiArtifactsQuery(query))
	if err != nil {
		t.Fatalf("failed to parse query: %v", err)
	}
	if got := values.Get("limit"); got != "25" {
		t.Fatalf("expected limit=25, got %q", got)
	}
}

func TestBuildCiTestResultsQuery(t *testing.T) {
	query := &ciTestResultsQuery{}
	WithCiTestResultsLimit(30)(query)

	values, err := url.ParseQuery(buildCiTestResultsQuery(query))
	if err != nil {
		t.Fatalf("failed to parse query: %v", err)
	}
	if got := values.Get("limit"); got != "30" {
		t.Fatalf("expected limit=30, got %q", got)
	}
}

func TestBuildCiIssuesQuery(t *testing.T) {
	query := &ciIssuesQuery{}
	WithCiIssuesLimit(35)(query)

	values, err := url.ParseQuery(buildCiIssuesQuery(query))
	if err != nil {
		t.Fatalf("failed to parse query: %v", err)
	}
	if got := values.Get("limit"); got != "35" {
		t.Fatalf("expected limit=35, got %q", got)
	}
}

func TestBuildCiMacOsVersionsQuery(t *testing.T) {
	query := &ciMacOsVersionsQuery{}
	WithCiMacOsVersionsLimit(15)(query)

	values, err := url.ParseQuery(buildCiMacOsVersionsQuery(query))
	if err != nil {
		t.Fatalf("failed to parse query: %v", err)
	}
	if got := values.Get("limit"); got != "15" {
		t.Fatalf("expected limit=15, got %q", got)
	}
}

func TestBuildCiXcodeVersionsQuery(t *testing.T) {
	query := &ciXcodeVersionsQuery{}
	WithCiXcodeVersionsLimit(20)(query)

	values, err := url.ParseQuery(buildCiXcodeVersionsQuery(query))
	if err != nil {
		t.Fatalf("failed to parse query: %v", err)
	}
	if got := values.Get("limit"); got != "20" {
		t.Fatalf("expected limit=20, got %q", got)
	}
}

func TestBuildCiProductRepositoriesQuery(t *testing.T) {
	query := &ciProductRepositoriesQuery{}
	WithCiProductRepositoriesLimit(12)(query)

	values, err := url.ParseQuery(buildCiProductRepositoriesQuery(query))
	if err != nil {
		t.Fatalf("failed to parse query: %v", err)
	}
	if got := values.Get("limit"); got != "12" {
		t.Fatalf("expected limit=12, got %q", got)
	}
}

func TestBuildCiBuildRunBuildsQuery(t *testing.T) {
	query := &ciBuildRunBuildsQuery{}
	WithCiBuildRunBuildsLimit(8)(query)

	values, err := url.ParseQuery(buildCiBuildRunBuildsQuery(query))
	if err != nil {
		t.Fatalf("failed to parse query: %v", err)
	}
	if got := values.Get("limit"); got != "8" {
		t.Fatalf("expected limit=8, got %q", got)
	}
}

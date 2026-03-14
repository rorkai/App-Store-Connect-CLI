package cmdtest

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func runXcodeCloudInvalidNextURLCases(
	t *testing.T,
	argsPrefix []string,
	wantErrPrefix string,
) {
	t.Helper()

	tests := []struct {
		name    string
		next    string
		wantErr string
	}{
		{
			name:    "invalid scheme",
			next:    "http://api.appstoreconnect.apple.com/v1/ciBuildActions/action-1/artifacts?cursor=AQ",
			wantErr: wantErrPrefix + " must be an App Store Connect URL",
		},
		{
			name:    "malformed URL",
			next:    "https://api.appstoreconnect.apple.com/%zz",
			wantErr: wantErrPrefix + " must be a valid URL:",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args := append(append([]string{}, argsPrefix...), "--next", test.next)

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			var runErr error
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = root.Run(context.Background())
			})

			if runErr == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(runErr.Error(), test.wantErr) {
				t.Fatalf("expected error %q, got %v", test.wantErr, runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	}
}

func runXcodeCloudPaginateFromNext(
	t *testing.T,
	argsPrefix []string,
	firstURL string,
	secondURL string,
	firstBody string,
	secondBody string,
	wantIDs ...string,
) {
	t.Helper()

	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.String() != firstURL {
				t.Fatalf("unexpected first request: %s %s", req.Method, req.URL.String())
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(firstBody)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.String() != secondURL {
				t.Fatalf("unexpected second request: %s %s", req.Method, req.URL.String())
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(secondBody)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	args := append(append([]string{}, argsPrefix...), "--paginate", "--next", firstURL)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse(args); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	for _, id := range wantIDs {
		needle := `"id":"` + id + `"`
		if !strings.Contains(stdout, needle) {
			t.Fatalf("expected output to contain %q, got %q", needle, stdout)
		}
	}
}

func TestXcodeCloudBuildRunsListRejectsInvalidNextURL(t *testing.T) {
	runXcodeCloudInvalidNextURLCases(
		t,
		[]string{"xcode-cloud", "build-runs", "list"},
		"xcode-cloud build-runs: --next",
	)
}

func TestXcodeCloudBuildRunsListPaginateFromNextWithoutWorkflowID(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/ciWorkflows/workflow-1/buildRuns?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/ciWorkflows/workflow-1/buildRuns?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"ciBuildRuns","id":"ci-run-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"ciBuildRuns","id":"ci-run-next-2"}],"links":{"next":""}}`

	runXcodeCloudPaginateFromNext(
		t,
		[]string{"xcode-cloud", "build-runs", "list"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"ci-run-next-1",
		"ci-run-next-2",
	)
}

func TestXcodeCloudBuildRunsBuildsRejectsInvalidNextURL(t *testing.T) {
	runXcodeCloudInvalidNextURLCases(
		t,
		[]string{"xcode-cloud", "build-runs", "builds"},
		"xcode-cloud build-runs builds: --next",
	)
}

func TestXcodeCloudBuildRunsBuildsPaginateFromNextWithoutRunID(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/ciBuildRuns/run-1/builds?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/ciBuildRuns/run-1/builds?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"builds","id":"ci-run-build-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"builds","id":"ci-run-build-next-2"}],"links":{"next":""}}`

	runXcodeCloudPaginateFromNext(
		t,
		[]string{"xcode-cloud", "build-runs", "builds"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"ci-run-build-next-1",
		"ci-run-build-next-2",
	)
}

func TestXcodeCloudIssuesListRejectsInvalidNextURL(t *testing.T) {
	runXcodeCloudInvalidNextURLCases(
		t,
		[]string{"xcode-cloud", "issues", "list"},
		"xcode-cloud issues list: --next",
	)
}

func TestXcodeCloudIssuesListPaginateFromNextWithoutActionID(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/ciBuildActions/action-1/issues?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/ciBuildActions/action-1/issues?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"ciIssues","id":"ci-issue-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"ciIssues","id":"ci-issue-next-2"}],"links":{"next":""}}`

	runXcodeCloudPaginateFromNext(
		t,
		[]string{"xcode-cloud", "issues", "list"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"ci-issue-next-1",
		"ci-issue-next-2",
	)
}

func TestXcodeCloudTestResultsListRejectsInvalidNextURL(t *testing.T) {
	runXcodeCloudInvalidNextURLCases(
		t,
		[]string{"xcode-cloud", "test-results", "list"},
		"xcode-cloud test-results list: --next",
	)
}

func TestXcodeCloudTestResultsListPaginateFromNextWithoutActionID(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/ciBuildActions/action-1/testResults?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/ciBuildActions/action-1/testResults?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"ciTestResults","id":"ci-test-result-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"ciTestResults","id":"ci-test-result-next-2"}],"links":{"next":""}}`

	runXcodeCloudPaginateFromNext(
		t,
		[]string{"xcode-cloud", "test-results", "list"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"ci-test-result-next-1",
		"ci-test-result-next-2",
	)
}

func TestXcodeCloudArtifactsListRejectsInvalidNextURL(t *testing.T) {
	runXcodeCloudInvalidNextURLCases(
		t,
		[]string{"xcode-cloud", "artifacts", "list"},
		"xcode-cloud artifacts list: --next",
	)
}

func TestXcodeCloudArtifactsListPaginateFromNextWithoutActionID(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/ciBuildActions/action-1/artifacts?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/ciBuildActions/action-1/artifacts?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"ciArtifacts","id":"ci-artifact-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"ciArtifacts","id":"ci-artifact-next-2"}],"links":{"next":""}}`

	runXcodeCloudPaginateFromNext(
		t,
		[]string{"xcode-cloud", "artifacts", "list"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"ci-artifact-next-1",
		"ci-artifact-next-2",
	)
}

func TestXcodeCloudProductsListRejectsInvalidNextURL(t *testing.T) {
	runXcodeCloudInvalidNextURLCases(
		t,
		[]string{"xcode-cloud", "products", "list"},
		"xcode-cloud products: --next",
	)
}

func TestXcodeCloudProductsListPaginateFromNext(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/ciProducts?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/ciProducts?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"ciProducts","id":"ci-product-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"ciProducts","id":"ci-product-next-2"}],"links":{"next":""}}`

	runXcodeCloudPaginateFromNext(
		t,
		[]string{"xcode-cloud", "products", "list"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"ci-product-next-1",
		"ci-product-next-2",
	)
}

func TestXcodeCloudProductsBuildRunsRejectsInvalidNextURL(t *testing.T) {
	runXcodeCloudInvalidNextURLCases(
		t,
		[]string{"xcode-cloud", "products", "build-runs"},
		"xcode-cloud products build-runs: --next",
	)
}

func TestXcodeCloudProductsBuildRunsPaginateFromNextWithoutID(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/ciProducts/prod-1/buildRuns?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/ciProducts/prod-1/buildRuns?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"ciBuildRuns","id":"ci-product-run-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"ciBuildRuns","id":"ci-product-run-next-2"}],"links":{"next":""}}`

	runXcodeCloudPaginateFromNext(
		t,
		[]string{"xcode-cloud", "products", "build-runs"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"ci-product-run-next-1",
		"ci-product-run-next-2",
	)
}

func TestXcodeCloudProductsWorkflowsRejectsInvalidNextURL(t *testing.T) {
	runXcodeCloudInvalidNextURLCases(
		t,
		[]string{"xcode-cloud", "products", "workflows"},
		"xcode-cloud products workflows: --next",
	)
}

func TestXcodeCloudProductsWorkflowsPaginateFromNextWithoutID(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/ciProducts/prod-1/workflows?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/ciProducts/prod-1/workflows?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"ciWorkflows","id":"ci-product-workflow-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"ciWorkflows","id":"ci-product-workflow-next-2"}],"links":{"next":""}}`

	runXcodeCloudPaginateFromNext(
		t,
		[]string{"xcode-cloud", "products", "workflows"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"ci-product-workflow-next-1",
		"ci-product-workflow-next-2",
	)
}

func TestXcodeCloudProductsPrimaryRepositoriesRejectsInvalidNextURL(t *testing.T) {
	runXcodeCloudInvalidNextURLCases(
		t,
		[]string{"xcode-cloud", "products", "primary-repositories"},
		"xcode-cloud products primary-repositories: --next",
	)
}

func TestXcodeCloudProductsPrimaryRepositoriesPaginateFromNextWithoutID(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/ciProducts/prod-1/primaryRepositories?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/ciProducts/prod-1/primaryRepositories?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"scmRepositories","id":"ci-product-primary-repo-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"scmRepositories","id":"ci-product-primary-repo-next-2"}],"links":{"next":""}}`

	runXcodeCloudPaginateFromNext(
		t,
		[]string{"xcode-cloud", "products", "primary-repositories"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"ci-product-primary-repo-next-1",
		"ci-product-primary-repo-next-2",
	)
}

func TestXcodeCloudProductsAdditionalRepositoriesRejectsInvalidNextURL(t *testing.T) {
	runXcodeCloudInvalidNextURLCases(
		t,
		[]string{"xcode-cloud", "products", "additional-repositories"},
		"xcode-cloud products additional-repositories: --next",
	)
}

func TestXcodeCloudProductsAdditionalRepositoriesPaginateFromNextWithoutID(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/ciProducts/prod-1/additionalRepositories?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/ciProducts/prod-1/additionalRepositories?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"scmRepositories","id":"ci-product-additional-repo-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"scmRepositories","id":"ci-product-additional-repo-next-2"}],"links":{"next":""}}`

	runXcodeCloudPaginateFromNext(
		t,
		[]string{"xcode-cloud", "products", "additional-repositories"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"ci-product-additional-repo-next-1",
		"ci-product-additional-repo-next-2",
	)
}

func TestXcodeCloudMacOSVersionsListRejectsInvalidNextURL(t *testing.T) {
	runXcodeCloudInvalidNextURLCases(
		t,
		[]string{"xcode-cloud", "macos-versions", "list"},
		"xcode-cloud macos-versions: --next",
	)
}

func TestXcodeCloudMacOSVersionsListPaginateFromNext(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/ciMacOsVersions?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/ciMacOsVersions?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"ciMacOsVersions","id":"ci-macos-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"ciMacOsVersions","id":"ci-macos-next-2"}],"links":{"next":""}}`

	runXcodeCloudPaginateFromNext(
		t,
		[]string{"xcode-cloud", "macos-versions", "list"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"ci-macos-next-1",
		"ci-macos-next-2",
	)
}

func TestXcodeCloudXcodeVersionsListRejectsInvalidNextURL(t *testing.T) {
	runXcodeCloudInvalidNextURLCases(
		t,
		[]string{"xcode-cloud", "xcode-versions", "list"},
		"xcode-cloud xcode-versions: --next",
	)
}

func TestXcodeCloudXcodeVersionsListPaginateFromNext(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/ciXcodeVersions?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/ciXcodeVersions?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"ciXcodeVersions","id":"ci-xcode-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"ciXcodeVersions","id":"ci-xcode-next-2"}],"links":{"next":""}}`

	runXcodeCloudPaginateFromNext(
		t,
		[]string{"xcode-cloud", "xcode-versions", "list"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"ci-xcode-next-1",
		"ci-xcode-next-2",
	)
}

func TestXcodeCloudMacOSVersionsXcodeVersionsRejectsInvalidNextURL(t *testing.T) {
	runXcodeCloudInvalidNextURLCases(
		t,
		[]string{"xcode-cloud", "macos-versions", "xcode-versions"},
		"xcode-cloud macos-versions xcode-versions: --next",
	)
}

func TestXcodeCloudMacOSVersionsXcodeVersionsPaginateFromNextWithoutID(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/ciMacOsVersions/macos-1/xcodeVersions?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/ciMacOsVersions/macos-1/xcodeVersions?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"ciXcodeVersions","id":"ci-macos-xcode-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"ciXcodeVersions","id":"ci-macos-xcode-next-2"}],"links":{"next":""}}`

	runXcodeCloudPaginateFromNext(
		t,
		[]string{"xcode-cloud", "macos-versions", "xcode-versions"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"ci-macos-xcode-next-1",
		"ci-macos-xcode-next-2",
	)
}

func TestXcodeCloudXcodeVersionsMacOSVersionsRejectsInvalidNextURL(t *testing.T) {
	runXcodeCloudInvalidNextURLCases(
		t,
		[]string{"xcode-cloud", "xcode-versions", "macos-versions"},
		"xcode-cloud xcode-versions macos-versions: --next",
	)
}

func TestXcodeCloudXcodeVersionsMacOSVersionsPaginateFromNextWithoutID(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/ciXcodeVersions/xcode-1/macOsVersions?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/ciXcodeVersions/xcode-1/macOsVersions?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"ciMacOsVersions","id":"ci-xcode-macos-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"ciMacOsVersions","id":"ci-xcode-macos-next-2"}],"links":{"next":""}}`

	runXcodeCloudPaginateFromNext(
		t,
		[]string{"xcode-cloud", "xcode-versions", "macos-versions"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"ci-xcode-macos-next-1",
		"ci-xcode-macos-next-2",
	)
}

func TestXcodeCloudScmProvidersListRejectsInvalidNextURL(t *testing.T) {
	runXcodeCloudInvalidNextURLCases(
		t,
		[]string{"xcode-cloud", "scm", "providers", "list"},
		"xcode-cloud scm providers: --next",
	)
}

func TestXcodeCloudScmProvidersListPaginateFromNext(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/scmProviders?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/scmProviders?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"scmProviders","id":"scm-provider-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"scmProviders","id":"scm-provider-next-2"}],"links":{"next":""}}`

	runXcodeCloudPaginateFromNext(
		t,
		[]string{"xcode-cloud", "scm", "providers", "list"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"scm-provider-next-1",
		"scm-provider-next-2",
	)
}

func TestXcodeCloudScmRepositoriesListRejectsInvalidNextURL(t *testing.T) {
	runXcodeCloudInvalidNextURLCases(
		t,
		[]string{"xcode-cloud", "scm", "repositories", "list"},
		"xcode-cloud scm repositories: --next",
	)
}

func TestXcodeCloudScmRepositoriesListPaginateFromNext(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/scmRepositories?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/scmRepositories?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"scmRepositories","id":"scm-repo-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"scmRepositories","id":"scm-repo-next-2"}],"links":{"next":""}}`

	runXcodeCloudPaginateFromNext(
		t,
		[]string{"xcode-cloud", "scm", "repositories", "list"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"scm-repo-next-1",
		"scm-repo-next-2",
	)
}

func TestXcodeCloudScmProvidersRepositoriesRejectsInvalidNextURL(t *testing.T) {
	runXcodeCloudInvalidNextURLCases(
		t,
		[]string{"xcode-cloud", "scm", "providers", "repositories"},
		"xcode-cloud scm providers repositories: --next",
	)
}

func TestXcodeCloudScmProvidersRepositoriesPaginateFromNextWithoutProviderID(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/scmProviders/provider-1/repositories?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/scmProviders/provider-1/repositories?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"scmRepositories","id":"scm-provider-repo-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"scmRepositories","id":"scm-provider-repo-next-2"}],"links":{"next":""}}`

	runXcodeCloudPaginateFromNext(
		t,
		[]string{"xcode-cloud", "scm", "providers", "repositories"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"scm-provider-repo-next-1",
		"scm-provider-repo-next-2",
	)
}

func TestXcodeCloudScmRepositoriesGitReferencesRejectsInvalidNextURL(t *testing.T) {
	runXcodeCloudInvalidNextURLCases(
		t,
		[]string{"xcode-cloud", "scm", "repositories", "git-references"},
		"xcode-cloud scm repositories git-references: --next",
	)
}

func TestXcodeCloudScmRepositoriesGitReferencesPaginateFromNextWithoutRepoID(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/scmRepositories/repo-1/gitReferences?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/scmRepositories/repo-1/gitReferences?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"scmGitReferences","id":"scm-git-ref-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"scmGitReferences","id":"scm-git-ref-next-2"}],"links":{"next":""}}`

	runXcodeCloudPaginateFromNext(
		t,
		[]string{"xcode-cloud", "scm", "repositories", "git-references"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"scm-git-ref-next-1",
		"scm-git-ref-next-2",
	)
}

func TestXcodeCloudScmRepositoriesPullRequestsRejectsInvalidNextURL(t *testing.T) {
	runXcodeCloudInvalidNextURLCases(
		t,
		[]string{"xcode-cloud", "scm", "repositories", "pull-requests"},
		"xcode-cloud scm repositories pull-requests: --next",
	)
}

func TestXcodeCloudScmRepositoriesPullRequestsPaginateFromNextWithoutRepoID(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/scmRepositories/repo-1/pullRequests?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/scmRepositories/repo-1/pullRequests?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"scmPullRequests","id":"scm-pull-request-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"scmPullRequests","id":"scm-pull-request-next-2"}],"links":{"next":""}}`

	runXcodeCloudPaginateFromNext(
		t,
		[]string{"xcode-cloud", "scm", "repositories", "pull-requests"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"scm-pull-request-next-1",
		"scm-pull-request-next-2",
	)
}

func TestXcodeCloudScmRepositoriesRelationshipsGitReferencesRejectsInvalidNextURL(t *testing.T) {
	runXcodeCloudInvalidNextURLCases(
		t,
		[]string{"xcode-cloud", "scm", "repositories", "links", "git-references"},
		"xcode-cloud scm repositories links git-references: --next",
	)
}

func TestXcodeCloudScmRepositoriesRelationshipsGitReferencesPaginateFromNextWithoutRepoID(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/scmRepositories/repo-1/relationships/gitReferences?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/scmRepositories/repo-1/relationships/gitReferences?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"scmGitReferences","id":"scm-git-ref-link-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"scmGitReferences","id":"scm-git-ref-link-next-2"}],"links":{"next":""}}`

	runXcodeCloudPaginateFromNext(
		t,
		[]string{"xcode-cloud", "scm", "repositories", "links", "git-references"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"scm-git-ref-link-next-1",
		"scm-git-ref-link-next-2",
	)
}

func TestXcodeCloudScmRepositoriesRelationshipsPullRequestsRejectsInvalidNextURL(t *testing.T) {
	runXcodeCloudInvalidNextURLCases(
		t,
		[]string{"xcode-cloud", "scm", "repositories", "links", "pull-requests"},
		"xcode-cloud scm repositories links pull-requests: --next",
	)
}

func TestXcodeCloudScmRepositoriesRelationshipsPullRequestsPaginateFromNextWithoutRepoID(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/scmRepositories/repo-1/relationships/pullRequests?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/scmRepositories/repo-1/relationships/pullRequests?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"scmPullRequests","id":"scm-pull-request-link-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"scmPullRequests","id":"scm-pull-request-link-next-2"}],"links":{"next":""}}`

	runXcodeCloudPaginateFromNext(
		t,
		[]string{"xcode-cloud", "scm", "repositories", "links", "pull-requests"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"scm-pull-request-link-next-1",
		"scm-pull-request-link-next-2",
	)
}

func TestXcodeCloudWorkflowsListRejectsInvalidNextURL(t *testing.T) {
	runXcodeCloudInvalidNextURLCases(
		t,
		[]string{"xcode-cloud", "workflows", "list"},
		"xcode-cloud workflows: --next",
	)
}

func TestXcodeCloudWorkflowsListPaginateFromNextWithoutAppID(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/ciProducts/product-1/workflows?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/ciProducts/product-1/workflows?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"ciWorkflows","id":"ci-workflow-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"ciWorkflows","id":"ci-workflow-next-2"}],"links":{"next":""}}`

	runXcodeCloudPaginateFromNext(
		t,
		[]string{"xcode-cloud", "workflows", "list"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"ci-workflow-next-1",
		"ci-workflow-next-2",
	)
}

func TestXcodeCloudActionsListRejectsInvalidNextURL(t *testing.T) {
	runXcodeCloudInvalidNextURLCases(
		t,
		[]string{"xcode-cloud", "actions", "list"},
		"xcode-cloud actions: --next",
	)
}

func TestXcodeCloudActionsListPaginateFromNextWithoutRunID(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/ciBuildRuns/run-1/actions?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/ciBuildRuns/run-1/actions?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"ciBuildActions","id":"ci-action-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"ciBuildActions","id":"ci-action-next-2"}],"links":{"next":""}}`

	runXcodeCloudPaginateFromNext(
		t,
		[]string{"xcode-cloud", "actions", "list"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"ci-action-next-1",
		"ci-action-next-2",
	)
}

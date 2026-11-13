package cmdtest

import "testing"

func TestXcodeCloudListsRejectsInvalidNextURL(t *testing.T) {
	runInvalidNextURLCases(t, []invalidNextURLCase{
		{args: []string{"xcode-cloud", "build-runs", "list"}, wantErr: "xcode-cloud build-runs: --next"},
		{args: []string{"xcode-cloud", "build-runs", "builds"}, wantErr: "xcode-cloud build-runs builds: --next"},
		{args: []string{"xcode-cloud", "issues", "list"}, wantErr: "xcode-cloud issues list: --next"},
		{args: []string{"xcode-cloud", "test-results", "list"}, wantErr: "xcode-cloud test-results list: --next"},
		{args: []string{"xcode-cloud", "artifacts", "list"}, wantErr: "xcode-cloud artifacts list: --next"},
		{args: []string{"xcode-cloud", "products", "list"}, wantErr: "xcode-cloud products: --next"},
		{args: []string{"xcode-cloud", "products", "build-runs"}, wantErr: "xcode-cloud products build-runs: --next"},
		{args: []string{"xcode-cloud", "products", "workflows"}, wantErr: "xcode-cloud products workflows: --next"},
		{args: []string{"xcode-cloud", "products", "primary-repositories"}, wantErr: "xcode-cloud products primary-repositories: --next"},
		{args: []string{"xcode-cloud", "products", "additional-repositories"}, wantErr: "xcode-cloud products additional-repositories: --next"},
		{args: []string{"xcode-cloud", "macos-versions", "list"}, wantErr: "xcode-cloud macos-versions: --next"},
		{args: []string{"xcode-cloud", "xcode-versions", "list"}, wantErr: "xcode-cloud xcode-versions: --next"},
		{args: []string{"xcode-cloud", "macos-versions", "xcode-versions"}, wantErr: "xcode-cloud macos-versions xcode-versions: --next"},
		{args: []string{"xcode-cloud", "xcode-versions", "macos-versions"}, wantErr: "xcode-cloud xcode-versions macos-versions: --next"},
		{args: []string{"xcode-cloud", "scm", "providers", "list"}, wantErr: "xcode-cloud scm providers: --next"},
		{args: []string{"xcode-cloud", "scm", "repositories", "list"}, wantErr: "xcode-cloud scm repositories: --next"},
		{args: []string{"xcode-cloud", "scm", "providers", "repositories"}, wantErr: "xcode-cloud scm providers repositories: --next"},
		{args: []string{"xcode-cloud", "scm", "repositories", "git-references"}, wantErr: "xcode-cloud scm repositories git-references: --next"},
		{args: []string{"xcode-cloud", "scm", "repositories", "pull-requests"}, wantErr: "xcode-cloud scm repositories pull-requests: --next"},
		{args: []string{"xcode-cloud", "scm", "repositories", "links", "git-references"}, wantErr: "xcode-cloud scm repositories links git-references: --next"},
		{args: []string{"xcode-cloud", "scm", "repositories", "links", "pull-requests"}, wantErr: "xcode-cloud scm repositories links pull-requests: --next"},
		{args: []string{"xcode-cloud", "workflows", "list"}, wantErr: "xcode-cloud workflows: --next"},
		{args: []string{"xcode-cloud", "actions", "list"}, wantErr: "xcode-cloud actions: --next"},
	})
}

func TestXcodeCloudListsPaginateFromNext(t *testing.T) {
	runPaginateFromNextCases(t, []paginateFromNextCase{
		{
			args:     []string{"xcode-cloud", "build-runs", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/ciWorkflows/workflow-1/buildRuns?cursor=AQ&limit=200",
			dataType: "ciBuildRuns",
		},
		{
			args:     []string{"xcode-cloud", "build-runs", "builds"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/ciBuildRuns/run-1/builds?cursor=AQ&limit=200",
			dataType: "builds",
		},
		{
			args:     []string{"xcode-cloud", "issues", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/ciBuildActions/action-1/issues?cursor=AQ&limit=200",
			dataType: "ciIssues",
		},
		{
			args:     []string{"xcode-cloud", "test-results", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/ciBuildActions/action-1/testResults?cursor=AQ&limit=200",
			dataType: "ciTestResults",
		},
		{
			args:     []string{"xcode-cloud", "artifacts", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/ciBuildActions/action-1/artifacts?cursor=AQ&limit=200",
			dataType: "ciArtifacts",
		},
		{
			args:     []string{"xcode-cloud", "products", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/ciProducts?cursor=AQ&limit=200",
			dataType: "ciProducts",
		},
		{
			args:     []string{"xcode-cloud", "products", "build-runs"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/ciProducts/prod-1/buildRuns?cursor=AQ&limit=200",
			dataType: "ciBuildRuns",
		},
		{
			args:     []string{"xcode-cloud", "products", "workflows"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/ciProducts/prod-1/workflows?cursor=AQ&limit=200",
			dataType: "ciWorkflows",
		},
		{
			args:     []string{"xcode-cloud", "products", "primary-repositories"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/ciProducts/prod-1/primaryRepositories?cursor=AQ&limit=200",
			dataType: "scmRepositories",
		},
		{
			args:     []string{"xcode-cloud", "products", "additional-repositories"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/ciProducts/prod-1/additionalRepositories?cursor=AQ&limit=200",
			dataType: "scmRepositories",
		},
		{
			args:     []string{"xcode-cloud", "macos-versions", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/ciMacOsVersions?cursor=AQ&limit=200",
			dataType: "ciMacOsVersions",
		},
		{
			args:     []string{"xcode-cloud", "xcode-versions", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/ciXcodeVersions?cursor=AQ&limit=200",
			dataType: "ciXcodeVersions",
		},
		{
			args:     []string{"xcode-cloud", "macos-versions", "xcode-versions"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/ciMacOsVersions/macos-1/xcodeVersions?cursor=AQ&limit=200",
			dataType: "ciXcodeVersions",
		},
		{
			args:     []string{"xcode-cloud", "xcode-versions", "macos-versions"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/ciXcodeVersions/xcode-1/macOsVersions?cursor=AQ&limit=200",
			dataType: "ciMacOsVersions",
		},
		{
			args:     []string{"xcode-cloud", "scm", "providers", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/scmProviders?cursor=AQ&limit=200",
			dataType: "scmProviders",
		},
		{
			args:     []string{"xcode-cloud", "scm", "repositories", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/scmRepositories?cursor=AQ&limit=200",
			dataType: "scmRepositories",
		},
		{
			args:     []string{"xcode-cloud", "scm", "providers", "repositories"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/scmProviders/provider-1/repositories?cursor=AQ&limit=200",
			dataType: "scmRepositories",
		},
		{
			args:     []string{"xcode-cloud", "scm", "repositories", "git-references"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/scmRepositories/repo-1/gitReferences?cursor=AQ&limit=200",
			dataType: "scmGitReferences",
		},
		{
			args:     []string{"xcode-cloud", "scm", "repositories", "pull-requests"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/scmRepositories/repo-1/pullRequests?cursor=AQ&limit=200",
			dataType: "scmPullRequests",
		},
		{
			args:     []string{"xcode-cloud", "scm", "repositories", "links", "git-references"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/scmRepositories/repo-1/relationships/gitReferences?cursor=AQ&limit=200",
			dataType: "scmGitReferences",
		},
		{
			args:     []string{"xcode-cloud", "scm", "repositories", "links", "pull-requests"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/scmRepositories/repo-1/relationships/pullRequests?cursor=AQ&limit=200",
			dataType: "scmPullRequests",
		},
		{
			args:     []string{"xcode-cloud", "workflows", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/ciProducts/product-1/workflows?cursor=AQ&limit=200",
			dataType: "ciWorkflows",
		},
		{
			args:     []string{"xcode-cloud", "actions", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/ciBuildRuns/run-1/actions?cursor=AQ&limit=200",
			dataType: "ciBuildActions",
		},
	})
}

package asc

import (
	"context"
	"testing"
)

func TestXcodeCloudRelationshipLinkages_WithLimit(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name  string
		path  string
		limit string
		call  func(*Client) error
	}{
		{
			name:  "GetCiBuildActionArtifactsRelationships",
			path:  "/v1/ciBuildActions/action-1/relationships/artifacts",
			limit: "5",
			call: func(client *Client) error {
				_, err := client.GetCiBuildActionArtifactsRelationships(ctx, "action-1", WithLinkagesLimit(5))
				return err
			},
		},
		{
			name:  "GetCiBuildActionIssuesRelationships",
			path:  "/v1/ciBuildActions/action-1/relationships/issues",
			limit: "6",
			call: func(client *Client) error {
				_, err := client.GetCiBuildActionIssuesRelationships(ctx, "action-1", WithLinkagesLimit(6))
				return err
			},
		},
		{
			name:  "GetCiBuildActionTestResultsRelationships",
			path:  "/v1/ciBuildActions/action-1/relationships/testResults",
			limit: "7",
			call: func(client *Client) error {
				_, err := client.GetCiBuildActionTestResultsRelationships(ctx, "action-1", WithLinkagesLimit(7))
				return err
			},
		},
		{
			name:  "GetCiBuildRunActionsRelationships",
			path:  "/v1/ciBuildRuns/run-1/relationships/actions",
			limit: "8",
			call: func(client *Client) error {
				_, err := client.GetCiBuildRunActionsRelationships(ctx, "run-1", WithLinkagesLimit(8))
				return err
			},
		},
		{
			name:  "GetCiBuildRunBuildsRelationships",
			path:  "/v1/ciBuildRuns/run-1/relationships/builds",
			limit: "9",
			call: func(client *Client) error {
				_, err := client.GetCiBuildRunBuildsRelationships(ctx, "run-1", WithLinkagesLimit(9))
				return err
			},
		},
		{
			name:  "GetCiMacOsVersionXcodeVersionsRelationships",
			path:  "/v1/ciMacOsVersions/macos-1/relationships/xcodeVersions",
			limit: "10",
			call: func(client *Client) error {
				_, err := client.GetCiMacOsVersionXcodeVersionsRelationships(ctx, "macos-1", WithLinkagesLimit(10))
				return err
			},
		},
		{
			name:  "GetCiProductAdditionalRepositoriesRelationships",
			path:  "/v1/ciProducts/prod-1/relationships/additionalRepositories",
			limit: "11",
			call: func(client *Client) error {
				_, err := client.GetCiProductAdditionalRepositoriesRelationships(ctx, "prod-1", WithLinkagesLimit(11))
				return err
			},
		},
		{
			name:  "GetCiProductBuildRunsRelationships",
			path:  "/v1/ciProducts/prod-1/relationships/buildRuns",
			limit: "12",
			call: func(client *Client) error {
				_, err := client.GetCiProductBuildRunsRelationships(ctx, "prod-1", WithLinkagesLimit(12))
				return err
			},
		},
		{
			name:  "GetCiProductPrimaryRepositoriesRelationships",
			path:  "/v1/ciProducts/prod-1/relationships/primaryRepositories",
			limit: "13",
			call: func(client *Client) error {
				_, err := client.GetCiProductPrimaryRepositoriesRelationships(ctx, "prod-1", WithLinkagesLimit(13))
				return err
			},
		},
		{
			name:  "GetCiProductWorkflowsRelationships",
			path:  "/v1/ciProducts/prod-1/relationships/workflows",
			limit: "14",
			call: func(client *Client) error {
				_, err := client.GetCiProductWorkflowsRelationships(ctx, "prod-1", WithLinkagesLimit(14))
				return err
			},
		},
		{
			name:  "GetCiWorkflowBuildRunsRelationships",
			path:  "/v1/ciWorkflows/wf-1/relationships/buildRuns",
			limit: "15",
			call: func(client *Client) error {
				_, err := client.GetCiWorkflowBuildRunsRelationships(ctx, "wf-1", WithLinkagesLimit(15))
				return err
			},
		},
		{
			name:  "GetCiXcodeVersionMacOsVersionsRelationships",
			path:  "/v1/ciXcodeVersions/xcode-1/relationships/macOsVersions",
			limit: "16",
			call: func(client *Client) error {
				_, err := client.GetCiXcodeVersionMacOsVersionsRelationships(ctx, "xcode-1", WithLinkagesLimit(16))
				return err
			},
		},
		{
			name:  "GetScmProviderRepositoriesRelationships",
			path:  "/v1/scmProviders/provider-1/relationships/repositories",
			limit: "17",
			call: func(client *Client) error {
				_, err := client.GetScmProviderRepositoriesRelationships(ctx, "provider-1", WithLinkagesLimit(17))
				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runListLimitTest(t, test.path, test.limit, test.call)
		})
	}
}

func TestXcodeCloudRelationshipLinkages_UsesNextURL(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name string
		next string
		call func(*Client) error
	}{
		{
			name: "GetCiBuildActionArtifactsRelationships",
			next: "https://api.appstoreconnect.apple.com/v1/ciBuildActions/action-1/relationships/artifacts?cursor=next",
			call: func(client *Client) error {
				_, err := client.GetCiBuildActionArtifactsRelationships(ctx, "action-1", WithLinkagesNextURL("https://api.appstoreconnect.apple.com/v1/ciBuildActions/action-1/relationships/artifacts?cursor=next"))
				return err
			},
		},
		{
			name: "GetCiBuildActionIssuesRelationships",
			next: "https://api.appstoreconnect.apple.com/v1/ciBuildActions/action-1/relationships/issues?cursor=next",
			call: func(client *Client) error {
				_, err := client.GetCiBuildActionIssuesRelationships(ctx, "action-1", WithLinkagesNextURL("https://api.appstoreconnect.apple.com/v1/ciBuildActions/action-1/relationships/issues?cursor=next"))
				return err
			},
		},
		{
			name: "GetCiBuildActionTestResultsRelationships",
			next: "https://api.appstoreconnect.apple.com/v1/ciBuildActions/action-1/relationships/testResults?cursor=next",
			call: func(client *Client) error {
				_, err := client.GetCiBuildActionTestResultsRelationships(ctx, "action-1", WithLinkagesNextURL("https://api.appstoreconnect.apple.com/v1/ciBuildActions/action-1/relationships/testResults?cursor=next"))
				return err
			},
		},
		{
			name: "GetCiBuildRunActionsRelationships",
			next: "https://api.appstoreconnect.apple.com/v1/ciBuildRuns/run-1/relationships/actions?cursor=next",
			call: func(client *Client) error {
				_, err := client.GetCiBuildRunActionsRelationships(ctx, "run-1", WithLinkagesNextURL("https://api.appstoreconnect.apple.com/v1/ciBuildRuns/run-1/relationships/actions?cursor=next"))
				return err
			},
		},
		{
			name: "GetCiBuildRunBuildsRelationships",
			next: "https://api.appstoreconnect.apple.com/v1/ciBuildRuns/run-1/relationships/builds?cursor=next",
			call: func(client *Client) error {
				_, err := client.GetCiBuildRunBuildsRelationships(ctx, "run-1", WithLinkagesNextURL("https://api.appstoreconnect.apple.com/v1/ciBuildRuns/run-1/relationships/builds?cursor=next"))
				return err
			},
		},
		{
			name: "GetCiMacOsVersionXcodeVersionsRelationships",
			next: "https://api.appstoreconnect.apple.com/v1/ciMacOsVersions/macos-1/relationships/xcodeVersions?cursor=next",
			call: func(client *Client) error {
				_, err := client.GetCiMacOsVersionXcodeVersionsRelationships(ctx, "macos-1", WithLinkagesNextURL("https://api.appstoreconnect.apple.com/v1/ciMacOsVersions/macos-1/relationships/xcodeVersions?cursor=next"))
				return err
			},
		},
		{
			name: "GetCiProductAdditionalRepositoriesRelationships",
			next: "https://api.appstoreconnect.apple.com/v1/ciProducts/prod-1/relationships/additionalRepositories?cursor=next",
			call: func(client *Client) error {
				_, err := client.GetCiProductAdditionalRepositoriesRelationships(ctx, "prod-1", WithLinkagesNextURL("https://api.appstoreconnect.apple.com/v1/ciProducts/prod-1/relationships/additionalRepositories?cursor=next"))
				return err
			},
		},
		{
			name: "GetCiProductBuildRunsRelationships",
			next: "https://api.appstoreconnect.apple.com/v1/ciProducts/prod-1/relationships/buildRuns?cursor=next",
			call: func(client *Client) error {
				_, err := client.GetCiProductBuildRunsRelationships(ctx, "prod-1", WithLinkagesNextURL("https://api.appstoreconnect.apple.com/v1/ciProducts/prod-1/relationships/buildRuns?cursor=next"))
				return err
			},
		},
		{
			name: "GetCiProductPrimaryRepositoriesRelationships",
			next: "https://api.appstoreconnect.apple.com/v1/ciProducts/prod-1/relationships/primaryRepositories?cursor=next",
			call: func(client *Client) error {
				_, err := client.GetCiProductPrimaryRepositoriesRelationships(ctx, "prod-1", WithLinkagesNextURL("https://api.appstoreconnect.apple.com/v1/ciProducts/prod-1/relationships/primaryRepositories?cursor=next"))
				return err
			},
		},
		{
			name: "GetCiProductWorkflowsRelationships",
			next: "https://api.appstoreconnect.apple.com/v1/ciProducts/prod-1/relationships/workflows?cursor=next",
			call: func(client *Client) error {
				_, err := client.GetCiProductWorkflowsRelationships(ctx, "prod-1", WithLinkagesNextURL("https://api.appstoreconnect.apple.com/v1/ciProducts/prod-1/relationships/workflows?cursor=next"))
				return err
			},
		},
		{
			name: "GetCiWorkflowBuildRunsRelationships",
			next: "https://api.appstoreconnect.apple.com/v1/ciWorkflows/wf-1/relationships/buildRuns?cursor=next",
			call: func(client *Client) error {
				_, err := client.GetCiWorkflowBuildRunsRelationships(ctx, "wf-1", WithLinkagesNextURL("https://api.appstoreconnect.apple.com/v1/ciWorkflows/wf-1/relationships/buildRuns?cursor=next"))
				return err
			},
		},
		{
			name: "GetCiXcodeVersionMacOsVersionsRelationships",
			next: "https://api.appstoreconnect.apple.com/v1/ciXcodeVersions/xcode-1/relationships/macOsVersions?cursor=next",
			call: func(client *Client) error {
				_, err := client.GetCiXcodeVersionMacOsVersionsRelationships(ctx, "xcode-1", WithLinkagesNextURL("https://api.appstoreconnect.apple.com/v1/ciXcodeVersions/xcode-1/relationships/macOsVersions?cursor=next"))
				return err
			},
		},
		{
			name: "GetScmProviderRepositoriesRelationships",
			next: "https://api.appstoreconnect.apple.com/v1/scmProviders/provider-1/relationships/repositories?cursor=next",
			call: func(client *Client) error {
				_, err := client.GetScmProviderRepositoriesRelationships(ctx, "provider-1", WithLinkagesNextURL("https://api.appstoreconnect.apple.com/v1/scmProviders/provider-1/relationships/repositories?cursor=next"))
				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runNextURLTest(t, test.next, test.call)
		})
	}
}

func TestXcodeCloudRelationshipLinkages_ToOne(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name string
		path string
		call func(*Client) error
	}{
		{
			name: "GetCiBuildActionBuildRunRelationship",
			path: "/v1/ciBuildActions/action-1/relationships/buildRun",
			call: func(client *Client) error {
				_, err := client.GetCiBuildActionBuildRunRelationship(ctx, "action-1")
				return err
			},
		},
		{
			name: "GetCiProductAppRelationship",
			path: "/v1/ciProducts/prod-1/relationships/app",
			call: func(client *Client) error {
				_, err := client.GetCiProductAppRelationship(ctx, "prod-1")
				return err
			},
		},
		{
			name: "GetCiWorkflowRepositoryRelationship",
			path: "/v1/ciWorkflows/wf-1/relationships/repository",
			call: func(client *Client) error {
				_, err := client.GetCiWorkflowRepositoryRelationship(ctx, "wf-1")
				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runSimpleGetTest(t, test.path, test.call)
		})
	}
}

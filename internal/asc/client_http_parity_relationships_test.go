package asc

import (
	"context"
	"net/http"
	"testing"
)

func runSimpleGetTest(t *testing.T, expectedPath string, call func(*Client) error) {
	t.Helper()

	response := jsonResponse(http.StatusOK, `{"data":{"type":"apps","id":"1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != expectedPath {
			t.Fatalf("expected path %s, got %s", expectedPath, req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := call(client); err != nil {
		t.Fatalf("request error: %v", err)
	}
}

func runListLimitTest(t *testing.T, expectedPath, expectedLimit string, call func(*Client) error) {
	t.Helper()

	response := jsonResponse(http.StatusOK, `{"data":[{"type":"apps","id":"1"}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != expectedPath {
			t.Fatalf("expected path %s, got %s", expectedPath, req.URL.Path)
		}
		if req.URL.Query().Get("limit") != expectedLimit {
			t.Fatalf("expected limit=%s, got %q", expectedLimit, req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if err := call(client); err != nil {
		t.Fatalf("request error: %v", err)
	}
}

func runNextURLTest(t *testing.T, nextURL string, call func(*Client) error) {
	t.Helper()

	response := jsonResponse(http.StatusOK, `{"data":[{"type":"apps","id":"1"}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.String() != nextURL {
			t.Fatalf("expected next URL %q, got %q", nextURL, req.URL.String())
		}
		assertAuthorized(t, req)
	}, response)

	if err := call(client); err != nil {
		t.Fatalf("request error: %v", err)
	}
}

func TestParityRelatedGETs(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name string
		path string
		call func(*Client) error
	}{
		{
			name: "GetAppBetaAppReviewDetail",
			path: "/v1/apps/app-1/betaAppReviewDetail",
			call: func(client *Client) error {
				_, err := client.GetAppBetaAppReviewDetail(ctx, "app-1")
				return err
			},
		},
		{
			name: "GetBetaAppLocalizationApp",
			path: "/v1/betaAppLocalizations/loc-1/app",
			call: func(client *Client) error {
				_, err := client.GetBetaAppLocalizationApp(ctx, "loc-1")
				return err
			},
		},
		{
			name: "GetBetaBuildLocalizationBuild",
			path: "/v1/betaBuildLocalizations/loc-1/build",
			call: func(client *Client) error {
				_, err := client.GetBetaBuildLocalizationBuild(ctx, "loc-1")
				return err
			},
		},
		{
			name: "GetBetaAppReviewDetailApp",
			path: "/v1/betaAppReviewDetails/detail-1/app",
			call: func(client *Client) error {
				_, err := client.GetBetaAppReviewDetailApp(ctx, "detail-1")
				return err
			},
		},
		{
			name: "GetBetaAppReviewSubmissionBuild",
			path: "/v1/betaAppReviewSubmissions/sub-1/build",
			call: func(client *Client) error {
				_, err := client.GetBetaAppReviewSubmissionBuild(ctx, "sub-1")
				return err
			},
		},
		{
			name: "GetBuildBetaDetailBuild",
			path: "/v1/buildBetaDetails/detail-1/build",
			call: func(client *Client) error {
				_, err := client.GetBuildBetaDetailBuild(ctx, "detail-1")
				return err
			},
		},
		{
			name: "GetBetaGroupApp",
			path: "/v1/betaGroups/group-1/app",
			call: func(client *Client) error {
				_, err := client.GetBetaGroupApp(ctx, "group-1")
				return err
			},
		},
		{
			name: "GetBetaGroupBetaRecruitmentCriteria",
			path: "/v1/betaGroups/group-1/betaRecruitmentCriteria",
			call: func(client *Client) error {
				_, err := client.GetBetaGroupBetaRecruitmentCriteria(ctx, "group-1")
				return err
			},
		},
		{
			name: "GetBetaGroupBetaRecruitmentCriterionCompatibleBuildCheck",
			path: "/v1/betaGroups/group-1/betaRecruitmentCriterionCompatibleBuildCheck",
			call: func(client *Client) error {
				_, err := client.GetBetaGroupBetaRecruitmentCriterionCompatibleBuildCheck(ctx, "group-1")
				return err
			},
		},
		{
			name: "GetBuildApp",
			path: "/v1/builds/build-1/app",
			call: func(client *Client) error {
				_, err := client.GetBuildApp(ctx, "build-1")
				return err
			},
		},
		{
			name: "GetBuildBetaAppReviewSubmission",
			path: "/v1/builds/build-1/betaAppReviewSubmission",
			call: func(client *Client) error {
				_, err := client.GetBuildBetaAppReviewSubmission(ctx, "build-1")
				return err
			},
		},
		{
			name: "GetBuildBuildBetaDetail",
			path: "/v1/builds/build-1/buildBetaDetail",
			call: func(client *Client) error {
				_, err := client.GetBuildBuildBetaDetail(ctx, "build-1")
				return err
			},
		},
		{
			name: "GetBuildPreReleaseVersion",
			path: "/v1/builds/build-1/preReleaseVersion",
			call: func(client *Client) error {
				_, err := client.GetBuildPreReleaseVersion(ctx, "build-1")
				return err
			},
		},
		{
			name: "GetPreReleaseVersionApp",
			path: "/v1/preReleaseVersions/pre-1/app",
			call: func(client *Client) error {
				_, err := client.GetPreReleaseVersionApp(ctx, "pre-1")
				return err
			},
		},
		{
			name: "GetBetaCrashLog",
			path: "/v1/betaCrashLogs/log-1",
			call: func(client *Client) error {
				_, err := client.GetBetaCrashLog(ctx, "log-1")
				return err
			},
		},
		{
			name: "GetBetaFeedbackCrashSubmission",
			path: "/v1/betaFeedbackCrashSubmissions/sub-1",
			call: func(client *Client) error {
				_, err := client.GetBetaFeedbackCrashSubmission(ctx, "sub-1")
				return err
			},
		},
		{
			name: "GetBetaFeedbackCrashSubmissionCrashLog",
			path: "/v1/betaFeedbackCrashSubmissions/sub-1/crashLog",
			call: func(client *Client) error {
				_, err := client.GetBetaFeedbackCrashSubmissionCrashLog(ctx, "sub-1")
				return err
			},
		},
		{
			name: "GetBetaFeedbackScreenshotSubmission",
			path: "/v1/betaFeedbackScreenshotSubmissions/sub-1",
			call: func(client *Client) error {
				_, err := client.GetBetaFeedbackScreenshotSubmission(ctx, "sub-1")
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

func TestParityRelatedListRequests_WithLimit(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name  string
		path  string
		limit string
		call  func(*Client) error
	}{
		{
			name:  "GetAppBetaAppLocalizations",
			path:  "/v1/apps/app-1/betaAppLocalizations",
			limit: "10",
			call: func(client *Client) error {
				_, err := client.GetAppBetaAppLocalizations(ctx, "app-1", Limit[AppBetaAppLocalizationsOption](10))
				return err
			},
		},
		{
			name:  "GetAppPreReleaseVersions",
			path:  "/v1/apps/app-1/preReleaseVersions",
			limit: "5",
			call: func(client *Client) error {
				_, err := client.GetAppPreReleaseVersions(ctx, "app-1", Limit[AppPreReleaseVersionsOption](5))
				return err
			},
		},
		{
			name:  "GetBetaTesterApps",
			path:  "/v1/betaTesters/tester-1/apps",
			limit: "3",
			call: func(client *Client) error {
				_, err := client.GetBetaTesterApps(ctx, "tester-1", Limit[BetaTesterAppsOption](3))
				return err
			},
		},
		{
			name:  "GetBetaTesterBetaGroups",
			path:  "/v1/betaTesters/tester-1/betaGroups",
			limit: "4",
			call: func(client *Client) error {
				_, err := client.GetBetaTesterBetaGroups(ctx, "tester-1", Limit[BetaTesterBetaGroupsOption](4))
				return err
			},
		},
		{
			name:  "GetBetaTesterBuilds",
			path:  "/v1/betaTesters/tester-1/builds",
			limit: "6",
			call: func(client *Client) error {
				_, err := client.GetBetaTesterBuilds(ctx, "tester-1", Limit[BetaTesterBuildsOption](6))
				return err
			},
		},
		{
			name:  "GetBuildIcons",
			path:  "/v1/builds/build-1/icons",
			limit: "9",
			call: func(client *Client) error {
				_, err := client.GetBuildIcons(ctx, "build-1", Limit[BuildIconsOption](9))
				return err
			},
		},
		{
			name:  "GetPreReleaseVersionBuilds",
			path:  "/v1/preReleaseVersions/pre-1/builds",
			limit: "7",
			call: func(client *Client) error {
				_, err := client.GetPreReleaseVersionBuilds(ctx, "pre-1", Limit[PreReleaseVersionBuildsOption](7))
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

func TestParityRelatedListRequests_UsesNextURL(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name string
		next string
		call func(*Client) error
	}{
		{
			name: "GetAppBetaAppLocalizations",
			next: "https://api.appstoreconnect.apple.com/v1/apps/app-1/betaAppLocalizations?cursor=next",
			call: func(client *Client) error {
				_, err := client.GetAppBetaAppLocalizations(ctx, "app-1", NextURL[AppBetaAppLocalizationsOption]("https://api.appstoreconnect.apple.com/v1/apps/app-1/betaAppLocalizations?cursor=next"))
				return err
			},
		},
		{
			name: "GetAppPreReleaseVersions",
			next: "https://api.appstoreconnect.apple.com/v1/apps/app-1/preReleaseVersions?cursor=next",
			call: func(client *Client) error {
				_, err := client.GetAppPreReleaseVersions(ctx, "app-1", NextURL[AppPreReleaseVersionsOption]("https://api.appstoreconnect.apple.com/v1/apps/app-1/preReleaseVersions?cursor=next"))
				return err
			},
		},
		{
			name: "GetBetaTesterApps",
			next: "https://api.appstoreconnect.apple.com/v1/betaTesters/tester-1/apps?cursor=next",
			call: func(client *Client) error {
				_, err := client.GetBetaTesterApps(ctx, "tester-1", NextURL[BetaTesterAppsOption]("https://api.appstoreconnect.apple.com/v1/betaTesters/tester-1/apps?cursor=next"))
				return err
			},
		},
		{
			name: "GetBetaTesterBetaGroups",
			next: "https://api.appstoreconnect.apple.com/v1/betaTesters/tester-1/betaGroups?cursor=next",
			call: func(client *Client) error {
				_, err := client.GetBetaTesterBetaGroups(ctx, "tester-1", NextURL[BetaTesterBetaGroupsOption]("https://api.appstoreconnect.apple.com/v1/betaTesters/tester-1/betaGroups?cursor=next"))
				return err
			},
		},
		{
			name: "GetBetaTesterBuilds",
			next: "https://api.appstoreconnect.apple.com/v1/betaTesters/tester-1/builds?cursor=next",
			call: func(client *Client) error {
				_, err := client.GetBetaTesterBuilds(ctx, "tester-1", NextURL[BetaTesterBuildsOption]("https://api.appstoreconnect.apple.com/v1/betaTesters/tester-1/builds?cursor=next"))
				return err
			},
		},
		{
			name: "GetBuildIcons",
			next: "https://api.appstoreconnect.apple.com/v1/builds/build-1/icons?cursor=next",
			call: func(client *Client) error {
				_, err := client.GetBuildIcons(ctx, "build-1", NextURL[BuildIconsOption]("https://api.appstoreconnect.apple.com/v1/builds/build-1/icons?cursor=next"))
				return err
			},
		},
		{
			name: "GetPreReleaseVersionBuilds",
			next: "https://api.appstoreconnect.apple.com/v1/preReleaseVersions/pre-1/builds?cursor=next",
			call: func(client *Client) error {
				_, err := client.GetPreReleaseVersionBuilds(ctx, "pre-1", NextURL[PreReleaseVersionBuildsOption]("https://api.appstoreconnect.apple.com/v1/preReleaseVersions/pre-1/builds?cursor=next"))
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

func TestGetAppBetaTesterUsagesMetrics_OmitsAppFilter(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"value":"1"}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/app-1/metrics/betaTesterUsages" {
			t.Fatalf("expected path /v1/apps/app-1/metrics/betaTesterUsages, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("period") != "P7D" {
			t.Fatalf("expected period=P7D, got %q", values.Get("period"))
		}
		if values.Get("limit") != "5" {
			t.Fatalf("expected limit=5, got %q", values.Get("limit"))
		}
		if values.Get("filter[apps]") != "" {
			t.Fatalf("expected filter[apps] to be empty, got %q", values.Get("filter[apps]"))
		}
		assertAuthorized(t, req)
	}, response)

	resp, err := client.GetAppBetaTesterUsagesMetrics(
		context.Background(),
		"app-1",
		WithBetaTesterUsagesPeriod("P7D"),
		Limit[BetaTesterUsagesOption](5),
		WithBetaTesterUsagesAppID("ignored"),
	)
	if err != nil {
		t.Fatalf("GetAppBetaTesterUsagesMetrics() error: %v", err)
	}
	if len(resp.Data) == 0 {
		t.Fatalf("expected response data")
	}
}

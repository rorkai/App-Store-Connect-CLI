package asc

import (
	"context"
	"net/http"
	"testing"
)

func runAppRelationshipLinkagesWithLimitTest(t *testing.T, expectedPath, expectedLimit string, call func(*Client) error) {
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

func runAppRelationshipLinkageTest(t *testing.T, expectedPath string, call func(*Client) error) {
	t.Helper()

	response := jsonResponse(http.StatusOK, `{"data":{"type":"apps","id":"1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != expectedPath {
			t.Fatalf("expected path %s, got %s", expectedPath, req.URL.Path)
		}
		if len(req.URL.Query()) != 0 {
			t.Fatalf("expected no query parameters, got %q", req.URL.RawQuery)
		}
		assertAuthorized(t, req)
	}, response)

	if err := call(client); err != nil {
		t.Fatalf("request error: %v", err)
	}
}

func TestGetAppRelationshipLinkagesEndpoints_WithLimit(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name string
		path string
		call func(*Client) error
	}{
		{
			name: "GetAppAccessibilityDeclarationsRelationships",
			path: "/v1/apps/app-1/relationships/accessibilityDeclarations",
			call: func(client *Client) error {
				_, err := client.GetAppAccessibilityDeclarationsRelationships(ctx, "app-1", WithLinkagesLimit(5))
				return err
			},
		},
		{
			name: "GetAppAnalyticsReportRequestsRelationships",
			path: "/v1/apps/app-1/relationships/analyticsReportRequests",
			call: func(client *Client) error {
				_, err := client.GetAppAnalyticsReportRequestsRelationships(ctx, "app-1", WithLinkagesLimit(5))
				return err
			},
		},
		{
			name: "GetAppAndroidToIosAppMappingDetailsRelationships",
			path: "/v1/apps/app-1/relationships/androidToIosAppMappingDetails",
			call: func(client *Client) error {
				_, err := client.GetAppAndroidToIosAppMappingDetailsRelationships(ctx, "app-1", WithLinkagesLimit(5))
				return err
			},
		},
		{
			name: "GetAppClipsRelationships",
			path: "/v1/apps/app-1/relationships/appClips",
			call: func(client *Client) error {
				_, err := client.GetAppClipsRelationships(ctx, "app-1", WithLinkagesLimit(5))
				return err
			},
		},
		{
			name: "GetAppCustomProductPagesRelationships",
			path: "/v1/apps/app-1/relationships/appCustomProductPages",
			call: func(client *Client) error {
				_, err := client.GetAppCustomProductPagesRelationships(ctx, "app-1", WithLinkagesLimit(5))
				return err
			},
		},
		{
			name: "GetAppEventsRelationships",
			path: "/v1/apps/app-1/relationships/appEvents",
			call: func(client *Client) error {
				_, err := client.GetAppEventsRelationships(ctx, "app-1", WithLinkagesLimit(5))
				return err
			},
		},
		{
			name: "GetAppInfosRelationships",
			path: "/v1/apps/app-1/relationships/appInfos",
			call: func(client *Client) error {
				_, err := client.GetAppInfosRelationships(ctx, "app-1", WithLinkagesLimit(5))
				return err
			},
		},
		{
			name: "GetAppPricePointsRelationships",
			path: "/v1/apps/app-1/relationships/appPricePoints",
			call: func(client *Client) error {
				_, err := client.GetAppPricePointsRelationships(ctx, "app-1", WithLinkagesLimit(5))
				return err
			},
		},
		{
			name: "GetAppAppStoreVersionExperimentsV2Relationships",
			path: "/v1/apps/app-1/relationships/appStoreVersionExperimentsV2",
			call: func(client *Client) error {
				_, err := client.GetAppAppStoreVersionExperimentsV2Relationships(ctx, "app-1", WithLinkagesLimit(5))
				return err
			},
		},
		{
			name: "GetAppStoreVersionsRelationships",
			path: "/v1/apps/app-1/relationships/appStoreVersions",
			call: func(client *Client) error {
				_, err := client.GetAppStoreVersionsRelationships(ctx, "app-1", WithLinkagesLimit(5))
				return err
			},
		},
		{
			name: "GetAppBackgroundAssetsRelationships",
			path: "/v1/apps/app-1/relationships/backgroundAssets",
			call: func(client *Client) error {
				_, err := client.GetAppBackgroundAssetsRelationships(ctx, "app-1", WithLinkagesLimit(5))
				return err
			},
		},
		{
			name: "GetAppBetaAppLocalizationsRelationships",
			path: "/v1/apps/app-1/relationships/betaAppLocalizations",
			call: func(client *Client) error {
				_, err := client.GetAppBetaAppLocalizationsRelationships(ctx, "app-1", WithLinkagesLimit(5))
				return err
			},
		},
		{
			name: "GetAppBetaFeedbackCrashSubmissionsRelationships",
			path: "/v1/apps/app-1/relationships/betaFeedbackCrashSubmissions",
			call: func(client *Client) error {
				_, err := client.GetAppBetaFeedbackCrashSubmissionsRelationships(ctx, "app-1", WithLinkagesLimit(5))
				return err
			},
		},
		{
			name: "GetAppBetaFeedbackScreenshotSubmissionsRelationships",
			path: "/v1/apps/app-1/relationships/betaFeedbackScreenshotSubmissions",
			call: func(client *Client) error {
				_, err := client.GetAppBetaFeedbackScreenshotSubmissionsRelationships(ctx, "app-1", WithLinkagesLimit(5))
				return err
			},
		},
		{
			name: "GetAppBetaGroupsRelationships",
			path: "/v1/apps/app-1/relationships/betaGroups",
			call: func(client *Client) error {
				_, err := client.GetAppBetaGroupsRelationships(ctx, "app-1", WithLinkagesLimit(5))
				return err
			},
		},
		{
			name: "GetAppBuildUploadsRelationships",
			path: "/v1/apps/app-1/relationships/buildUploads",
			call: func(client *Client) error {
				_, err := client.GetAppBuildUploadsRelationships(ctx, "app-1", WithLinkagesLimit(5))
				return err
			},
		},
		{
			name: "GetAppBuildsRelationships",
			path: "/v1/apps/app-1/relationships/builds",
			call: func(client *Client) error {
				_, err := client.GetAppBuildsRelationships(ctx, "app-1", WithLinkagesLimit(5))
				return err
			},
		},
		{
			name: "GetAppCustomerReviewsRelationships",
			path: "/v1/apps/app-1/relationships/customerReviews",
			call: func(client *Client) error {
				_, err := client.GetAppCustomerReviewsRelationships(ctx, "app-1", WithLinkagesLimit(5))
				return err
			},
		},
		{
			name: "GetAppGameCenterEnabledVersionsRelationships",
			path: "/v1/apps/app-1/relationships/gameCenterEnabledVersions",
			call: func(client *Client) error {
				_, err := client.GetAppGameCenterEnabledVersionsRelationships(ctx, "app-1", WithLinkagesLimit(5))
				return err
			},
		},
		{
			name: "GetAppInAppPurchasesRelationships",
			path: "/v1/apps/app-1/relationships/inAppPurchases",
			call: func(client *Client) error {
				_, err := client.GetAppInAppPurchasesRelationships(ctx, "app-1", WithLinkagesLimit(5))
				return err
			},
		},
		{
			name: "GetAppInAppPurchasesV2Relationships",
			path: "/v1/apps/app-1/relationships/inAppPurchasesV2",
			call: func(client *Client) error {
				_, err := client.GetAppInAppPurchasesV2Relationships(ctx, "app-1", WithLinkagesLimit(5))
				return err
			},
		},
		{
			name: "GetAppPreReleaseVersionsRelationships",
			path: "/v1/apps/app-1/relationships/preReleaseVersions",
			call: func(client *Client) error {
				_, err := client.GetAppPreReleaseVersionsRelationships(ctx, "app-1", WithLinkagesLimit(5))
				return err
			},
		},
		{
			name: "GetAppReviewSubmissionsRelationships",
			path: "/v1/apps/app-1/relationships/reviewSubmissions",
			call: func(client *Client) error {
				_, err := client.GetAppReviewSubmissionsRelationships(ctx, "app-1", WithLinkagesLimit(5))
				return err
			},
		},
		{
			name: "GetAppSubscriptionGroupsRelationships",
			path: "/v1/apps/app-1/relationships/subscriptionGroups",
			call: func(client *Client) error {
				_, err := client.GetAppSubscriptionGroupsRelationships(ctx, "app-1", WithLinkagesLimit(5))
				return err
			},
		},
		{
			name: "GetAppWebhooksRelationships",
			path: "/v1/apps/app-1/relationships/webhooks",
			call: func(client *Client) error {
				_, err := client.GetAppWebhooksRelationships(ctx, "app-1", WithLinkagesLimit(5))
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runAppRelationshipLinkagesWithLimitTest(t, tt.path, "5", tt.call)
		})
	}
}

func TestGetAppRelationshipLinkageEndpoints(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name string
		path string
		call func(*Client) error
	}{
		{
			name: "GetAppAvailabilityV2Relationship",
			path: "/v1/apps/app-1/relationships/appAvailabilityV2",
			call: func(client *Client) error {
				_, err := client.GetAppAvailabilityV2Relationship(ctx, "app-1")
				return err
			},
		},
		{
			name: "GetAppPriceScheduleRelationship",
			path: "/v1/apps/app-1/relationships/appPriceSchedule",
			call: func(client *Client) error {
				_, err := client.GetAppPriceScheduleRelationship(ctx, "app-1")
				return err
			},
		},
		{
			name: "GetAppBetaAppReviewDetailRelationship",
			path: "/v1/apps/app-1/relationships/betaAppReviewDetail",
			call: func(client *Client) error {
				_, err := client.GetAppBetaAppReviewDetailRelationship(ctx, "app-1")
				return err
			},
		},
		{
			name: "GetAppCiProductRelationship",
			path: "/v1/apps/app-1/relationships/ciProduct",
			call: func(client *Client) error {
				_, err := client.GetAppCiProductRelationship(ctx, "app-1")
				return err
			},
		},
		{
			name: "GetAppEndUserLicenseAgreementRelationship",
			path: "/v1/apps/app-1/relationships/endUserLicenseAgreement",
			call: func(client *Client) error {
				_, err := client.GetAppEndUserLicenseAgreementRelationship(ctx, "app-1")
				return err
			},
		},
		{
			name: "GetAppGameCenterDetailRelationship",
			path: "/v1/apps/app-1/relationships/gameCenterDetail",
			call: func(client *Client) error {
				_, err := client.GetAppGameCenterDetailRelationship(ctx, "app-1")
				return err
			},
		},
		{
			name: "GetAppSubscriptionGracePeriodRelationship",
			path: "/v1/apps/app-1/relationships/subscriptionGracePeriod",
			call: func(client *Client) error {
				_, err := client.GetAppSubscriptionGracePeriodRelationship(ctx, "app-1")
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runAppRelationshipLinkageTest(t, tt.path, tt.call)
		})
	}
}

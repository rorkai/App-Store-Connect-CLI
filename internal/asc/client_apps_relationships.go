package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// AppAppAvailabilityV2LinkageResponse is the response for app availability v2 relationship.
type AppAppAvailabilityV2LinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}

// AppAppPriceScheduleLinkageResponse is the response for app price schedule relationship.
type AppAppPriceScheduleLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}

// AppBetaAppReviewDetailLinkageResponse is the response for beta app review detail relationship.
type AppBetaAppReviewDetailLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}

// AppCiProductLinkageResponse is the response for CI product relationship.
type AppCiProductLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}

// AppEndUserLicenseAgreementLinkageResponse is the response for end user license agreement relationship.
type AppEndUserLicenseAgreementLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}

// AppGameCenterDetailLinkageResponse is the response for Game Center detail relationship.
type AppGameCenterDetailLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}

// AppSubscriptionGracePeriodLinkageResponse is the response for subscription grace period relationship.
type AppSubscriptionGracePeriodLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}

// GetAppAccessibilityDeclarationsRelationships retrieves accessibility declaration linkages for an app.
func (c *Client) GetAppAccessibilityDeclarationsRelationships(ctx context.Context, appID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getAppLinkages(ctx, appID, "accessibilityDeclarations", opts...)
}

// GetAppAnalyticsReportRequestsRelationships retrieves analytics report request linkages for an app.
func (c *Client) GetAppAnalyticsReportRequestsRelationships(ctx context.Context, appID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getAppLinkages(ctx, appID, "analyticsReportRequests", opts...)
}

// GetAppAndroidToIosAppMappingDetailsRelationships retrieves Android-to-iOS mapping detail linkages for an app.
func (c *Client) GetAppAndroidToIosAppMappingDetailsRelationships(ctx context.Context, appID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getAppLinkages(ctx, appID, "androidToIosAppMappingDetails", opts...)
}

// GetAppAvailabilityV2Relationship retrieves the app availability v2 linkage for an app.
func (c *Client) GetAppAvailabilityV2Relationship(ctx context.Context, appID string) (*AppAppAvailabilityV2LinkageResponse, error) {
	var response AppAppAvailabilityV2LinkageResponse
	if err := c.getAppLinkage(ctx, appID, "appAvailabilityV2", &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// GetAppClipsRelationships retrieves App Clip linkages for an app.
func (c *Client) GetAppClipsRelationships(ctx context.Context, appID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getAppLinkages(ctx, appID, "appClips", opts...)
}

// GetAppCustomProductPagesRelationships retrieves custom product page linkages for an app.
func (c *Client) GetAppCustomProductPagesRelationships(ctx context.Context, appID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getAppLinkages(ctx, appID, "appCustomProductPages", opts...)
}

// GetAppEventsRelationships retrieves app event linkages for an app.
func (c *Client) GetAppEventsRelationships(ctx context.Context, appID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getAppLinkages(ctx, appID, "appEvents", opts...)
}

// GetAppInfosRelationships retrieves app info linkages for an app.
func (c *Client) GetAppInfosRelationships(ctx context.Context, appID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getAppLinkages(ctx, appID, "appInfos", opts...)
}

// GetAppPricePointsRelationships retrieves app price point linkages for an app.
func (c *Client) GetAppPricePointsRelationships(ctx context.Context, appID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getAppLinkages(ctx, appID, "appPricePoints", opts...)
}

// GetAppPriceScheduleRelationship retrieves the app price schedule linkage for an app.
func (c *Client) GetAppPriceScheduleRelationship(ctx context.Context, appID string) (*AppAppPriceScheduleLinkageResponse, error) {
	var response AppAppPriceScheduleLinkageResponse
	if err := c.getAppLinkage(ctx, appID, "appPriceSchedule", &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// GetAppAppStoreVersionExperimentsV2Relationships retrieves v2 app store version experiment linkages for an app.
//
// Note: The method name includes "AppAppStore..." to avoid colliding with the existing
// GetAppStoreVersionExperimentsV2Relationships method (for /v1/appStoreVersions/...).
func (c *Client) GetAppAppStoreVersionExperimentsV2Relationships(ctx context.Context, appID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getAppLinkages(ctx, appID, "appStoreVersionExperimentsV2", opts...)
}

// GetAppStoreVersionsRelationships retrieves app store version linkages for an app.
func (c *Client) GetAppStoreVersionsRelationships(ctx context.Context, appID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getAppLinkages(ctx, appID, "appStoreVersions", opts...)
}

// GetAppBackgroundAssetsRelationships retrieves background asset linkages for an app.
func (c *Client) GetAppBackgroundAssetsRelationships(ctx context.Context, appID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getAppLinkages(ctx, appID, "backgroundAssets", opts...)
}

// GetAppBetaAppLocalizationsRelationships retrieves beta app localization linkages for an app.
func (c *Client) GetAppBetaAppLocalizationsRelationships(ctx context.Context, appID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getAppLinkages(ctx, appID, "betaAppLocalizations", opts...)
}

// GetAppBetaAppReviewDetailRelationship retrieves the beta app review detail linkage for an app.
func (c *Client) GetAppBetaAppReviewDetailRelationship(ctx context.Context, appID string) (*AppBetaAppReviewDetailLinkageResponse, error) {
	var response AppBetaAppReviewDetailLinkageResponse
	if err := c.getAppLinkage(ctx, appID, "betaAppReviewDetail", &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// GetAppBetaFeedbackCrashSubmissionsRelationships retrieves beta feedback crash submission linkages for an app.
func (c *Client) GetAppBetaFeedbackCrashSubmissionsRelationships(ctx context.Context, appID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getAppLinkages(ctx, appID, "betaFeedbackCrashSubmissions", opts...)
}

// GetAppBetaFeedbackScreenshotSubmissionsRelationships retrieves beta feedback screenshot submission linkages for an app.
func (c *Client) GetAppBetaFeedbackScreenshotSubmissionsRelationships(ctx context.Context, appID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getAppLinkages(ctx, appID, "betaFeedbackScreenshotSubmissions", opts...)
}

// GetAppBetaGroupsRelationships retrieves beta group linkages for an app.
func (c *Client) GetAppBetaGroupsRelationships(ctx context.Context, appID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getAppLinkages(ctx, appID, "betaGroups", opts...)
}

// GetAppBuildUploadsRelationships retrieves build upload linkages for an app.
func (c *Client) GetAppBuildUploadsRelationships(ctx context.Context, appID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getAppLinkages(ctx, appID, "buildUploads", opts...)
}

// GetAppBuildsRelationships retrieves build linkages for an app.
func (c *Client) GetAppBuildsRelationships(ctx context.Context, appID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getAppLinkages(ctx, appID, "builds", opts...)
}

// GetAppCiProductRelationship retrieves the CI product linkage for an app.
func (c *Client) GetAppCiProductRelationship(ctx context.Context, appID string) (*AppCiProductLinkageResponse, error) {
	var response AppCiProductLinkageResponse
	if err := c.getAppLinkage(ctx, appID, "ciProduct", &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// GetAppCustomerReviewsRelationships retrieves customer review linkages for an app.
func (c *Client) GetAppCustomerReviewsRelationships(ctx context.Context, appID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getAppLinkages(ctx, appID, "customerReviews", opts...)
}

// GetAppEndUserLicenseAgreementRelationship retrieves the end user license agreement linkage for an app.
func (c *Client) GetAppEndUserLicenseAgreementRelationship(ctx context.Context, appID string) (*AppEndUserLicenseAgreementLinkageResponse, error) {
	var response AppEndUserLicenseAgreementLinkageResponse
	if err := c.getAppLinkage(ctx, appID, "endUserLicenseAgreement", &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// GetAppGameCenterDetailRelationship retrieves the Game Center detail linkage for an app.
func (c *Client) GetAppGameCenterDetailRelationship(ctx context.Context, appID string) (*AppGameCenterDetailLinkageResponse, error) {
	var response AppGameCenterDetailLinkageResponse
	if err := c.getAppLinkage(ctx, appID, "gameCenterDetail", &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// GetAppGameCenterEnabledVersionsRelationships retrieves Game Center enabled version linkages for an app.
func (c *Client) GetAppGameCenterEnabledVersionsRelationships(ctx context.Context, appID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getAppLinkages(ctx, appID, "gameCenterEnabledVersions", opts...)
}

// GetAppInAppPurchasesRelationships retrieves in-app purchase linkages for an app (v1).
func (c *Client) GetAppInAppPurchasesRelationships(ctx context.Context, appID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getAppLinkages(ctx, appID, "inAppPurchases", opts...)
}

// GetAppInAppPurchasesV2Relationships retrieves in-app purchase linkages for an app (v2).
func (c *Client) GetAppInAppPurchasesV2Relationships(ctx context.Context, appID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getAppLinkages(ctx, appID, "inAppPurchasesV2", opts...)
}

// GetAppPreReleaseVersionsRelationships retrieves pre-release version linkages for an app.
func (c *Client) GetAppPreReleaseVersionsRelationships(ctx context.Context, appID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getAppLinkages(ctx, appID, "preReleaseVersions", opts...)
}

// GetAppReviewSubmissionsRelationships retrieves review submission linkages for an app.
func (c *Client) GetAppReviewSubmissionsRelationships(ctx context.Context, appID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getAppLinkages(ctx, appID, "reviewSubmissions", opts...)
}

// GetAppSubscriptionGracePeriodRelationship retrieves the subscription grace period linkage for an app.
func (c *Client) GetAppSubscriptionGracePeriodRelationship(ctx context.Context, appID string) (*AppSubscriptionGracePeriodLinkageResponse, error) {
	var response AppSubscriptionGracePeriodLinkageResponse
	if err := c.getAppLinkage(ctx, appID, "subscriptionGracePeriod", &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// GetAppSubscriptionGroupsRelationships retrieves subscription group linkages for an app.
func (c *Client) GetAppSubscriptionGroupsRelationships(ctx context.Context, appID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getAppLinkages(ctx, appID, "subscriptionGroups", opts...)
}

// GetAppWebhooksRelationships retrieves webhook linkages for an app.
func (c *Client) GetAppWebhooksRelationships(ctx context.Context, appID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getAppLinkages(ctx, appID, "webhooks", opts...)
}

func (c *Client) getAppLinkages(ctx context.Context, appID, relationship string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	query := &linkagesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	appID = strings.TrimSpace(appID)
	if query.nextURL == "" && appID == "" {
		return nil, fmt.Errorf("appID is required")
	}

	path := fmt.Sprintf("/v1/apps/%s/relationships/%s", appID, relationship)
	if query.nextURL != "" {
		// Validate nextURL to prevent credential exfiltration.
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("appRelationships: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildLinkagesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response LinkagesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse %s relationship response: %w", relationship, err)
	}

	return &response, nil
}

func (c *Client) getAppLinkage(ctx context.Context, appID, relationship string, out any) error {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return fmt.Errorf("appID is required")
	}

	path := fmt.Sprintf("/v1/apps/%s/relationships/%s", appID, relationship)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("failed to parse %s relationship response: %w", relationship, err)
	}

	return nil
}

package asc

import (
	"context"
	"fmt"
	"strings"
)

// ContentRightsDeclaration represents an app's content rights declaration.
type ContentRightsDeclaration string

const (
	ContentRightsDeclarationDoesNotUseThirdPartyContent ContentRightsDeclaration = "DOES_NOT_USE_THIRD_PARTY_CONTENT"
	ContentRightsDeclarationUsesThirdPartyContent       ContentRightsDeclaration = "USES_THIRD_PARTY_CONTENT"
)

// SubscriptionStatusURLVersion is the App Store Server Notifications payload version.
type SubscriptionStatusURLVersion string

const (
	SubscriptionStatusURLVersionV1 SubscriptionStatusURLVersion = "V1"
	SubscriptionStatusURLVersionV2 SubscriptionStatusURLVersion = "V2"
)

// AppAttributes describes an app resource.
type AppAttributes struct {
	Name                                   string                        `json:"name"`
	BundleID                               string                        `json:"bundleId"`
	SKU                                    string                        `json:"sku"`
	PrimaryLocale                          string                        `json:"primaryLocale,omitempty"`
	ContentRightsDeclaration               *ContentRightsDeclaration     `json:"contentRightsDeclaration,omitempty"`
	AccessibilityURL                       *string                       `json:"accessibilityUrl,omitempty"`
	SubscriptionStatusURL                  *string                       `json:"subscriptionStatusUrl,omitempty"`
	SubscriptionStatusURLVersion           *SubscriptionStatusURLVersion `json:"subscriptionStatusUrlVersion,omitempty"`
	SubscriptionStatusURLForSandbox        *string                       `json:"subscriptionStatusUrlForSandbox,omitempty"`
	SubscriptionStatusURLVersionForSandbox *SubscriptionStatusURLVersion `json:"subscriptionStatusUrlVersionForSandbox,omitempty"`

	api apiAttributes
}

// AppUpdateAttributes describes fields for updating an app.
type AppUpdateAttributes struct {
	BundleID                               *string                       `json:"bundleId,omitempty"`
	PrimaryLocale                          *string                       `json:"primaryLocale,omitempty"`
	ContentRightsDeclaration               *ContentRightsDeclaration     `json:"contentRightsDeclaration,omitempty"`
	AccessibilityURL                       *string                       `json:"accessibilityUrl,omitempty"`
	SubscriptionStatusURL                  *string                       `json:"subscriptionStatusUrl,omitempty"`
	SubscriptionStatusURLVersion           *SubscriptionStatusURLVersion `json:"subscriptionStatusUrlVersion,omitempty"`
	SubscriptionStatusURLForSandbox        *string                       `json:"subscriptionStatusUrlForSandbox,omitempty"`
	SubscriptionStatusURLVersionForSandbox *SubscriptionStatusURLVersion `json:"subscriptionStatusUrlVersionForSandbox,omitempty"`
	StreamlinedPurchasingEnabled           *bool                         `json:"streamlinedPurchasingEnabled,omitempty"`
}

// AppUpdateData is the data portion of an app update request.
type AppUpdateData struct {
	Type       ResourceType         `json:"type"`
	ID         string               `json:"id"`
	Attributes *AppUpdateAttributes `json:"attributes,omitempty"`
}

// AppUpdateRequest is a request to update an app.
type AppUpdateRequest struct {
	Data AppUpdateData `json:"data"`
}

// AppsResponse is the response from apps endpoint.
type AppsResponse = Response[AppAttributes]

// AppResponse is the response from app detail endpoint.
type AppResponse = SingleResponse[AppAttributes]

// GetApps retrieves the list of apps
func (c *Client) GetApps(ctx context.Context, opts ...AppsOption) (*AppsResponse, error) {
	query := applyOptions(opts)
	return getJSON[AppsResponse](ctx, c, readRequest{
		Path:        "/v1/apps",
		Query:       buildAppsQuery(query),
		Next:        query.nextURL,
		NextContext: "apps",
	})
}

// GetApp retrieves a single app by ID.
func (c *Client) GetApp(ctx context.Context, appID string) (*AppResponse, error) {
	return c.GetAppWithOptions(ctx, appID)
}

// GetAppWithOptions retrieves a single app by ID with sparse fields and includes.
func (c *Client) GetAppWithOptions(ctx context.Context, appID string, opts ...AppOption) (*AppResponse, error) {
	query := applyOptions(opts)

	appID = strings.TrimSpace(appID)
	return getJSON[AppResponse](ctx, c, readRequest{
		Path:  fmt.Sprintf("/v1/apps/%s", appID),
		Query: buildAppQuery(query),
	})
}

// GetAppBetaAppReviewDetail retrieves beta app review detail for an app.
func (c *Client) GetAppBetaAppReviewDetail(ctx context.Context, appID string) (*BetaAppReviewDetailResponse, error) {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return nil, fmt.Errorf("appID is required")
	}

	return getJSON[BetaAppReviewDetailResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/apps/%s/betaAppReviewDetail", appID)})
}

// GetAppSubscriptionGracePeriod retrieves the subscription grace period for an app.
func (c *Client) GetAppSubscriptionGracePeriod(ctx context.Context, appID string) (*SubscriptionGracePeriodResponse, error) {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return nil, fmt.Errorf("app ID is required")
	}

	return getJSON[SubscriptionGracePeriodResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/apps/%s/subscriptionGracePeriod", appID)})
}

// GetAppBetaTesterUsagesMetrics retrieves beta tester usage metrics for an app.
func (c *Client) GetAppBetaTesterUsagesMetrics(ctx context.Context, appID string, opts ...BetaTesterUsagesOption) (*BetaTesterUsagesResponse, error) {
	query := &betaTesterUsagesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	appID = strings.TrimSpace(appID)
	if query.nextURL == "" && appID == "" {
		return nil, fmt.Errorf("appID is required")
	}

	query.appID = ""
	path := fmt.Sprintf("/v1/apps/%s/metrics/betaTesterUsages", appID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("betaTesterUsages: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildBetaTesterUsagesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	return &BetaTesterUsagesResponse{Data: data}, nil
}

// UpdateApp updates an app by ID.
func (c *Client) UpdateApp(ctx context.Context, appID string, attrs AppUpdateAttributes) (*AppResponse, error) {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return nil, fmt.Errorf("app ID is required")
	}

	payload := AppUpdateRequest{
		Data: AppUpdateData{
			Type: ResourceTypeApps,
			ID:   appID,
		},
	}
	if attrs != (AppUpdateAttributes{}) {
		payload.Data.Attributes = &attrs
	}

	return sendJSON[AppResponse](ctx, c, writeRequest{
		Method:  "PATCH",
		Path:    fmt.Sprintf("/v1/apps/%s", appID),
		Payload: payload,
	})
}

// GetAppSearchKeywords retrieves search keywords for an app.
func (c *Client) GetAppSearchKeywords(ctx context.Context, appID string, opts ...AppSearchKeywordsOption) (*AppKeywordsResponse, error) {
	query := applyOptions(opts)

	appID = strings.TrimSpace(appID)
	if query.nextURL == "" && appID == "" {
		return nil, fmt.Errorf("appID is required")
	}

	return getJSON[AppKeywordsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/apps/%s/searchKeywords", appID),
		Query:       buildAppSearchKeywordsQuery(query),
		Next:        query.nextURL,
		NextContext: "searchKeywords",
	})
}

// GetAppCiProduct retrieves the CI product for an app.
func (c *Client) GetAppCiProduct(ctx context.Context, appID string) (*CiProductResponse, error) {
	appID = strings.TrimSpace(appID)
	return getJSON[CiProductResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/apps/%s/ciProduct", appID)})
}

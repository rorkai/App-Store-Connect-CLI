package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// ContentRightsDeclaration represents an app's content rights declaration.
type ContentRightsDeclaration string

const (
	ContentRightsDeclarationDoesNotUseThirdPartyContent ContentRightsDeclaration = "DOES_NOT_USE_THIRD_PARTY_CONTENT"
	ContentRightsDeclarationUsesThirdPartyContent       ContentRightsDeclaration = "USES_THIRD_PARTY_CONTENT"
)

// AppAttributes describes an app resource.
type AppAttributes struct {
	Name                     string                    `json:"name"`
	BundleID                 string                    `json:"bundleId"`
	SKU                      string                    `json:"sku"`
	PrimaryLocale            string                    `json:"primaryLocale,omitempty"`
	ContentRightsDeclaration *ContentRightsDeclaration `json:"contentRightsDeclaration,omitempty"`
}

// AppUpdateAttributes describes fields for updating an app.
type AppUpdateAttributes struct {
	BundleID                 *string                   `json:"bundleId,omitempty"`
	PrimaryLocale            *string                   `json:"primaryLocale,omitempty"`
	ContentRightsDeclaration *ContentRightsDeclaration `json:"contentRightsDeclaration,omitempty"`
}

// AppCreateAttributes describes attributes for creating an app.
type AppCreateAttributes struct {
	Name          string `json:"name"`
	BundleID      string `json:"bundleId"`
	SKU           string `json:"sku"`
	PrimaryLocale string `json:"primaryLocale,omitempty"`
}

// AppCreateData is the data portion of an app create request.
type AppCreateData struct {
	Type       ResourceType         `json:"type"`
	Attributes *AppCreateAttributes `json:"attributes,omitempty"`
}

// AppCreateRequest is a request to create an app.
type AppCreateRequest struct {
	Data AppCreateData `json:"data"`
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
	query := &appsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := "/v1/apps"
	if query.nextURL != "" {
		// Validate nextURL to prevent credential exfiltration
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("apps: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildAppsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetApp retrieves a single app by ID.
func (c *Client) GetApp(ctx context.Context, appID string) (*AppResponse, error) {
	appID = strings.TrimSpace(appID)
	path := fmt.Sprintf("/v1/apps/%s", appID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppBetaAppLocalizations retrieves beta app localizations for an app.
func (c *Client) GetAppBetaAppLocalizations(ctx context.Context, appID string, opts ...AppBetaAppLocalizationsOption) (*BetaAppLocalizationsResponse, error) {
	query := &listQuery{}
	for _, opt := range opts {
		opt(query)
	}

	appID = strings.TrimSpace(appID)
	if query.nextURL == "" && appID == "" {
		return nil, fmt.Errorf("appID is required")
	}

	path := fmt.Sprintf("/v1/apps/%s/betaAppLocalizations", appID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("appBetaAppLocalizations: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildListQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response BetaAppLocalizationsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppBetaAppReviewDetail retrieves beta app review detail for an app.
func (c *Client) GetAppBetaAppReviewDetail(ctx context.Context, appID string) (*BetaAppReviewDetailResponse, error) {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return nil, fmt.Errorf("appID is required")
	}

	path := fmt.Sprintf("/v1/apps/%s/betaAppReviewDetail", appID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response BetaAppReviewDetailResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppSubscriptionGracePeriod retrieves the subscription grace period for an app.
func (c *Client) GetAppSubscriptionGracePeriod(ctx context.Context, appID string) (*SubscriptionGracePeriodResponse, error) {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return nil, fmt.Errorf("app ID is required")
	}

	path := fmt.Sprintf("/v1/apps/%s/subscriptionGracePeriod", appID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response SubscriptionGracePeriodResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
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

// GetAppPreReleaseVersions retrieves pre-release versions for an app.
func (c *Client) GetAppPreReleaseVersions(ctx context.Context, appID string, opts ...AppPreReleaseVersionsOption) (*PreReleaseVersionsResponse, error) {
	query := &listQuery{}
	for _, opt := range opts {
		opt(query)
	}

	appID = strings.TrimSpace(appID)
	if query.nextURL == "" && appID == "" {
		return nil, fmt.Errorf("appID is required")
	}

	path := fmt.Sprintf("/v1/apps/%s/preReleaseVersions", appID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("appPreReleaseVersions: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildListQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response PreReleaseVersionsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
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
	if attrs.BundleID != nil || attrs.PrimaryLocale != nil || attrs.ContentRightsDeclaration != nil {
		payload.Data.Attributes = &attrs
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "PATCH", fmt.Sprintf("/v1/apps/%s", appID), body)
	if err != nil {
		return nil, err
	}

	var response AppResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateApp creates a new app in App Store Connect.
func (c *Client) CreateApp(ctx context.Context, attrs AppCreateAttributes) (*AppResponse, error) {
	if strings.TrimSpace(attrs.Name) == "" {
		return nil, fmt.Errorf("name is required")
	}
	if strings.TrimSpace(attrs.BundleID) == "" {
		return nil, fmt.Errorf("bundleId is required")
	}
	if strings.TrimSpace(attrs.SKU) == "" {
		return nil, fmt.Errorf("sku is required")
	}

	payload := AppCreateRequest{
		Data: AppCreateData{
			Type:       ResourceTypeApps,
			Attributes: &attrs,
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "POST", "/v1/apps", body)
	if err != nil {
		return nil, err
	}

	var response AppResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppSearchKeywords retrieves search keywords for an app.
func (c *Client) GetAppSearchKeywords(ctx context.Context, appID string, opts ...AppSearchKeywordsOption) (*AppKeywordsResponse, error) {
	query := &appSearchKeywordsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	appID = strings.TrimSpace(appID)
	if query.nextURL == "" && appID == "" {
		return nil, fmt.Errorf("appID is required")
	}

	path := fmt.Sprintf("/v1/apps/%s/searchKeywords", appID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("searchKeywords: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildAppSearchKeywordsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppKeywordsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// SetAppSearchKeywords replaces the search keywords for an app.
func (c *Client) SetAppSearchKeywords(ctx context.Context, appID string, keywords []string) error {
	appID = strings.TrimSpace(appID)
	keywords = normalizeList(keywords)
	if appID == "" {
		return fmt.Errorf("appID is required")
	}
	if len(keywords) == 0 {
		return fmt.Errorf("keywords are required")
	}

	payload := RelationshipRequest{
		Data: make([]RelationshipData, 0, len(keywords)),
	}
	for _, keyword := range keywords {
		payload.Data = append(payload.Data, RelationshipData{
			Type: ResourceTypeAppKeywords,
			ID:   keyword,
		})
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/v1/apps/%s/relationships/searchKeywords", appID)
	_, err = c.do(ctx, "PATCH", path, body)
	return err
}

// GetAppCiProduct retrieves the CI product for an app.
func (c *Client) GetAppCiProduct(ctx context.Context, appID string) (*CiProductResponse, error) {
	appID = strings.TrimSpace(appID)
	path := fmt.Sprintf("/v1/apps/%s/ciProduct", appID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response CiProductResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

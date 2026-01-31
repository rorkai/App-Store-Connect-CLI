package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// AppAttributes describes an app resource.
type AppAttributes struct {
	Name          string `json:"name"`
	BundleID      string `json:"bundleId"`
	SKU           string `json:"sku"`
	PrimaryLocale string `json:"primaryLocale,omitempty"`
}

// AppUpdateAttributes describes fields for updating an app.
type AppUpdateAttributes struct {
	BundleID      *string `json:"bundleId,omitempty"`
	PrimaryLocale *string `json:"primaryLocale,omitempty"`
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
	if attrs.BundleID != nil || attrs.PrimaryLocale != nil {
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

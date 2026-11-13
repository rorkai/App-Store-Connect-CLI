package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// GetAppStoreVersionLocalizationSearchKeywords retrieves search keywords for a localization.
func (c *Client) GetAppStoreVersionLocalizationSearchKeywords(ctx context.Context, localizationID string) (*AppKeywordsResponse, error) {
	localizationID = strings.TrimSpace(localizationID)
	if localizationID == "" {
		return nil, fmt.Errorf("localizationID is required")
	}

	path := fmt.Sprintf("/v1/appStoreVersionLocalizations/%s/searchKeywords", localizationID)
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

// AddAppStoreVersionLocalizationSearchKeywords adds search keywords to a localization.
func (c *Client) AddAppStoreVersionLocalizationSearchKeywords(ctx context.Context, localizationID string, keywords []string) error {
	localizationID = strings.TrimSpace(localizationID)
	keywords = normalizeList(keywords)
	if localizationID == "" {
		return fmt.Errorf("localizationID is required")
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

	path := fmt.Sprintf("/v1/appStoreVersionLocalizations/%s/relationships/searchKeywords", localizationID)
	_, err = c.do(ctx, "POST", path, body)
	return err
}

// DeleteAppStoreVersionLocalizationSearchKeywords removes search keywords from a localization.
func (c *Client) DeleteAppStoreVersionLocalizationSearchKeywords(ctx context.Context, localizationID string, keywords []string) error {
	localizationID = strings.TrimSpace(localizationID)
	keywords = normalizeList(keywords)
	if localizationID == "" {
		return fmt.Errorf("localizationID is required")
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

	path := fmt.Sprintf("/v1/appStoreVersionLocalizations/%s/relationships/searchKeywords", localizationID)
	_, err = c.do(ctx, "DELETE", path, body)
	return err
}

// GetAppStoreVersionLocalizationPreviewSets retrieves preview sets for a localization.
func (c *Client) GetAppStoreVersionLocalizationPreviewSets(ctx context.Context, localizationID string, opts ...AppStoreVersionLocalizationPreviewSetsOption) (*AppPreviewSetsResponse, error) {
	query := &appStoreVersionLocalizationPreviewSetsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	localizationID = strings.TrimSpace(localizationID)
	if query.nextURL == "" && localizationID == "" {
		return nil, fmt.Errorf("localizationID is required")
	}

	path := fmt.Sprintf("/v1/appStoreVersionLocalizations/%s/appPreviewSets", localizationID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("appPreviewSets: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildAppStoreVersionLocalizationPreviewSetsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppPreviewSetsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppStoreVersionLocalizationPreviewSetsRelationships retrieves preview set relationships.
func (c *Client) GetAppStoreVersionLocalizationPreviewSetsRelationships(ctx context.Context, localizationID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	query := &linkagesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	localizationID = strings.TrimSpace(localizationID)
	if query.nextURL == "" && localizationID == "" {
		return nil, fmt.Errorf("localizationID is required")
	}

	path := fmt.Sprintf("/v1/appStoreVersionLocalizations/%s/relationships/appPreviewSets", localizationID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("appPreviewSetsRelationships: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildLinkagesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response LinkagesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppStoreVersionLocalizationScreenshotSets retrieves screenshot sets for a localization.
func (c *Client) GetAppStoreVersionLocalizationScreenshotSets(ctx context.Context, localizationID string, opts ...AppStoreVersionLocalizationScreenshotSetsOption) (*AppScreenshotSetsResponse, error) {
	query := &appStoreVersionLocalizationScreenshotSetsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	localizationID = strings.TrimSpace(localizationID)
	if query.nextURL == "" && localizationID == "" {
		return nil, fmt.Errorf("localizationID is required")
	}

	path := fmt.Sprintf("/v1/appStoreVersionLocalizations/%s/appScreenshotSets", localizationID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("appScreenshotSets: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildAppStoreVersionLocalizationScreenshotSetsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppScreenshotSetsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppStoreVersionLocalizationScreenshotSetsRelationships retrieves screenshot set relationships.
func (c *Client) GetAppStoreVersionLocalizationScreenshotSetsRelationships(ctx context.Context, localizationID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	query := &linkagesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	localizationID = strings.TrimSpace(localizationID)
	if query.nextURL == "" && localizationID == "" {
		return nil, fmt.Errorf("localizationID is required")
	}

	path := fmt.Sprintf("/v1/appStoreVersionLocalizations/%s/relationships/appScreenshotSets", localizationID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("appScreenshotSetsRelationships: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildLinkagesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response LinkagesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

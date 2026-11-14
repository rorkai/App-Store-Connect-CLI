package asc

import (
	"context"
	"fmt"
	"strings"
)

// GetAppStoreVersionLocalizationSearchKeywords retrieves search keywords for a localization.
func (c *Client) GetAppStoreVersionLocalizationSearchKeywords(ctx context.Context, localizationID string) (*AppKeywordsResponse, error) {
	localizationID = strings.TrimSpace(localizationID)
	if localizationID == "" {
		return nil, fmt.Errorf("localizationID is required")
	}
	return getJSON[AppKeywordsResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/appStoreVersionLocalizations/%s/searchKeywords", localizationID)})
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
	return sendNoContent(ctx, c, "POST", fmt.Sprintf("/v1/appStoreVersionLocalizations/%s/relationships/searchKeywords", localizationID), payload)
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
	return sendNoContent(ctx, c, "DELETE", fmt.Sprintf("/v1/appStoreVersionLocalizations/%s/relationships/searchKeywords", localizationID), payload)
}

// GetAppStoreVersionLocalizationPreviewSets retrieves preview sets for a localization.
func (c *Client) GetAppStoreVersionLocalizationPreviewSets(ctx context.Context, localizationID string, opts ...AppStoreVersionLocalizationPreviewSetsOption) (*AppPreviewSetsResponse, error) {
	query := applyOptions(opts)
	localizationID = strings.TrimSpace(localizationID)
	if query.nextURL == "" && localizationID == "" {
		return nil, fmt.Errorf("localizationID is required")
	}
	return getJSON[AppPreviewSetsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/appStoreVersionLocalizations/%s/appPreviewSets", localizationID),
		Query:       buildAppStoreVersionLocalizationPreviewSetsQuery(query),
		Next:        query.nextURL,
		NextContext: "appPreviewSets",
	})
}

// GetAppStoreVersionLocalizationPreviewSetsRelationships retrieves preview set relationships.
func (c *Client) GetAppStoreVersionLocalizationPreviewSetsRelationships(ctx context.Context, localizationID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	query := applyOptions(opts)
	localizationID = strings.TrimSpace(localizationID)
	if query.nextURL == "" && localizationID == "" {
		return nil, fmt.Errorf("localizationID is required")
	}
	return getJSON[LinkagesResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/appStoreVersionLocalizations/%s/relationships/appPreviewSets", localizationID),
		Query:       buildLinkagesQuery(query),
		Next:        query.nextURL,
		NextContext: "appPreviewSetsRelationships",
	})
}

// GetAppStoreVersionLocalizationScreenshotSets retrieves screenshot sets for a localization.
func (c *Client) GetAppStoreVersionLocalizationScreenshotSets(ctx context.Context, localizationID string, opts ...AppStoreVersionLocalizationScreenshotSetsOption) (*AppScreenshotSetsResponse, error) {
	query := applyOptions(opts)
	localizationID = strings.TrimSpace(localizationID)
	if query.nextURL == "" && localizationID == "" {
		return nil, fmt.Errorf("localizationID is required")
	}
	return getJSON[AppScreenshotSetsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/appStoreVersionLocalizations/%s/appScreenshotSets", localizationID),
		Query:       buildAppStoreVersionLocalizationScreenshotSetsQuery(query),
		Next:        query.nextURL,
		NextContext: "appScreenshotSets",
	})
}

// GetAppStoreVersionLocalizationScreenshotSetsRelationships retrieves screenshot set relationships.
func (c *Client) GetAppStoreVersionLocalizationScreenshotSetsRelationships(ctx context.Context, localizationID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	query := applyOptions(opts)
	localizationID = strings.TrimSpace(localizationID)
	if query.nextURL == "" && localizationID == "" {
		return nil, fmt.Errorf("localizationID is required")
	}
	return getJSON[LinkagesResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/appStoreVersionLocalizations/%s/relationships/appScreenshotSets", localizationID),
		Query:       buildLinkagesQuery(query),
		Next:        query.nextURL,
		NextContext: "appScreenshotSetsRelationships",
	})
}

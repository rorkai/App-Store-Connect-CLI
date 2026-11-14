package asc

import (
	"context"
	"fmt"
	"strings"
)

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

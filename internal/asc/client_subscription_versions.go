package asc

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// CreateSubscriptionLocalizationV2 creates a version-scoped localization.
func (c *Client) CreateSubscriptionLocalizationV2(ctx context.Context, versionID string, attrs SubscriptionLocalizationV2CreateAttributes) (*SubscriptionLocalizationV2Response, error) {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return nil, fmt.Errorf("version ID is required")
	}
	if strings.TrimSpace(attrs.Name) == "" {
		return nil, fmt.Errorf("name is required")
	}
	if strings.TrimSpace(attrs.Locale) == "" {
		return nil, fmt.Errorf("locale is required")
	}
	payload := SubscriptionLocalizationV2CreateRequest{Data: SubscriptionLocalizationV2CreateData{
		Type:       ResourceTypeSubscriptionLocalizations,
		Attributes: attrs,
		Relationships: &SubscriptionVersionRelationship{Version: &Relationship{Data: ResourceData{
			Type: ResourceTypeSubscriptionVersions,
			ID:   versionID,
		}}},
	}}
	return sendJSON[SubscriptionLocalizationV2Response](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v2/subscriptionLocalizations", Payload: payload})
}

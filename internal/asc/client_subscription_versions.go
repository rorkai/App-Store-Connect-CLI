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

// CreateSubscriptionImageV2 reserves a version-scoped image upload.
func (c *Client) CreateSubscriptionImageV2(ctx context.Context, versionID, fileName string, fileSize int64) (*SubscriptionImageV2Response, error) {
	versionID = strings.TrimSpace(versionID)
	fileName = strings.TrimSpace(fileName)
	if versionID == "" {
		return nil, fmt.Errorf("version ID is required")
	}
	if fileName == "" {
		return nil, fmt.Errorf("file name is required")
	}
	if fileSize <= 0 {
		return nil, fmt.Errorf("file size must be greater than zero")
	}
	payload := SubscriptionImageV2CreateRequest{Data: SubscriptionImageV2CreateData{
		Type: ResourceTypeSubscriptionImages,
		Attributes: SubscriptionImageV2CreateAttributes{
			FileName: fileName,
			FileSize: fileSize,
		},
		Relationships: &SubscriptionVersionRelationship{Version: &Relationship{Data: ResourceData{
			Type: ResourceTypeSubscriptionVersions,
			ID:   versionID,
		}}},
	}}
	return sendJSON[SubscriptionImageV2Response](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v2/subscriptionImages", Payload: payload})
}

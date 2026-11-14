package asc

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// CreateSubscriptionVersion creates a version for a subscription.
func (c *Client) CreateSubscriptionVersion(ctx context.Context, subscriptionID string) (*SubscriptionVersionResponse, error) {
	subscriptionID = strings.TrimSpace(subscriptionID)
	if subscriptionID == "" {
		return nil, fmt.Errorf("subscription ID is required")
	}
	payload := SubscriptionVersionCreateRequest{Data: SubscriptionVersionCreateData{
		Type: ResourceTypeSubscriptionVersions,
		Relationships: &SubscriptionVersionRelationships{Subscription: &Relationship{Data: ResourceData{
			Type: ResourceTypeSubscriptions,
			ID:   subscriptionID,
		}}},
	}}
	return sendJSON[SubscriptionVersionResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v1/subscriptionVersions", Payload: payload})
}

// GetSubscriptionVersionsRelationships retrieves version linkages for a subscription.
func (c *Client) GetSubscriptionVersionsRelationships(ctx context.Context, subscriptionID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getSubscriptionVersionLinkages(ctx, subscriptionID, "/v1/subscriptions/%s/relationships/versions", "subscriptionVersionsRelationships", opts...)
}

// GetSubscriptionVersionLocalizationsRelationships retrieves localization linkages for a version.
func (c *Client) GetSubscriptionVersionLocalizationsRelationships(ctx context.Context, versionID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getSubscriptionVersionLinkages(ctx, versionID, "/v1/subscriptionVersions/%s/relationships/localizations", "subscriptionVersionLocalizationsRelationships", opts...)
}

// GetSubscriptionVersionImagesRelationships retrieves image linkages for a version.
func (c *Client) GetSubscriptionVersionImagesRelationships(ctx context.Context, versionID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getSubscriptionVersionLinkages(ctx, versionID, "/v1/subscriptionVersions/%s/relationships/images", "subscriptionVersionImagesRelationships", opts...)
}

func (c *Client) getSubscriptionVersionLinkages(ctx context.Context, ownerID, pathFormat, label string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	query := applyOptions(opts)
	ownerID = strings.TrimSpace(ownerID)
	if query.nextURL == "" && ownerID == "" {
		return nil, fmt.Errorf("owner ID is required")
	}
	return getJSON[LinkagesResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf(pathFormat, ownerID),
		Query:       buildLinkagesQuery(query),
		Next:        query.nextURL,
		NextContext: label,
	})
}

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

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

// GroupPurchaseInvitationMode controls how group purchase invitations are hosted.
type GroupPurchaseInvitationMode string

const (
	GroupPurchaseInvitationModeAppleHosted     GroupPurchaseInvitationMode = "APPLE_HOSTED"
	GroupPurchaseInvitationModeDeveloperHosted GroupPurchaseInvitationMode = "DEVELOPER_HOSTED"
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
	GroupPurchaseInvitationMode            *GroupPurchaseInvitationMode  `json:"groupPurchaseInvitationMode,omitempty"`
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
	GroupPurchaseInvitationMode            *GroupPurchaseInvitationMode  `json:"groupPurchaseInvitationMode,omitempty"`
	AccessibilityURL                       *string                       `json:"accessibilityUrl,omitempty"`
	SubscriptionStatusURL                  *string                       `json:"subscriptionStatusUrl,omitempty"`
	SubscriptionStatusURLVersion           *SubscriptionStatusURLVersion `json:"subscriptionStatusUrlVersion,omitempty"`
	SubscriptionStatusURLForSandbox        *string                       `json:"subscriptionStatusUrlForSandbox,omitempty"`
	SubscriptionStatusURLVersionForSandbox *SubscriptionStatusURLVersion `json:"subscriptionStatusUrlVersionForSandbox,omitempty"`
	StreamlinedPurchasingEnabled           *bool                         `json:"streamlinedPurchasingEnabled,omitempty"`
}

// GetApp retrieves a single app by ID.
func (c *Client) GetApp(ctx context.Context, appID string) (*AppResponse, error) {
	return c.GetAppWithOptions(ctx, appID)
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

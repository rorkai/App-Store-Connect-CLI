package iris

import (
	"encoding/json"
	"fmt"
	"time"
)

// AppCreateAttributes represents attributes for creating an app
type AppCreateAttributes struct {
	// Name is used for the included localization (display name). IRIS rejects
	// `data.attributes.name` on app create.
	Name          string `json:"-"`
	SKU           string `json:"sku"`
	PrimaryLocale string `json:"primaryLocale"`
	BundleID      string `json:"bundleId"`
	CompanyName   string `json:"companyName,omitempty"`

	// Platform is used for the included App Store Version.
	Platform string `json:"-"`
}

// AppCreateAPIAttributes represents the allowed app-level attributes for IRIS create.
// Note: IRIS rejects `name` on create; name must be set via appInfoLocalizations.
type AppCreateAPIAttributes struct {
	SKU           string `json:"sku"`
	PrimaryLocale string `json:"primaryLocale"`
	BundleID      string `json:"bundleId"`
	CompanyName   string `json:"companyName,omitempty"`
}

// AppCreateRelationships represents relationships for app creation
type AppCreateRelationships struct {
	AppStoreVersions struct {
		Data []RelationshipData `json:"data"`
	} `json:"appStoreVersions"`
	AppInfos struct {
		Data []RelationshipData `json:"data"`
	} `json:"appInfos"`
}

// RelationshipData represents a relationship data item
type RelationshipData struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

// AppInfoData represents app info for creation
type AppInfoData struct {
	Type          string               `json:"type"`
	ID            string               `json:"id"`
	Relationships AppInfoRelationships `json:"relationships"`
}

// AppInfoRelationships represents app info relationships
type AppInfoRelationships struct {
	AppInfoLocalizations struct {
		Data []RelationshipData `json:"data"`
	} `json:"appInfoLocalizations"`
}

// AppInfoLocalizationData represents app info localization
type AppInfoLocalizationData struct {
	Type       string                        `json:"type"`
	ID         string                        `json:"id"`
	Attributes AppInfoLocalizationAttributes `json:"attributes"`
}

// AppInfoLocalizationAttributes represents app info localization attributes
type AppInfoLocalizationAttributes struct {
	Locale string `json:"locale"`
	Name   string `json:"name"`
}

// AppStoreVersionData represents app store version for creation
type AppStoreVersionData struct {
	Type          string                        `json:"type"`
	ID            string                        `json:"id"`
	Attributes    AppStoreVersionAttributes     `json:"attributes"`
	Relationships *AppStoreVersionRelationships `json:"relationships,omitempty"`
}

// AppStoreVersionRelationships represents app store version relationships
type AppStoreVersionRelationships struct {
	AppStoreVersionLocalizations struct {
		Data []RelationshipData `json:"data"`
	} `json:"appStoreVersionLocalizations"`
}

// AppStoreVersionAttributes represents app store version attributes
type AppStoreVersionAttributes struct {
	VersionString string `json:"versionString"`
	Platform      string `json:"platform"`
}

// AppStoreVersionLocalizationData represents app store version localization for creation
type AppStoreVersionLocalizationData struct {
	Type       string                                `json:"type"`
	ID         string                                `json:"id"`
	Attributes AppStoreVersionLocalizationAttributes `json:"attributes"`
	// Relationships omitted on purpose: IRIS rejects referencing the inline-created
	// appStoreVersion local-id from within the localization for this request.
}

// AppStoreVersionLocalizationAttributes represents app store version localization attributes
type AppStoreVersionLocalizationAttributes struct {
	Locale string `json:"locale"`
}

// AppCreateRequest represents the full app creation request
type AppCreateRequest struct {
	Data struct {
		Type          string                 `json:"type"`
		Attributes    AppCreateAPIAttributes `json:"attributes"`
		Relationships AppCreateRelationships `json:"relationships"`
	} `json:"data"`
	Included []interface{} `json:"included"`
}

// AppResponse represents the response from app creation
type AppResponse struct {
	Data struct {
		ID         string                 `json:"id"`
		Type       string                 `json:"type"`
		Attributes map[string]interface{} `json:"attributes"`
	} `json:"data"`
}

// CreateApp creates a new app in App Store Connect using the IRIS API
func (c *Client) CreateApp(attrs AppCreateAttributes) (*AppResponse, error) {
	// Set defaults
	if attrs.PrimaryLocale == "" {
		attrs.PrimaryLocale = "en-US"
	}

	// Generate SKU if not provided
	if attrs.SKU == "" {
		attrs.SKU = fmt.Sprintf("APP%d", time.Now().Unix())
	}

	// Build the request with included resources
	req := buildAppCreateRequest(attrs)

	respBody, err := c.doRequest("POST", "/apps", req)
	if err != nil {
		return nil, err
	}

	var result AppResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse app response: %w", err)
	}

	return &result, nil
}

// buildAppCreateRequest builds the full app creation request
func buildAppCreateRequest(attrs AppCreateAttributes) *AppCreateRequest {
	req := &AppCreateRequest{}

	// Main app data
	req.Data.Type = "apps"
	req.Data.Attributes = AppCreateAPIAttributes{
		SKU:           attrs.SKU,
		PrimaryLocale: attrs.PrimaryLocale,
		BundleID:      attrs.BundleID,
		CompanyName:   attrs.CompanyName,
	}

	// Build relationships
	// App Store Version
	// For inline creation, IRIS requires "local ids" in the literal `${...}` format.
	storeVersionID := "${new-appStoreVersion}"
	storeVersionLocalizationID := "${new-appStoreVersionLocalization}"
	req.Data.Relationships.AppStoreVersions.Data = []RelationshipData{
		{Type: "appStoreVersions", ID: storeVersionID},
	}

	// App Info
	appInfoID := "${new-appInfo}"
	req.Data.Relationships.AppInfos.Data = []RelationshipData{
		{Type: "appInfos", ID: appInfoID},
	}

	// Build included resources
	req.Included = []interface{}{}

	// App Store Version (iOS)
	platform := attrs.Platform
	if platform == "" {
		platform = "IOS"
	}
	storeVersion := AppStoreVersionData{
		Type: "appStoreVersions",
		ID:   storeVersionID,
		Attributes: AppStoreVersionAttributes{
			VersionString: "1.0",
			Platform:      platform,
		},
	}
	storeVersion.Relationships = &AppStoreVersionRelationships{}
	storeVersion.Relationships.AppStoreVersionLocalizations.Data = []RelationshipData{
		{Type: "appStoreVersionLocalizations", ID: storeVersionLocalizationID},
	}
	req.Included = append(req.Included, storeVersion)

	// App Store Version Localization (required relationship)
	storeVersionLoc := AppStoreVersionLocalizationData{
		Type: "appStoreVersionLocalizations",
		ID:   storeVersionLocalizationID,
		Attributes: AppStoreVersionLocalizationAttributes{
			Locale: attrs.PrimaryLocale,
		},
	}
	req.Included = append(req.Included, storeVersionLoc)

	// App Info
	appInfo := AppInfoData{
		Type:          "appInfos",
		ID:            appInfoID,
		Relationships: AppInfoRelationships{},
	}
	appInfo.Relationships.AppInfoLocalizations.Data = []RelationshipData{
		{Type: "appInfoLocalizations", ID: "${new-appInfoLocalization}"},
	}
	req.Included = append(req.Included, appInfo)

	// App Info Localization
	appName := attrs.Name
	if appName == "" {
		appName = attrs.SKU // Use SKU as fallback name
	}
	appInfoLoc := AppInfoLocalizationData{
		Type: "appInfoLocalizations",
		ID:   "${new-appInfoLocalization}",
		Attributes: AppInfoLocalizationAttributes{
			Locale: attrs.PrimaryLocale,
			Name:   appName,
		},
	}
	req.Included = append(req.Included, appInfoLoc)

	return req
}

// FindApp finds an existing app by bundle ID
func (c *Client) FindApp(bundleID string) (*AppResponse, error) {
	path := fmt.Sprintf("/apps?filter[bundleId]=%s", bundleID)

	respBody, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data []struct {
			ID         string                 `json:"id"`
			Type       string                 `json:"type"`
			Attributes map[string]interface{} `json:"attributes"`
		} `json:"data"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse app response: %w", err)
	}

	if len(result.Data) == 0 {
		return nil, nil
	}

	return &AppResponse{
		Data: result.Data[0],
	}, nil
}

package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

const (
	defaultPrimaryLocale = "en-US"
	defaultPlatform      = "IOS"
	defaultVersion       = "1.0"
)

// AppCreateAttributes defines app creation inputs for the internal web API.
type AppCreateAttributes struct {
	Name          string `json:"-"`
	SKU           string `json:"sku"`
	PrimaryLocale string `json:"primaryLocale"`
	BundleID      string `json:"bundleId"`
	CompanyName   string `json:"companyName,omitempty"`
	Platform      string `json:"-"`
	VersionString string `json:"-"`
}

// AppResponse is the app response payload from internal create/find calls.
type AppResponse struct {
	Data struct {
		ID         string         `json:"id"`
		Type       string         `json:"type"`
		Attributes map[string]any `json:"attributes"`
	} `json:"data"`
}

type relationshipData struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type appCreateRequest struct {
	Data struct {
		Type       string `json:"type"`
		Attributes struct {
			SKU           string `json:"sku"`
			PrimaryLocale string `json:"primaryLocale"`
			BundleID      string `json:"bundleId"`
			CompanyName   string `json:"companyName,omitempty"`
		} `json:"attributes"`
		Relationships struct {
			AppStoreVersions struct {
				Data []relationshipData `json:"data"`
			} `json:"appStoreVersions"`
			AppInfos struct {
				Data []relationshipData `json:"data"`
			} `json:"appInfos"`
		} `json:"relationships"`
	} `json:"data"`
	Included []any `json:"included"`
}

func normalizeCreateAttrs(attrs AppCreateAttributes) (AppCreateAttributes, error) {
	attrs.Name = strings.TrimSpace(attrs.Name)
	attrs.SKU = strings.TrimSpace(attrs.SKU)
	attrs.PrimaryLocale = strings.TrimSpace(attrs.PrimaryLocale)
	attrs.BundleID = strings.TrimSpace(attrs.BundleID)
	attrs.CompanyName = strings.TrimSpace(attrs.CompanyName)
	attrs.Platform = strings.ToUpper(strings.TrimSpace(attrs.Platform))
	attrs.VersionString = strings.TrimSpace(attrs.VersionString)

	if attrs.Name == "" {
		return attrs, fmt.Errorf("name is required")
	}
	if attrs.BundleID == "" {
		return attrs, fmt.Errorf("bundle id is required")
	}
	if attrs.SKU == "" {
		return attrs, fmt.Errorf("sku is required")
	}
	if attrs.PrimaryLocale == "" {
		attrs.PrimaryLocale = defaultPrimaryLocale
	}
	if attrs.Platform == "" {
		attrs.Platform = defaultPlatform
	}
	if attrs.VersionString == "" {
		attrs.VersionString = defaultVersion
	}

	switch attrs.Platform {
	case "IOS", "MAC_OS", "UNIVERSAL", "TV_OS":
	default:
		return attrs, fmt.Errorf("platform must be one of IOS, MAC_OS, TV_OS, UNIVERSAL")
	}
	return attrs, nil
}

func buildAppCreateRequest(attrs AppCreateAttributes) *appCreateRequest {
	req := &appCreateRequest{}
	req.Data.Type = "apps"
	req.Data.Attributes.SKU = attrs.SKU
	req.Data.Attributes.PrimaryLocale = attrs.PrimaryLocale
	req.Data.Attributes.BundleID = attrs.BundleID
	req.Data.Attributes.CompanyName = attrs.CompanyName

	storeVersionID := "${new-appStoreVersion}"
	storeVersionLocalizationID := "${new-appStoreVersionLocalization}"
	appInfoID := "${new-appInfo}"
	appInfoLocalizationID := "${new-appInfoLocalization}"

	req.Data.Relationships.AppStoreVersions.Data = []relationshipData{
		{Type: "appStoreVersions", ID: storeVersionID},
	}
	req.Data.Relationships.AppInfos.Data = []relationshipData{
		{Type: "appInfos", ID: appInfoID},
	}

	type appStoreVersionData struct {
		Type       string `json:"type"`
		ID         string `json:"id"`
		Attributes struct {
			VersionString string `json:"versionString"`
			Platform      string `json:"platform"`
		} `json:"attributes"`
		Relationships struct {
			AppStoreVersionLocalizations struct {
				Data []relationshipData `json:"data"`
			} `json:"appStoreVersionLocalizations"`
		} `json:"relationships"`
	}
	version := appStoreVersionData{Type: "appStoreVersions", ID: storeVersionID}
	version.Attributes.VersionString = attrs.VersionString
	version.Attributes.Platform = attrs.Platform
	version.Relationships.AppStoreVersionLocalizations.Data = []relationshipData{
		{Type: "appStoreVersionLocalizations", ID: storeVersionLocalizationID},
	}

	type appStoreVersionLocalizationData struct {
		Type       string `json:"type"`
		ID         string `json:"id"`
		Attributes struct {
			Locale string `json:"locale"`
		} `json:"attributes"`
	}
	versionLoc := appStoreVersionLocalizationData{
		Type: "appStoreVersionLocalizations",
		ID:   storeVersionLocalizationID,
	}
	versionLoc.Attributes.Locale = attrs.PrimaryLocale

	type appInfoData struct {
		Type          string `json:"type"`
		ID            string `json:"id"`
		Relationships struct {
			AppInfoLocalizations struct {
				Data []relationshipData `json:"data"`
			} `json:"appInfoLocalizations"`
		} `json:"relationships"`
	}
	info := appInfoData{Type: "appInfos", ID: appInfoID}
	info.Relationships.AppInfoLocalizations.Data = []relationshipData{
		{Type: "appInfoLocalizations", ID: appInfoLocalizationID},
	}

	type appInfoLocalizationData struct {
		Type       string `json:"type"`
		ID         string `json:"id"`
		Attributes struct {
			Locale string `json:"locale"`
			Name   string `json:"name"`
		} `json:"attributes"`
	}
	infoLoc := appInfoLocalizationData{
		Type: "appInfoLocalizations",
		ID:   appInfoLocalizationID,
	}
	infoLoc.Attributes.Locale = attrs.PrimaryLocale
	infoLoc.Attributes.Name = attrs.Name

	req.Included = []any{
		version,
		versionLoc,
		info,
		infoLoc,
	}
	return req
}

// CreateApp creates an app with the internal web API.
func (c *Client) CreateApp(ctx context.Context, attrs AppCreateAttributes) (*AppResponse, error) {
	normalized, err := normalizeCreateAttrs(attrs)
	if err != nil {
		return nil, err
	}
	req := buildAppCreateRequest(normalized)

	respBody, err := c.doRequest(ctx, "POST", "/apps", req)
	if err != nil {
		return nil, err
	}

	var result AppResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse app response: %w", err)
	}
	return &result, nil
}

// FindApp finds an existing app by bundle ID.
func (c *Client) FindApp(ctx context.Context, bundleID string) (*AppResponse, error) {
	bundleID = strings.TrimSpace(bundleID)
	if bundleID == "" {
		return nil, fmt.Errorf("bundle id is required")
	}
	path := fmt.Sprintf("/apps?filter[bundleId]=%s", url.QueryEscape(bundleID))

	respBody, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var payload struct {
		Data []struct {
			ID         string         `json:"id"`
			Type       string         `json:"type"`
			Attributes map[string]any `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse app response: %w", err)
	}
	if len(payload.Data) == 0 {
		return nil, nil
	}

	result := &AppResponse{}
	result.Data.ID = payload.Data[0].ID
	result.Data.Type = payload.Data[0].Type
	result.Data.Attributes = payload.Data[0].Attributes
	return result, nil
}

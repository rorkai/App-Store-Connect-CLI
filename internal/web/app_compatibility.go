package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const (
	macCompatibilityRelationship    = "iosAppToMacAppStoreOptInSetting"
	visionCompatibilityRelationship = "iosAppToAppStoreOnVisionOsOptInSetting"
	macCompatibilityAttribute       = "isOptedInToDistributeIosAppOnMacAppStore"
	visionCompatibilityAttribute    = "isOptedInToDistributeIosAppToAppStoreOnVisionOs"
)

// AppCompatibility captures app-level App Store compatibility opt-in settings.
type AppCompatibility struct {
	AppID              string `json:"appId"`
	IOSAppOnMac        *bool  `json:"iosAppOnMac,omitempty"`
	IOSAppOnVisionPro  *bool  `json:"iosAppOnVisionPro,omitempty"`
	MacSettingID       string `json:"macSettingId,omitempty"`
	VisionProSettingID string `json:"visionProSettingId,omitempty"`
}

type appCompatibilitySetting struct {
	ID        string
	Type      string
	Attribute string
	Value     *bool
}

func findIncludedResource(resources []jsonAPIResource, ref resourceRef) *jsonAPIResource {
	for i := range resources {
		if resources[i].ID == ref.ID && resources[i].Type == ref.Type {
			return &resources[i]
		}
	}
	return nil
}

func boolAttrPtr(attrs map[string]any, key string) *bool {
	if attrs == nil {
		return nil
	}
	value, ok := attrs[key]
	if !ok {
		return nil
	}
	typed, ok := value.(bool)
	if !ok {
		return nil
	}
	return &typed
}

func decodeCompatibilitySetting(app jsonAPIResource, included []jsonAPIResource, relationshipName, attributeName string) appCompatibilitySetting {
	setting := appCompatibilitySetting{Attribute: attributeName}
	ref := firstRelationshipRef(app, relationshipName)
	if ref == nil {
		return setting
	}
	setting.ID = strings.TrimSpace(ref.ID)
	setting.Type = strings.TrimSpace(ref.Type)
	if resource := findIncludedResource(included, *ref); resource != nil {
		setting.Value = boolAttrPtr(resource.Attributes, attributeName)
	}
	return setting
}

func decodeAppCompatibility(appID string, app jsonAPIResource, included []jsonAPIResource) (*AppCompatibility, map[string]appCompatibilitySetting) {
	mac := decodeCompatibilitySetting(app, included, macCompatibilityRelationship, macCompatibilityAttribute)
	vision := decodeCompatibilitySetting(app, included, visionCompatibilityRelationship, visionCompatibilityAttribute)

	result := &AppCompatibility{
		AppID:              strings.TrimSpace(appID),
		IOSAppOnMac:        mac.Value,
		IOSAppOnVisionPro:  vision.Value,
		MacSettingID:       mac.ID,
		VisionProSettingID: vision.ID,
	}
	if result.AppID == "" {
		result.AppID = strings.TrimSpace(app.ID)
	}
	return result, map[string]appCompatibilitySetting{
		"mac":    mac,
		"vision": vision,
	}
}

// GetAppCompatibility retrieves app-level App Store compatibility opt-in settings.
func (c *Client) GetAppCompatibility(ctx context.Context, appID string) (*AppCompatibility, error) {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return nil, fmt.Errorf("app id is required")
	}

	path := "/apps/" + url.PathEscape(appID) + "?include=" + url.QueryEscape(macCompatibilityRelationship+","+visionCompatibilityRelationship)
	responseBody, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var payload struct {
		Data     jsonAPIResource   `json:"data"`
		Included []jsonAPIResource `json:"included"`
	}
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse app compatibility response: %w", err)
	}

	result, _ := decodeAppCompatibility(appID, payload.Data, payload.Included)
	return result, nil
}

// UpdateAppCompatibility edits app-level App Store compatibility opt-in settings.
// When both settings are provided, PATCH requests are sent sequentially and are
// not transactional; if a later PATCH fails, any earlier setting may remain
// updated and can be retried by the caller.
func (c *Client) UpdateAppCompatibility(ctx context.Context, appID string, iosAppOnMac, iosAppOnVisionPro *bool) (*AppCompatibility, error) {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return nil, fmt.Errorf("app id is required")
	}
	if iosAppOnMac == nil && iosAppOnVisionPro == nil {
		return nil, fmt.Errorf("at least one compatibility setting is required")
	}

	path := "/apps/" + url.PathEscape(appID) + "?include=" + url.QueryEscape(macCompatibilityRelationship+","+visionCompatibilityRelationship)
	responseBody, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var payload struct {
		Data     jsonAPIResource   `json:"data"`
		Included []jsonAPIResource `json:"included"`
	}
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse app compatibility response: %w", err)
	}

	result, settings := decodeAppCompatibility(appID, payload.Data, payload.Included)
	updates := []struct {
		name    string
		value   *bool
		setting appCompatibilitySetting
	}{
		{name: "Mac App Store", value: iosAppOnMac, setting: settings["mac"]},
		{name: "Apple Vision Pro", value: iosAppOnVisionPro, setting: settings["vision"]},
	}

	for _, update := range updates {
		if update.value == nil {
			continue
		}
		if update.setting.ID == "" || update.setting.Type == "" {
			return nil, fmt.Errorf("%s compatibility setting is not available for app %q", update.name, appID)
		}

		body := map[string]any{
			"data": map[string]any{
				"type": update.setting.Type,
				"id":   update.setting.ID,
				"attributes": map[string]bool{
					update.setting.Attribute: *update.value,
				},
			},
		}
		if _, err := c.doRequest(ctx, http.MethodPatch, "/"+url.PathEscape(update.setting.Type)+"/"+url.PathEscape(update.setting.ID), body); err != nil {
			return nil, err
		}
		switch update.name {
		case "Mac App Store":
			result.IOSAppOnMac = update.value
		case "Apple Vision Pro":
			result.IOSAppOnVisionPro = update.value
		}
	}

	return result, nil
}

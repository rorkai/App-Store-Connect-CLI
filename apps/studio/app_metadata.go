package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// GetVersionMetadata returns all localizations for a given App Store version.
// Pass versionID from AppVersion.ID. Returns all locales so the frontend can
// render a picker without an extra round-trip.
func (a *App) GetVersionMetadata(versionID string) (VersionMetadataResponse, error) {
	if strings.TrimSpace(versionID) == "" {
		return VersionMetadataResponse{Error: "version ID is required"}, nil
	}

	ascPath, err := a.resolveASCPath()
	if err != nil {
		return VersionMetadataResponse{Error: "Could not find asc binary: " + err.Error()}, nil
	}

	ctx, cancel := context.WithTimeout(a.contextOrBackground(), 20*time.Second)
	defer cancel()

	out, err := a.runASCCombinedOutput(ctx, ascPath, "localizations", "list",
		"--version", versionID, "--output", "json")
	if err != nil {
		return VersionMetadataResponse{Error: strings.TrimSpace(string(out))}, nil
	}

	type rawAttrs struct {
		Locale          string `json:"locale"`
		Description     string `json:"description"`
		Keywords        string `json:"keywords"`
		WhatsNew        string `json:"whatsNew"`
		PromotionalText string `json:"promotionalText"`
		SupportURL      string `json:"supportUrl"`
		MarketingURL    string `json:"marketingUrl"`
	}
	type rawItem struct {
		ID         string   `json:"id"`
		Attributes rawAttrs `json:"attributes"`
	}
	var envelope struct {
		Data []rawItem `json:"data"`
	}
	if json.Unmarshal(out, &envelope) != nil {
		return VersionMetadataResponse{Error: "failed to parse localizations"}, nil
	}

	locs := make([]AppLocalization, 0, len(envelope.Data))
	for _, item := range envelope.Data {
		attrs := item.Attributes
		locs = append(locs, AppLocalization{
			LocalizationID:  item.ID,
			Locale:          attrs.Locale,
			Description:     attrs.Description,
			Keywords:        attrs.Keywords,
			WhatsNew:        attrs.WhatsNew,
			PromotionalText: attrs.PromotionalText,
			SupportURL:      attrs.SupportURL,
			MarketingURL:    attrs.MarketingURL,
		})
	}
	return VersionMetadataResponse{Localizations: locs}, nil
}

// GetScreenshots returns screenshot sets for a version localization.
// Pass LocalizationID from AppLocalization.
func (a *App) GetScreenshots(localizationID string) (ScreenshotsResponse, error) {
	if strings.TrimSpace(localizationID) == "" {
		return ScreenshotsResponse{Error: "localization ID is required"}, nil
	}

	ascPath, err := a.resolveASCPath()
	if err != nil {
		return ScreenshotsResponse{Error: "Could not find asc binary: " + err.Error()}, nil
	}

	ctx, cancel := context.WithTimeout(a.contextOrBackground(), 20*time.Second)
	defer cancel()

	out, err := a.runASCCombinedOutput(ctx, ascPath, "screenshots", "list",
		"--version-localization", localizationID, "--output", "json")
	if err != nil {
		return ScreenshotsResponse{Error: strings.TrimSpace(string(out))}, nil
	}

	type rawImageAsset struct {
		TemplateURL string `json:"templateUrl"`
		Width       int    `json:"width"`
		Height      int    `json:"height"`
	}
	type rawScreenshot struct {
		Attributes struct {
			ImageAsset rawImageAsset `json:"imageAsset"`
		} `json:"attributes"`
	}
	type rawSet struct {
		Set struct {
			Attributes struct {
				DisplayType string `json:"screenshotDisplayType"`
			} `json:"attributes"`
		} `json:"set"`
		Screenshots []rawScreenshot `json:"screenshots"`
	}
	var result struct {
		Sets []rawSet `json:"sets"`
	}
	if json.Unmarshal(out, &result) != nil {
		return ScreenshotsResponse{Error: "failed to parse screenshots"}, nil
	}

	sets := make([]ScreenshotSet, 0, len(result.Sets))
	for _, rs := range result.Sets {
		if len(rs.Screenshots) == 0 {
			continue
		}
		shots := make([]AppScreenshot, 0, len(rs.Screenshots))
		for _, s := range rs.Screenshots {
			ia := s.Attributes.ImageAsset
			if ia.TemplateURL == "" {
				continue
			}
			// Build a ~400px-wide thumbnail URL from the template.
			thumbW := 600
			thumbH := thumbW
			if ia.Width > 0 && ia.Height > 0 {
				thumbH = thumbW * ia.Height / ia.Width
			}
			thumbURL := strings.NewReplacer(
				"{w}", fmt.Sprintf("%d", thumbW),
				"{h}", fmt.Sprintf("%d", thumbH),
				"{f}", "webp",
			).Replace(ia.TemplateURL)
			shots = append(shots, AppScreenshot{
				ThumbnailURL: thumbURL,
				Width:        ia.Width,
				Height:       ia.Height,
			})
		}
		if len(shots) > 0 {
			sets = append(sets, ScreenshotSet{
				DisplayType: rs.Set.Attributes.DisplayType,
				Screenshots: shots,
			})
		}
	}
	return ScreenshotsResponse{Sets: sets}, nil
}

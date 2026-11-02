package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

const appBundleEligibleAppsPath = "/apps/approvedForBundles?platform=IOS,OSX"

// AppBundleEligibleApp is an app Apple offers for adding to a bundle.
type AppBundleEligibleApp struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

// AppBundleEligibleAppsResponse is the normalized output for eligible apps.
type AppBundleEligibleAppsResponse struct {
	Data []AppBundleEligibleApp `json:"data"`
}

type appBundleEligibleApp struct {
	AdamID json.Number `json:"adamId"`
	Name   string      `json:"name"`
}

// ListAppBundleEligibleApps lists the iOS and macOS apps that can join a bundle.
func (c *Client) ListAppBundleEligibleApps(ctx context.Context) (*AppBundleEligibleAppsResponse, error) {
	apps, err := doAppBundleJSON[[]appBundleEligibleApp](ctx, c, http.MethodGet, appBundleEligibleAppsPath, nil)
	if err != nil {
		return nil, err
	}
	result := &AppBundleEligibleAppsResponse{Data: make([]AppBundleEligibleApp, 0, len(*apps))}
	for _, app := range *apps {
		result.Data = append(result.Data, AppBundleEligibleApp{ID: app.AdamID.String(), Name: app.Name})
	}
	return result, nil
}

type appBundleSaveRequest struct {
	AdamID            json.Number           `json:"adamId"`
	VendorIdentifier  string                `json:"vendorIdentifier"`
	PrimaryLocaleCode string                `json:"primaryLocaleCode"`
	PriceTierStem     string                `json:"priceTierStem,omitempty"`
	ClearedForSale    bool                  `json:"clearedForSale"`
	Details           []appBundleSaveLocale `json:"details"`
	MemberApps        []appBundleSaveApp    `json:"memberApps"`
	UserBundleIcon    json.RawMessage       `json:"userBundleIcon"`
}

type appBundleSaveLocale struct {
	Name          string  `json:"name"`
	LocaleCode    string  `json:"localeCode"`
	MarketingText *string `json:"marketingText"`
	MarketingURL  *string `json:"marketingUrl"`
}

type appBundleSaveApp struct {
	AdamID    json.Number `json:"adamId"`
	PriceTier string      `json:"priceTier,omitempty"`
}

// SetAppBundleApps replaces a bundle's member apps with appIDs, in that order.
// The save endpoint takes the whole bundle, so every other field is sent back
// as read. Apple's bundle page only allows reordering once a bundle has been
// approved, and no edits while it is in review. The first app is the primary
// app, so moving it also changes metadata Apple derives from it.
func (c *Client) SetAppBundleApps(ctx context.Context, bundleID string, appIDs []string) (*asc.WebAppBundleAppsResult, error) {
	bundleID = strings.TrimSpace(bundleID)
	if bundleID == "" {
		return nil, errors.New("app bundle ID is required")
	}
	if len(appIDs) == 0 {
		return nil, errors.New("at least one app ID is required")
	}

	current, err := c.fetchAppBundleDetail(ctx, bundleID)
	if err != nil {
		return nil, err
	}
	members := make(map[string]appBundleMemberApp, len(current.MemberApps))
	currentIDs := make([]string, 0, len(current.MemberApps))
	for _, app := range current.MemberApps {
		members[app.AdamID.String()] = app
		currentIDs = append(currentIDs, app.AdamID.String())
	}
	result := &asc.WebAppBundleAppsResult{BundleID: bundleID, PreviousApps: currentIDs, Apps: appIDs}
	if slices.Equal(currentIDs, appIDs) {
		return result, nil
	}

	switch current.StateKey {
	case "inReview", "waitingForReview":
		return nil, fmt.Errorf("app bundle is %s; remove it from review before changing its apps", current.StateKey)
	}
	newIDs := slices.DeleteFunc(slices.Clone(appIDs), func(id string) bool {
		_, member := members[id]
		return member
	})
	if len(newIDs) > 0 || len(appIDs) != len(currentIDs) {
		switch current.StateKey {
		case "readyForSale", "developerRemovedFromSale":
			return nil, fmt.Errorf("app bundle is %s; Apple only allows reordering its apps", current.StateKey)
		}
	}
	if err := c.requireAppBundleEligible(ctx, newIDs); err != nil {
		return nil, err
	}

	if err := c.postAppBundle(ctx, "/appbundles/createOrUpdate", newAppBundleSaveRequest(current, members, appIDs)); err != nil {
		return nil, err
	}

	saved, err := c.fetchAppBundleDetail(ctx, bundleID)
	if err != nil {
		return nil, fmt.Errorf("could not verify app bundle apps: %w", err)
	}
	savedIDs := make([]string, 0, len(saved.MemberApps))
	for _, app := range saved.MemberApps {
		savedIDs = append(savedIDs, app.AdamID.String())
	}
	if !slices.Equal(savedIDs, appIDs) {
		return nil, fmt.Errorf("app bundle readback does not match the requested apps: got %s", strings.Join(savedIDs, ","))
	}
	result.Changed = true
	return result, nil
}

func (c *Client) requireAppBundleEligible(ctx context.Context, appIDs []string) error {
	if len(appIDs) == 0 {
		return nil
	}
	eligible, err := c.ListAppBundleEligibleApps(ctx)
	if err != nil {
		return err
	}
	for _, id := range appIDs {
		if !slices.ContainsFunc(eligible.Data, func(app AppBundleEligibleApp) bool { return app.ID == id }) {
			return fmt.Errorf("app %s is not eligible for an app bundle", id)
		}
	}
	return nil
}

func newAppBundleSaveRequest(current *appBundleDetailData, members map[string]appBundleMemberApp, appIDs []string) appBundleSaveRequest {
	request := appBundleSaveRequest{
		AdamID:            current.AdamID,
		VendorIdentifier:  current.VendorIdentifier,
		PrimaryLocaleCode: current.PrimaryLocaleCode,
		PriceTierStem:     current.PriceTierStem,
		ClearedForSale:    current.ClearedForSale,
		Details:           make([]appBundleSaveLocale, 0, len(current.Details)),
		MemberApps:        make([]appBundleSaveApp, 0, len(appIDs)),
		UserBundleIcon:    current.UserBundleIcon,
	}
	for _, locale := range current.Details {
		request.Details = append(request.Details, appBundleSaveLocale{
			Name:          locale.Name,
			LocaleCode:    locale.LocaleCode,
			MarketingText: locale.MarketingText.Value,
			MarketingURL:  locale.MarketingURL.Value,
		})
	}
	for _, id := range appIDs {
		request.MemberApps = append(request.MemberApps, appBundleSaveApp{AdamID: json.Number(id), PriceTier: members[id].PriceTier})
	}
	return request
}

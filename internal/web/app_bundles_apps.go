package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
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

// appBundleEligibleRow keeps Apple's row so an added app can be sent back the
// way the bundle page sends it.
type appBundleEligibleRow map[string]json.RawMessage

// appBundleDisplayOnlyFields are the keys the bundle page strips from a member
// before saving.
var appBundleDisplayOnlyFields = []string{"iconUrl", "appType", "lastModifiedDate", "name", "type", "vendorId", "versions", "latestSupportedHardware", "bundleId"}

func (r appBundleEligibleRow) id() string {
	return strings.Trim(string(r["adamId"]), `"`)
}

func (r appBundleEligibleRow) name() string {
	var name string
	if err := json.Unmarshal(r["name"], &name); err != nil {
		return ""
	}
	return name
}

// ListAppBundleEligibleApps lists the iOS and macOS apps that can join a bundle.
func (c *Client) ListAppBundleEligibleApps(ctx context.Context) (*AppBundleEligibleAppsResponse, error) {
	rows, err := doAppBundleJSON[[]appBundleEligibleRow](ctx, c, http.MethodGet, appBundleEligibleAppsPath, nil)
	if err != nil {
		return nil, err
	}
	result := &AppBundleEligibleAppsResponse{Data: make([]AppBundleEligibleApp, 0, len(*rows))}
	for _, row := range *rows {
		result.Data = append(result.Data, AppBundleEligibleApp{ID: row.id(), Name: row.name()})
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
	MemberApps        []any                 `json:"memberApps"`
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

	if appBundleInReview(current) {
		return nil, fmt.Errorf("app bundle is in review (%s); remove it from review before changing its apps", current.StateKey)
	}
	newIDs := slices.DeleteFunc(slices.Clone(appIDs), func(id string) bool {
		_, member := members[id]
		return member
	})
	if len(newIDs) > 0 || len(appIDs) != len(currentIDs) {
		switch current.StateKey {
		case "readyForSale", "developerRemovedFromSale", "removedFromSale", "pendingContract":
			return nil, fmt.Errorf("app bundle is %s; Apple only allows reordering its apps", current.StateKey)
		}
	}
	added, err := c.eligibleAppBundleRows(ctx, newIDs)
	if err != nil {
		return nil, err
	}

	if err := c.postAppBundle(ctx, "/appbundles/createOrUpdate", newAppBundleSaveRequest(current, members, added, appIDs)); err != nil {
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

// appBundleInReview reports a pending review. A released bundle keeps its sale
// state while a newer version is reviewed, so inFlightState and Apple's offer to
// withdraw it count too; a Rejected in-flight version stays editable.
func appBundleInReview(detail *appBundleDetailData) bool {
	return detail.StateKey == "inReview" || detail.StateKey == "waitingForReview" ||
		detail.CanDevReject || strings.Contains(strings.ToLower(detail.InFlightState), "review")
}

func (c *Client) eligibleAppBundleRows(ctx context.Context, appIDs []string) (map[string]appBundleEligibleRow, error) {
	added := make(map[string]appBundleEligibleRow, len(appIDs))
	if len(appIDs) == 0 {
		return added, nil
	}
	rows, err := doAppBundleJSON[[]appBundleEligibleRow](ctx, c, http.MethodGet, appBundleEligibleAppsPath, nil)
	if err != nil {
		return nil, err
	}
	for _, id := range appIDs {
		index := slices.IndexFunc(*rows, func(row appBundleEligibleRow) bool { return row.id() == id })
		if index < 0 {
			return nil, fmt.Errorf("app %s is not eligible for an app bundle", id)
		}
		row := maps.Clone((*rows)[index])
		for _, key := range appBundleDisplayOnlyFields {
			delete(row, key)
		}
		added[id] = row
	}
	return added, nil
}

func newAppBundleSaveRequest(current *appBundleDetailData, members map[string]appBundleMemberApp, added map[string]appBundleEligibleRow, appIDs []string) appBundleSaveRequest {
	request := appBundleSaveRequest{
		AdamID:            current.AdamID,
		VendorIdentifier:  current.VendorIdentifier,
		PrimaryLocaleCode: current.PrimaryLocaleCode,
		PriceTierStem:     current.PriceTierStem,
		ClearedForSale:    current.ClearedForSale,
		Details:           make([]appBundleSaveLocale, 0, len(current.Details)),
		MemberApps:        make([]any, 0, len(appIDs)),
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
		if row, ok := added[id]; ok {
			request.MemberApps = append(request.MemberApps, row)
			continue
		}
		request.MemberApps = append(request.MemberApps, appBundleSaveApp{AdamID: json.Number(id), PriceTier: members[id].PriceTier})
	}
	return request
}

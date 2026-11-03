package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
)

// appBundleReference is Apple's starting metadata for a new bundle of the given
// apps, which the bundle page fills its form from.
type appBundleReference struct {
	PrimaryLocaleCode string `json:"primaryLocaleCode"`
	Locales           []struct {
		Code string `json:"code"`
	} `json:"locales"`
	DefaultDetails  []appBundleLocale    `json:"defaultDetails"`
	ValidPriceTiers []appBundlePriceTier `json:"validPriceTiers"`
}

type appBundlePriceTier struct {
	TierStem string `json:"tierStem"`
}

// CreateAppBundle creates a bundle of appIDs, in that order, that is not cleared
// for sale. Like Apple's bundle page, it starts from Apple's default metadata for
// the apps and names the bundle in the primary locale. Unless priceTier is set,
// it picks the first price tier Apple marks compatible with the apps. Reading the
// new bundle back confirms its apps.
func (c *Client) CreateAppBundle(ctx context.Context, name, sku string, appIDs []string, priceTier string) (*AppBundleDetail, error) {
	name, sku, priceTier = strings.TrimSpace(name), strings.TrimSpace(sku), strings.TrimSpace(priceTier)
	if name == "" {
		return nil, errors.New("app bundle name is required")
	}
	if sku == "" {
		return nil, errors.New("app bundle SKU is required")
	}
	if len(appIDs) == 0 {
		return nil, errors.New("at least one app ID is required")
	}

	rows, err := c.eligibleAppBundleRows(ctx, appIDs)
	if err != nil {
		return nil, err
	}
	reference, err := doAppBundleJSON[appBundleReference](ctx, c, http.MethodGet, "/appbundles/metareference?adamIds="+strings.Join(appIDs, ","), nil)
	if err != nil {
		return nil, err
	}
	if priceTier == "" {
		if len(reference.ValidPriceTiers) == 0 {
			return nil, errors.New("apple marks no price tier compatible with these apps; choose one with --price-tier")
		}
		priceTier = reference.ValidPriceTiers[0].TierStem
	}

	request := appBundleSaveRequest{
		VendorIdentifier:  sku,
		PrimaryLocaleCode: reference.PrimaryLocaleCode,
		PriceTierStem:     priceTier,
		Details:           newAppBundleLocales(reference, name),
		MemberApps:        make([]any, 0, len(appIDs)),
		UserBundleIcon:    json.RawMessage(`{"value":null}`),
	}
	for _, id := range appIDs {
		request.MemberApps = append(request.MemberApps, rows[id])
	}
	created, err := doAppBundleJSON[struct {
		AdamID json.Number `json:"adamId"`
	}](ctx, c, http.MethodPost, "/appbundles/createOrUpdate", request)
	if err != nil {
		return nil, err
	}
	if created.AdamID == "" {
		return nil, errors.New("app bundle response is missing the new bundle ID")
	}

	detail, err := c.GetAppBundle(ctx, created.AdamID.String())
	if err != nil {
		return nil, fmt.Errorf("created app bundle %s but could not read it back: %w", created.AdamID, err)
	}
	savedIDs := make([]string, 0, len(detail.Apps))
	for _, app := range detail.Apps {
		savedIDs = append(savedIDs, app.ID)
	}
	if !slices.Equal(savedIDs, appIDs) {
		return nil, fmt.Errorf("created app bundle %s, but its apps read back as %s", created.AdamID, strings.Join(savedIDs, ","))
	}
	return detail, nil
}

// newAppBundleLocales mirrors the page: one row per locale Apple offers, filled
// from Apple's defaults, with the bundle name in the primary locale.
func newAppBundleLocales(reference *appBundleReference, name string) []appBundleSaveLocale {
	locales := make([]appBundleSaveLocale, 0, len(reference.Locales))
	for _, locale := range reference.Locales {
		row := appBundleSaveLocale{LocaleCode: locale.Code, MarketingURL: new(string)}
		index := slices.IndexFunc(reference.DefaultDetails, func(detail appBundleLocale) bool { return detail.LocaleCode == locale.Code })
		if index >= 0 {
			detail := reference.DefaultDetails[index]
			row.Name, row.MarketingText, row.MarketingURL = detail.Name, detail.MarketingText.Value, detail.MarketingURL.Value
		}
		if locale.Code == reference.PrimaryLocaleCode {
			row.Name = name
		}
		locales = append(locales, row)
	}
	return locales
}

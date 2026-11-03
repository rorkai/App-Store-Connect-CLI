package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const (
	appBundlesListPath   = "/appBundles?include=appBundleVersions&limit=300"
	appBundlesLegacyPath = "/WebObjects/iTunesConnect.woa/ra"
)

// AppBundlesListResponse is the normalized output for App Store app bundles.
type AppBundlesListResponse struct {
	Data []AppBundle `json:"data"`
}

// AppBundle summarizes one bundle from App Store Connect's Apps view.
type AppBundle struct {
	ID        string             `json:"id"`
	Name      string             `json:"name,omitempty"`
	Platforms []string           `json:"platforms,omitempty"`
	Status    string             `json:"status,omitempty"`
	Versions  []AppBundleVersion `json:"versions,omitempty"`
}

// AppBundleVersion is one review version of a bundle.
type AppBundleVersion struct {
	ID          string `json:"id"`
	ReviewState string `json:"reviewState,omitempty"`
}

// AppBundleDetail is the normalized bundle detail page.
type AppBundleDetail struct {
	ID                     string         `json:"id"`
	State                  string         `json:"state,omitempty"`
	StateKey               string         `json:"stateKey,omitempty"`
	InFlightState          string         `json:"inFlightState,omitempty"`
	PrimaryLocale          string         `json:"primaryLocale,omitempty"`
	PriceTier              string         `json:"priceTier,omitempty"`
	ClearedForSale         bool           `json:"clearedForSale"`
	ClearedForSaleEditable bool           `json:"clearedForSaleEditable"`
	CanSubmit              bool           `json:"canSubmit"`
	CanRemoveFromReview    bool           `json:"canRemoveFromReview"`
	Apps                   []AppBundleApp `json:"apps"`
}

// AppBundleApp is one member app, in the bundle's display order.
type AppBundleApp struct {
	ID        string `json:"id"`
	Name      string `json:"name,omitempty"`
	Type      string `json:"type,omitempty"`
	PriceTier string `json:"priceTier,omitempty"`
}

// ListAppBundles lists every App Store app bundle the session can see.
func (c *Client) ListAppBundles(ctx context.Context) (*AppBundlesListResponse, error) {
	payload, err := c.fetchJSONAPIPages(ctx, appBundlesListPath, "app bundles")
	if err != nil {
		return nil, err
	}

	included := make(map[string]jsonAPIResource, len(payload.Included))
	for _, resource := range payload.Included {
		included[jsonAPIResourceKey(resource.Type, resource.ID)] = resource
	}

	result := &AppBundlesListResponse{Data: make([]AppBundle, 0, len(payload.Data))}
	for _, resource := range payload.Data {
		result.Data = append(result.Data, decodeAppBundle(resource, included))
	}
	return result, nil
}

func decodeAppBundle(resource jsonAPIResource, included map[string]jsonAPIResource) AppBundle {
	bundle := AppBundle{
		ID:        strings.TrimSpace(resource.ID),
		Name:      stringAttr(resource.Attributes, "name"),
		Platforms: stringListAttr(resource.Attributes, "platforms"),
	}
	for _, ref := range relationshipRefs(resource, "appBundleVersions") {
		version, ok := included[jsonAPIResourceKey(ref.Type, ref.ID)]
		if !ok {
			continue
		}
		bundle.Versions = append(bundle.Versions, AppBundleVersion{
			ID:          strings.TrimSpace(version.ID),
			ReviewState: stringAttr(version.Attributes, "bundleReviewState"),
		})
	}
	if len(bundle.Versions) > 0 {
		bundle.Status = bundle.Versions[0].ReviewState
	}
	return bundle
}

func stringListAttr(attrs map[string]any, key string) []string {
	raw, ok := attrs[key].([]any)
	if !ok {
		return nil
	}
	values := make([]string, 0, len(raw))
	for _, item := range raw {
		if value, ok := item.(string); ok && strings.TrimSpace(value) != "" {
			values = append(values, strings.TrimSpace(value))
		}
	}
	return values
}

// GetAppBundle reads one bundle's detail, including its member apps in order.
func (c *Client) GetAppBundle(ctx context.Context, bundleID string) (*AppBundleDetail, error) {
	bundleID = strings.TrimSpace(bundleID)
	if bundleID == "" {
		return nil, errors.New("app bundle ID is required")
	}
	data, err := c.fetchAppBundleDetail(ctx, bundleID)
	if err != nil {
		return nil, err
	}

	detail := &AppBundleDetail{
		ID:                     bundleID,
		State:                  data.State,
		StateKey:               data.StateKey,
		InFlightState:          data.InFlightState,
		PrimaryLocale:          data.PrimaryLocaleCode,
		PriceTier:              data.PriceTierStem,
		ClearedForSale:         data.ClearedForSale,
		ClearedForSaleEditable: data.ClearedForSaleEditable,
		CanSubmit:              data.CanSubmit,
		CanRemoveFromReview:    data.CanDevReject,
		Apps:                   make([]AppBundleApp, 0, len(data.MemberApps)),
	}
	for _, app := range data.MemberApps {
		detail.Apps = append(detail.Apps, AppBundleApp{
			ID:        app.AdamID.String(),
			Name:      app.Name,
			Type:      app.AppType,
			PriceTier: app.PriceTier,
		})
	}
	return detail, nil
}

func (c *Client) fetchAppBundleDetail(ctx context.Context, bundleID string) (*appBundleDetailData, error) {
	return doAppBundleJSON[appBundleDetailData](ctx, c, http.MethodGet, "/appbundles/metadetail/"+url.PathEscape(bundleID), nil)
}

func doAppBundleJSON[T any](ctx context.Context, c *Client, method, path string, body any) (*T, error) {
	responseBody, err := c.doAppBundleRequest(ctx, method, path, body)
	if err != nil {
		return nil, err
	}
	var payload appBundleEnvelope[T]
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse app bundle response: %w", err)
	}
	if err := payload.err(); err != nil {
		return nil, err
	}
	if payload.Data == nil {
		return nil, errors.New("app bundle response is missing data")
	}
	return payload.Data, nil
}

// postAppBundle sends a change and checks only Apple's verdict; callers confirm
// the outcome by reading the bundle back.
func (c *Client) postAppBundle(ctx context.Context, path string, body any) error {
	responseBody, err := c.doAppBundleRequest(ctx, http.MethodPost, path, body)
	if err != nil {
		return err
	}
	var payload appBundleEnvelope[json.RawMessage]
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return fmt.Errorf("failed to parse app bundle response: %w", err)
	}
	return payload.err()
}

// appBundleEnvelope is the response wrapper of the legacy bundle endpoints,
// which can report a failure with an HTTP 200.
type appBundleEnvelope[T any] struct {
	Data       *T     `json:"data"`
	StatusCode string `json:"statusCode"`
	Messages   struct {
		Error []string `json:"error"`
	} `json:"messages"`
}

func (e appBundleEnvelope[T]) err() error {
	if len(e.Messages.Error) > 0 {
		return fmt.Errorf("app bundle request failed: %s", htmlToPlainText(strings.Join(e.Messages.Error, "; ")))
	}
	if !strings.EqualFold(e.StatusCode, "SUCCESS") {
		return fmt.Errorf("app bundle request failed with status %q", e.StatusCode)
	}
	return nil
}

type appBundleDetailData struct {
	AdamID                 json.Number          `json:"adamId"`
	VendorIdentifier       string               `json:"vendorIdentifier"`
	State                  string               `json:"state"`
	StateKey               string               `json:"stateKey"`
	InFlightState          string               `json:"inFlightState"`
	PrimaryLocaleCode      string               `json:"primaryLocaleCode"`
	PriceTierStem          string               `json:"priceTierStem"`
	ClearedForSale         bool                 `json:"clearedForSale"`
	ClearedForSaleEditable bool                 `json:"clearedForSaleEditable"`
	CanSubmit              bool                 `json:"canSubmit"`
	CanDevReject           bool                 `json:"canDevReject"`
	MemberApps             []appBundleMemberApp `json:"memberApps"`
	Details                []appBundleLocale    `json:"details"`
	UserBundleIcon         json.RawMessage      `json:"userBundleIcon"`
}

type appBundleLocale struct {
	Name          string             `json:"name"`
	LocaleCode    string             `json:"localeCode"`
	MarketingText appBundleTextField `json:"marketingText"`
	MarketingURL  appBundleTextField `json:"marketingUrl"`
}

type appBundleTextField struct {
	Value *string `json:"value"`
}

type appBundleMemberApp struct {
	AdamID    json.Number `json:"adamId"`
	Name      string      `json:"name"`
	AppType   string      `json:"appType"`
	PriceTier string      `json:"priceTier"`
}

func (c *Client) doAppBundleRequest(ctx context.Context, method, path string, body any) ([]byte, error) {
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")
	headers.Set("X-Requested-With", "XMLHttpRequest")
	headers.Set("Origin", appStoreBaseURL)
	headers.Set("Referer", appStoreBaseURL+"/")
	if method != http.MethodGet {
		headers.Set("X-Csrf-Itc", "itc")
	}
	return c.doRequestBase(ctx, c.complianceFormBaseURL(), method, appBundlesLegacyPath+path, body, headers)
}

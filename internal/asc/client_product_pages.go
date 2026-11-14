package asc

import (
	"context"
	"fmt"
	"strings"
)

// AppCustomProductPageAttributes describes custom product page attributes.
type AppCustomProductPageAttributes struct {
	Name    string `json:"name,omitempty"`
	URL     string `json:"url,omitempty"`
	Visible *bool  `json:"visible,omitempty"`
}

// AppCustomProductPagesResponse is the response from custom product page list endpoints.
type AppCustomProductPagesResponse = Response[AppCustomProductPageAttributes]

// AppCustomProductPageCreateRelationships describes create relationships.
type AppCustomProductPageCreateRelationships struct {
	App                          *Relationship    `json:"app"`
	AppCustomProductPageVersions RelationshipList `json:"appCustomProductPageVersions"`
	AppStoreVersionTemplate      *Relationship    `json:"appStoreVersionTemplate,omitempty"`
	CustomProductPageTemplate    *Relationship    `json:"customProductPageTemplate,omitempty"`
}

// AppCustomProductPageTemplate selects what a new custom product page starts
// from. A zero value creates a page without a template.
type AppCustomProductPageTemplate struct {
	AppStoreVersionID string
	CustomPageID      string
}

// AppCustomProductPageCreateData is the data payload for create requests.
type AppCustomProductPageCreateData struct {
	Type          ResourceType                             `json:"type"`
	Attributes    AppCustomProductPageCreateAttributes     `json:"attributes"`
	Relationships *AppCustomProductPageCreateRelationships `json:"relationships"`
}

// AppCustomProductPageCreateRequest is a request to create a custom product page.
type AppCustomProductPageCreateRequest struct {
	Data     AppCustomProductPageCreateData       `json:"data"`
	Included []AppCustomProductPageIncludedCreate `json:"included"`
}

// AppCustomProductPageIncludedCreate is an inline custom product page version or localization.
type AppCustomProductPageIncludedCreate struct {
	Type          ResourceType                                          `json:"type"`
	ID            string                                                `json:"id"`
	Attributes    *AppCustomProductPageLocalizationCreateAttributes     `json:"attributes,omitempty"`
	Relationships *AppCustomProductPageVersionInlineCreateRelationships `json:"relationships,omitempty"`
}

// AppCustomProductPageVersionInlineCreateRelationships describes inline version relationships.
type AppCustomProductPageVersionInlineCreateRelationships struct {
	AppCustomProductPageLocalizations RelationshipList `json:"appCustomProductPageLocalizations"`
}

// AppCustomProductPageDeleteResult represents CLI output for custom page deletions.
type AppCustomProductPageDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// AppCustomProductPageVersionsResponse is the response from custom product page version list endpoints.
type AppCustomProductPageVersionsResponse = Response[AppCustomProductPageVersionAttributes]

// AppCustomProductPageVersionCreateAttributes describes create payload attributes.
type AppCustomProductPageVersionCreateAttributes struct {
	DeepLink string `json:"deepLink,omitempty"`
}

// AppCustomProductPageVersionCreateRelationships describes create relationships.
type AppCustomProductPageVersionCreateRelationships struct {
	AppCustomProductPage *Relationship `json:"appCustomProductPage"`
}

// AppCustomProductPageVersionCreateData is the data payload for create requests.
type AppCustomProductPageVersionCreateData struct {
	Type          ResourceType                                    `json:"type"`
	Attributes    AppCustomProductPageVersionCreateAttributes     `json:"attributes"`
	Relationships *AppCustomProductPageVersionCreateRelationships `json:"relationships"`
}

// AppCustomProductPageLocalizationsResponse is the response from custom product page localization list endpoints.
type AppCustomProductPageLocalizationsResponse = Response[AppCustomProductPageLocalizationAttributes]

// AppKeywordAttributes describes an app keyword resource.
type AppKeywordAttributes struct{}

// AppKeywordsResponse is the response from app keyword list endpoints.
type AppKeywordsResponse = Response[AppKeywordAttributes]

// AppCustomProductPageLocalizationCreateAttributes describes create payload attributes.
type AppCustomProductPageLocalizationCreateAttributes struct {
	Locale          string `json:"locale"`
	PromotionalText string `json:"promotionalText,omitempty"`
}

// AppCustomProductPageLocalizationCreateRelationships describes create relationships.
type AppCustomProductPageLocalizationCreateRelationships struct {
	AppCustomProductPageVersion *Relationship `json:"appCustomProductPageVersion"`
}

// AppCustomProductPageLocalizationCreateData is the data payload for create requests.
type AppCustomProductPageLocalizationCreateData struct {
	Type          ResourceType                                         `json:"type"`
	Attributes    AppCustomProductPageLocalizationCreateAttributes     `json:"attributes"`
	Relationships *AppCustomProductPageLocalizationCreateRelationships `json:"relationships"`
}

// AppCustomProductPageLocalizationDeleteResult represents CLI output for custom page localization deletions.
type AppCustomProductPageLocalizationDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// GetAppCustomProductPages retrieves custom product pages for an app.
func (c *Client) GetAppCustomProductPages(ctx context.Context, appID string, opts ...AppCustomProductPagesOption) (*AppCustomProductPagesResponse, error) {
	query := applyOptions(opts)
	appID = strings.TrimSpace(appID)
	if query.nextURL == "" && appID == "" {
		return nil, fmt.Errorf("appID is required")
	}
	return getJSON[AppCustomProductPagesResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/apps/%s/appCustomProductPages", appID),
		Query:       buildAppCustomProductPagesQuery(query),
		Next:        query.nextURL,
		NextContext: "appCustomProductPages",
	})
}

// CreateAppCustomProductPage creates a custom product page with an initial version and
// a localization that uses the app's primary locale.
func (c *Client) CreateAppCustomProductPage(ctx context.Context, appID, name string, template AppCustomProductPageTemplate) (*AppCustomProductPageResponse, error) {
	const initialVersionID = "${new-appCustomProductPageVersion-id}"
	const initialLocalizationID = "${new-appCustomProductPageLocalization-id}"

	appID = strings.TrimSpace(appID)
	name = strings.TrimSpace(name)
	if appID == "" {
		return nil, fmt.Errorf("appID is required")
	}
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}

	app, err := c.GetApp(ctx, appID)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve app primary locale: %w", err)
	}
	primaryLocale := strings.TrimSpace(app.Data.Attributes.PrimaryLocale)
	if primaryLocale == "" {
		return nil, fmt.Errorf("app primary locale is required")
	}

	payload := AppCustomProductPageCreateRequest{
		Data: AppCustomProductPageCreateData{
			Type:       ResourceTypeAppCustomProductPages,
			Attributes: AppCustomProductPageCreateAttributes{Name: name},
			Relationships: &AppCustomProductPageCreateRelationships{
				App: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeApps,
						ID:   appID,
					},
				},
				AppCustomProductPageVersions: RelationshipList{Data: []ResourceData{{
					Type: ResourceTypeAppCustomProductPageVersions,
					ID:   initialVersionID,
				}}},
			},
		},
		Included: []AppCustomProductPageIncludedCreate{{
			Type: ResourceTypeAppCustomProductPageVersions,
			ID:   initialVersionID,
			Relationships: &AppCustomProductPageVersionInlineCreateRelationships{
				AppCustomProductPageLocalizations: RelationshipList{Data: []ResourceData{{
					Type: ResourceTypeAppCustomProductPageLocalizations,
					ID:   initialLocalizationID,
				}}},
			},
		}, {
			Type: ResourceTypeAppCustomProductPageLocalizations,
			ID:   initialLocalizationID,
			Attributes: &AppCustomProductPageLocalizationCreateAttributes{
				Locale: primaryLocale,
			},
		}},
	}
	if template.AppStoreVersionID != "" {
		payload.Data.Relationships.AppStoreVersionTemplate = &Relationship{Data: ResourceData{Type: ResourceTypeAppStoreVersions, ID: template.AppStoreVersionID}}
	}
	if template.CustomPageID != "" {
		payload.Data.Relationships.CustomProductPageTemplate = &Relationship{Data: ResourceData{Type: ResourceTypeAppCustomProductPages, ID: template.CustomPageID}}
	}
	return sendJSON[AppCustomProductPageResponse](ctx, c, writeRequest{Method: "POST", Path: "/v1/appCustomProductPages", Payload: payload})
}

// UpdateAppCustomProductPage updates a custom product page.
func (c *Client) UpdateAppCustomProductPage(ctx context.Context, pageID string, attrs AppCustomProductPageUpdateAttributes) (*AppCustomProductPageResponse, error) {
	pageID = strings.TrimSpace(pageID)
	if pageID == "" {
		return nil, fmt.Errorf("pageID is required")
	}

	payload := AppCustomProductPageUpdateRequest{
		Data: AppCustomProductPageUpdateData{
			Type: ResourceTypeAppCustomProductPages,
			ID:   pageID,
		},
	}
	if attrs.Name != nil || attrs.Visible != nil {
		payload.Data.Attributes = &attrs
	}
	return sendJSON[AppCustomProductPageResponse](ctx, c, writeRequest{Method: "PATCH", Path: fmt.Sprintf("/v1/appCustomProductPages/%s", pageID), Payload: payload})
}

// CreateAppCustomProductPageVersion creates a custom product page version.
func (c *Client) CreateAppCustomProductPageVersion(ctx context.Context, pageID, deepLink string) (*AppCustomProductPageVersionResponse, error) {
	pageID = strings.TrimSpace(pageID)
	deepLink = strings.TrimSpace(deepLink)
	if pageID == "" {
		return nil, fmt.Errorf("pageID is required")
	}

	payload := AppCustomProductPageVersionCreateRequest{
		Data: AppCustomProductPageVersionCreateData{
			Type:       ResourceTypeAppCustomProductPageVersions,
			Attributes: AppCustomProductPageVersionCreateAttributes{DeepLink: deepLink},
			Relationships: &AppCustomProductPageVersionCreateRelationships{
				AppCustomProductPage: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeAppCustomProductPages,
						ID:   pageID,
					},
				},
			},
		},
	}
	return sendJSON[AppCustomProductPageVersionResponse](ctx, c, writeRequest{Method: "POST", Path: "/v1/appCustomProductPageVersions", Payload: payload})
}

// UpdateAppCustomProductPageVersion updates a custom product page version.
func (c *Client) UpdateAppCustomProductPageVersion(ctx context.Context, versionID string, attrs AppCustomProductPageVersionUpdateAttributes) (*AppCustomProductPageVersionResponse, error) {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return nil, fmt.Errorf("versionID is required")
	}

	payload := AppCustomProductPageVersionUpdateRequest{
		Data: AppCustomProductPageVersionUpdateData{
			Type: ResourceTypeAppCustomProductPageVersions,
			ID:   versionID,
		},
	}
	if attrs.DeepLink != nil {
		payload.Data.Attributes = &attrs
	}
	return sendJSON[AppCustomProductPageVersionResponse](ctx, c, writeRequest{Method: "PATCH", Path: fmt.Sprintf("/v1/appCustomProductPageVersions/%s", versionID), Payload: payload})
}

// GetAppCustomProductPageLocalization retrieves a custom product page localization by ID.
func (c *Client) GetAppCustomProductPageLocalization(ctx context.Context, localizationID string, opts ...LocalizationReadOption) (*AppCustomProductPageLocalizationResponse, error) {
	localizationID = strings.TrimSpace(localizationID)
	if localizationID == "" {
		return nil, fmt.Errorf("localizationID is required")
	}
	return getJSON[AppCustomProductPageLocalizationResponse](ctx, c, readRequest{Path: localizationReadPath(fmt.Sprintf("/v1/appCustomProductPageLocalizations/%s", localizationID), opts)})
}

// CreateAppCustomProductPageLocalization creates a custom product page localization.
func (c *Client) CreateAppCustomProductPageLocalization(ctx context.Context, versionID, locale, promotionalText string) (*AppCustomProductPageLocalizationResponse, error) {
	versionID = strings.TrimSpace(versionID)
	locale = strings.TrimSpace(locale)
	promotionalText = strings.TrimSpace(promotionalText)
	if versionID == "" {
		return nil, fmt.Errorf("versionID is required")
	}
	if locale == "" {
		return nil, fmt.Errorf("locale is required")
	}

	payload := AppCustomProductPageLocalizationCreateRequest{
		Data: AppCustomProductPageLocalizationCreateData{
			Type:       ResourceTypeAppCustomProductPageLocalizations,
			Attributes: AppCustomProductPageLocalizationCreateAttributes{Locale: locale, PromotionalText: promotionalText},
			Relationships: &AppCustomProductPageLocalizationCreateRelationships{
				AppCustomProductPageVersion: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeAppCustomProductPageVersions,
						ID:   versionID,
					},
				},
			},
		},
	}
	return sendJSON[AppCustomProductPageLocalizationResponse](ctx, c, writeRequest{Method: "POST", Path: "/v1/appCustomProductPageLocalizations", Payload: payload})
}

// UpdateAppCustomProductPageLocalization updates a custom product page localization.
func (c *Client) UpdateAppCustomProductPageLocalization(ctx context.Context, localizationID string, attrs AppCustomProductPageLocalizationUpdateAttributes) (*AppCustomProductPageLocalizationResponse, error) {
	localizationID = strings.TrimSpace(localizationID)
	if localizationID == "" {
		return nil, fmt.Errorf("localizationID is required")
	}

	payload := AppCustomProductPageLocalizationUpdateRequest{
		Data: AppCustomProductPageLocalizationUpdateData{
			Type: ResourceTypeAppCustomProductPageLocalizations,
			ID:   localizationID,
		},
	}
	if attrs.PromotionalText != nil {
		payload.Data.Attributes = &attrs
	}
	return sendJSON[AppCustomProductPageLocalizationResponse](ctx, c, writeRequest{Method: "PATCH", Path: fmt.Sprintf("/v1/appCustomProductPageLocalizations/%s", localizationID), Payload: payload})
}

// AddAppCustomProductPageLocalizationSearchKeywords adds search keywords to a localization.
func (c *Client) AddAppCustomProductPageLocalizationSearchKeywords(ctx context.Context, localizationID string, keywords []string) error {
	localizationID = strings.TrimSpace(localizationID)
	keywords = normalizeList(keywords)
	if localizationID == "" {
		return fmt.Errorf("localizationID is required")
	}
	if len(keywords) == 0 {
		return fmt.Errorf("keywords are required")
	}

	payload := RelationshipRequest{
		Data: make([]RelationshipData, 0, len(keywords)),
	}
	for _, keyword := range keywords {
		payload.Data = append(payload.Data, RelationshipData{
			Type: ResourceTypeAppKeywords,
			ID:   keyword,
		})
	}
	return sendNoContent(ctx, c, "POST", fmt.Sprintf("/v1/appCustomProductPageLocalizations/%s/relationships/searchKeywords", localizationID), payload)
}

// DeleteAppCustomProductPageLocalizationSearchKeywords removes search keywords from a localization.
func (c *Client) DeleteAppCustomProductPageLocalizationSearchKeywords(ctx context.Context, localizationID string, keywords []string) error {
	localizationID = strings.TrimSpace(localizationID)
	keywords = normalizeList(keywords)
	if localizationID == "" {
		return fmt.Errorf("localizationID is required")
	}
	if len(keywords) == 0 {
		return fmt.Errorf("keywords are required")
	}

	payload := RelationshipRequest{
		Data: make([]RelationshipData, 0, len(keywords)),
	}
	for _, keyword := range keywords {
		payload.Data = append(payload.Data, RelationshipData{
			Type: ResourceTypeAppKeywords,
			ID:   keyword,
		})
	}
	return sendNoContent(ctx, c, "DELETE", fmt.Sprintf("/v1/appCustomProductPageLocalizations/%s/relationships/searchKeywords", localizationID), payload)
}

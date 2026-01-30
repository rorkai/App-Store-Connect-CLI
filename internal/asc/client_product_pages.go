package asc

import (
	"context"
	"encoding/json"
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

// AppCustomProductPageResponse is the response from custom product page endpoints.
type AppCustomProductPageResponse = SingleResponse[AppCustomProductPageAttributes]

// AppCustomProductPageCreateAttributes describes create payload attributes.
type AppCustomProductPageCreateAttributes struct {
	Name string `json:"name"`
}

// AppCustomProductPageCreateRelationships describes create relationships.
type AppCustomProductPageCreateRelationships struct {
	App *Relationship `json:"app"`
}

// AppCustomProductPageCreateData is the data payload for create requests.
type AppCustomProductPageCreateData struct {
	Type          ResourceType                             `json:"type"`
	Attributes    AppCustomProductPageCreateAttributes     `json:"attributes"`
	Relationships *AppCustomProductPageCreateRelationships `json:"relationships"`
}

// AppCustomProductPageCreateRequest is a request to create a custom product page.
type AppCustomProductPageCreateRequest struct {
	Data AppCustomProductPageCreateData `json:"data"`
}

// AppCustomProductPageUpdateAttributes describes update payload attributes.
type AppCustomProductPageUpdateAttributes struct {
	Name    *string `json:"name,omitempty"`
	Visible *bool   `json:"visible,omitempty"`
}

// AppCustomProductPageUpdateData is the data payload for update requests.
type AppCustomProductPageUpdateData struct {
	Type       ResourceType                          `json:"type"`
	ID         string                                `json:"id"`
	Attributes *AppCustomProductPageUpdateAttributes `json:"attributes,omitempty"`
}

// AppCustomProductPageUpdateRequest is a request to update a custom product page.
type AppCustomProductPageUpdateRequest struct {
	Data AppCustomProductPageUpdateData `json:"data"`
}

// AppCustomProductPageDeleteResult represents CLI output for custom page deletions.
type AppCustomProductPageDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// AppCustomProductPageVersionAttributes describes custom product page version attributes.
type AppCustomProductPageVersionAttributes struct {
	Version  string `json:"version,omitempty"`
	State    string `json:"state,omitempty"`
	DeepLink string `json:"deepLink,omitempty"`
}

// AppCustomProductPageVersionsResponse is the response from custom product page version list endpoints.
type AppCustomProductPageVersionsResponse = Response[AppCustomProductPageVersionAttributes]

// AppCustomProductPageVersionResponse is the response from custom product page version endpoints.
type AppCustomProductPageVersionResponse = SingleResponse[AppCustomProductPageVersionAttributes]

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
	Attributes    AppCustomProductPageVersionCreateAttributes     `json:"attributes,omitempty"`
	Relationships *AppCustomProductPageVersionCreateRelationships `json:"relationships"`
}

// AppCustomProductPageVersionCreateRequest is a request to create a custom product page version.
type AppCustomProductPageVersionCreateRequest struct {
	Data AppCustomProductPageVersionCreateData `json:"data"`
}

// AppCustomProductPageVersionUpdateAttributes describes update payload attributes.
type AppCustomProductPageVersionUpdateAttributes struct {
	DeepLink *string `json:"deepLink,omitempty"`
}

// AppCustomProductPageVersionUpdateData is the data payload for update requests.
type AppCustomProductPageVersionUpdateData struct {
	Type       ResourceType                                 `json:"type"`
	ID         string                                       `json:"id"`
	Attributes *AppCustomProductPageVersionUpdateAttributes `json:"attributes,omitempty"`
}

// AppCustomProductPageVersionUpdateRequest is a request to update a custom product page version.
type AppCustomProductPageVersionUpdateRequest struct {
	Data AppCustomProductPageVersionUpdateData `json:"data"`
}

// AppCustomProductPageLocalizationAttributes describes custom product page localization attributes.
type AppCustomProductPageLocalizationAttributes struct {
	Locale          string `json:"locale,omitempty"`
	PromotionalText string `json:"promotionalText,omitempty"`
}

// AppCustomProductPageLocalizationsResponse is the response from custom product page localization list endpoints.
type AppCustomProductPageLocalizationsResponse = Response[AppCustomProductPageLocalizationAttributes]

// AppCustomProductPageLocalizationResponse is the response from custom product page localization endpoints.
type AppCustomProductPageLocalizationResponse = SingleResponse[AppCustomProductPageLocalizationAttributes]

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

// AppCustomProductPageLocalizationCreateRequest is a request to create a custom product page localization.
type AppCustomProductPageLocalizationCreateRequest struct {
	Data AppCustomProductPageLocalizationCreateData `json:"data"`
}

// AppCustomProductPageLocalizationUpdateAttributes describes update payload attributes.
type AppCustomProductPageLocalizationUpdateAttributes struct {
	PromotionalText *string `json:"promotionalText,omitempty"`
}

// AppCustomProductPageLocalizationUpdateData is the data payload for update requests.
type AppCustomProductPageLocalizationUpdateData struct {
	Type       ResourceType                                      `json:"type"`
	ID         string                                            `json:"id"`
	Attributes *AppCustomProductPageLocalizationUpdateAttributes `json:"attributes,omitempty"`
}

// AppCustomProductPageLocalizationUpdateRequest is a request to update a custom product page localization.
type AppCustomProductPageLocalizationUpdateRequest struct {
	Data AppCustomProductPageLocalizationUpdateData `json:"data"`
}

// AppCustomProductPageLocalizationDeleteResult represents CLI output for custom page localization deletions.
type AppCustomProductPageLocalizationDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// GetAppCustomProductPages retrieves custom product pages for an app.
func (c *Client) GetAppCustomProductPages(ctx context.Context, appID string, opts ...AppCustomProductPagesOption) (*AppCustomProductPagesResponse, error) {
	query := &appCustomProductPagesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	appID = strings.TrimSpace(appID)
	if query.nextURL == "" && appID == "" {
		return nil, fmt.Errorf("appID is required")
	}
	path := fmt.Sprintf("/v1/apps/%s/appCustomProductPages", appID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("appCustomProductPages: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildAppCustomProductPagesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppCustomProductPagesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppCustomProductPage retrieves a custom product page by ID.
func (c *Client) GetAppCustomProductPage(ctx context.Context, pageID string) (*AppCustomProductPageResponse, error) {
	pageID = strings.TrimSpace(pageID)
	if pageID == "" {
		return nil, fmt.Errorf("pageID is required")
	}
	data, err := c.do(ctx, "GET", fmt.Sprintf("/v1/appCustomProductPages/%s", pageID), nil)
	if err != nil {
		return nil, err
	}

	var response AppCustomProductPageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateAppCustomProductPage creates a custom product page.
func (c *Client) CreateAppCustomProductPage(ctx context.Context, appID, name string) (*AppCustomProductPageResponse, error) {
	appID = strings.TrimSpace(appID)
	name = strings.TrimSpace(name)
	if appID == "" {
		return nil, fmt.Errorf("appID is required")
	}
	if name == "" {
		return nil, fmt.Errorf("name is required")
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
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "POST", "/v1/appCustomProductPages", body)
	if err != nil {
		return nil, err
	}

	var response AppCustomProductPageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
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

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "PATCH", fmt.Sprintf("/v1/appCustomProductPages/%s", pageID), body)
	if err != nil {
		return nil, err
	}

	var response AppCustomProductPageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteAppCustomProductPage deletes a custom product page.
func (c *Client) DeleteAppCustomProductPage(ctx context.Context, pageID string) error {
	pageID = strings.TrimSpace(pageID)
	if pageID == "" {
		return fmt.Errorf("pageID is required")
	}
	_, err := c.do(ctx, "DELETE", fmt.Sprintf("/v1/appCustomProductPages/%s", pageID), nil)
	return err
}

// GetAppCustomProductPageVersions retrieves versions for a custom product page.
func (c *Client) GetAppCustomProductPageVersions(ctx context.Context, pageID string, opts ...AppCustomProductPageVersionsOption) (*AppCustomProductPageVersionsResponse, error) {
	query := &appCustomProductPageVersionsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	pageID = strings.TrimSpace(pageID)
	if query.nextURL == "" && pageID == "" {
		return nil, fmt.Errorf("pageID is required")
	}
	path := fmt.Sprintf("/v1/appCustomProductPages/%s/appCustomProductPageVersions", pageID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("appCustomProductPageVersions: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildAppCustomProductPageVersionsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppCustomProductPageVersionsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppCustomProductPageVersion retrieves a custom product page version by ID.
func (c *Client) GetAppCustomProductPageVersion(ctx context.Context, versionID string) (*AppCustomProductPageVersionResponse, error) {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return nil, fmt.Errorf("versionID is required")
	}
	data, err := c.do(ctx, "GET", fmt.Sprintf("/v1/appCustomProductPageVersions/%s", versionID), nil)
	if err != nil {
		return nil, err
	}

	var response AppCustomProductPageVersionResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
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

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "POST", "/v1/appCustomProductPageVersions", body)
	if err != nil {
		return nil, err
	}

	var response AppCustomProductPageVersionResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
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

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "PATCH", fmt.Sprintf("/v1/appCustomProductPageVersions/%s", versionID), body)
	if err != nil {
		return nil, err
	}

	var response AppCustomProductPageVersionResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppCustomProductPageLocalizations retrieves custom product page localizations for a version.
func (c *Client) GetAppCustomProductPageLocalizations(ctx context.Context, versionID string, opts ...AppCustomProductPageLocalizationsOption) (*AppCustomProductPageLocalizationsResponse, error) {
	query := &appCustomProductPageLocalizationsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	versionID = strings.TrimSpace(versionID)
	if query.nextURL == "" && versionID == "" {
		return nil, fmt.Errorf("versionID is required")
	}
	path := fmt.Sprintf("/v1/appCustomProductPageVersions/%s/appCustomProductPageLocalizations", versionID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("appCustomProductPageLocalizations: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildAppCustomProductPageLocalizationsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppCustomProductPageLocalizationsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppCustomProductPageLocalization retrieves a custom product page localization by ID.
func (c *Client) GetAppCustomProductPageLocalization(ctx context.Context, localizationID string) (*AppCustomProductPageLocalizationResponse, error) {
	localizationID = strings.TrimSpace(localizationID)
	if localizationID == "" {
		return nil, fmt.Errorf("localizationID is required")
	}
	data, err := c.do(ctx, "GET", fmt.Sprintf("/v1/appCustomProductPageLocalizations/%s", localizationID), nil)
	if err != nil {
		return nil, err
	}

	var response AppCustomProductPageLocalizationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
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

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "POST", "/v1/appCustomProductPageLocalizations", body)
	if err != nil {
		return nil, err
	}

	var response AppCustomProductPageLocalizationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
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

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "PATCH", fmt.Sprintf("/v1/appCustomProductPageLocalizations/%s", localizationID), body)
	if err != nil {
		return nil, err
	}

	var response AppCustomProductPageLocalizationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteAppCustomProductPageLocalization deletes a custom product page localization.
func (c *Client) DeleteAppCustomProductPageLocalization(ctx context.Context, localizationID string) error {
	localizationID = strings.TrimSpace(localizationID)
	if localizationID == "" {
		return fmt.Errorf("localizationID is required")
	}
	_, err := c.do(ctx, "DELETE", fmt.Sprintf("/v1/appCustomProductPageLocalizations/%s", localizationID), nil)
	return err
}

// GetAppCustomProductPageLocalizationSearchKeywords retrieves search keywords for a localization.
func (c *Client) GetAppCustomProductPageLocalizationSearchKeywords(ctx context.Context, localizationID string) (*AppKeywordsResponse, error) {
	localizationID = strings.TrimSpace(localizationID)
	if localizationID == "" {
		return nil, fmt.Errorf("localizationID is required")
	}
	path := fmt.Sprintf("/v1/appCustomProductPageLocalizations/%s/searchKeywords", localizationID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppKeywordsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
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

	body, err := BuildRequestBody(payload)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/v1/appCustomProductPageLocalizations/%s/relationships/searchKeywords", localizationID)
	_, err = c.do(ctx, "POST", path, body)
	return err
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

	body, err := BuildRequestBody(payload)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/v1/appCustomProductPageLocalizations/%s/relationships/searchKeywords", localizationID)
	_, err = c.do(ctx, "DELETE", path, body)
	return err
}

// GetAppCustomProductPageLocalizationPreviewSets retrieves preview sets for a localization.
func (c *Client) GetAppCustomProductPageLocalizationPreviewSets(ctx context.Context, localizationID string, opts ...AppCustomProductPageLocalizationPreviewSetsOption) (*AppPreviewSetsResponse, error) {
	query := &appCustomProductPageLocalizationPreviewSetsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	localizationID = strings.TrimSpace(localizationID)
	if query.nextURL == "" && localizationID == "" {
		return nil, fmt.Errorf("localizationID is required")
	}
	path := fmt.Sprintf("/v1/appCustomProductPageLocalizations/%s/appPreviewSets", localizationID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("appPreviewSets: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildAppCustomProductPageLocalizationPreviewSetsQuery(query); queryString != "" {
		path += "?" + queryString
	}
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppPreviewSetsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppCustomProductPageLocalizationScreenshotSets retrieves screenshot sets for a localization.
func (c *Client) GetAppCustomProductPageLocalizationScreenshotSets(ctx context.Context, localizationID string, opts ...AppCustomProductPageLocalizationScreenshotSetsOption) (*AppScreenshotSetsResponse, error) {
	query := &appCustomProductPageLocalizationScreenshotSetsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	localizationID = strings.TrimSpace(localizationID)
	if query.nextURL == "" && localizationID == "" {
		return nil, fmt.Errorf("localizationID is required")
	}
	path := fmt.Sprintf("/v1/appCustomProductPageLocalizations/%s/appScreenshotSets", localizationID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("appScreenshotSets: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildAppCustomProductPageLocalizationScreenshotSetsQuery(query); queryString != "" {
		path += "?" + queryString
	}
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppScreenshotSetsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

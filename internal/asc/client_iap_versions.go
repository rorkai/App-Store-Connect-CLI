package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type (
	IAPVersionsOption       func(*iapVersionsQuery)
	IAPVersionRelatedOption func(*iapVersionRelatedQuery)
)

type iapVersionsQuery struct {
	listQuery
	states             []string
	include            []string
	versionFields      []string
	iapFields          []string
	imageFields        []string
	localizationFields []string
	imagesLimit        int
	localizationsLimit int
}

type iapVersionRelatedQuery struct {
	listQuery
	include       []string
	fields        []string
	versionFields []string
}

func WithIAPVersionsLimit(limit int) IAPVersionsOption {
	return func(q *iapVersionsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

func WithIAPVersionsNextURL(next string) IAPVersionsOption {
	return func(q *iapVersionsQuery) { q.nextURL = strings.TrimSpace(next) }
}

func WithIAPVersionsStates(states []string) IAPVersionsOption {
	return func(q *iapVersionsQuery) { q.states = normalizeUniqueList(states) }
}

func WithIAPVersionsInclude(include []string) IAPVersionsOption {
	return func(q *iapVersionsQuery) { q.include = normalizeUniqueList(include) }
}

func WithIAPVersionsFields(fields []string) IAPVersionsOption {
	return func(q *iapVersionsQuery) { q.versionFields = normalizeUniqueList(fields) }
}

func WithIAPVersionsIAPFields(fields []string) IAPVersionsOption {
	return func(q *iapVersionsQuery) { q.iapFields = normalizeUniqueList(fields) }
}

func WithIAPVersionsImageFields(fields []string) IAPVersionsOption {
	return func(q *iapVersionsQuery) { q.imageFields = normalizeUniqueList(fields) }
}

func WithIAPVersionsLocalizationFields(fields []string) IAPVersionsOption {
	return func(q *iapVersionsQuery) { q.localizationFields = normalizeUniqueList(fields) }
}

func WithIAPVersionsImagesLimit(limit int) IAPVersionsOption {
	return func(q *iapVersionsQuery) {
		if limit > 0 {
			q.imagesLimit = limit
		}
	}
}

func WithIAPVersionsLocalizationsLimit(limit int) IAPVersionsOption {
	return func(q *iapVersionsQuery) {
		if limit > 0 {
			q.localizationsLimit = limit
		}
	}
}

func WithIAPVersionRelatedLimit(limit int) IAPVersionRelatedOption {
	return func(q *iapVersionRelatedQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

func WithIAPVersionRelatedNextURL(next string) IAPVersionRelatedOption {
	return func(q *iapVersionRelatedQuery) { q.nextURL = strings.TrimSpace(next) }
}

func WithIAPVersionRelatedInclude(include []string) IAPVersionRelatedOption {
	return func(q *iapVersionRelatedQuery) { q.include = normalizeUniqueList(include) }
}

func WithIAPVersionRelatedFields(fields []string) IAPVersionRelatedOption {
	return func(q *iapVersionRelatedQuery) { q.fields = normalizeUniqueList(fields) }
}

func WithIAPVersionRelatedVersionFields(fields []string) IAPVersionRelatedOption {
	return func(q *iapVersionRelatedQuery) { q.versionFields = normalizeUniqueList(fields) }
}

func buildIAPVersionsQuery(q *iapVersionsQuery, includeListControls bool) string {
	values := url.Values{}
	if includeListControls {
		addLimit(values, q.limit)
		addCSV(values, "filter[state]", q.states)
	}
	addCSV(values, "fields[inAppPurchaseVersions]", q.versionFields)
	addCSV(values, "fields[inAppPurchases]", q.iapFields)
	addCSV(values, "fields[inAppPurchaseImages]", q.imageFields)
	addCSV(values, "fields[inAppPurchaseLocalizations]", q.localizationFields)
	addCSV(values, "include", q.include)
	if q.imagesLimit > 0 {
		values.Set("limit[images]", strconv.Itoa(q.imagesLimit))
	}
	if q.localizationsLimit > 0 {
		values.Set("limit[localizations]", strconv.Itoa(q.localizationsLimit))
	}
	return values.Encode()
}

func buildIAPVersionRelatedQuery(q *iapVersionRelatedQuery, fieldKey string) string {
	values := url.Values{}
	addLimit(values, q.limit)
	addCSV(values, fieldKey, q.fields)
	addCSV(values, "fields[inAppPurchaseVersions]", q.versionFields)
	addCSV(values, "include", q.include)
	return values.Encode()
}

func (c *Client) CreateInAppPurchaseVersion(ctx context.Context, iapID string) (*InAppPurchaseVersionResponse, error) {
	iapID = strings.TrimSpace(iapID)
	if iapID == "" {
		return nil, fmt.Errorf("iapID is required")
	}
	payload := InAppPurchaseVersionCreateRequest{Data: InAppPurchaseVersionCreateData{
		Type:          ResourceTypeInAppPurchaseVersions,
		Relationships: InAppPurchaseVersionCreateRelationships{InAppPurchase: Relationship{Data: ResourceData{Type: ResourceTypeInAppPurchases, ID: iapID}}},
	}}
	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}
	data, err := c.do(ctx, http.MethodPost, "/v1/inAppPurchaseVersions", body)
	if err != nil {
		return nil, err
	}
	var response InAppPurchaseVersionResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

func (c *Client) GetInAppPurchaseVersion(ctx context.Context, versionID string, opts ...IAPVersionsOption) (*InAppPurchaseVersionResponse, error) {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return nil, fmt.Errorf("versionID is required")
	}
	q := &iapVersionsQuery{}
	for _, opt := range opts {
		opt(q)
	}
	path := fmt.Sprintf("/v1/inAppPurchaseVersions/%s", versionID)
	if query := buildIAPVersionsQuery(q, false); query != "" {
		path += "?" + query
	}
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var response InAppPurchaseVersionResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

func (c *Client) GetInAppPurchaseVersions(ctx context.Context, iapID string, opts ...IAPVersionsOption) (*InAppPurchaseVersionsResponse, error) {
	q := &iapVersionsQuery{}
	for _, opt := range opts {
		opt(q)
	}
	iapID = strings.TrimSpace(iapID)
	if q.nextURL == "" && iapID == "" {
		return nil, fmt.Errorf("iapID is required")
	}
	path := fmt.Sprintf("/v2/inAppPurchases/%s/versions", iapID)
	if q.nextURL != "" {
		if err := validateNextURL(q.nextURL); err != nil {
			return nil, fmt.Errorf("in-app-purchase-versions: %w", err)
		}
		path = q.nextURL
	} else if query := buildIAPVersionsQuery(q, true); query != "" {
		path += "?" + query
	}
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var response InAppPurchaseVersionsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

func (c *Client) GetInAppPurchaseVersionsRelationships(ctx context.Context, iapID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getResourceLinkages(ctx, iapID, "versions", "iapID", "/v2/inAppPurchases/%s/relationships/%s", "inAppPurchaseVersionsRelationships", opts...)
}

func (c *Client) GetInAppPurchaseVersionImage(ctx context.Context, versionID string, opts ...IAPVersionRelatedOption) (*InAppPurchaseImageV2Response, error) {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return nil, fmt.Errorf("versionID is required")
	}
	q := &iapVersionRelatedQuery{}
	for _, opt := range opts {
		opt(q)
	}
	path := fmt.Sprintf("/v1/inAppPurchaseVersions/%s/image", versionID)
	if query := buildIAPVersionRelatedQuery(q, "fields[inAppPurchaseImages]"); query != "" {
		path += "?" + query
	}
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var response InAppPurchaseImageV2Response
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

func (c *Client) GetInAppPurchaseVersionImageRelationship(ctx context.Context, versionID string) (*InAppPurchaseVersionImageLinkageResponse, error) {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return nil, fmt.Errorf("versionID is required")
	}
	data, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/v1/inAppPurchaseVersions/%s/relationships/image", versionID), nil)
	if err != nil {
		return nil, err
	}
	var response InAppPurchaseVersionImageLinkageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

func (c *Client) getIAPVersionRelated(ctx context.Context, versionID, relationship, fieldKey string, target any, opts ...IAPVersionRelatedOption) error {
	q := &iapVersionRelatedQuery{}
	for _, opt := range opts {
		opt(q)
	}
	versionID = strings.TrimSpace(versionID)
	if q.nextURL == "" && versionID == "" {
		return fmt.Errorf("versionID is required")
	}
	path := fmt.Sprintf("/v1/inAppPurchaseVersions/%s/%s", versionID, relationship)
	if q.nextURL != "" {
		if err := validateNextURL(q.nextURL); err != nil {
			return fmt.Errorf("in-app-purchase-version-%s: %w", relationship, err)
		}
		path = q.nextURL
	} else if query := buildIAPVersionRelatedQuery(q, fieldKey); query != "" {
		path += "?" + query
	}
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}
	return nil
}

func (c *Client) GetInAppPurchaseVersionImages(ctx context.Context, versionID string, opts ...IAPVersionRelatedOption) (*InAppPurchaseImagesV2Response, error) {
	var response InAppPurchaseImagesV2Response
	if err := c.getIAPVersionRelated(ctx, versionID, "images", "fields[inAppPurchaseImages]", &response, opts...); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) GetInAppPurchaseVersionImagesRelationships(ctx context.Context, versionID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getResourceLinkages(ctx, versionID, "images", "versionID", "/v1/inAppPurchaseVersions/%s/relationships/%s", "inAppPurchaseVersionImagesRelationships", opts...)
}

func (c *Client) GetInAppPurchaseVersionLocalizations(ctx context.Context, versionID string, opts ...IAPVersionRelatedOption) (*InAppPurchaseLocalizationsResponse, error) {
	var response InAppPurchaseLocalizationsResponse
	if err := c.getIAPVersionRelated(ctx, versionID, "localizations", "fields[inAppPurchaseLocalizations]", &response, opts...); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) GetInAppPurchaseVersionLocalizationsRelationships(ctx context.Context, versionID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getResourceLinkages(ctx, versionID, "localizations", "versionID", "/v1/inAppPurchaseVersions/%s/relationships/%s", "inAppPurchaseVersionLocalizationsRelationships", opts...)
}

func (c *Client) CreateInAppPurchaseLocalizationV2(ctx context.Context, versionID string, attrs InAppPurchaseLocalizationCreateAttributes) (*InAppPurchaseLocalizationResponse, error) {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return nil, fmt.Errorf("versionID is required")
	}
	attrs.Name = strings.TrimSpace(attrs.Name)
	attrs.Locale = strings.TrimSpace(attrs.Locale)
	if attrs.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if attrs.Locale == "" {
		return nil, fmt.Errorf("locale is required")
	}
	payload := InAppPurchaseLocalizationV2CreateRequest{Data: InAppPurchaseLocalizationV2CreateData{Type: ResourceTypeInAppPurchaseLocalizations, Attributes: attrs, Relationships: InAppPurchaseLocalizationV2CreateRelationships{Version: Relationship{Data: ResourceData{Type: ResourceTypeInAppPurchaseVersions, ID: versionID}}}}}
	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}
	data, err := c.do(ctx, http.MethodPost, "/v2/inAppPurchaseLocalizations", body)
	if err != nil {
		return nil, err
	}
	var response InAppPurchaseLocalizationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

func (c *Client) GetInAppPurchaseLocalizationV2(ctx context.Context, localizationID string, opts ...IAPVersionRelatedOption) (*InAppPurchaseLocalizationResponse, error) {
	localizationID = strings.TrimSpace(localizationID)
	if localizationID == "" {
		return nil, fmt.Errorf("localizationID is required")
	}
	q := &iapVersionRelatedQuery{}
	for _, opt := range opts {
		opt(q)
	}
	path := fmt.Sprintf("/v2/inAppPurchaseLocalizations/%s", localizationID)
	if query := buildIAPVersionRelatedQuery(q, "fields[inAppPurchaseLocalizations]"); query != "" {
		path += "?" + query
	}
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var response InAppPurchaseLocalizationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

func (c *Client) UpdateInAppPurchaseLocalizationV2(ctx context.Context, localizationID string, attrs InAppPurchaseLocalizationUpdateAttributes) (*InAppPurchaseLocalizationResponse, error) {
	localizationID = strings.TrimSpace(localizationID)
	if localizationID == "" {
		return nil, fmt.Errorf("localizationID is required")
	}
	payload := InAppPurchaseLocalizationV2UpdateRequest{Data: InAppPurchaseLocalizationV2UpdateData{Type: ResourceTypeInAppPurchaseLocalizations, ID: localizationID}}
	if attrs.Name != nil || attrs.Description != nil {
		payload.Data.Attributes = &attrs
	}
	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}
	data, err := c.do(ctx, http.MethodPatch, fmt.Sprintf("/v2/inAppPurchaseLocalizations/%s", localizationID), body)
	if err != nil {
		return nil, err
	}
	var response InAppPurchaseLocalizationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

func (c *Client) DeleteInAppPurchaseLocalizationV2(ctx context.Context, localizationID string) error {
	localizationID = strings.TrimSpace(localizationID)
	if localizationID == "" {
		return fmt.Errorf("localizationID is required")
	}
	_, err := c.do(ctx, http.MethodDelete, fmt.Sprintf("/v2/inAppPurchaseLocalizations/%s", localizationID), nil)
	return err
}

func (c *Client) CreateInAppPurchaseImageV2(ctx context.Context, versionID, fileName string, fileSize int64) (*InAppPurchaseImageV2Response, error) {
	versionID = strings.TrimSpace(versionID)
	fileName = strings.TrimSpace(fileName)
	if versionID == "" {
		return nil, fmt.Errorf("versionID is required")
	}
	if fileName == "" {
		return nil, fmt.Errorf("fileName is required")
	}
	if fileSize <= 0 {
		return nil, fmt.Errorf("fileSize is required")
	}
	payload := InAppPurchaseImageV2CreateRequest{Data: InAppPurchaseImageV2CreateData{Type: ResourceTypeInAppPurchaseImages, Attributes: InAppPurchaseImageV2CreateAttributes{FileName: fileName, FileSize: fileSize}, Relationships: InAppPurchaseImageV2CreateRelationships{Version: Relationship{Data: ResourceData{Type: ResourceTypeInAppPurchaseVersions, ID: versionID}}}}}
	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}
	data, err := c.do(ctx, http.MethodPost, "/v2/inAppPurchaseImages", body)
	if err != nil {
		return nil, err
	}
	var response InAppPurchaseImageV2Response
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

func (c *Client) GetInAppPurchaseImageV2(ctx context.Context, imageID string, opts ...IAPVersionRelatedOption) (*InAppPurchaseImageV2Response, error) {
	imageID = strings.TrimSpace(imageID)
	if imageID == "" {
		return nil, fmt.Errorf("imageID is required")
	}
	q := &iapVersionRelatedQuery{}
	for _, opt := range opts {
		opt(q)
	}
	path := fmt.Sprintf("/v2/inAppPurchaseImages/%s", imageID)
	if query := buildIAPVersionRelatedQuery(q, "fields[inAppPurchaseImages]"); query != "" {
		path += "?" + query
	}
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var response InAppPurchaseImageV2Response
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

func (c *Client) UpdateInAppPurchaseImageV2(ctx context.Context, imageID string, attrs InAppPurchaseImageV2UpdateAttributes) (*InAppPurchaseImageV2Response, error) {
	imageID = strings.TrimSpace(imageID)
	if imageID == "" {
		return nil, fmt.Errorf("imageID is required")
	}
	payload := InAppPurchaseImageV2UpdateRequest{Data: InAppPurchaseImageV2UpdateData{Type: ResourceTypeInAppPurchaseImages, ID: imageID}}
	if attrs.Uploaded != nil {
		payload.Data.Attributes = &attrs
	}
	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}
	data, err := c.do(ctx, http.MethodPatch, fmt.Sprintf("/v2/inAppPurchaseImages/%s", imageID), body)
	if err != nil {
		return nil, err
	}
	var response InAppPurchaseImageV2Response
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

func (c *Client) DeleteInAppPurchaseImageV2(ctx context.Context, imageID string) error {
	imageID = strings.TrimSpace(imageID)
	if imageID == "" {
		return fmt.Errorf("imageID is required")
	}
	_, err := c.do(ctx, http.MethodDelete, fmt.Sprintf("/v2/inAppPurchaseImages/%s", imageID), nil)
	return err
}

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
	IAPVersionsOption             func(*iapVersionsQuery)
	IAPVersionGetOption           func(*iapVersionGetQuery)
	IAPVersionImageOption         func(*iapImageFieldsQuery)
	IAPVersionImagesOption        func(*iapVersionImagesQuery)
	IAPVersionLocalizationsOption func(*iapVersionLocalizationsQuery)
	IAPLocalizationV2Option       func(*iapLocalizationV2Query)
	IAPImageV2Option              func(*iapImageFieldsQuery)
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

type iapVersionGetQuery struct {
	versionFields      []string
	iapFields          []string
	imageFields        []string
	localizationFields []string
	include            []string
	imagesLimit        int
	localizationsLimit int
}

type iapImageFieldsQuery struct {
	fields []string
}

type iapVersionImagesQuery struct {
	listQuery
	fields []string
}

type iapVersionLocalizationsQuery struct {
	listQuery
	localizationFields []string
	versionFields      []string
	include            []string
}

type iapLocalizationV2Query struct {
	localizationFields []string
	versionFields      []string
	include            []string
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

func WithIAPVersionGetFields(fields []string) IAPVersionGetOption {
	return func(q *iapVersionGetQuery) { q.versionFields = normalizeUniqueList(fields) }
}

func WithIAPVersionGetIAPFields(fields []string) IAPVersionGetOption {
	return func(q *iapVersionGetQuery) { q.iapFields = normalizeUniqueList(fields) }
}

func WithIAPVersionGetImageFields(fields []string) IAPVersionGetOption {
	return func(q *iapVersionGetQuery) { q.imageFields = normalizeUniqueList(fields) }
}

func WithIAPVersionGetLocalizationFields(fields []string) IAPVersionGetOption {
	return func(q *iapVersionGetQuery) { q.localizationFields = normalizeUniqueList(fields) }
}

func WithIAPVersionGetInclude(include []string) IAPVersionGetOption {
	return func(q *iapVersionGetQuery) { q.include = normalizeUniqueList(include) }
}

func WithIAPVersionGetImagesLimit(limit int) IAPVersionGetOption {
	return func(q *iapVersionGetQuery) {
		if limit > 0 {
			q.imagesLimit = limit
		}
	}
}

func WithIAPVersionGetLocalizationsLimit(limit int) IAPVersionGetOption {
	return func(q *iapVersionGetQuery) {
		if limit > 0 {
			q.localizationsLimit = limit
		}
	}
}

func WithIAPVersionImageFields(fields []string) IAPVersionImageOption {
	return func(q *iapImageFieldsQuery) { q.fields = normalizeUniqueList(fields) }
}

func WithIAPVersionImagesLimit(limit int) IAPVersionImagesOption {
	return func(q *iapVersionImagesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

func WithIAPVersionImagesNextURL(next string) IAPVersionImagesOption {
	return func(q *iapVersionImagesQuery) { q.nextURL = strings.TrimSpace(next) }
}

func WithIAPVersionImagesFields(fields []string) IAPVersionImagesOption {
	return func(q *iapVersionImagesQuery) { q.fields = normalizeUniqueList(fields) }
}

func WithIAPVersionLocalizationsLimit(limit int) IAPVersionLocalizationsOption {
	return func(q *iapVersionLocalizationsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

func WithIAPVersionLocalizationsNextURL(next string) IAPVersionLocalizationsOption {
	return func(q *iapVersionLocalizationsQuery) { q.nextURL = strings.TrimSpace(next) }
}

func WithIAPVersionLocalizationsFields(fields []string) IAPVersionLocalizationsOption {
	return func(q *iapVersionLocalizationsQuery) { q.localizationFields = normalizeUniqueList(fields) }
}

func WithIAPVersionLocalizationsVersionFields(fields []string) IAPVersionLocalizationsOption {
	return func(q *iapVersionLocalizationsQuery) { q.versionFields = normalizeUniqueList(fields) }
}

func WithIAPVersionLocalizationsInclude(include []string) IAPVersionLocalizationsOption {
	return func(q *iapVersionLocalizationsQuery) { q.include = normalizeUniqueList(include) }
}

func WithIAPLocalizationV2Fields(fields []string) IAPLocalizationV2Option {
	return func(q *iapLocalizationV2Query) { q.localizationFields = normalizeUniqueList(fields) }
}

func WithIAPLocalizationV2VersionFields(fields []string) IAPLocalizationV2Option {
	return func(q *iapLocalizationV2Query) { q.versionFields = normalizeUniqueList(fields) }
}

func WithIAPLocalizationV2Include(include []string) IAPLocalizationV2Option {
	return func(q *iapLocalizationV2Query) { q.include = normalizeUniqueList(include) }
}

func WithIAPImageV2Fields(fields []string) IAPImageV2Option {
	return func(q *iapImageFieldsQuery) { q.fields = normalizeUniqueList(fields) }
}

func buildIAPVersionsQuery(q *iapVersionsQuery) string {
	values := url.Values{}
	addLimit(values, q.limit)
	addCSV(values, "filter[state]", q.states)
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

func buildIAPVersionGetQuery(q *iapVersionGetQuery) string {
	values := url.Values{}
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

func buildIAPImageFieldsQuery(q *iapImageFieldsQuery) string {
	values := url.Values{}
	addCSV(values, "fields[inAppPurchaseImages]", q.fields)
	return values.Encode()
}

func buildIAPVersionImagesQuery(q *iapVersionImagesQuery) string {
	values := url.Values{}
	addLimit(values, q.limit)
	addCSV(values, "fields[inAppPurchaseImages]", q.fields)
	return values.Encode()
}

func buildIAPVersionLocalizationsQuery(q *iapVersionLocalizationsQuery) string {
	values := url.Values{}
	addLimit(values, q.limit)
	addCSV(values, "fields[inAppPurchaseLocalizations]", q.localizationFields)
	addCSV(values, "fields[inAppPurchaseVersions]", q.versionFields)
	addCSV(values, "include", q.include)
	return values.Encode()
}

func buildIAPLocalizationV2Query(q *iapLocalizationV2Query) string {
	values := url.Values{}
	addCSV(values, "fields[inAppPurchaseLocalizations]", q.localizationFields)
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

func (c *Client) GetInAppPurchaseVersion(ctx context.Context, versionID string, opts ...IAPVersionGetOption) (*InAppPurchaseVersionResponse, error) {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return nil, fmt.Errorf("versionID is required")
	}
	q := &iapVersionGetQuery{}
	for _, opt := range opts {
		opt(q)
	}
	path := fmt.Sprintf("/v1/inAppPurchaseVersions/%s", versionID)
	if query := buildIAPVersionGetQuery(q); query != "" {
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
	} else if query := buildIAPVersionsQuery(q); query != "" {
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

func (c *Client) GetInAppPurchaseVersionImage(ctx context.Context, versionID string, opts ...IAPVersionImageOption) (*InAppPurchaseImageV2Response, error) {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return nil, fmt.Errorf("versionID is required")
	}
	q := &iapImageFieldsQuery{}
	for _, opt := range opts {
		opt(q)
	}
	path := fmt.Sprintf("/v1/inAppPurchaseVersions/%s/image", versionID)
	if query := buildIAPImageFieldsQuery(q); query != "" {
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

func (c *Client) GetInAppPurchaseVersionImages(ctx context.Context, versionID string, opts ...IAPVersionImagesOption) (*InAppPurchaseImagesV2Response, error) {
	q := &iapVersionImagesQuery{}
	for _, opt := range opts {
		opt(q)
	}
	versionID = strings.TrimSpace(versionID)
	if q.nextURL == "" && versionID == "" {
		return nil, fmt.Errorf("versionID is required")
	}
	path := fmt.Sprintf("/v1/inAppPurchaseVersions/%s/images", versionID)
	if q.nextURL != "" {
		if err := validateNextURL(q.nextURL); err != nil {
			return nil, fmt.Errorf("in-app-purchase-version-images: %w", err)
		}
		path = q.nextURL
	} else if query := buildIAPVersionImagesQuery(q); query != "" {
		path += "?" + query
	}
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var response InAppPurchaseImagesV2Response
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

func (c *Client) GetInAppPurchaseVersionImagesRelationships(ctx context.Context, versionID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getResourceLinkages(ctx, versionID, "images", "versionID", "/v1/inAppPurchaseVersions/%s/relationships/%s", "inAppPurchaseVersionImagesRelationships", opts...)
}

func (c *Client) GetInAppPurchaseVersionLocalizations(ctx context.Context, versionID string, opts ...IAPVersionLocalizationsOption) (*InAppPurchaseLocalizationsResponse, error) {
	q := &iapVersionLocalizationsQuery{}
	for _, opt := range opts {
		opt(q)
	}
	versionID = strings.TrimSpace(versionID)
	if q.nextURL == "" && versionID == "" {
		return nil, fmt.Errorf("versionID is required")
	}
	path := fmt.Sprintf("/v1/inAppPurchaseVersions/%s/localizations", versionID)
	if q.nextURL != "" {
		if err := validateNextURL(q.nextURL); err != nil {
			return nil, fmt.Errorf("in-app-purchase-version-localizations: %w", err)
		}
		path = q.nextURL
	} else if query := buildIAPVersionLocalizationsQuery(q); query != "" {
		path += "?" + query
	}
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var response InAppPurchaseLocalizationsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
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

func (c *Client) GetInAppPurchaseLocalizationV2(ctx context.Context, localizationID string, opts ...IAPLocalizationV2Option) (*InAppPurchaseLocalizationResponse, error) {
	localizationID = strings.TrimSpace(localizationID)
	if localizationID == "" {
		return nil, fmt.Errorf("localizationID is required")
	}
	q := &iapLocalizationV2Query{}
	for _, opt := range opts {
		opt(q)
	}
	path := fmt.Sprintf("/v2/inAppPurchaseLocalizations/%s", localizationID)
	if query := buildIAPLocalizationV2Query(q); query != "" {
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

func (c *Client) GetInAppPurchaseImageV2(ctx context.Context, imageID string, opts ...IAPImageV2Option) (*InAppPurchaseImageV2Response, error) {
	imageID = strings.TrimSpace(imageID)
	if imageID == "" {
		return nil, fmt.Errorf("imageID is required")
	}
	q := &iapImageFieldsQuery{}
	for _, opt := range opts {
		opt(q)
	}
	path := fmt.Sprintf("/v2/inAppPurchaseImages/%s", imageID)
	if query := buildIAPImageFieldsQuery(q); query != "" {
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

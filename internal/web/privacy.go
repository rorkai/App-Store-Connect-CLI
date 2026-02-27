package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const appDataUsagesInclude = "category,purpose,dataProtection"

const (
	appDataUsageCategoriesInclude = "grouping"
	defaultAppDataUsagePageLimit  = "500"
	defaultCatalogPageLimit       = "200"
)

// DataUsageTuple is the normalized tuple used to create/manage app data usages.
type DataUsageTuple struct {
	Category       string `json:"category,omitempty"`
	Purpose        string `json:"purpose,omitempty"`
	DataProtection string `json:"dataProtection"`
}

// AppDataUsage models one appDataUsages resource.
type AppDataUsage struct {
	ID             string `json:"id"`
	Category       string `json:"category,omitempty"`
	Purpose        string `json:"purpose,omitempty"`
	DataProtection string `json:"dataProtection,omitempty"`
}

// AppDataUsagesPublishState captures publication state for app privacy data usages.
type AppDataUsagesPublishState struct {
	ID        string `json:"id"`
	Published bool   `json:"published"`
}

// AppDataUsageCategory models one appDataUsageCategories resource.
type AppDataUsageCategory struct {
	ID       string `json:"id"`
	Deleted  bool   `json:"deleted,omitempty"`
	Grouping string `json:"grouping,omitempty"`
}

// AppDataUsagePurpose models one appDataUsagePurposes resource.
type AppDataUsagePurpose struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted,omitempty"`
}

// AppDataUsageDataProtection models one appDataUsageDataProtections resource.
type AppDataUsageDataProtection struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted,omitempty"`
}

func decodeAppDataUsageResource(resource jsonAPIResource) AppDataUsage {
	usage := AppDataUsage{
		ID: strings.TrimSpace(resource.ID),
	}
	if ref := firstRelationshipRef(resource, "category"); ref != nil {
		usage.Category = strings.TrimSpace(ref.ID)
	}
	if ref := firstRelationshipRef(resource, "purpose"); ref != nil {
		usage.Purpose = strings.TrimSpace(ref.ID)
	}
	if ref := firstRelationshipRef(resource, "dataProtection"); ref != nil {
		usage.DataProtection = strings.TrimSpace(ref.ID)
	}
	if usage.DataProtection == "" {
		usage.DataProtection = stringAttr(
			resource.Attributes,
			"appDataUsageDataProtection",
			"appDataUsageDataProtectionId",
		)
	}
	return usage
}

func decodeAppDataUsages(resources []jsonAPIResource) []AppDataUsage {
	if len(resources) == 0 {
		return []AppDataUsage{}
	}
	result := make([]AppDataUsage, 0, len(resources))
	for _, resource := range resources {
		result = append(result, decodeAppDataUsageResource(resource))
	}
	return result
}

func decodeAppDataUsagesPublishStateResource(resource jsonAPIResource) AppDataUsagesPublishState {
	return AppDataUsagesPublishState{
		ID:        strings.TrimSpace(resource.ID),
		Published: boolAttr(resource.Attributes, "published"),
	}
}

func decodeAppDataUsageCategoryResource(resource jsonAPIResource) AppDataUsageCategory {
	category := AppDataUsageCategory{
		ID:      strings.TrimSpace(resource.ID),
		Deleted: boolAttr(resource.Attributes, "deleted"),
	}
	if ref := firstRelationshipRef(resource, "grouping"); ref != nil {
		category.Grouping = strings.TrimSpace(ref.ID)
	}
	return category
}

func decodeAppDataUsagePurposeResource(resource jsonAPIResource) AppDataUsagePurpose {
	return AppDataUsagePurpose{
		ID:      strings.TrimSpace(resource.ID),
		Deleted: boolAttr(resource.Attributes, "deleted"),
	}
}

func decodeAppDataUsageDataProtectionResource(resource jsonAPIResource) AppDataUsageDataProtection {
	return AppDataUsageDataProtection{
		ID:      strings.TrimSpace(resource.ID),
		Deleted: boolAttr(resource.Attributes, "deleted"),
	}
}

func extractNextLink(links map[string]any) (string, error) {
	if len(links) == 0 {
		return "", nil
	}
	raw, ok := links["next"]
	if !ok || raw == nil {
		return "", nil
	}

	switch value := raw.(type) {
	case string:
		return strings.TrimSpace(value), nil
	case map[string]any:
		if href, ok := value["href"].(string); ok {
			return strings.TrimSpace(href), nil
		}
		if urlValue, ok := value["url"].(string); ok {
			return strings.TrimSpace(urlValue), nil
		}
		return "", fmt.Errorf("next link object does not contain href/url")
	default:
		return "", fmt.Errorf("unsupported next link type %T", raw)
	}
}

func normalizeNextPath(nextLink, baseURL string) (string, error) {
	nextLink = strings.TrimSpace(nextLink)
	if nextLink == "" {
		return "", nil
	}
	baseURLParsed, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid base url: %w", err)
	}
	nextURL, err := url.Parse(nextLink)
	if err != nil {
		return "", fmt.Errorf("invalid next link: %w", err)
	}

	var nextPath string
	if nextURL.IsAbs() {
		if !strings.EqualFold(nextURL.Scheme, baseURLParsed.Scheme) || !strings.EqualFold(nextURL.Host, baseURLParsed.Host) {
			return "", fmt.Errorf("next link host %q does not match client host %q", nextURL.Host, baseURLParsed.Host)
		}
		nextPath = nextURL.EscapedPath()
	} else {
		nextPath = nextURL.EscapedPath()
		if nextPath == "" {
			nextPath = "/"
		}
	}

	basePath := strings.TrimSuffix(baseURLParsed.EscapedPath(), "/")
	if basePath != "" && strings.HasPrefix(nextPath, basePath) {
		nextPath = strings.TrimPrefix(nextPath, basePath)
	}
	if nextPath == "" {
		nextPath = "/"
	}
	if !strings.HasPrefix(nextPath, "/") {
		nextPath = "/" + nextPath
	}
	if nextURL.RawQuery != "" {
		nextPath = nextPath + "?" + nextURL.RawQuery
	}
	return nextPath, nil
}

func (c *Client) listPaginatedResources(ctx context.Context, path, responseName string) ([]jsonAPIResource, error) {
	nextPath := strings.TrimSpace(path)
	if nextPath == "" {
		return nil, fmt.Errorf("%s path is required", responseName)
	}

	allResources := make([]jsonAPIResource, 0, 128)
	visited := map[string]struct{}{}

	for nextPath != "" {
		if _, seen := visited[nextPath]; seen {
			return nil, fmt.Errorf("%s pagination loop detected", responseName)
		}
		visited[nextPath] = struct{}{}

		responseBody, err := c.doRequest(ctx, http.MethodGet, nextPath, nil)
		if err != nil {
			return nil, err
		}

		var payload jsonAPIListPayload
		if err := json.Unmarshal(responseBody, &payload); err != nil {
			return nil, fmt.Errorf("failed to parse %s response: %w", responseName, err)
		}
		allResources = append(allResources, payload.Data...)

		nextLink, err := extractNextLink(payload.Links)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s pagination links: %w", responseName, err)
		}
		if strings.TrimSpace(nextLink) == "" {
			nextPath = ""
			continue
		}

		nextPath, err = normalizeNextPath(nextLink, c.baseURL)
		if err != nil {
			return nil, fmt.Errorf("failed to normalize %s pagination link: %w", responseName, err)
		}
	}

	return allResources, nil
}

func normalizeDataUsageTuple(tuple DataUsageTuple) (DataUsageTuple, error) {
	tuple.Category = strings.TrimSpace(tuple.Category)
	tuple.Purpose = strings.TrimSpace(tuple.Purpose)
	tuple.DataProtection = strings.TrimSpace(tuple.DataProtection)
	if tuple.DataProtection == "" {
		return DataUsageTuple{}, fmt.Errorf("data protection is required")
	}
	return tuple, nil
}

func dataUsageRelationshipsForTuple(appID string, tuple DataUsageTuple, includeApp bool) map[string]any {
	relationships := map[string]any{
		"dataProtection": map[string]any{
			"data": map[string]string{
				"type": "appDataUsageDataProtections",
				"id":   tuple.DataProtection,
			},
		},
	}
	if includeApp {
		relationships["app"] = map[string]any{
			"data": map[string]string{
				"type": "apps",
				"id":   appID,
			},
		}
	}
	if tuple.Category != "" {
		relationships["category"] = map[string]any{
			"data": map[string]string{
				"type": "appDataUsageCategories",
				"id":   tuple.Category,
			},
		}
	}
	if tuple.Purpose != "" {
		relationships["purpose"] = map[string]any{
			"data": map[string]string{
				"type": "appDataUsagePurposes",
				"id":   tuple.Purpose,
			},
		}
	}
	return relationships
}

// ListAppDataUsages lists data usage tuples for a specific app.
func (c *Client) ListAppDataUsages(ctx context.Context, appID string) ([]AppDataUsage, error) {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return nil, fmt.Errorf("app id is required")
	}
	query := url.Values{}
	query.Set("include", appDataUsagesInclude)
	query.Set("limit", defaultAppDataUsagePageLimit)
	path := queryPath("/apps/"+url.PathEscape(appID)+"/dataUsages", query)
	resources, err := c.listPaginatedResources(ctx, path, "app data usages")
	if err != nil {
		return nil, err
	}
	return decodeAppDataUsages(resources), nil
}

// ListAppDataUsageCategories lists available data usage category tokens.
func (c *Client) ListAppDataUsageCategories(ctx context.Context) ([]AppDataUsageCategory, error) {
	query := url.Values{}
	query.Set("include", appDataUsageCategoriesInclude)
	query.Set("limit", defaultCatalogPageLimit)
	path := queryPath("/appDataUsageCategories", query)

	resources, err := c.listPaginatedResources(ctx, path, "app data usage categories")
	if err != nil {
		return nil, err
	}
	categories := make([]AppDataUsageCategory, 0, len(resources))
	for _, resource := range resources {
		categories = append(categories, decodeAppDataUsageCategoryResource(resource))
	}
	return categories, nil
}

// ListAppDataUsagePurposes lists available data usage purpose tokens.
func (c *Client) ListAppDataUsagePurposes(ctx context.Context) ([]AppDataUsagePurpose, error) {
	query := url.Values{}
	query.Set("limit", defaultCatalogPageLimit)
	path := queryPath("/appDataUsagePurposes", query)

	resources, err := c.listPaginatedResources(ctx, path, "app data usage purposes")
	if err != nil {
		return nil, err
	}
	purposes := make([]AppDataUsagePurpose, 0, len(resources))
	for _, resource := range resources {
		purposes = append(purposes, decodeAppDataUsagePurposeResource(resource))
	}
	return purposes, nil
}

// ListAppDataUsageDataProtections lists available data usage data protection tokens.
func (c *Client) ListAppDataUsageDataProtections(ctx context.Context) ([]AppDataUsageDataProtection, error) {
	query := url.Values{}
	query.Set("limit", defaultCatalogPageLimit)
	path := queryPath("/appDataUsageDataProtections", query)

	resources, err := c.listPaginatedResources(ctx, path, "app data usage data protections")
	if err != nil {
		return nil, err
	}
	protections := make([]AppDataUsageDataProtection, 0, len(resources))
	for _, resource := range resources {
		protections = append(protections, decodeAppDataUsageDataProtectionResource(resource))
	}
	return protections, nil
}

// CreateAppDataUsage creates one data usage tuple for an app.
func (c *Client) CreateAppDataUsage(ctx context.Context, appID string, tuple DataUsageTuple) (*AppDataUsage, error) {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return nil, fmt.Errorf("app id is required")
	}
	normalized, err := normalizeDataUsageTuple(tuple)
	if err != nil {
		return nil, err
	}

	requestBody := map[string]any{
		"data": map[string]any{
			"type":          "appDataUsages",
			"relationships": dataUsageRelationshipsForTuple(appID, normalized, true),
		},
	}
	responseBody, err := c.doRequest(ctx, http.MethodPost, "/appDataUsages", requestBody)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Data jsonAPIResource `json:"data"`
	}
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse create app data usage response: %w", err)
	}
	usage := decodeAppDataUsageResource(payload.Data)
	return &usage, nil
}

// UpdateAppDataUsage updates one appDataUsages resource to a target tuple.
func (c *Client) UpdateAppDataUsage(ctx context.Context, appDataUsageID string, tuple DataUsageTuple) (*AppDataUsage, error) {
	appDataUsageID = strings.TrimSpace(appDataUsageID)
	if appDataUsageID == "" {
		return nil, fmt.Errorf("app data usage id is required")
	}
	normalized, err := normalizeDataUsageTuple(tuple)
	if err != nil {
		return nil, err
	}

	requestBody := map[string]any{
		"data": map[string]any{
			"type": "appDataUsages",
			"id":   appDataUsageID,
			"relationships": dataUsageRelationshipsForTuple(
				"",
				normalized,
				false,
			),
		},
	}
	responseBody, err := c.doRequest(
		ctx,
		http.MethodPatch,
		"/appDataUsages/"+url.PathEscape(appDataUsageID),
		requestBody,
	)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Data jsonAPIResource `json:"data"`
	}
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse update app data usage response: %w", err)
	}
	usage := decodeAppDataUsageResource(payload.Data)
	return &usage, nil
}

// DeleteAppDataUsage deletes one appDataUsages resource.
func (c *Client) DeleteAppDataUsage(ctx context.Context, appDataUsageID string) error {
	appDataUsageID = strings.TrimSpace(appDataUsageID)
	if appDataUsageID == "" {
		return fmt.Errorf("app data usage id is required")
	}
	_, err := c.doRequest(ctx, http.MethodDelete, "/appDataUsages/"+url.PathEscape(appDataUsageID), nil)
	return err
}

// GetAppDataUsagesPublishState fetches publication state for app data usages.
func (c *Client) GetAppDataUsagesPublishState(ctx context.Context, appID string) (*AppDataUsagesPublishState, error) {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return nil, fmt.Errorf("app id is required")
	}
	path := "/apps/" + url.PathEscape(appID) + "/dataUsagePublishState"
	responseBody, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Data jsonAPIResource `json:"data"`
	}
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse data usage publish state response: %w", err)
	}
	state := decodeAppDataUsagesPublishStateResource(payload.Data)
	return &state, nil
}

// SetAppDataUsagesPublished updates publication state for app data usages.
func (c *Client) SetAppDataUsagesPublished(ctx context.Context, publishStateID string, published bool) (*AppDataUsagesPublishState, error) {
	publishStateID = strings.TrimSpace(publishStateID)
	if publishStateID == "" {
		return nil, fmt.Errorf("publish state id is required")
	}

	requestBody := map[string]any{
		"data": map[string]any{
			"type": "appDataUsagesPublishState",
			"id":   publishStateID,
			"attributes": map[string]bool{
				"published": published,
			},
		},
	}
	path := "/appDataUsagesPublishState/" + url.PathEscape(publishStateID)
	responseBody, err := c.doRequest(ctx, http.MethodPatch, path, requestBody)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Data jsonAPIResource `json:"data"`
	}
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse publish state update response: %w", err)
	}
	state := decodeAppDataUsagesPublishStateResource(payload.Data)
	return &state, nil
}

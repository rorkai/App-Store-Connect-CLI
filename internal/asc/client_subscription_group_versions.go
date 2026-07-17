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
	SubscriptionGroupVersionsOption             func(*subscriptionGroupVersionsQuery)
	SubscriptionGroupVersionLocalizationsOption func(*subscriptionGroupVersionLocalizationsQuery)
)

type subscriptionGroupVersionsQuery struct {
	listQuery
	states             []string
	include            []string
	versionFields      []string
	groupFields        []string
	localizationFields []string
	localizationsLimit int
}

type subscriptionGroupVersionLocalizationsQuery struct {
	listQuery
	include       []string
	fields        []string
	versionFields []string
}

func WithSubscriptionGroupVersionsLimit(limit int) SubscriptionGroupVersionsOption {
	return func(q *subscriptionGroupVersionsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

func WithSubscriptionGroupVersionsNextURL(next string) SubscriptionGroupVersionsOption {
	return func(q *subscriptionGroupVersionsQuery) { q.nextURL = strings.TrimSpace(next) }
}

func WithSubscriptionGroupVersionsStates(states []string) SubscriptionGroupVersionsOption {
	return func(q *subscriptionGroupVersionsQuery) { q.states = normalizeUniqueList(states) }
}

func WithSubscriptionGroupVersionsInclude(include []string) SubscriptionGroupVersionsOption {
	return func(q *subscriptionGroupVersionsQuery) { q.include = normalizeUniqueList(include) }
}

func WithSubscriptionGroupVersionsFields(fields []string) SubscriptionGroupVersionsOption {
	return func(q *subscriptionGroupVersionsQuery) { q.versionFields = normalizeUniqueList(fields) }
}

func WithSubscriptionGroupVersionsGroupFields(fields []string) SubscriptionGroupVersionsOption {
	return func(q *subscriptionGroupVersionsQuery) { q.groupFields = normalizeUniqueList(fields) }
}

func WithSubscriptionGroupVersionsLocalizationFields(fields []string) SubscriptionGroupVersionsOption {
	return func(q *subscriptionGroupVersionsQuery) { q.localizationFields = normalizeUniqueList(fields) }
}

func WithSubscriptionGroupVersionsLocalizationsLimit(limit int) SubscriptionGroupVersionsOption {
	return func(q *subscriptionGroupVersionsQuery) {
		if limit > 0 {
			q.localizationsLimit = limit
		}
	}
}

func WithSubscriptionGroupVersionLocalizationsLimit(limit int) SubscriptionGroupVersionLocalizationsOption {
	return func(q *subscriptionGroupVersionLocalizationsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

func WithSubscriptionGroupVersionLocalizationsNextURL(next string) SubscriptionGroupVersionLocalizationsOption {
	return func(q *subscriptionGroupVersionLocalizationsQuery) { q.nextURL = strings.TrimSpace(next) }
}

func WithSubscriptionGroupVersionLocalizationsInclude(include []string) SubscriptionGroupVersionLocalizationsOption {
	return func(q *subscriptionGroupVersionLocalizationsQuery) { q.include = normalizeUniqueList(include) }
}

func WithSubscriptionGroupVersionLocalizationsFields(fields []string) SubscriptionGroupVersionLocalizationsOption {
	return func(q *subscriptionGroupVersionLocalizationsQuery) { q.fields = normalizeUniqueList(fields) }
}

func WithSubscriptionGroupVersionLocalizationsVersionFields(fields []string) SubscriptionGroupVersionLocalizationsOption {
	return func(q *subscriptionGroupVersionLocalizationsQuery) { q.versionFields = normalizeUniqueList(fields) }
}

func buildSubscriptionGroupVersionsQuery(q *subscriptionGroupVersionsQuery, list bool) string {
	values := url.Values{}
	if list {
		addLimit(values, q.limit)
		addCSV(values, "filter[state]", q.states)
	}
	addCSV(values, "fields[subscriptionGroupVersions]", q.versionFields)
	addCSV(values, "fields[subscriptionGroups]", q.groupFields)
	addCSV(values, "fields[subscriptionGroupLocalizations]", q.localizationFields)
	addCSV(values, "include", q.include)
	if q.localizationsLimit > 0 {
		values.Set("limit[localizations]", strconv.Itoa(q.localizationsLimit))
	}
	return values.Encode()
}

func buildSubscriptionGroupVersionLocalizationsQuery(q *subscriptionGroupVersionLocalizationsQuery, list bool) string {
	values := url.Values{}
	if list {
		addLimit(values, q.limit)
	}
	addCSV(values, "fields[subscriptionGroupLocalizations]", q.fields)
	addCSV(values, "fields[subscriptionGroupVersions]", q.versionFields)
	addCSV(values, "include", q.include)
	return values.Encode()
}

// CreateSubscriptionGroupVersion creates a discrete version owned by a group.
func (c *Client) CreateSubscriptionGroupVersion(ctx context.Context, groupID string) (*SubscriptionGroupVersionResponse, error) {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return nil, fmt.Errorf("groupID is required")
	}
	payload := SubscriptionGroupVersionCreateRequest{Data: SubscriptionGroupVersionCreateData{
		Type: ResourceTypeSubscriptionGroupVersions,
		Relationships: SubscriptionGroupVersionCreateRelationships{SubscriptionGroup: Relationship{Data: ResourceData{
			Type: ResourceTypeSubscriptionGroups,
			ID:   groupID,
		}}},
	}}
	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}
	data, err := c.do(ctx, http.MethodPost, "/v1/subscriptionGroupVersions", body)
	if err != nil {
		return nil, err
	}
	var response SubscriptionGroupVersionResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

// GetSubscriptionGroupVersion retrieves a version by ID.
func (c *Client) GetSubscriptionGroupVersion(ctx context.Context, versionID string, opts ...SubscriptionGroupVersionsOption) (*SubscriptionGroupVersionResponse, error) {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return nil, fmt.Errorf("versionID is required")
	}
	q := &subscriptionGroupVersionsQuery{}
	for _, opt := range opts {
		opt(q)
	}
	if q.limit > 0 {
		return nil, fmt.Errorf("limit is only supported when listing subscription group versions")
	}
	if q.nextURL != "" {
		return nil, fmt.Errorf("next URL is only supported when listing subscription group versions")
	}
	if len(q.states) > 0 {
		return nil, fmt.Errorf("state filter is only supported when listing subscription group versions")
	}
	path := fmt.Sprintf("/v1/subscriptionGroupVersions/%s", versionID)
	if query := buildSubscriptionGroupVersionsQuery(q, false); query != "" {
		path += "?" + query
	}
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var response SubscriptionGroupVersionResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

// GetSubscriptionGroupVersions lists versions owned by a group.
func (c *Client) GetSubscriptionGroupVersions(ctx context.Context, groupID string, opts ...SubscriptionGroupVersionsOption) (*SubscriptionGroupVersionsResponse, error) {
	q := &subscriptionGroupVersionsQuery{}
	for _, opt := range opts {
		opt(q)
	}
	groupID = strings.TrimSpace(groupID)
	if q.nextURL == "" && groupID == "" {
		return nil, fmt.Errorf("groupID is required")
	}
	path := fmt.Sprintf("/v1/subscriptionGroups/%s/versions", groupID)
	if q.nextURL != "" {
		if err := validateNextURL(q.nextURL); err != nil {
			return nil, fmt.Errorf("subscription-group-versions: %w", err)
		}
		path = q.nextURL
	} else if query := buildSubscriptionGroupVersionsQuery(q, true); query != "" {
		path += "?" + query
	}
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var response SubscriptionGroupVersionsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

// GetSubscriptionGroupVersionsRelationships retrieves raw version linkages for a group.
func (c *Client) GetSubscriptionGroupVersionsRelationships(ctx context.Context, groupID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getResourceLinkages(ctx, groupID, "versions", "groupID", "/v1/subscriptionGroups/%s/relationships/%s", "subscriptionGroupVersionsRelationships", opts...)
}

// GetSubscriptionGroupVersionLocalizations lists v2 localizations owned by a version.
func (c *Client) GetSubscriptionGroupVersionLocalizations(ctx context.Context, versionID string, opts ...SubscriptionGroupVersionLocalizationsOption) (*SubscriptionGroupLocalizationsV2Response, error) {
	q := &subscriptionGroupVersionLocalizationsQuery{}
	for _, opt := range opts {
		opt(q)
	}
	versionID = strings.TrimSpace(versionID)
	if q.nextURL == "" && versionID == "" {
		return nil, fmt.Errorf("versionID is required")
	}
	path := fmt.Sprintf("/v1/subscriptionGroupVersions/%s/localizations", versionID)
	if q.nextURL != "" {
		if err := validateNextURL(q.nextURL); err != nil {
			return nil, fmt.Errorf("subscription-group-version-localizations: %w", err)
		}
		path = q.nextURL
	} else if query := buildSubscriptionGroupVersionLocalizationsQuery(q, true); query != "" {
		path += "?" + query
	}
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var response SubscriptionGroupLocalizationsV2Response
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

// GetSubscriptionGroupVersionLocalizationsRelationships retrieves raw localization linkages.
func (c *Client) GetSubscriptionGroupVersionLocalizationsRelationships(ctx context.Context, versionID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getResourceLinkages(ctx, versionID, "localizations", "versionID", "/v1/subscriptionGroupVersions/%s/relationships/%s", "subscriptionGroupVersionLocalizationsRelationships", opts...)
}

// CreateSubscriptionGroupLocalizationV2 creates a version-scoped localization.
func (c *Client) CreateSubscriptionGroupLocalizationV2(ctx context.Context, versionID string, attrs SubscriptionGroupLocalizationV2CreateAttributes) (*SubscriptionGroupLocalizationV2Response, error) {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return nil, fmt.Errorf("versionID is required")
	}
	attrs.Name = strings.TrimSpace(attrs.Name)
	attrs.Locale = strings.TrimSpace(attrs.Locale)
	if attrs.CustomAppName != nil && attrs.CustomAppName.Value != nil {
		trimmed := strings.TrimSpace(*attrs.CustomAppName.Value)
		attrs.CustomAppName = &NullableString{Value: &trimmed}
	}
	if attrs.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if attrs.Locale == "" {
		return nil, fmt.Errorf("locale is required")
	}
	payload := SubscriptionGroupLocalizationV2CreateRequest{Data: SubscriptionGroupLocalizationV2CreateData{
		Type:       ResourceTypeSubscriptionGroupLocalizations,
		Attributes: attrs,
		Relationships: SubscriptionGroupLocalizationV2CreateRelationships{Version: Relationship{Data: ResourceData{
			Type: ResourceTypeSubscriptionGroupVersions,
			ID:   versionID,
		}}},
	}}
	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}
	data, err := c.do(ctx, http.MethodPost, "/v2/subscriptionGroupLocalizations", body)
	if err != nil {
		return nil, err
	}
	var response SubscriptionGroupLocalizationV2Response
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

// GetSubscriptionGroupLocalizationV2 retrieves a version-scoped localization.
func (c *Client) GetSubscriptionGroupLocalizationV2(ctx context.Context, localizationID string, opts ...SubscriptionGroupVersionLocalizationsOption) (*SubscriptionGroupLocalizationV2Response, error) {
	localizationID = strings.TrimSpace(localizationID)
	if localizationID == "" {
		return nil, fmt.Errorf("localizationID is required")
	}
	q := &subscriptionGroupVersionLocalizationsQuery{}
	for _, opt := range opts {
		opt(q)
	}
	if q.limit > 0 {
		return nil, fmt.Errorf("limit is only supported when listing subscription group version localizations")
	}
	if q.nextURL != "" {
		return nil, fmt.Errorf("next URL is only supported when listing subscription group version localizations")
	}
	path := fmt.Sprintf("/v2/subscriptionGroupLocalizations/%s", localizationID)
	if query := buildSubscriptionGroupVersionLocalizationsQuery(q, false); query != "" {
		path += "?" + query
	}
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var response SubscriptionGroupLocalizationV2Response
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

// UpdateSubscriptionGroupLocalizationV2 updates nullable version-scoped attributes.
func (c *Client) UpdateSubscriptionGroupLocalizationV2(ctx context.Context, localizationID string, attrs SubscriptionGroupLocalizationV2UpdateAttributes) (*SubscriptionGroupLocalizationV2Response, error) {
	localizationID = strings.TrimSpace(localizationID)
	if localizationID == "" {
		return nil, fmt.Errorf("localizationID is required")
	}
	payload := SubscriptionGroupLocalizationV2UpdateRequest{Data: SubscriptionGroupLocalizationV2UpdateData{
		Type:       ResourceTypeSubscriptionGroupLocalizations,
		ID:         localizationID,
		Attributes: &attrs,
	}}
	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}
	data, err := c.do(ctx, http.MethodPatch, fmt.Sprintf("/v2/subscriptionGroupLocalizations/%s", localizationID), body)
	if err != nil {
		return nil, err
	}
	var response SubscriptionGroupLocalizationV2Response
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

// DeleteSubscriptionGroupLocalizationV2 deletes a version-scoped localization.
func (c *Client) DeleteSubscriptionGroupLocalizationV2(ctx context.Context, localizationID string) error {
	localizationID = strings.TrimSpace(localizationID)
	if localizationID == "" {
		return fmt.Errorf("localizationID is required")
	}
	_, err := c.do(ctx, http.MethodDelete, fmt.Sprintf("/v2/subscriptionGroupLocalizations/%s", localizationID), nil)
	return err
}

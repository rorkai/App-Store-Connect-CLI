package asc

import (
	"context"
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
	return sendJSON[SubscriptionGroupVersionResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v1/subscriptionGroupVersions", Payload: payload})
}

// GetSubscriptionGroupVersion retrieves a version by ID.
func (c *Client) GetSubscriptionGroupVersion(ctx context.Context, versionID string, opts ...SubscriptionGroupVersionsOption) (*SubscriptionGroupVersionResponse, error) {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return nil, fmt.Errorf("versionID is required")
	}
	q := applyOptions(opts)
	if q.limit > 0 {
		return nil, fmt.Errorf("limit is only supported when listing subscription group versions")
	}
	if q.nextURL != "" {
		return nil, fmt.Errorf("next URL is only supported when listing subscription group versions")
	}
	if len(q.states) > 0 {
		return nil, fmt.Errorf("state filter is only supported when listing subscription group versions")
	}
	return getJSON[SubscriptionGroupVersionResponse](ctx, c, readRequest{
		Path:  fmt.Sprintf("/v1/subscriptionGroupVersions/%s", versionID),
		Query: buildSubscriptionGroupVersionsQuery(q, false),
	})
}

// GetSubscriptionGroupVersions lists versions owned by a group.
func (c *Client) GetSubscriptionGroupVersions(ctx context.Context, groupID string, opts ...SubscriptionGroupVersionsOption) (*SubscriptionGroupVersionsResponse, error) {
	q := applyOptions(opts)
	groupID = strings.TrimSpace(groupID)
	if q.nextURL == "" && groupID == "" {
		return nil, fmt.Errorf("groupID is required")
	}
	return getJSON[SubscriptionGroupVersionsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/subscriptionGroups/%s/versions", groupID),
		Query:       buildSubscriptionGroupVersionsQuery(q, true),
		Next:        q.nextURL,
		NextContext: "subscription-group-versions",
	})
}

// GetSubscriptionGroupVersionsRelationships retrieves raw version linkages for a group.
func (c *Client) GetSubscriptionGroupVersionsRelationships(ctx context.Context, groupID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getResourceLinkages(ctx, groupID, "versions", "groupID", "/v1/subscriptionGroups/%s/relationships/%s", "subscriptionGroupVersionsRelationships", opts...)
}

// GetSubscriptionGroupVersionLocalizations lists v2 localizations owned by a version.
func (c *Client) GetSubscriptionGroupVersionLocalizations(ctx context.Context, versionID string, opts ...SubscriptionGroupVersionLocalizationsOption) (*SubscriptionGroupLocalizationsV2Response, error) {
	q := applyOptions(opts)
	versionID = strings.TrimSpace(versionID)
	if q.nextURL == "" && versionID == "" {
		return nil, fmt.Errorf("versionID is required")
	}
	return getJSON[SubscriptionGroupLocalizationsV2Response](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/subscriptionGroupVersions/%s/localizations", versionID),
		Query:       buildSubscriptionGroupVersionLocalizationsQuery(q, true),
		Next:        q.nextURL,
		NextContext: "subscription-group-version-localizations",
	})
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
		if trimmed == "" {
			attrs.CustomAppName = nil
		} else {
			attrs.CustomAppName = &NullableString{Value: &trimmed}
		}
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
	return sendJSON[SubscriptionGroupLocalizationV2Response](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v2/subscriptionGroupLocalizations", Payload: payload})
}

// GetSubscriptionGroupLocalizationV2 retrieves a version-scoped localization.
func (c *Client) GetSubscriptionGroupLocalizationV2(ctx context.Context, localizationID string, opts ...SubscriptionGroupVersionLocalizationsOption) (*SubscriptionGroupLocalizationV2Response, error) {
	localizationID = strings.TrimSpace(localizationID)
	if localizationID == "" {
		return nil, fmt.Errorf("localizationID is required")
	}
	q := applyOptions(opts)
	if q.limit > 0 {
		return nil, fmt.Errorf("limit is only supported when listing subscription group version localizations")
	}
	if q.nextURL != "" {
		return nil, fmt.Errorf("next URL is only supported when listing subscription group version localizations")
	}
	return getJSON[SubscriptionGroupLocalizationV2Response](ctx, c, readRequest{
		Path:  fmt.Sprintf("/v2/subscriptionGroupLocalizations/%s", localizationID),
		Query: buildSubscriptionGroupVersionLocalizationsQuery(q, false),
	})
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
	return sendJSON[SubscriptionGroupLocalizationV2Response](ctx, c, writeRequest{Method: http.MethodPatch, Path: fmt.Sprintf("/v2/subscriptionGroupLocalizations/%s", localizationID), Payload: payload})
}

// DeleteSubscriptionGroupLocalizationV2 deletes a version-scoped localization.
func (c *Client) DeleteSubscriptionGroupLocalizationV2(ctx context.Context, localizationID string) error {
	localizationID = strings.TrimSpace(localizationID)
	if localizationID == "" {
		return fmt.Errorf("localizationID is required")
	}
	return sendNoContent(ctx, c, http.MethodDelete, fmt.Sprintf("/v2/subscriptionGroupLocalizations/%s", localizationID), nil)
}

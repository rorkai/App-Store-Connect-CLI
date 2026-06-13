package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

const ResourceTypeSubscriptionPlanAvailabilities ResourceType = "subscriptionPlanAvailabilities"

// SubscriptionPlanType identifies a subscription billing plan.
type SubscriptionPlanType string

const (
	SubscriptionPlanTypeMonthly SubscriptionPlanType = "MONTHLY"
	SubscriptionPlanTypeUpfront SubscriptionPlanType = "UPFRONT"
)

// SubscriptionPlanAvailabilityAttributes describes a subscription plan availability.
type SubscriptionPlanAvailabilityAttributes struct {
	AvailableInNewTerritories *bool                `json:"availableInNewTerritories,omitempty"`
	PlanType                  SubscriptionPlanType `json:"planType,omitempty"`
}

// SubscriptionPlanAvailabilityRelationships describes relationships for plan availability.
type SubscriptionPlanAvailabilityRelationships struct {
	Subscription         *Relationship     `json:"subscription,omitempty"`
	AvailableTerritories *RelationshipList `json:"availableTerritories,omitempty"`
}

// SubscriptionPlanAvailabilityCreateData is the data portion of plan availability create requests.
type SubscriptionPlanAvailabilityCreateData struct {
	Type          ResourceType                               `json:"type"`
	Attributes    SubscriptionPlanAvailabilityAttributes     `json:"attributes"`
	Relationships *SubscriptionPlanAvailabilityRelationships `json:"relationships"`
}

// SubscriptionPlanAvailabilityCreateRequest is a request to create plan availability.
type SubscriptionPlanAvailabilityCreateRequest struct {
	Data SubscriptionPlanAvailabilityCreateData `json:"data"`
}

// SubscriptionPlanAvailabilityUpdateData is the data portion of plan availability update requests.
type SubscriptionPlanAvailabilityUpdateData struct {
	Type          ResourceType                               `json:"type"`
	ID            string                                     `json:"id"`
	Attributes    *SubscriptionPlanAvailabilityAttributes    `json:"attributes,omitempty"`
	Relationships *SubscriptionPlanAvailabilityRelationships `json:"relationships,omitempty"`
}

// SubscriptionPlanAvailabilityUpdateRequest is a request to update plan availability.
type SubscriptionPlanAvailabilityUpdateRequest struct {
	Data SubscriptionPlanAvailabilityUpdateData `json:"data"`
}

// SubscriptionPlanAvailabilityResponse is the response from plan availability endpoints.
type SubscriptionPlanAvailabilityResponse = SingleResponse[SubscriptionPlanAvailabilityAttributes]

// SubscriptionPlanAvailabilitiesResponse is the response from plan availability list endpoints.
type SubscriptionPlanAvailabilitiesResponse = Response[SubscriptionPlanAvailabilityAttributes]

// SubscriptionPlanAvailabilitiesOption is a functional option for plan availability list endpoints.
type SubscriptionPlanAvailabilitiesOption func(*subscriptionPlanAvailabilitiesQuery)

type subscriptionPlanAvailabilitiesQuery struct {
	planTypes []SubscriptionPlanType
}

// WithSubscriptionPlanAvailabilitiesPlanTypes filters returned plan availabilities by plan type.
// The App Store Connect API does not expose filter[planType] on planAvailabilities, so filtering
// is applied client-side after the list response is fetched.
func WithSubscriptionPlanAvailabilitiesPlanTypes(planTypes ...SubscriptionPlanType) SubscriptionPlanAvailabilitiesOption {
	return func(q *subscriptionPlanAvailabilitiesQuery) {
		for _, planType := range planTypes {
			if planType != "" {
				q.planTypes = append(q.planTypes, planType)
			}
		}
	}
}

// CreateSubscriptionPlanAvailability sets subscription plan availability in territories.
func (c *Client) CreateSubscriptionPlanAvailability(ctx context.Context, subID string, territoryIDs []string, attrs SubscriptionPlanAvailabilityAttributes) (*SubscriptionPlanAvailabilityResponse, error) {
	subID = strings.TrimSpace(subID)
	territoryIDs = normalizeList(territoryIDs)
	if subID == "" {
		return nil, fmt.Errorf("subscription ID is required")
	}
	if len(territoryIDs) == 0 {
		return nil, fmt.Errorf("territory IDs are required")
	}
	if attrs.PlanType == "" {
		return nil, fmt.Errorf("plan type is required")
	}

	relData := make([]ResourceData, 0, len(territoryIDs))
	for _, territoryID := range territoryIDs {
		relData = append(relData, ResourceData{Type: ResourceTypeTerritories, ID: territoryID})
	}

	payload := SubscriptionPlanAvailabilityCreateRequest{
		Data: SubscriptionPlanAvailabilityCreateData{
			Type:       ResourceTypeSubscriptionPlanAvailabilities,
			Attributes: attrs,
			Relationships: &SubscriptionPlanAvailabilityRelationships{
				Subscription: &Relationship{
					Data: ResourceData{Type: ResourceTypeSubscriptions, ID: subID},
				},
				AvailableTerritories: &RelationshipList{Data: relData},
			},
		},
	}

	data, err := c.doRetriedSubscriptionMutation(ctx, func() ([]byte, error) {
		body, err := BuildRequestBody(payload)
		if err != nil {
			return nil, err
		}
		return c.do(ctx, http.MethodPost, "/v1/subscriptionPlanAvailabilities", body)
	})
	if err != nil {
		return nil, err
	}

	var response SubscriptionPlanAvailabilityResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateSubscriptionPlanAvailability updates subscription plan availability in territories.
func (c *Client) UpdateSubscriptionPlanAvailability(ctx context.Context, planAvailabilityID string, territoryIDs []string, attrs *SubscriptionPlanAvailabilityAttributes) (*SubscriptionPlanAvailabilityResponse, error) {
	planAvailabilityID = strings.TrimSpace(planAvailabilityID)
	territoryIDs = normalizeList(territoryIDs)
	if planAvailabilityID == "" {
		return nil, fmt.Errorf("plan availability ID is required")
	}

	relData := make([]ResourceData, 0, len(territoryIDs))
	for _, territoryID := range territoryIDs {
		relData = append(relData, ResourceData{Type: ResourceTypeTerritories, ID: territoryID})
	}

	payload := SubscriptionPlanAvailabilityUpdateRequest{
		Data: SubscriptionPlanAvailabilityUpdateData{
			Type:       ResourceTypeSubscriptionPlanAvailabilities,
			ID:         planAvailabilityID,
			Attributes: attrs,
			Relationships: &SubscriptionPlanAvailabilityRelationships{
				AvailableTerritories: &RelationshipList{Data: relData},
			},
		},
	}

	data, err := c.doRetriedSubscriptionMutation(ctx, func() ([]byte, error) {
		body, err := BuildRequestBody(payload)
		if err != nil {
			return nil, err
		}
		path := fmt.Sprintf("/v1/subscriptionPlanAvailabilities/%s", planAvailabilityID)
		return c.do(ctx, http.MethodPatch, path, body)
	})
	if err != nil {
		return nil, err
	}

	var response SubscriptionPlanAvailabilityResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetSubscriptionPlanAvailabilitiesForSubscription retrieves plan availabilities for a subscription.
func (c *Client) GetSubscriptionPlanAvailabilitiesForSubscription(ctx context.Context, subID string, opts ...SubscriptionPlanAvailabilitiesOption) (*SubscriptionPlanAvailabilitiesResponse, error) {
	subID = strings.TrimSpace(subID)
	if subID == "" {
		return nil, fmt.Errorf("subscription ID is required")
	}

	query := &subscriptionPlanAvailabilitiesQuery{}
	for _, opt := range opts {
		if opt != nil {
			opt(query)
		}
	}

	path := fmt.Sprintf("/v1/subscriptions/%s/planAvailabilities", subID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response SubscriptionPlanAvailabilitiesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return filterSubscriptionPlanAvailabilities(&response, query.planTypes), nil
}

func filterSubscriptionPlanAvailabilities(resp *SubscriptionPlanAvailabilitiesResponse, planTypes []SubscriptionPlanType) *SubscriptionPlanAvailabilitiesResponse {
	if resp == nil || len(planTypes) == 0 {
		return resp
	}

	allowed := make(map[SubscriptionPlanType]struct{}, len(planTypes))
	for _, planType := range planTypes {
		allowed[planType] = struct{}{}
	}

	filtered := make([]Resource[SubscriptionPlanAvailabilityAttributes], 0, len(resp.Data))
	for _, item := range resp.Data {
		if _, ok := allowed[item.Attributes.PlanType]; ok {
			filtered = append(filtered, item)
		}
	}

	out := *resp
	out.Data = filtered
	out.Links.Next = ""
	out.Meta = adjustFilteredPagingMetadata(out.Meta, len(filtered))
	return &out
}

func adjustFilteredPagingMetadata(meta json.RawMessage, filteredCount int) json.RawMessage {
	if len(meta) == 0 {
		return meta
	}

	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(meta, &parsed); err != nil {
		return meta
	}

	pagingRaw, ok := parsed["paging"]
	if !ok {
		return meta
	}

	var paging map[string]any
	if err := json.Unmarshal(pagingRaw, &paging); err != nil {
		return meta
	}
	if _, hasTotal := paging["total"]; !hasTotal {
		return meta
	}

	paging["total"] = filteredCount
	updatedPaging, err := json.Marshal(paging)
	if err != nil {
		return meta
	}
	parsed["paging"] = updatedPaging

	updated, err := json.Marshal(parsed)
	if err != nil {
		return meta
	}
	return updated
}

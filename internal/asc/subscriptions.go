package asc

import (
	"net/url"
	"strings"
)

// SubscriptionGroupAttributes describes a subscription group resource.
type SubscriptionGroupAttributes struct {
	ReferenceName string `json:"referenceName"`
}

// SubscriptionGroupCreateAttributes describes attributes for creating a group.
type SubscriptionGroupCreateAttributes struct {
	ReferenceName string `json:"referenceName"`
}

// SubscriptionGroupUpdateAttributes describes attributes for updating a group.
type SubscriptionGroupUpdateAttributes struct {
	ReferenceName *string `json:"referenceName,omitempty"`
}

// SubscriptionGroupRelationships describes relationships for groups.
type SubscriptionGroupRelationships struct {
	App *Relationship `json:"app"`
}

// SubscriptionGroupCreateData is the data portion of a group create request.
type SubscriptionGroupCreateData struct {
	Type          ResourceType                      `json:"type"`
	Attributes    SubscriptionGroupCreateAttributes `json:"attributes"`
	Relationships *SubscriptionGroupRelationships   `json:"relationships,omitempty"`
}

// SubscriptionGroupCreateRequest is a request to create a group.
type SubscriptionGroupCreateRequest struct {
	Data SubscriptionGroupCreateData `json:"data"`
}

// SubscriptionGroupUpdateData is the data portion of a group update request.
type SubscriptionGroupUpdateData struct {
	Type       ResourceType                      `json:"type"`
	ID         string                            `json:"id"`
	Attributes SubscriptionGroupUpdateAttributes `json:"attributes"`
}

// SubscriptionGroupUpdateRequest is a request to update a group.
type SubscriptionGroupUpdateRequest struct {
	Data SubscriptionGroupUpdateData `json:"data"`
}

// SubscriptionAttributes describes a subscription resource.
type SubscriptionAttributes struct {
	Name                      string `json:"name"`
	ProductID                 string `json:"productId"`
	FamilySharable            bool   `json:"familySharable,omitempty"`
	State                     string `json:"state,omitempty"`
	SubscriptionPeriod        string `json:"subscriptionPeriod,omitempty"`
	ReviewNote                string `json:"reviewNote,omitempty"`
	GroupLevel                int    `json:"groupLevel,omitempty"`
	AvailableInAllTerritories bool   `json:"availableInAllTerritories,omitempty"`
}

// SubscriptionCreateAttributes describes attributes for creating a subscription.
type SubscriptionCreateAttributes struct {
	Name                      string `json:"name"`
	ProductID                 string `json:"productId"`
	FamilySharable            *bool  `json:"familySharable,omitempty"`
	SubscriptionPeriod        string `json:"subscriptionPeriod,omitempty"`
	ReviewNote                string `json:"reviewNote,omitempty"`
	GroupLevel                *int   `json:"groupLevel,omitempty"`
	AvailableInAllTerritories *bool  `json:"availableInAllTerritories,omitempty"`
}

// SubscriptionUpdateAttributes describes attributes for updating a subscription.
type SubscriptionUpdateAttributes struct {
	Name                      *string `json:"name,omitempty"`
	ReviewNote                *string `json:"reviewNote,omitempty"`
	FamilySharable            *bool   `json:"familySharable,omitempty"`
	SubscriptionPeriod        *string `json:"subscriptionPeriod,omitempty"`
	GroupLevel                *int    `json:"groupLevel,omitempty"`
	AvailableInAllTerritories *bool   `json:"availableInAllTerritories,omitempty"`
}

// SubscriptionRelationships describes relationships for subscriptions.
type SubscriptionRelationships struct {
	Group *Relationship `json:"group"`
}

// SubscriptionCreateData is the data portion of a subscription create request.
type SubscriptionCreateData struct {
	Type          ResourceType                 `json:"type"`
	Attributes    SubscriptionCreateAttributes `json:"attributes"`
	Relationships *SubscriptionRelationships   `json:"relationships,omitempty"`
}

// SubscriptionCreateRequest is a request to create a subscription.
type SubscriptionCreateRequest struct {
	Data SubscriptionCreateData `json:"data"`
}

// SubscriptionUpdateData is the data portion of a subscription update request.
type SubscriptionUpdateData struct {
	Type       ResourceType                 `json:"type"`
	ID         string                       `json:"id"`
	Attributes SubscriptionUpdateAttributes `json:"attributes"`
}

// SubscriptionUpdateRequest is a request to update a subscription.
type SubscriptionUpdateRequest struct {
	Data SubscriptionUpdateData `json:"data"`
}

// SubscriptionPriceAttributes describes a subscription price resource.
type SubscriptionPriceAttributes struct {
	StartDate string `json:"startDate,omitempty"`
	Preserved bool   `json:"preserved,omitempty"`
}

// SubscriptionPriceCreateAttributes describes attributes for creating a price.
type SubscriptionPriceCreateAttributes struct {
	StartDate string `json:"startDate,omitempty"`
	Preserved *bool  `json:"preserved,omitempty"`
}

// SubscriptionPriceRelationships describes relationships for prices.
type SubscriptionPriceRelationships struct {
	Subscription           *Relationship `json:"subscription"`
	SubscriptionPricePoint *Relationship `json:"subscriptionPricePoint"`
	Territory              *Relationship `json:"territory,omitempty"`
}

// SubscriptionPriceCreateData is the data portion of a price create request.
type SubscriptionPriceCreateData struct {
	Type          ResourceType                       `json:"type"`
	Attributes    *SubscriptionPriceCreateAttributes `json:"attributes,omitempty"`
	Relationships *SubscriptionPriceRelationships    `json:"relationships"`
}

// SubscriptionPriceCreateRequest is a request to create a price.
type SubscriptionPriceCreateRequest struct {
	Data SubscriptionPriceCreateData `json:"data"`
}

// SubscriptionAvailabilityAttributes describes a subscription availability.
type SubscriptionAvailabilityAttributes struct {
	AvailableInNewTerritories bool `json:"availableInNewTerritories"`
}

// SubscriptionAvailabilityRelationships describes relationships for availability.
type SubscriptionAvailabilityRelationships struct {
	Subscription         *Relationship     `json:"subscription"`
	AvailableTerritories *RelationshipList `json:"availableTerritories"`
}

// SubscriptionAvailabilityCreateData is the data portion of availability create requests.
type SubscriptionAvailabilityCreateData struct {
	Type          ResourceType                           `json:"type"`
	Attributes    SubscriptionAvailabilityAttributes     `json:"attributes"`
	Relationships *SubscriptionAvailabilityRelationships `json:"relationships"`
}

// SubscriptionAvailabilityCreateRequest is a request to create availability.
type SubscriptionAvailabilityCreateRequest struct {
	Data SubscriptionAvailabilityCreateData `json:"data"`
}

// SubscriptionGroupsResponse is the response from subscription groups endpoints.
type SubscriptionGroupsResponse = Response[SubscriptionGroupAttributes]

// SubscriptionGroupResponse is the response from subscription group detail endpoints.
type SubscriptionGroupResponse = SingleResponse[SubscriptionGroupAttributes]

// SubscriptionsResponse is the response from subscriptions list endpoints.
type SubscriptionsResponse = Response[SubscriptionAttributes]

// SubscriptionResponse is the response from subscription detail endpoints.
type SubscriptionResponse = SingleResponse[SubscriptionAttributes]

// SubscriptionPricesResponse is the response from subscription prices list endpoints.
type SubscriptionPricesResponse = Response[SubscriptionPriceAttributes]

// SubscriptionPriceResponse is the response from subscription price create endpoints.
type SubscriptionPriceResponse = SingleResponse[SubscriptionPriceAttributes]

// SubscriptionAvailabilityResponse is the response from availability endpoints.
type SubscriptionAvailabilityResponse = SingleResponse[SubscriptionAvailabilityAttributes]

// SubscriptionGroupsOption is a functional option for GetSubscriptionGroups.
type SubscriptionGroupsOption func(*subscriptionGroupsQuery)

// SubscriptionsOption is a functional option for GetSubscriptions.
type SubscriptionsOption func(*subscriptionsQuery)

// SubscriptionAvailabilityTerritoriesOption is a functional option for availability territory listings.
type SubscriptionAvailabilityTerritoriesOption func(*subscriptionAvailabilityTerritoriesQuery)

type subscriptionGroupsQuery struct {
	listQuery
}

type subscriptionsQuery struct {
	listQuery
}

type subscriptionAvailabilityTerritoriesQuery struct {
	listQuery
}

// WithSubscriptionGroupsLimit sets the max number of groups to return.
func WithSubscriptionGroupsLimit(limit int) SubscriptionGroupsOption {
	return func(q *subscriptionGroupsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithSubscriptionGroupsNextURL uses a next page URL directly.
func WithSubscriptionGroupsNextURL(next string) SubscriptionGroupsOption {
	return func(q *subscriptionGroupsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithSubscriptionsLimit sets the max number of subscriptions to return.
func WithSubscriptionsLimit(limit int) SubscriptionsOption {
	return func(q *subscriptionsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithSubscriptionsNextURL uses a next page URL directly.
func WithSubscriptionsNextURL(next string) SubscriptionsOption {
	return func(q *subscriptionsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithSubscriptionAvailabilityTerritoriesLimit sets the max number of territories to return.
func WithSubscriptionAvailabilityTerritoriesLimit(limit int) SubscriptionAvailabilityTerritoriesOption {
	return func(q *subscriptionAvailabilityTerritoriesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithSubscriptionAvailabilityTerritoriesNextURL uses a next page URL directly.
func WithSubscriptionAvailabilityTerritoriesNextURL(next string) SubscriptionAvailabilityTerritoriesOption {
	return func(q *subscriptionAvailabilityTerritoriesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

func buildSubscriptionGroupsQuery(query *subscriptionGroupsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildSubscriptionsQuery(query *subscriptionsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildSubscriptionAvailabilityTerritoriesQuery(query *subscriptionAvailabilityTerritoriesQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

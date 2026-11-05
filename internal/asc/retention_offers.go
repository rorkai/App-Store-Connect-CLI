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

// RetentionOfferAttributes describes a retention offer.
type RetentionOfferAttributes struct {
	ReferenceName                                       string                    `json:"referenceName,omitempty"`
	OfferID                                             string                    `json:"offerId,omitempty"`
	Duration                                            SubscriptionOfferDuration `json:"duration,omitempty"`
	OfferMode                                           SubscriptionOfferMode     `json:"offerMode,omitempty"`
	PeriodCount                                         int                       `json:"periodCount,omitempty"`
	CustomerEligibilityPaidSubscriptionDurationInMonths int                       `json:"customerEligibilityPaidSubscriptionDurationInMonths,omitempty"`
	// CustomerEligibilityWaitBetweenOffersInMonths stays raw so an explicit
	// null from Apple survives output instead of disappearing.
	CustomerEligibilityWaitBetweenOffersInMonths json.RawMessage      `json:"customerEligibilityWaitBetweenOffersInMonths,omitempty"`
	TargetSubscriptionPlanType                   SubscriptionPlanType `json:"targetSubscriptionPlanType,omitempty"`
}

// RetentionOfferCreateAttributes describes attributes for creating a retention offer.
type RetentionOfferCreateAttributes struct {
	ReferenceName                                       string                    `json:"referenceName"`
	OfferID                                             string                    `json:"offerId"`
	Duration                                            SubscriptionOfferDuration `json:"duration"`
	OfferMode                                           SubscriptionOfferMode     `json:"offerMode"`
	PeriodCount                                         int                       `json:"periodCount"`
	CustomerEligibilityPaidSubscriptionDurationInMonths int                       `json:"customerEligibilityPaidSubscriptionDurationInMonths"`
	CustomerEligibilityWaitBetweenOffersInMonths        *int                      `json:"customerEligibilityWaitBetweenOffersInMonths,omitempty"`
	TargetSubscriptionPlanType                          SubscriptionPlanType      `json:"targetSubscriptionPlanType,omitempty"`
}

// RetentionOfferUpdateAttributes describes the eligibility attributes a
// retention offer update can change. The spec marks both nullable, but Apple
// returns HTTP 500 for null, so clearing is not offered.
type RetentionOfferUpdateAttributes struct {
	CustomerEligibilityPaidSubscriptionDurationInMonths *int `json:"customerEligibilityPaidSubscriptionDurationInMonths,omitempty"`
	CustomerEligibilityWaitBetweenOffersInMonths        *int `json:"customerEligibilityWaitBetweenOffersInMonths,omitempty"`
}

// RetentionOfferCreateRelationships describes relationships for creating a retention offer.
type RetentionOfferCreateRelationships struct {
	Subscription Relationship     `json:"subscription"`
	Prices       RelationshipList `json:"prices"`
}

// RetentionOfferCreateData is the data portion of a create request.
type RetentionOfferCreateData struct {
	Type          ResourceType                      `json:"type"`
	Attributes    RetentionOfferCreateAttributes    `json:"attributes"`
	Relationships RetentionOfferCreateRelationships `json:"relationships"`
}

// RetentionOfferCreateRequest is a request to create a retention offer.
type RetentionOfferCreateRequest struct {
	Data     RetentionOfferCreateData          `json:"data"`
	Included []RetentionOfferPriceInlineCreate `json:"included,omitempty"`
}

// RetentionOfferPriceInlineRelationships links an inline retention offer price
// to its subscription price point. Unlike win-back offer prices, retention
// offer prices carry no territory relationship; the price point implies it.
type RetentionOfferPriceInlineRelationships struct {
	SubscriptionPricePoint Relationship `json:"subscriptionPricePoint"`
}

// RetentionOfferPriceInlineCreate is a price created inline with its offer
// through the request's `included` array, under a `${local-id}` ID.
type RetentionOfferPriceInlineCreate struct {
	Type          ResourceType                           `json:"type"`
	ID            string                                 `json:"id"`
	Relationships RetentionOfferPriceInlineRelationships `json:"relationships"`
}

// RetentionOfferPriceAttributes describes a retention offer price (attributes are empty).
type RetentionOfferPriceAttributes struct{}

// RetentionOffersResponse is the response from retention offer list endpoints.
type RetentionOffersResponse = Response[RetentionOfferAttributes]

// RetentionOfferResponse is the response from retention offer detail endpoints.
type RetentionOfferResponse = SingleResponse[RetentionOfferAttributes]

// RetentionOfferPricesResponse is the response from the retention offer prices endpoint.
type RetentionOfferPricesResponse = Response[RetentionOfferPriceAttributes]

type retentionOffersQuery struct {
	listQuery
	includedFieldsQuery
	fields      []string
	include     []string
	pricesLimit int
}

type retentionOfferPricesQuery struct {
	listQuery
	includedFieldsQuery
	territoryIDs []string
	fields       []string
	include      []string
}

// RetentionOffersOption is a functional option for retention offer reads.
type RetentionOffersOption func(*retentionOffersQuery)

// RetentionOfferPricesOption is a functional option for retention offer price reads.
type RetentionOfferPricesOption func(*retentionOfferPricesQuery)

// WithRetentionOffersLimit sets the max number of retention offers to return.
func WithRetentionOffersLimit(limit int) RetentionOffersOption {
	return func(q *retentionOffersQuery) { q.limit = limit }
}

// WithRetentionOffersNextURL uses a next page URL directly.
func WithRetentionOffersNextURL(next string) RetentionOffersOption {
	return func(q *retentionOffersQuery) { q.nextURL = strings.TrimSpace(next) }
}

// WithRetentionOffersFields sets fields[retentionOffers].
func WithRetentionOffersFields(fields []string) RetentionOffersOption {
	return func(q *retentionOffersQuery) { q.fields = normalizeList(fields) }
}

// WithRetentionOffersInclude sets include for retention offer reads.
func WithRetentionOffersInclude(include []string) RetentionOffersOption {
	return func(q *retentionOffersQuery) { q.include = normalizeList(include) }
}

// WithRetentionOffersPricesLimit sets limit[prices] for included prices.
func WithRetentionOffersPricesLimit(limit int) RetentionOffersOption {
	return func(q *retentionOffersQuery) { q.pricesLimit = limit }
}

// WithRetentionOfferPricesLimit sets the max number of retention offer prices to return.
func WithRetentionOfferPricesLimit(limit int) RetentionOfferPricesOption {
	return func(q *retentionOfferPricesQuery) { q.limit = limit }
}

// WithRetentionOfferPricesNextURL uses a next page URL directly.
func WithRetentionOfferPricesNextURL(next string) RetentionOfferPricesOption {
	return func(q *retentionOfferPricesQuery) { q.nextURL = strings.TrimSpace(next) }
}

// WithRetentionOfferPricesTerritoryFilter filters retention offer prices by territory ID(s).
func WithRetentionOfferPricesTerritoryFilter(ids []string) RetentionOfferPricesOption {
	return func(q *retentionOfferPricesQuery) { q.territoryIDs = normalizeList(ids) }
}

// WithRetentionOfferPricesFields sets fields[retentionOfferPrices].
func WithRetentionOfferPricesFields(fields []string) RetentionOfferPricesOption {
	return func(q *retentionOfferPricesQuery) { q.fields = normalizeList(fields) }
}

// WithRetentionOfferPricesInclude sets include for retention offer price reads.
func WithRetentionOfferPricesInclude(include []string) RetentionOfferPricesOption {
	return func(q *retentionOfferPricesQuery) { q.include = normalizeList(include) }
}

func buildRetentionOffersQuery(query *retentionOffersQuery) string {
	values := url.Values{}
	addCSV(values, "fields[retentionOffers]", query.fields)
	addCSV(values, "include", query.include)
	addLimit(values, query.limit)
	if query.pricesLimit > 0 {
		values.Set("limit[prices]", strconv.Itoa(query.pricesLimit))
	}
	query.includedFields.AddTo(values)
	return values.Encode()
}

func buildRetentionOfferPricesQuery(query *retentionOfferPricesQuery) string {
	values := url.Values{}
	addCSV(values, "filter[territory]", query.territoryIDs)
	addCSV(values, "fields[retentionOfferPrices]", query.fields)
	addCSV(values, "include", query.include)
	addLimit(values, query.limit)
	query.includedFields.AddTo(values)
	return values.Encode()
}

// GetSubscriptionRetentionOffers lists the retention offers of a subscription.
func (c *Client) GetSubscriptionRetentionOffers(ctx context.Context, subscriptionID string, opts ...RetentionOffersOption) (*RetentionOffersResponse, error) {
	return c.getRetentionOffers(ctx, fmt.Sprintf("/v1/subscriptions/%s/retentionOffers", strings.TrimSpace(subscriptionID)), opts...)
}

// GetSubscriptionRetentionMessageOffers lists the retention offers presented
// with a subscription's retention message.
func (c *Client) GetSubscriptionRetentionMessageOffers(ctx context.Context, subscriptionID string, opts ...RetentionOffersOption) (*RetentionOffersResponse, error) {
	return c.getRetentionOffers(ctx, fmt.Sprintf("/v1/subscriptions/%s/retentionMessageOffers", strings.TrimSpace(subscriptionID)), opts...)
}

func (c *Client) getRetentionOffers(ctx context.Context, path string, opts ...RetentionOffersOption) (*RetentionOffersResponse, error) {
	query := &retentionOffersQuery{}
	for _, opt := range opts {
		opt(query)
	}
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("retentionOffers: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildRetentionOffersQuery(query); queryString != "" {
		path += "?" + queryString
	}
	var response RetentionOffersResponse
	if err := c.getRetentionResource(ctx, path, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// GetRetentionOffer retrieves a retention offer by ID.
func (c *Client) GetRetentionOffer(ctx context.Context, offerID string, opts ...RetentionOffersOption) (*RetentionOfferResponse, error) {
	query := &retentionOffersQuery{}
	for _, opt := range opts {
		opt(query)
	}
	path := fmt.Sprintf("/v1/retentionOffers/%s", strings.TrimSpace(offerID))
	if queryString := buildRetentionOffersQuery(query); queryString != "" {
		path += "?" + queryString
	}
	var response RetentionOfferResponse
	if err := c.getRetentionResource(ctx, path, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// CreateRetentionOffer creates a retention offer with its inline prices.
func (c *Client) CreateRetentionOffer(ctx context.Context, req RetentionOfferCreateRequest) (*RetentionOfferResponse, error) {
	return c.sendRetentionOffer(ctx, http.MethodPost, "/v1/retentionOffers", req)
}

// UpdateRetentionOffer updates a retention offer's eligibility attributes.
func (c *Client) UpdateRetentionOffer(ctx context.Context, offerID string, attrs RetentionOfferUpdateAttributes) (*RetentionOfferResponse, error) {
	offerID = strings.TrimSpace(offerID)
	payload := struct {
		Data struct {
			Type       ResourceType                   `json:"type"`
			ID         string                         `json:"id"`
			Attributes RetentionOfferUpdateAttributes `json:"attributes"`
		} `json:"data"`
	}{}
	payload.Data.Type = ResourceTypeRetentionOffers
	payload.Data.ID = offerID
	payload.Data.Attributes = attrs
	return c.sendRetentionOffer(ctx, http.MethodPatch, "/v1/retentionOffers/"+offerID, payload)
}

func (c *Client) sendRetentionOffer(ctx context.Context, method, path string, payload any) (*RetentionOfferResponse, error) {
	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}
	data, err := c.do(ctx, method, path, body)
	if err != nil {
		return nil, err
	}
	var response RetentionOfferResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

// DeleteRetentionOffer deletes a retention offer.
func (c *Client) DeleteRetentionOffer(ctx context.Context, offerID string) error {
	_, err := c.do(ctx, http.MethodDelete, "/v1/retentionOffers/"+strings.TrimSpace(offerID), nil)
	return err
}

// GetRetentionOfferPrices lists the prices of a retention offer.
func (c *Client) GetRetentionOfferPrices(ctx context.Context, offerID string, opts ...RetentionOfferPricesOption) (*RetentionOfferPricesResponse, error) {
	query := &retentionOfferPricesQuery{}
	for _, opt := range opts {
		opt(query)
	}
	path := fmt.Sprintf("/v1/retentionOffers/%s/prices", strings.TrimSpace(offerID))
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("retentionOfferPrices: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildRetentionOfferPricesQuery(query); queryString != "" {
		path += "?" + queryString
	}
	var response RetentionOfferPricesResponse
	if err := c.getRetentionResource(ctx, path, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// GetRetentionOfferPricesRelationships lists the price linkages of a retention offer.
func (c *Client) GetRetentionOfferPricesRelationships(ctx context.Context, offerID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getResourceLinkages(ctx, offerID, "prices", "offerID", "/v1/retentionOffers/%s/relationships/%s", "retentionOfferPrices relationships", opts...)
}

// GetSubscriptionRetentionOffersRelationships lists the retention offer linkages of a subscription.
func (c *Client) GetSubscriptionRetentionOffersRelationships(ctx context.Context, subscriptionID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getResourceLinkages(ctx, subscriptionID, "retentionOffers", "subscriptionID", "/v1/subscriptions/%s/relationships/%s", "retentionOffers relationships", opts...)
}

// GetSubscriptionRetentionMessageOffersRelationships lists the linkages of the
// retention offers presented with a subscription's retention message.
func (c *Client) GetSubscriptionRetentionMessageOffersRelationships(ctx context.Context, subscriptionID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getResourceLinkages(ctx, subscriptionID, "retentionMessageOffers", "subscriptionID", "/v1/subscriptions/%s/relationships/%s", "retentionMessageOffers relationships", opts...)
}

// AddSubscriptionRetentionMessageOffers presents retention offers with a
// subscription's retention message.
func (c *Client) AddSubscriptionRetentionMessageOffers(ctx context.Context, subscriptionID string, offerIDs []string) error {
	return c.changeSubscriptionRetentionMessageOffers(ctx, http.MethodPost, subscriptionID, offerIDs)
}

// RemoveSubscriptionRetentionMessageOffers stops presenting retention offers
// with a subscription's retention message.
func (c *Client) RemoveSubscriptionRetentionMessageOffers(ctx context.Context, subscriptionID string, offerIDs []string) error {
	return c.changeSubscriptionRetentionMessageOffers(ctx, http.MethodDelete, subscriptionID, offerIDs)
}

func (c *Client) changeSubscriptionRetentionMessageOffers(ctx context.Context, method, subscriptionID string, offerIDs []string) error {
	body, err := BuildRequestBody(RelationshipRequest{Data: buildRelationshipData(ResourceTypeRetentionOffers, offerIDs)})
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/v1/subscriptions/%s/relationships/retentionMessageOffers", strings.TrimSpace(subscriptionID))
	_, err = c.do(ctx, method, path, body)
	return err
}

func (c *Client) getRetentionResource(ctx context.Context, path string, out any) error {
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}
	return nil
}

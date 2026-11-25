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

// ProposedSubscriptionPriceTier describes an additional quantity-based price.
type ProposedSubscriptionPriceTier struct {
	MinQuantity              int    `json:"minQuantity"`
	SubscriptionPricePointID string `json:"subscriptionPricePointId"`
}

// SubscriptionPriceTierAttributes describes a subscription price tier.
type SubscriptionPriceTierAttributes struct {
	MinQuantity *int `json:"minQuantity,omitempty"`
	api         apiAttributes
}

type SubscriptionPriceTiersResponse = Response[SubscriptionPriceTierAttributes]

// SubscriptionEqualizedPriceTiersForTerritory is a territory's equalized price matrix.
type SubscriptionEqualizedPriceTiersForTerritory struct {
	TerritoryID          string                          `json:"territoryId,omitempty"`
	BasePricePointID     string                          `json:"basePricePointId,omitempty"`
	AdditionalPriceTiers []ProposedSubscriptionPriceTier `json:"additionalPriceTiers,omitempty"`
}

type SubscriptionPriceTierEqualizationAttributes struct {
	EqualizedPrices []SubscriptionEqualizedPriceTiersForTerritory `json:"equalizedPrices,omitempty"`
	api             apiAttributes
}

type SubscriptionPriceTierEqualizationResponse = SingleResponse[SubscriptionPriceTierEqualizationAttributes]

// ProposedSubscriptionPriceChange describes one territory's proposed prices.
type ProposedSubscriptionPriceChange struct {
	TerritoryID          string                           `json:"territoryId,omitempty"`
	StartDate            string                           `json:"startDate,omitempty"`
	BasePricePointID     string                           `json:"basePricePointId,omitempty"`
	AdditionalPriceTiers *[]ProposedSubscriptionPriceTier `json:"additionalPriceTiers,omitempty"`
}

type SubscriptionPriceChangeResult struct {
	TerritoryID string `json:"territoryId,omitempty"`
	ChangeType  string `json:"changeType,omitempty"`
}

type SubscriptionPriceChangePreviewAttributes struct {
	ChangeResults []SubscriptionPriceChangeResult `json:"changeResults,omitempty"`
	api           apiAttributes
}

type SubscriptionPriceChangePreviewResponse = SingleResponse[SubscriptionPriceChangePreviewAttributes]

// SubscriptionPriceTiersOption configures public tier list and related-tier reads.
type SubscriptionPriceTiersOption func(*subscriptionPriceTiersQuery)

type subscriptionPriceTiersQuery struct {
	listQuery
	subscriptionIDs  []string
	territories      []string
	fields           []string
	pricePointFields []string
	include          []string
}

func WithSubscriptionPriceTiersSubscriptions(ids []string) SubscriptionPriceTiersOption {
	return func(q *subscriptionPriceTiersQuery) {
		q.subscriptionIDs = normalizeList(ids)
	}
}

func WithSubscriptionPriceTiersTerritories(ids []string) SubscriptionPriceTiersOption {
	return func(q *subscriptionPriceTiersQuery) {
		q.territories = normalizeList(ids)
	}
}

func WithSubscriptionPriceTiersFields(fields []string) SubscriptionPriceTiersOption {
	return func(q *subscriptionPriceTiersQuery) {
		q.fields = normalizeList(fields)
	}
}

func WithSubscriptionPriceTiersPricePointFields(fields []string) SubscriptionPriceTiersOption {
	return func(q *subscriptionPriceTiersQuery) {
		q.pricePointFields = normalizeList(fields)
	}
}

func WithSubscriptionPriceTiersInclude(include []string) SubscriptionPriceTiersOption {
	return func(q *subscriptionPriceTiersQuery) {
		q.include = normalizeList(include)
	}
}

func buildSubscriptionPriceTiersQuery(q *subscriptionPriceTiersQuery) string {
	values := url.Values{}
	addLimit(values, q.limit)
	addCSV(values, "filter[subscription]", q.subscriptionIDs)
	addCSV(values, "filter[territory]", q.territories)
	addRelatedSparseFields(values, "fields[subscriptionPriceTiers]", q.fields, q.include, relatedSparseField{"fields[subscriptionPricePoints]", "subscriptionPricePoint", q.pricePointFields})
	return values.Encode()
}

func (c *Client) GetSubscriptionPriceTiers(ctx context.Context, opts ...SubscriptionPriceTiersOption) (*SubscriptionPriceTiersResponse, error) {
	query := applyOptions(opts)
	if query.nextURL == "" && len(query.subscriptionIDs) == 0 {
		return nil, fmt.Errorf("subscription IDs are required")
	}
	return getJSON[SubscriptionPriceTiersResponse](ctx, c, readRequest{
		Path:        "/v1/subscriptionPriceTiers",
		Query:       buildSubscriptionPriceTiersQuery(query),
		Next:        query.nextURL,
		NextContext: "subscription price tiers",
	})
}

func (c *Client) GetSubscriptionPriceAdditionalPriceTiers(ctx context.Context, priceID string, opts ...SubscriptionPriceTiersOption) (*SubscriptionPriceTiersResponse, error) {
	query := applyOptions(opts)
	priceID = strings.TrimSpace(priceID)
	if query.nextURL == "" && priceID == "" {
		return nil, fmt.Errorf("price ID is required")
	}
	return getJSON[SubscriptionPriceTiersResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/subscriptionPrices/%s/additionalPriceTiers", priceID),
		Query:       buildSubscriptionPriceTiersQuery(query),
		Next:        query.nextURL,
		NextContext: "subscription price additional tiers",
	})
}

func (c *Client) GetSubscriptionPriceAdditionalPriceTiersRelationships(ctx context.Context, priceID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	query := applyOptions(opts)
	priceID = strings.TrimSpace(priceID)
	if query.nextURL == "" && priceID == "" {
		return nil, fmt.Errorf("price ID is required")
	}
	return getJSON[LinkagesResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/subscriptionPrices/%s/relationships/additionalPriceTiers", priceID),
		Query:       buildLinkagesQuery(query),
		Next:        query.nextURL,
		NextContext: "subscription price additional tier relationships",
	})
}

// SubscriptionPriceTierEqualizationQuery selects fields and included price points.
type SubscriptionPriceTierEqualizationQuery struct {
	Fields           []string
	PricePointFields []string
	Include          []string
	PricePointsLimit int
}

// CreateSubscriptionPriceTierEqualization computes territory prices for proposed tiers.
func (c *Client) CreateSubscriptionPriceTierEqualization(ctx context.Context, subscriptionID, basePricePointID string, tiers *[]ProposedSubscriptionPriceTier, query SubscriptionPriceTierEqualizationQuery) (*SubscriptionPriceTierEqualizationResponse, error) {
	subscriptionID = strings.TrimSpace(subscriptionID)
	basePricePointID = strings.TrimSpace(basePricePointID)
	if subscriptionID == "" || basePricePointID == "" {
		return nil, fmt.Errorf("subscription ID and base price point ID are required")
	}
	if tiers != nil {
		if err := ValidateProposedSubscriptionPriceTiers(*tiers); err != nil {
			return nil, err
		}
	}
	if query.PricePointsLimit < 0 || query.PricePointsLimit > 50 {
		return nil, fmt.Errorf("price points limit must be between 1 and 50")
	}
	payload := struct {
		Data struct {
			Type       ResourceType `json:"type"`
			Attributes struct {
				AdditionalPriceTiers *[]ProposedSubscriptionPriceTier `json:"additionalPriceTiers,omitempty"`
			} `json:"attributes,omitempty"`
			Relationships struct {
				Subscription   Relationship `json:"subscription"`
				BasePricePoint Relationship `json:"basePricePoint"`
			} `json:"relationships"`
		} `json:"data"`
	}{}
	payload.Data.Type = "subscriptionPriceTierEqualizations"
	payload.Data.Attributes.AdditionalPriceTiers = tiers
	payload.Data.Relationships.Subscription = Relationship{Data: ResourceData{Type: ResourceTypeSubscriptions, ID: subscriptionID}}
	payload.Data.Relationships.BasePricePoint = Relationship{Data: ResourceData{Type: ResourceTypeSubscriptionPricePoints, ID: basePricePointID}}
	values := url.Values{}
	addRelatedSparseFields(values, "fields[subscriptionPriceTierEqualizations]", query.Fields, query.Include, relatedSparseField{"fields[subscriptionPricePoints]", "priceTierPricePoints", query.PricePointFields})
	if query.PricePointsLimit > 0 {
		values.Set("limit[priceTierPricePoints]", strconv.Itoa(query.PricePointsLimit))
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	data, err := c.RawRequestWithQuery(ctx, http.MethodPost, "/v1/subscriptionPriceTierEqualizations", values, body)
	if err != nil {
		return nil, err
	}
	return decodeJSON[SubscriptionPriceTierEqualizationResponse](data, "")
}

// CreateSubscriptionPriceChangePreview evaluates changes without applying prices.
func (c *Client) CreateSubscriptionPriceChangePreview(ctx context.Context, subscriptionID string, changes *[]ProposedSubscriptionPriceChange) (*SubscriptionPriceChangePreviewResponse, error) {
	subscriptionID = strings.TrimSpace(subscriptionID)
	if subscriptionID == "" {
		return nil, fmt.Errorf("subscription ID is required")
	}
	if changes != nil {
		for i, change := range *changes {
			if change.AdditionalPriceTiers != nil {
				if *change.AdditionalPriceTiers == nil {
					return nil, fmt.Errorf("proposed change %d: additionalPriceTiers must be an array", i+1)
				}
				if err := ValidateProposedSubscriptionPriceTiers(*change.AdditionalPriceTiers); err != nil {
					return nil, fmt.Errorf("proposed change %d: %w", i+1, err)
				}
			}
		}
	}
	payload := struct {
		Data struct {
			Type       ResourceType `json:"type"`
			Attributes struct {
				ProposedChanges *[]ProposedSubscriptionPriceChange `json:"proposedChanges,omitempty"`
			} `json:"attributes,omitempty"`
			Relationships struct {
				Subscription Relationship `json:"subscription"`
			} `json:"relationships"`
		} `json:"data"`
	}{}
	payload.Data.Type = "subscriptionPriceChangePreviews"
	payload.Data.Attributes.ProposedChanges = changes
	payload.Data.Relationships.Subscription = Relationship{Data: ResourceData{Type: ResourceTypeSubscriptions, ID: subscriptionID}}
	return sendJSON[SubscriptionPriceChangePreviewResponse](ctx, c, writeRequest{
		Method:  http.MethodPost,
		Path:    "/v1/subscriptionPriceChangePreviews",
		Payload: payload,
	})
}

func ValidateProposedSubscriptionPriceTiers(tiers []ProposedSubscriptionPriceTier) error {
	if len(tiers) > 4 {
		return fmt.Errorf("a price supports at most four additional tiers")
	}
	for i, tier := range tiers {
		if tier.MinQuantity < 2 {
			return fmt.Errorf("price tier %d: minQuantity must be at least 2", i+1)
		}
		if i > 0 && tier.MinQuantity <= tiers[i-1].MinQuantity {
			return fmt.Errorf("price tiers must have distinct minQuantity values in ascending order")
		}
		if strings.TrimSpace(tier.SubscriptionPricePointID) == "" {
			return fmt.Errorf("price tier %d: subscriptionPricePointId is required", i+1)
		}
	}
	return nil
}

func (a *SubscriptionPriceTierAttributes) UnmarshalJSON(data []byte) error {
	type alias SubscriptionPriceTierAttributes
	return unmarshalAPIAttributes(data, (*alias)(a), &a.api)
}

func (a SubscriptionPriceTierAttributes) MarshalJSON() ([]byte, error) {
	type alias SubscriptionPriceTierAttributes
	return a.api.marshal(alias(a))
}

func (a *SubscriptionPriceTierEqualizationAttributes) UnmarshalJSON(data []byte) error {
	type alias SubscriptionPriceTierEqualizationAttributes
	return unmarshalAPIAttributes(data, (*alias)(a), &a.api)
}

func (a SubscriptionPriceTierEqualizationAttributes) MarshalJSON() ([]byte, error) {
	type alias SubscriptionPriceTierEqualizationAttributes
	return a.api.marshal(alias(a))
}

func (a *SubscriptionPriceChangePreviewAttributes) UnmarshalJSON(data []byte) error {
	type alias SubscriptionPriceChangePreviewAttributes
	return unmarshalAPIAttributes(data, (*alias)(a), &a.api)
}

func (a SubscriptionPriceChangePreviewAttributes) MarshalJSON() ([]byte, error) {
	type alias SubscriptionPriceChangePreviewAttributes
	return a.api.marshal(alias(a))
}

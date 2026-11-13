package asc

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// GetSubscriptionWinBackOffers retrieves win-back offers for a subscription.
func (c *Client) GetSubscriptionWinBackOffers(ctx context.Context, subscriptionID string, opts ...WinBackOffersOption) (*WinBackOffersResponse, error) {
	query := applyOptions(opts)
	return getJSON[WinBackOffersResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/subscriptions/%s/winBackOffers", strings.TrimSpace(subscriptionID)),
		Query:       buildWinBackOffersQuery(query),
		Next:        query.nextURL,
		NextContext: "winBackOffers",
	})
}

// GetSubscriptionWinBackOffersRelationships retrieves win-back offer relationships for a subscription.
func (c *Client) GetSubscriptionWinBackOffersRelationships(ctx context.Context, subscriptionID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	query := applyOptions(opts)
	return getJSON[LinkagesResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/subscriptions/%s/relationships/winBackOffers", strings.TrimSpace(subscriptionID)),
		Query:       buildLinkagesQuery(query),
		Next:        query.nextURL,
		NextContext: "winBackOffers relationships",
	})
}

// GetWinBackOffer retrieves a win-back offer by ID.
func (c *Client) GetWinBackOffer(ctx context.Context, offerID string) (*WinBackOfferResponse, error) {
	return getJSON[WinBackOfferResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/winBackOffers/%s", strings.TrimSpace(offerID))})
}

// CreateWinBackOffer creates a win-back offer.
func (c *Client) CreateWinBackOffer(ctx context.Context, req WinBackOfferCreateRequest) (*WinBackOfferResponse, error) {
	return sendJSON[WinBackOfferResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v1/winBackOffers", Payload: req})
}

// UpdateWinBackOffer updates a win-back offer.
func (c *Client) UpdateWinBackOffer(ctx context.Context, offerID string, attrs WinBackOfferUpdateAttributes) (*WinBackOfferResponse, error) {
	payload := WinBackOfferUpdateRequest{
		Data: WinBackOfferUpdateData{
			Type:       ResourceTypeWinBackOffers,
			ID:         strings.TrimSpace(offerID),
			Attributes: attrs,
		},
	}

	return sendJSON[WinBackOfferResponse](ctx, c, writeRequest{Method: http.MethodPatch, Path: fmt.Sprintf("/v1/winBackOffers/%s", strings.TrimSpace(offerID)), Payload: payload})
}

// DeleteWinBackOffer deletes a win-back offer.
func (c *Client) DeleteWinBackOffer(ctx context.Context, offerID string) error {
	return sendNoContent(ctx, c, http.MethodDelete, fmt.Sprintf("/v1/winBackOffers/%s", strings.TrimSpace(offerID)), nil)
}

// GetWinBackOfferPrices retrieves prices for a win-back offer.
func (c *Client) GetWinBackOfferPrices(ctx context.Context, offerID string, opts ...WinBackOfferPricesOption) (*WinBackOfferPricesResponse, error) {
	query := applyOptions(opts)
	return getJSON[WinBackOfferPricesResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/winBackOffers/%s/prices", strings.TrimSpace(offerID)),
		Query:       buildWinBackOfferPricesQuery(query),
		Next:        query.nextURL,
		NextContext: "winBackOfferPrices",
	})
}

// GetWinBackOfferPricesRelationships retrieves price relationships for a win-back offer.
func (c *Client) GetWinBackOfferPricesRelationships(ctx context.Context, offerID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	query := applyOptions(opts)
	return getJSON[LinkagesResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/winBackOffers/%s/relationships/prices", strings.TrimSpace(offerID)),
		Query:       buildLinkagesQuery(query),
		Next:        query.nextURL,
		NextContext: "winBackOfferPrices relationships",
	})
}

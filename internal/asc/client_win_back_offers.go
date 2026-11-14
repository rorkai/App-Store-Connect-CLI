package asc

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

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

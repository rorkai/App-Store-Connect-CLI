package asc

import (
	"context"
	"net/http"
)

// CreateWinBackOffer creates a win-back offer.
func (c *Client) CreateWinBackOffer(ctx context.Context, req WinBackOfferCreateRequest) (*WinBackOfferResponse, error) {
	return sendJSON[WinBackOfferResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v1/winBackOffers", Payload: req})
}

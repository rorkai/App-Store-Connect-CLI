package asc

import (
	"context"
	"net/http"
)

// CreatePromotedPurchase creates a new promoted purchase.
func (c *Client) CreatePromotedPurchase(ctx context.Context, attrs PromotedPurchaseCreateAttributes, relationships PromotedPurchaseCreateRelationships) (*PromotedPurchaseResponse, error) {
	payload := PromotedPurchaseCreateRequest{
		Data: PromotedPurchaseCreateData{
			Type:          ResourceTypePromotedPurchases,
			Attributes:    attrs,
			Relationships: relationships,
		},
	}
	return sendJSON[PromotedPurchaseResponse](ctx, c, writeRequest{
		Method:   http.MethodPost,
		Path:     "/v1/promotedPurchases",
		Payload:  payload,
		ParseErr: "failed to parse promoted purchase response",
	})
}

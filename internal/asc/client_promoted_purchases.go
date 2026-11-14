package asc

import (
	"context"
	"fmt"
	"net/http"
	"strings"
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

// UpdatePromotedPurchase updates a promoted purchase by ID.
func (c *Client) UpdatePromotedPurchase(ctx context.Context, promotedPurchaseID string, attrs PromotedPurchaseUpdateAttributes) (*PromotedPurchaseResponse, error) {
	promotedPurchaseID = strings.TrimSpace(promotedPurchaseID)
	payload := PromotedPurchaseUpdateRequest{
		Data: PromotedPurchaseUpdateData{
			Type:       ResourceTypePromotedPurchases,
			ID:         promotedPurchaseID,
			Attributes: &attrs,
		},
	}
	return sendJSON[PromotedPurchaseResponse](ctx, c, writeRequest{
		Method:   http.MethodPatch,
		Path:     fmt.Sprintf("/v1/promotedPurchases/%s", promotedPurchaseID),
		Payload:  payload,
		ParseErr: "failed to parse promoted purchase response",
	})
}

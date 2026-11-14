package asc

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// SetAppPromotedPurchases replaces promoted purchases for an app.
func (c *Client) SetAppPromotedPurchases(ctx context.Context, appID string, promotedPurchaseIDs []string) error {
	appID = strings.TrimSpace(appID)
	promotedPurchaseIDs = normalizeList(promotedPurchaseIDs)
	if appID == "" {
		return fmt.Errorf("appID is required")
	}

	payload := RelationshipRequest{
		Data: make([]RelationshipData, 0, len(promotedPurchaseIDs)),
	}
	for _, id := range promotedPurchaseIDs {
		payload.Data = append(payload.Data, RelationshipData{
			Type: ResourceTypePromotedPurchases,
			ID:   id,
		})
	}

	return sendNoContent(ctx, c, http.MethodPatch, fmt.Sprintf("/v1/apps/%s/relationships/promotedPurchases", appID), payload)
}

// GetPromotedPurchase retrieves a promoted purchase by ID.
func (c *Client) GetPromotedPurchase(ctx context.Context, promotedPurchaseID string, opts ...PromotedPurchaseGetOption) (*PromotedPurchaseResponse, error) {
	promotedPurchaseID = strings.TrimSpace(promotedPurchaseID)
	query := applyOptions(opts)
	return getJSON[PromotedPurchaseResponse](ctx, c, readRequest{
		Path:     fmt.Sprintf("/v1/promotedPurchases/%s", promotedPurchaseID),
		Query:    buildPromotedPurchaseGetQuery(query),
		ParseErr: "failed to parse promoted purchase response",
	})
}

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

// DeletePromotedPurchase deletes a promoted purchase by ID.
func (c *Client) DeletePromotedPurchase(ctx context.Context, promotedPurchaseID string) error {
	return sendNoContent(ctx, c, http.MethodDelete, fmt.Sprintf("/v1/promotedPurchases/%s", strings.TrimSpace(promotedPurchaseID)), nil)
}

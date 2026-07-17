package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// GetAppPromotedPurchases retrieves promoted purchases for an app.
func (c *Client) GetAppPromotedPurchases(ctx context.Context, appID string, opts ...PromotedPurchasesOption) (*PromotedPurchasesResponse, error) {
	query := &promotedPurchasesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	appID = strings.TrimSpace(appID)
	path := fmt.Sprintf("/v1/apps/%s/promotedPurchases", appID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("promotedPurchases: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildPromotedPurchasesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response PromotedPurchasesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse promoted purchases response: %w", err)
	}

	return &response, nil
}

// GetAppPromotedPurchasesRelationships retrieves promoted purchase linkages for an app.
func (c *Client) GetAppPromotedPurchasesRelationships(ctx context.Context, appID string, opts ...LinkagesOption) (*AppPromotedPurchasesLinkagesResponse, error) {
	query := &linkagesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	appID = strings.TrimSpace(appID)
	path := fmt.Sprintf("/v1/apps/%s/relationships/promotedPurchases", appID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("promotedPurchasesRelationships: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildLinkagesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response AppPromotedPurchasesLinkagesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse promoted purchase relationships response: %w", err)
	}

	return &response, nil
}

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

	body, err := BuildRequestBody(payload)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/v1/apps/%s/relationships/promotedPurchases", appID)
	_, err = c.do(ctx, http.MethodPatch, path, body)
	return err
}

// GetPromotedPurchase retrieves a promoted purchase by ID.
func (c *Client) GetPromotedPurchase(ctx context.Context, promotedPurchaseID string, opts ...PromotedPurchaseGetOption) (*PromotedPurchaseResponse, error) {
	promotedPurchaseID = strings.TrimSpace(promotedPurchaseID)
	query := &promotedPurchaseGetQuery{}
	for _, opt := range opts {
		opt(query)
	}
	path := fmt.Sprintf("/v1/promotedPurchases/%s", promotedPurchaseID)
	if queryString := buildPromotedPurchaseGetQuery(query); queryString != "" {
		path += "?" + queryString
	}
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response PromotedPurchaseResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse promoted purchase response: %w", err)
	}

	return &response, nil
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
	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/promotedPurchases", body)
	if err != nil {
		return nil, err
	}

	var response PromotedPurchaseResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse promoted purchase response: %w", err)
	}

	return &response, nil
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
	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPatch, fmt.Sprintf("/v1/promotedPurchases/%s", promotedPurchaseID), body)
	if err != nil {
		return nil, err
	}

	var response PromotedPurchaseResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse promoted purchase response: %w", err)
	}

	return &response, nil
}

// DeletePromotedPurchase deletes a promoted purchase by ID.
func (c *Client) DeletePromotedPurchase(ctx context.Context, promotedPurchaseID string) error {
	path := fmt.Sprintf("/v1/promotedPurchases/%s", strings.TrimSpace(promotedPurchaseID))
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	return err
}

package asc

import (
	"context"
	"fmt"
	"strings"
)

// GetMerchantID retrieves a single merchant ID by ID.
func (c *Client) GetMerchantID(ctx context.Context, id string, opts ...MerchantIDsOption) (*MerchantIDResponse, error) {
	query := applyOptions(opts)
	id = strings.TrimSpace(id)
	return getJSON[MerchantIDResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/merchantIds/%s", id), Query: buildMerchantIDsQuery(query)})
}

// CreateMerchantID creates a new merchant ID.
func (c *Client) CreateMerchantID(ctx context.Context, attrs MerchantIDCreateAttributes) (*MerchantIDResponse, error) {
	request := MerchantIDCreateRequest{
		Data: MerchantIDCreateData{
			Type:       ResourceTypeMerchantIds,
			Attributes: attrs,
		},
	}
	return sendJSON[MerchantIDResponse](ctx, c, writeRequest{Method: "POST", Path: "/v1/merchantIds", Payload: request})
}

// UpdateMerchantID updates an existing merchant ID.
func (c *Client) UpdateMerchantID(ctx context.Context, id string, attrs MerchantIDUpdateAttributes) (*MerchantIDResponse, error) {
	id = strings.TrimSpace(id)
	request := MerchantIDUpdateRequest{
		Data: MerchantIDUpdateData{
			Type:       ResourceTypeMerchantIds,
			ID:         id,
			Attributes: &attrs,
		},
	}
	return sendJSON[MerchantIDResponse](ctx, c, writeRequest{Method: "PATCH", Path: fmt.Sprintf("/v1/merchantIds/%s", id), Payload: request})
}

// GetMerchantIDCertificatesRelationships retrieves certificate linkages for a merchant ID.
func (c *Client) GetMerchantIDCertificatesRelationships(ctx context.Context, merchantID string, opts ...LinkagesOption) (*MerchantIDCertificatesLinkagesResponse, error) {
	return getTypedResourceLinkages[MerchantIDCertificatesLinkagesResponse](
		c,
		ctx,
		merchantID,
		"certificates",
		"merchant ID",
		"/v1/merchantIds/%s/relationships/%s",
		"merchantIdCertificatesRelationships",
		opts...,
	)
}

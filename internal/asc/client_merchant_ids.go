package asc

import (
	"context"
	"fmt"
	"strings"
)

// GetMerchantIDs retrieves the list of merchant IDs.
func (c *Client) GetMerchantIDs(ctx context.Context, opts ...MerchantIDsOption) (*MerchantIDsResponse, error) {
	query := applyOptions(opts)
	return getJSON[MerchantIDsResponse](ctx, c, readRequest{
		Path:        "/v1/merchantIds",
		Query:       buildMerchantIDsQuery(query),
		Next:        query.nextURL,
		NextContext: "merchantIds",
	})
}

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

// DeleteMerchantID deletes a merchant ID by ID.
func (c *Client) DeleteMerchantID(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	return sendNoContent(ctx, c, "DELETE", fmt.Sprintf("/v1/merchantIds/%s", id), nil)
}

// GetMerchantIDCertificates retrieves certificates for a merchant ID.
func (c *Client) GetMerchantIDCertificates(ctx context.Context, merchantID string, opts ...MerchantIDCertificatesOption) (*CertificatesResponse, error) {
	query := applyOptions(opts)
	merchantID = strings.TrimSpace(merchantID)
	return getJSON[CertificatesResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/merchantIds/%s/certificates", merchantID),
		Query:       buildMerchantIDCertificatesQuery(query),
		Next:        query.nextURL,
		NextContext: "merchantIdCertificates",
	})
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

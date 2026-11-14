package asc

import (
	"context"
	"fmt"
	"strings"
)

// CreatePassTypeID creates a new pass type ID.
func (c *Client) CreatePassTypeID(ctx context.Context, attrs PassTypeIDCreateAttributes) (*PassTypeIDResponse, error) {
	request := PassTypeIDCreateRequest{
		Data: PassTypeIDCreateData{
			Type:       ResourceTypePassTypeIds,
			Attributes: attrs,
		},
	}
	return sendJSON[PassTypeIDResponse](ctx, c, writeRequest{Method: "POST", Path: "/v1/passTypeIds", Payload: request})
}

// UpdatePassTypeID updates an existing pass type ID.
func (c *Client) UpdatePassTypeID(ctx context.Context, id string, attrs PassTypeIDUpdateAttributes) (*PassTypeIDResponse, error) {
	id = strings.TrimSpace(id)
	request := PassTypeIDUpdateRequest{
		Data: PassTypeIDUpdateData{
			Type:       ResourceTypePassTypeIds,
			ID:         id,
			Attributes: &attrs,
		},
	}
	return sendJSON[PassTypeIDResponse](ctx, c, writeRequest{Method: "PATCH", Path: fmt.Sprintf("/v1/passTypeIds/%s", id), Payload: request})
}

// GetPassTypeIDCertificatesRelationships retrieves certificate linkages for a pass type ID.
func (c *Client) GetPassTypeIDCertificatesRelationships(ctx context.Context, passTypeID string, opts ...LinkagesOption) (*PassTypeIDCertificatesLinkagesResponse, error) {
	return getTypedResourceLinkages[PassTypeIDCertificatesLinkagesResponse](
		c,
		ctx,
		passTypeID,
		"certificates",
		"pass type ID",
		"/v1/passTypeIds/%s/relationships/%s",
		"passTypeIdCertificatesRelationships",
		opts...,
	)
}

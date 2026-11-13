package asc

import (
	"context"
	"fmt"
	"strings"
)

// GetDevices retrieves the list of devices.
func (c *Client) GetDevices(ctx context.Context, opts ...DevicesOption) (*DevicesResponse, error) {
	query := applyOptions(opts)
	return getJSON[DevicesResponse](ctx, c, readRequest{
		Path:        "/v1/devices",
		Query:       buildDevicesQuery(query),
		Next:        query.nextURL,
		NextContext: "devices",
	})
}

// GetDevice retrieves a single device by ID.
func (c *Client) GetDevice(ctx context.Context, deviceID string, fields []string) (*DeviceResponse, error) {
	deviceID = strings.TrimSpace(deviceID)
	return getJSON[DeviceResponse](ctx, c, readRequest{
		Path:  fmt.Sprintf("/v1/devices/%s", deviceID),
		Query: buildDevicesFieldsQuery(fields),
	})
}

// CreateDevice registers a new device.
func (c *Client) CreateDevice(ctx context.Context, attrs DeviceCreateAttributes) (*DeviceResponse, error) {
	payload := DeviceCreateRequest{
		Data: DeviceCreateData{
			Type:       ResourceTypeDevices,
			Attributes: attrs,
		},
	}
	return sendJSON[DeviceResponse](ctx, c, writeRequest{Method: "POST", Path: "/v1/devices", Payload: payload})
}

// UpdateDevice updates a device by ID.
func (c *Client) UpdateDevice(ctx context.Context, deviceID string, attrs DeviceUpdateAttributes) (*DeviceResponse, error) {
	deviceID = strings.TrimSpace(deviceID)
	payload := DeviceUpdateRequest{
		Data: DeviceUpdateData{
			Type: ResourceTypeDevices,
			ID:   deviceID,
		},
	}
	if attrs.Name != nil || attrs.Status != nil {
		payload.Data.Attributes = &attrs
	}
	return sendJSON[DeviceResponse](ctx, c, writeRequest{Method: "PATCH", Path: fmt.Sprintf("/v1/devices/%s", deviceID), Payload: payload})
}

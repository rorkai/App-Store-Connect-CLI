package asc

import (
	"context"
	"fmt"
	"strings"
)

// GetDevice retrieves a single device by ID.
func (c *Client) GetDevice(ctx context.Context, deviceID string, fields []string) (*DeviceResponse, error) {
	deviceID = strings.TrimSpace(deviceID)
	return getJSON[DeviceResponse](ctx, c, readRequest{
		Path:  fmt.Sprintf("/v1/devices/%s", deviceID),
		Query: buildDevicesFieldsQuery(fields),
	})
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

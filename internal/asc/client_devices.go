package asc

import (
	"context"
	"fmt"
	"strings"
)

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

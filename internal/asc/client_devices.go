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

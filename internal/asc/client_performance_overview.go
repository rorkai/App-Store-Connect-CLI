package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"
)

// GetPerformanceOverviewForApp retrieves the raw Xcode performance overview.
func (c *Client) GetPerformanceOverviewForApp(ctx context.Context, appID string, deviceTypes []string) (*PerformanceOverviewResponse, error) {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return nil, fmt.Errorf("app ID is required")
	}
	path := "/v1/apps/" + url.PathEscape(appID) + "/performanceOverviews"
	if values := normalizeList(deviceTypes); len(values) > 0 {
		query := url.Values{}
		query.Set("filter[deviceType]", strings.Join(values, ","))
		path += "?" + query.Encode()
	}
	resp, err := c.doStream(ctx, path, "application/vnd.apple.xcode-overview+json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read performance overview: %w", err)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil || object == nil {
		return nil, fmt.Errorf("performance overview response must be a JSON object")
	}
	return &PerformanceOverviewResponse{Data: data}, nil
}

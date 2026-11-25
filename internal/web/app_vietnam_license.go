package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// GetAppVietnamGameLicense reads the Vietnam game license detail shown in App Compliance.
func (c *Client) GetAppVietnamGameLicense(ctx context.Context, appID string) (json.RawMessage, error) {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return nil, fmt.Errorf("app id is required")
	}
	return c.doRequest(ctx, http.MethodGet, "/apps/"+url.PathEscape(appID)+"/vietnamGameLicenseDetail", nil)
}

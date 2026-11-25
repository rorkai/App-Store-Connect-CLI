package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// GetAppPublicationPermit reads the publication permit relationship shown on App Compliance.
func (c *Client) GetAppPublicationPermit(ctx context.Context, appID string) (json.RawMessage, error) {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return nil, fmt.Errorf("app id is required")
	}
	query := url.Values{"fields[apps]": {"appPublicationPermit"}, "include": {"appPublicationPermit"}}
	return c.doRequest(ctx, http.MethodGet, "/apps/"+url.PathEscape(appID)+"?"+query.Encode(), nil)
}

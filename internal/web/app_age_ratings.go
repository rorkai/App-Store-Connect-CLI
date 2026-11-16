package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// ListAppAgeRatings reads app info records with their age rating declarations.
func (c *Client) ListAppAgeRatings(ctx context.Context, appID string) (json.RawMessage, error) {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return nil, fmt.Errorf("app id is required")
	}

	path := "/apps/" + url.PathEscape(appID) + "/appInfos?include=ageRatingDeclaration"
	return c.doRequest(ctx, http.MethodGet, path, nil)
}

// ListAppTerritoryAgeRatings reads the territory age ratings for an app info record.
func (c *Client) ListAppTerritoryAgeRatings(ctx context.Context, appInfoID string) (json.RawMessage, error) {
	appInfoID = strings.TrimSpace(appInfoID)
	if appInfoID == "" {
		return nil, fmt.Errorf("app info id is required")
	}

	path := "/appInfos/" + url.PathEscape(appInfoID) + "/territoryAgeRatings?include=territory&limit=500"
	return c.doRequest(ctx, http.MethodGet, path, nil)
}

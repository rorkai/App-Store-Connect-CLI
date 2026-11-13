package asc

import (
	"context"
	"fmt"
	"strings"
)

// TerritoryAgeRatingAttributes describes territory age rating attributes.
type TerritoryAgeRatingAttributes struct {
	AppStoreAgeRating AppStoreAgeRating `json:"appStoreAgeRating,omitempty"`
}

// TerritoryAgeRatingRelationships describes territory age rating relationships.
type TerritoryAgeRatingRelationships struct {
	Territory Relationship `json:"territory"`
}

// TerritoryAgeRatingsResponse is the response from territory age ratings endpoints.
type TerritoryAgeRatingsResponse = Response[TerritoryAgeRatingAttributes]

// GetAppInfoTerritoryAgeRatings retrieves territory age ratings for an app info.
func (c *Client) GetAppInfoTerritoryAgeRatings(ctx context.Context, appInfoID string, opts ...TerritoryAgeRatingsOption) (*TerritoryAgeRatingsResponse, error) {
	query := applyOptions(opts)
	appInfoID = strings.TrimSpace(appInfoID)
	if query.nextURL == "" && appInfoID == "" {
		return nil, fmt.Errorf("appInfoID is required")
	}
	return getJSON[TerritoryAgeRatingsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/appInfos/%s/territoryAgeRatings", appInfoID),
		Query:       buildTerritoryAgeRatingsQuery(query),
		Next:        query.nextURL,
		NextContext: "territoryAgeRatings",
	})
}

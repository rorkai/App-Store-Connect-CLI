package asc

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// CreateGameCenterDetail creates a new Game Center detail.
func (c *Client) CreateGameCenterDetail(ctx context.Context, appID string, attrs *GameCenterDetailCreateAttributes) (*GameCenterDetailResponse, error) {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return nil, fmt.Errorf("appID is required")
	}
	if attrs != nil && attrs.ChallengeEnabled != nil {
		return nil, fmt.Errorf("challengeEnabled is no longer supported by App Store Connect")
	}

	payload := GameCenterDetailCreateRequest{
		Data: GameCenterDetailCreateData{
			Type:       ResourceTypeGameCenterDetails,
			Attributes: attrs,
			Relationships: &GameCenterDetailCreateRelationships{
				App: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeApps,
						ID:   appID,
					},
				},
			},
		},
	}
	return sendJSON[GameCenterDetailResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v1/gameCenterDetails", Payload: payload})
}

// UpdateGameCenterDetail updates an existing Game Center detail.
func (c *Client) UpdateGameCenterDetail(ctx context.Context, detailID string, attrs *GameCenterDetailUpdateAttributes, rels *GameCenterDetailUpdateRelationships) (*GameCenterDetailResponse, error) {
	detailID = strings.TrimSpace(detailID)
	if detailID == "" {
		return nil, fmt.Errorf("detailID is required")
	}
	if attrs != nil && attrs.ChallengeEnabled != nil {
		return nil, fmt.Errorf("challengeEnabled is no longer supported by App Store Connect")
	}

	hasAttributeUpdate := false
	hasRelationshipUpdate := false
	if rels != nil {
		for _, relationship := range []struct {
			name  string
			value *Relationship
		}{
			{"gameCenterGroup", rels.GameCenterGroup},
			{"defaultLeaderboard", rels.DefaultLeaderboard},
			{"defaultLeaderboardV2", rels.DefaultLeaderboardV2},
			{"defaultGroupLeaderboard", rels.DefaultGroupLeaderboard},
			{"defaultGroupLeaderboardV2", rels.DefaultGroupLeaderboardV2},
		} {
			if relationship.value == nil {
				continue
			}
			id := strings.TrimSpace(relationship.value.Data.ID)
			if id == "" {
				return nil, fmt.Errorf("%s relationship ID is required", relationship.name)
			}
			relationship.value.Data.ID = id
			hasRelationshipUpdate = true
		}
	}
	if !hasAttributeUpdate && !hasRelationshipUpdate {
		return nil, fmt.Errorf("at least one update field is required")
	}

	payload := GameCenterDetailUpdateRequest{
		Data: GameCenterDetailUpdateData{
			Type:          ResourceTypeGameCenterDetails,
			ID:            detailID,
			Attributes:    attrs,
			Relationships: rels,
		},
	}
	return sendJSON[GameCenterDetailResponse](ctx, c, writeRequest{Method: http.MethodPatch, Path: fmt.Sprintf("/v1/gameCenterDetails/%s", detailID), Payload: payload})
}

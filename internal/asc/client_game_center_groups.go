package asc

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// CreateGameCenterGroup creates a new Game Center group.
func (c *Client) CreateGameCenterGroup(ctx context.Context, referenceName *string) (*GameCenterGroupResponse, error) {
	var attrs *GameCenterGroupCreateAttributes
	if referenceName != nil {
		value := strings.TrimSpace(*referenceName)
		attrs = &GameCenterGroupCreateAttributes{
			ReferenceName: &value,
		}
	}

	payload := GameCenterGroupCreateRequest{
		Data: GameCenterGroupCreateData{
			Type:       ResourceTypeGameCenterGroups,
			Attributes: attrs,
		},
	}
	return sendJSON[GameCenterGroupResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v1/gameCenterGroups", Payload: payload})
}

// UpdateGameCenterGroup updates an existing Game Center group.
func (c *Client) UpdateGameCenterGroup(ctx context.Context, groupID string, referenceName *string) (*GameCenterGroupResponse, error) {
	var attrs *GameCenterGroupUpdateAttributes
	if referenceName != nil {
		value := strings.TrimSpace(*referenceName)
		attrs = &GameCenterGroupUpdateAttributes{
			ReferenceName: &value,
		}
	}

	payload := GameCenterGroupUpdateRequest{
		Data: GameCenterGroupUpdateData{
			Type:       ResourceTypeGameCenterGroups,
			ID:         strings.TrimSpace(groupID),
			Attributes: attrs,
		},
	}
	return sendJSON[GameCenterGroupResponse](ctx, c, writeRequest{Method: http.MethodPatch, Path: fmt.Sprintf("/v1/gameCenterGroups/%s", strings.TrimSpace(groupID)), Payload: payload})
}

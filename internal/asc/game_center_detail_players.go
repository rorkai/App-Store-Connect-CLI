package asc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// GameCenterDetailPlayerAttributes describes a player associated with a game.
type GameCenterDetailPlayerAttributes struct {
	Nickname *string `json:"nickname,omitempty"`
	Blocked  *bool   `json:"blocked,omitempty"`
	BundleID *string `json:"bundleId,omitempty"`
}

// GameCenterDetailPlayersResponse is a collection of players.
type GameCenterDetailPlayersResponse = Response[GameCenterDetailPlayerAttributes]

// GameCenterDetailPlayerResponse is a single player.
type GameCenterDetailPlayerResponse = SingleResponse[GameCenterDetailPlayerAttributes]

// GCBlockedPlayersQuery controls the blocked players collection endpoint.
type GCBlockedPlayersQuery struct {
	Fields  []string
	Limit   int
	NextURL string
}

// GetGameCenterBlockedPlayers lists only blocked players for a Game Center detail.
func (c *Client) GetGameCenterBlockedPlayers(ctx context.Context, detailID string, query GCBlockedPlayersQuery) (*GameCenterDetailPlayersResponse, error) {
	path := "/v1/gameCenterDetails/" + url.PathEscape(strings.TrimSpace(detailID)) + "/blockedPlayers"
	if query.NextURL != "" {
		if err := validateNextURL(query.NextURL); err != nil {
			return nil, fmt.Errorf("game-center blocked players: %w", err)
		}
		path = query.NextURL
	} else {
		values := url.Values{}
		if len(query.Fields) > 0 {
			values.Set("fields[gameCenterDetailPlayers]", strings.Join(query.Fields, ","))
		}
		if query.Limit > 0 {
			values.Set("limit", strconv.Itoa(query.Limit))
		}
		if len(values) > 0 {
			path += "?" + values.Encode()
		}
	}
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var response GameCenterDetailPlayersResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse blocked players: %w", err)
	}
	return &response, nil
}

// UpdateGameCenterDetailPlayer changes a player's blocked status and optional bundle ID.
func (c *Client) UpdateGameCenterDetailPlayer(ctx context.Context, id string, blocked bool, bundleID *string) (*GameCenterDetailPlayerResponse, error) {
	id = strings.TrimSpace(id)
	payload := struct {
		Data Resource[gameCenterDetailPlayerUpdateAttributes] `json:"data"`
	}{Data: Resource[gameCenterDetailPlayerUpdateAttributes]{Type: ResourceTypeGameCenterDetailPlayers, ID: id, Attributes: gameCenterDetailPlayerUpdateAttributes{Blocked: blocked, BundleID: bundleID}}}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	data, err := c.do(ctx, http.MethodPatch, "/v1/gameCenterDetailPlayers/"+url.PathEscape(id), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	var response GameCenterDetailPlayerResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse player: %w", err)
	}
	return &response, nil
}

type gameCenterDetailPlayerUpdateAttributes struct {
	Blocked  bool    `json:"blocked"`
	BundleID *string `json:"bundleId,omitempty"`
}

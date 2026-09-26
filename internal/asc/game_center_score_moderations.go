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

// GameCenterScoreModerationAttributes describes a submitted leaderboard score.
// Numeric strings stay strings to preserve Apple's full integer precision.
type GameCenterScoreModerationAttributes struct {
	Rank          *string   `json:"rank,omitempty"`
	Score         *string   `json:"score,omitempty"`
	SubmittedDate *string   `json:"submittedDate,omitempty"`
	Blocked       *bool     `json:"blocked,omitempty"`
	PreReleased   *bool     `json:"preReleased,omitempty"`
	Context       *string   `json:"context,omitempty"`
	ChallengeIDs  *[]string `json:"challengeIds,omitempty"`
}

// GameCenterScoreModerationsResponse is a collection of submitted scores.
type GameCenterScoreModerationsResponse = Response[GameCenterScoreModerationAttributes]

// GameCenterScoreModerationResponse is a single submitted score.
type GameCenterScoreModerationResponse = SingleResponse[GameCenterScoreModerationAttributes]

// GCScoreModerationsQuery controls the score moderation collection endpoint.
type GCScoreModerationsQuery struct {
	ExistsBlocked *bool
	Fields        []string
	PlayerFields  []string
	Include       []string
	Limit         int
	NextURL       string
}

// GetGameCenterScoreModerations lists scores for a v2 leaderboard.
func (c *Client) GetGameCenterScoreModerations(ctx context.Context, leaderboardID string, query GCScoreModerationsQuery) (*GameCenterScoreModerationsResponse, error) {
	path := "/v2/gameCenterLeaderboards/" + url.PathEscape(strings.TrimSpace(leaderboardID)) + "/gameCenterScoreModerations"
	if query.NextURL != "" {
		if err := validateNextURL(query.NextURL); err != nil {
			return nil, fmt.Errorf("game-center score moderations: %w", err)
		}
		path = query.NextURL
	} else {
		values := url.Values{}
		if query.ExistsBlocked != nil {
			values.Set("exists[blocked]", strconv.FormatBool(*query.ExistsBlocked))
		}
		if len(query.Fields) > 0 {
			values.Set("fields[gameCenterScoreModerations]", strings.Join(query.Fields, ","))
		}
		if len(query.PlayerFields) > 0 {
			values.Set("fields[gameCenterDetailPlayers]", strings.Join(query.PlayerFields, ","))
		}
		if len(query.Include) > 0 {
			values.Set("include", strings.Join(query.Include, ","))
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
	var response GameCenterScoreModerationsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse score moderations: %w", err)
	}
	return &response, nil
}

// UpdateGameCenterScoreModeration blocks or unblocks a submitted score.
func (c *Client) UpdateGameCenterScoreModeration(ctx context.Context, id string, blocked bool) (*GameCenterScoreModerationResponse, error) {
	id = strings.TrimSpace(id)
	payload := struct {
		Data Resource[gameCenterScoreModerationUpdateAttributes] `json:"data"`
	}{
		Data: Resource[gameCenterScoreModerationUpdateAttributes]{Type: ResourceTypeGameCenterScoreModerations, ID: id, Attributes: gameCenterScoreModerationUpdateAttributes{Blocked: blocked}},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	data, err := c.do(ctx, http.MethodPatch, "/v1/gameCenterScoreModerations/"+url.PathEscape(id), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	var response GameCenterScoreModerationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse score moderation: %w", err)
	}
	return &response, nil
}

type gameCenterScoreModerationUpdateAttributes struct {
	Blocked bool `json:"blocked"`
}

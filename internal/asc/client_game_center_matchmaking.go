package asc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// CreateGameCenterMatchmakingRuleSetTest creates a rule set test from a JSON payload.
func (c *Client) CreateGameCenterMatchmakingRuleSetTest(ctx context.Context, payload json.RawMessage) (*GameCenterMatchmakingRuleSetTestResponse, error) {
	if len(bytes.TrimSpace(payload)) == 0 {
		return nil, fmt.Errorf("empty rule set test payload")
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/gameCenterMatchmakingRuleSetTests", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	var response GameCenterMatchmakingRuleSetTestResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

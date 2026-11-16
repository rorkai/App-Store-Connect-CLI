package asc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// CreateGameCenterMatchmakingQueue creates a new matchmaking queue.
func (c *Client) CreateGameCenterMatchmakingQueue(ctx context.Context, ruleSetID string, attrs GameCenterMatchmakingQueueCreateAttributes, experimentRuleSetID string) (*GameCenterMatchmakingQueueResponse, error) {
	relationships := &GameCenterMatchmakingQueueRelationships{
		RuleSet: &Relationship{
			Data: ResourceData{
				Type: ResourceTypeGameCenterMatchmakingRuleSets,
				ID:   strings.TrimSpace(ruleSetID),
			},
		},
	}
	if id := strings.TrimSpace(experimentRuleSetID); id != "" {
		relationships.ExperimentRuleSet = &Relationship{
			Data: ResourceData{
				Type: ResourceTypeGameCenterMatchmakingRuleSets,
				ID:   id,
			},
		}
	}
	payload := GameCenterMatchmakingQueueCreateRequest{
		Data: GameCenterMatchmakingQueueCreateData{
			Type:          ResourceTypeGameCenterMatchmakingQueues,
			Attributes:    attrs,
			Relationships: relationships,
		},
	}
	return sendJSON[GameCenterMatchmakingQueueResponse](ctx, c, writeRequest{
		Method:  http.MethodPost,
		Path:    "/v1/gameCenterMatchmakingQueues",
		Payload: payload,
	})
}

// UpdateGameCenterMatchmakingQueue updates a matchmaking queue.
func (c *Client) UpdateGameCenterMatchmakingQueue(ctx context.Context, queueID string, attrs GameCenterMatchmakingQueueUpdateAttributes, ruleSetID, experimentRuleSetID string) (*GameCenterMatchmakingQueueResponse, error) {
	var relationships *GameCenterMatchmakingQueueRelationships
	if id := strings.TrimSpace(ruleSetID); id != "" {
		if relationships == nil {
			relationships = &GameCenterMatchmakingQueueRelationships{}
		}
		relationships.RuleSet = &Relationship{
			Data: ResourceData{
				Type: ResourceTypeGameCenterMatchmakingRuleSets,
				ID:   id,
			},
		}
	}
	if id := strings.TrimSpace(experimentRuleSetID); id != "" {
		if relationships == nil {
			relationships = &GameCenterMatchmakingQueueRelationships{}
		}
		relationships.ExperimentRuleSet = &Relationship{
			Data: ResourceData{
				Type: ResourceTypeGameCenterMatchmakingRuleSets,
				ID:   id,
			},
		}
	}
	payload := GameCenterMatchmakingQueueUpdateRequest{
		Data: GameCenterMatchmakingQueueUpdateData{
			Type:          ResourceTypeGameCenterMatchmakingQueues,
			ID:            strings.TrimSpace(queueID),
			Attributes:    &attrs,
			Relationships: relationships,
		},
	}
	return sendJSON[GameCenterMatchmakingQueueResponse](ctx, c, writeRequest{
		Method:  http.MethodPatch,
		Path:    fmt.Sprintf("/v1/gameCenterMatchmakingQueues/%s", strings.TrimSpace(queueID)),
		Payload: payload,
	})
}

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

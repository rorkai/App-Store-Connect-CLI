package asc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// GetGameCenterMatchmakingQueues retrieves the list of matchmaking queues.
func (c *Client) GetGameCenterMatchmakingQueues(ctx context.Context, opts ...GCMatchmakingQueuesOption) (*GameCenterMatchmakingQueuesResponse, error) {
	query := applyOptions(opts)
	return getJSON[GameCenterMatchmakingQueuesResponse](ctx, c, readRequest{
		Path:        "/v1/gameCenterMatchmakingQueues",
		Query:       buildGCMatchmakingQueuesQuery(query),
		Next:        query.nextURL,
		NextContext: "game-center-matchmaking-queues",
	})
}

// GetGameCenterMatchmakingQueue retrieves a matchmaking queue by ID.
func (c *Client) GetGameCenterMatchmakingQueue(ctx context.Context, queueID string) (*GameCenterMatchmakingQueueResponse, error) {
	return getJSON[GameCenterMatchmakingQueueResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/gameCenterMatchmakingQueues/%s", strings.TrimSpace(queueID))})
}

// CreateGameCenterMatchmakingQueue creates a new matchmaking queue.
func (c *Client) CreateGameCenterMatchmakingQueue(ctx context.Context, attrs GameCenterMatchmakingQueueCreateAttributes, ruleSetID string, experimentRuleSetID string) (*GameCenterMatchmakingQueueResponse, error) {
	relationships := &GameCenterMatchmakingQueueRelationships{
		RuleSet: &Relationship{
			Data: ResourceData{
				Type: ResourceTypeGameCenterMatchmakingRuleSets,
				ID:   strings.TrimSpace(ruleSetID),
			},
		},
	}
	if strings.TrimSpace(experimentRuleSetID) != "" {
		relationships.ExperimentRuleSet = &Relationship{
			Data: ResourceData{
				Type: ResourceTypeGameCenterMatchmakingRuleSets,
				ID:   strings.TrimSpace(experimentRuleSetID),
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
	return sendJSON[GameCenterMatchmakingQueueResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v1/gameCenterMatchmakingQueues", Payload: payload})
}

// UpdateGameCenterMatchmakingQueue updates a matchmaking queue.
func (c *Client) UpdateGameCenterMatchmakingQueue(ctx context.Context, queueID string, attrs GameCenterMatchmakingQueueUpdateAttributes, ruleSetID string, experimentRuleSetID string) (*GameCenterMatchmakingQueueResponse, error) {
	var relationships *GameCenterMatchmakingQueueRelationships
	if strings.TrimSpace(ruleSetID) != "" || strings.TrimSpace(experimentRuleSetID) != "" {
		relationships = &GameCenterMatchmakingQueueRelationships{}
		if strings.TrimSpace(ruleSetID) != "" {
			relationships.RuleSet = &Relationship{
				Data: ResourceData{
					Type: ResourceTypeGameCenterMatchmakingRuleSets,
					ID:   strings.TrimSpace(ruleSetID),
				},
			}
		}
		if strings.TrimSpace(experimentRuleSetID) != "" {
			relationships.ExperimentRuleSet = &Relationship{
				Data: ResourceData{
					Type: ResourceTypeGameCenterMatchmakingRuleSets,
					ID:   strings.TrimSpace(experimentRuleSetID),
				},
			}
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
	return sendJSON[GameCenterMatchmakingQueueResponse](ctx, c, writeRequest{Method: http.MethodPatch, Path: fmt.Sprintf("/v1/gameCenterMatchmakingQueues/%s", strings.TrimSpace(queueID)), Payload: payload})
}

// DeleteGameCenterMatchmakingQueue deletes a matchmaking queue.
func (c *Client) DeleteGameCenterMatchmakingQueue(ctx context.Context, queueID string) error {
	return sendNoContent(ctx, c, http.MethodDelete, fmt.Sprintf("/v1/gameCenterMatchmakingQueues/%s", strings.TrimSpace(queueID)), nil)
}

// CreateGameCenterMatchmakingRule creates a new rule for a rule set.
func (c *Client) CreateGameCenterMatchmakingRule(ctx context.Context, ruleSetID string, attrs GameCenterMatchmakingRuleCreateAttributes) (*GameCenterMatchmakingRuleResponse, error) {
	payload := GameCenterMatchmakingRuleCreateRequest{
		Data: GameCenterMatchmakingRuleCreateData{
			Type:       ResourceTypeGameCenterMatchmakingRules,
			Attributes: attrs,
			Relationships: &GameCenterMatchmakingRuleRelationships{
				RuleSet: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeGameCenterMatchmakingRuleSets,
						ID:   strings.TrimSpace(ruleSetID),
					},
				},
			},
		},
	}
	return sendJSON[GameCenterMatchmakingRuleResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v1/gameCenterMatchmakingRules", Payload: payload})
}

// UpdateGameCenterMatchmakingRule updates a matchmaking rule.
func (c *Client) UpdateGameCenterMatchmakingRule(ctx context.Context, ruleID string, attrs GameCenterMatchmakingRuleUpdateAttributes) (*GameCenterMatchmakingRuleResponse, error) {
	payload := GameCenterMatchmakingRuleUpdateRequest{
		Data: GameCenterMatchmakingRuleUpdateData{
			Type:       ResourceTypeGameCenterMatchmakingRules,
			ID:         strings.TrimSpace(ruleID),
			Attributes: &attrs,
		},
	}
	return sendJSON[GameCenterMatchmakingRuleResponse](ctx, c, writeRequest{Method: http.MethodPatch, Path: fmt.Sprintf("/v1/gameCenterMatchmakingRules/%s", strings.TrimSpace(ruleID)), Payload: payload})
}

// DeleteGameCenterMatchmakingRule deletes a matchmaking rule.
func (c *Client) DeleteGameCenterMatchmakingRule(ctx context.Context, ruleID string) error {
	return sendNoContent(ctx, c, http.MethodDelete, fmt.Sprintf("/v1/gameCenterMatchmakingRules/%s", strings.TrimSpace(ruleID)), nil)
}

// CreateGameCenterMatchmakingTeam creates a new team for a rule set.
func (c *Client) CreateGameCenterMatchmakingTeam(ctx context.Context, ruleSetID string, attrs GameCenterMatchmakingTeamCreateAttributes) (*GameCenterMatchmakingTeamResponse, error) {
	payload := GameCenterMatchmakingTeamCreateRequest{
		Data: GameCenterMatchmakingTeamCreateData{
			Type:       ResourceTypeGameCenterMatchmakingTeams,
			Attributes: attrs,
			Relationships: &GameCenterMatchmakingTeamRelationships{
				RuleSet: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeGameCenterMatchmakingRuleSets,
						ID:   strings.TrimSpace(ruleSetID),
					},
				},
			},
		},
	}
	return sendJSON[GameCenterMatchmakingTeamResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v1/gameCenterMatchmakingTeams", Payload: payload})
}

// UpdateGameCenterMatchmakingTeam updates a matchmaking team.
func (c *Client) UpdateGameCenterMatchmakingTeam(ctx context.Context, teamID string, attrs GameCenterMatchmakingTeamUpdateAttributes) (*GameCenterMatchmakingTeamResponse, error) {
	payload := GameCenterMatchmakingTeamUpdateRequest{
		Data: GameCenterMatchmakingTeamUpdateData{
			Type:       ResourceTypeGameCenterMatchmakingTeams,
			ID:         strings.TrimSpace(teamID),
			Attributes: &attrs,
		},
	}
	return sendJSON[GameCenterMatchmakingTeamResponse](ctx, c, writeRequest{Method: http.MethodPatch, Path: fmt.Sprintf("/v1/gameCenterMatchmakingTeams/%s", strings.TrimSpace(teamID)), Payload: payload})
}

// DeleteGameCenterMatchmakingTeam deletes a matchmaking team.
func (c *Client) DeleteGameCenterMatchmakingTeam(ctx context.Context, teamID string) error {
	return sendNoContent(ctx, c, http.MethodDelete, fmt.Sprintf("/v1/gameCenterMatchmakingTeams/%s", strings.TrimSpace(teamID)), nil)
}

// GetGameCenterMatchmakingQueueSizes retrieves queue sizes metrics.
func (c *Client) GetGameCenterMatchmakingQueueSizes(ctx context.Context, queueID string, opts ...GCMatchmakingMetricsOption) (*GameCenterMatchmakingQueueSizesResponse, error) {
	query := applyOptions(opts)
	return getJSON[GameCenterMatchmakingQueueSizesResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/gameCenterMatchmakingQueues/%s/metrics/matchmakingQueueSizes", strings.TrimSpace(queueID)),
		Query:       buildGCMatchmakingQueueSizesQuery(query),
		Next:        query.nextURL,
		NextContext: "game-center-matchmaking-queue-sizes",
	})
}

// GetGameCenterMatchmakingQueueRequests retrieves queue requests metrics.
func (c *Client) GetGameCenterMatchmakingQueueRequests(ctx context.Context, queueID string, opts ...GCMatchmakingMetricsOption) (*GameCenterMatchmakingQueueRequestsResponse, error) {
	query := applyOptions(opts)
	return getJSON[GameCenterMatchmakingQueueRequestsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/gameCenterMatchmakingQueues/%s/metrics/matchmakingRequests", strings.TrimSpace(queueID)),
		Query:       buildGCMatchmakingQueueRequestsQuery(query),
		Next:        query.nextURL,
		NextContext: "game-center-matchmaking-queue-requests",
	})
}

// GetGameCenterMatchmakingQueueSessions retrieves matchmaking sessions metrics.
func (c *Client) GetGameCenterMatchmakingQueueSessions(ctx context.Context, queueID string, opts ...GCMatchmakingMetricsOption) (*GameCenterMatchmakingQueueSessionsResponse, error) {
	query := applyOptions(opts)
	return getJSON[GameCenterMatchmakingQueueSessionsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/gameCenterMatchmakingQueues/%s/metrics/matchmakingSessions", strings.TrimSpace(queueID)),
		Query:       buildGCMatchmakingQueueSessionsQuery(query),
		Next:        query.nextURL,
		NextContext: "game-center-matchmaking-queue-sessions",
	})
}

// GetGameCenterMatchmakingQueueExperimentSizes retrieves experiment queue sizes metrics.
func (c *Client) GetGameCenterMatchmakingQueueExperimentSizes(ctx context.Context, queueID string, opts ...GCMatchmakingMetricsOption) (*GameCenterMatchmakingQueueExperimentSizesResponse, error) {
	query := applyOptions(opts)
	return getJSON[GameCenterMatchmakingQueueExperimentSizesResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/gameCenterMatchmakingQueues/%s/metrics/experimentMatchmakingQueueSizes", strings.TrimSpace(queueID)),
		Query:       buildGCMatchmakingQueueSizesQuery(query),
		Next:        query.nextURL,
		NextContext: "game-center-matchmaking-experiment-queue-sizes",
	})
}

// GetGameCenterMatchmakingQueueExperimentRequests retrieves experiment queue requests metrics.
func (c *Client) GetGameCenterMatchmakingQueueExperimentRequests(ctx context.Context, queueID string, opts ...GCMatchmakingMetricsOption) (*GameCenterMatchmakingQueueExperimentRequestsResponse, error) {
	query := applyOptions(opts)
	return getJSON[GameCenterMatchmakingQueueExperimentRequestsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/gameCenterMatchmakingQueues/%s/metrics/experimentMatchmakingRequests", strings.TrimSpace(queueID)),
		Query:       buildGCMatchmakingQueueRequestsQuery(query),
		Next:        query.nextURL,
		NextContext: "game-center-matchmaking-experiment-requests",
	})
}

// GetGameCenterMatchmakingBooleanRuleResults retrieves boolean rule results metrics.
func (c *Client) GetGameCenterMatchmakingBooleanRuleResults(ctx context.Context, ruleID string, opts ...GCMatchmakingMetricsOption) (*GameCenterMatchmakingBooleanRuleResultsResponse, error) {
	query := applyOptions(opts)
	return getJSON[GameCenterMatchmakingBooleanRuleResultsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/gameCenterMatchmakingRules/%s/metrics/matchmakingBooleanRuleResults", strings.TrimSpace(ruleID)),
		Query:       buildGCMatchmakingRuleMetricsQuery(query),
		Next:        query.nextURL,
		NextContext: "game-center-matchmaking-boolean-rule-results",
	})
}

// GetGameCenterMatchmakingNumberRuleResults retrieves number rule results metrics.
func (c *Client) GetGameCenterMatchmakingNumberRuleResults(ctx context.Context, ruleID string, opts ...GCMatchmakingMetricsOption) (*GameCenterMatchmakingNumberRuleResultsResponse, error) {
	query := applyOptions(opts)
	return getJSON[GameCenterMatchmakingNumberRuleResultsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/gameCenterMatchmakingRules/%s/metrics/matchmakingNumberRuleResults", strings.TrimSpace(ruleID)),
		Query:       buildGCMatchmakingRuleMetricsQuery(query),
		Next:        query.nextURL,
		NextContext: "game-center-matchmaking-number-rule-results",
	})
}

// GetGameCenterMatchmakingRuleErrors retrieves rule errors metrics.
func (c *Client) GetGameCenterMatchmakingRuleErrors(ctx context.Context, ruleID string, opts ...GCMatchmakingMetricsOption) (*GameCenterMatchmakingRuleErrorsResponse, error) {
	query := applyOptions(opts)
	return getJSON[GameCenterMatchmakingRuleErrorsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/gameCenterMatchmakingRules/%s/metrics/matchmakingRuleErrors", strings.TrimSpace(ruleID)),
		Query:       buildGCMatchmakingRuleMetricsQuery(query),
		Next:        query.nextURL,
		NextContext: "game-center-matchmaking-rule-errors",
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

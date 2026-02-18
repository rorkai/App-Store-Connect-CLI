package asc

import (
	"context"
)

// GetGameCenterMatchmakingRuleSetQueuesRelationships retrieves matchmaking queue linkages for a rule set.
func (c *Client) GetGameCenterMatchmakingRuleSetQueuesRelationships(ctx context.Context, ruleSetID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getGameCenterMatchmakingRuleSetLinkages(ctx, ruleSetID, "matchmakingQueues", opts...)
}

// GetGameCenterMatchmakingRuleSetRulesRelationships retrieves rule linkages for a rule set.
func (c *Client) GetGameCenterMatchmakingRuleSetRulesRelationships(ctx context.Context, ruleSetID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getGameCenterMatchmakingRuleSetLinkages(ctx, ruleSetID, "rules", opts...)
}

// GetGameCenterMatchmakingRuleSetTeamsRelationships retrieves team linkages for a rule set.
func (c *Client) GetGameCenterMatchmakingRuleSetTeamsRelationships(ctx context.Context, ruleSetID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getGameCenterMatchmakingRuleSetLinkages(ctx, ruleSetID, "teams", opts...)
}

func (c *Client) getGameCenterMatchmakingRuleSetLinkages(ctx context.Context, ruleSetID, relationship string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getResourceLinkages(
		ctx,
		ruleSetID,
		relationship,
		"ruleSetID",
		"/v1/gameCenterMatchmakingRuleSets/%s/relationships/%s",
		"gameCenterMatchmakingRuleSetRelationships",
		opts...,
	)
}

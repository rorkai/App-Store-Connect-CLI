package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// GetBetaGroupBetaTestersRelationships retrieves beta tester linkages for a beta group.
func (c *Client) GetBetaGroupBetaTestersRelationships(ctx context.Context, groupID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getBetaGroupLinkages(ctx, groupID, "betaTesters", opts...)
}

// GetBetaGroupBuildsRelationships retrieves build linkages for a beta group.
func (c *Client) GetBetaGroupBuildsRelationships(ctx context.Context, groupID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getBetaGroupLinkages(ctx, groupID, "builds", opts...)
}

// GetBetaTesterAppsRelationships retrieves app linkages for a beta tester.
func (c *Client) GetBetaTesterAppsRelationships(ctx context.Context, testerID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getBetaTesterLinkages(ctx, testerID, "apps", opts...)
}

// GetBetaTesterBetaGroupsRelationships retrieves beta group linkages for a beta tester.
func (c *Client) GetBetaTesterBetaGroupsRelationships(ctx context.Context, testerID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getBetaTesterLinkages(ctx, testerID, "betaGroups", opts...)
}

// GetBetaTesterBuildsRelationships retrieves build linkages for a beta tester.
func (c *Client) GetBetaTesterBuildsRelationships(ctx context.Context, testerID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getBetaTesterLinkages(ctx, testerID, "builds", opts...)
}

func (c *Client) getBetaGroupLinkages(ctx context.Context, groupID, relationship string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	query := &linkagesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	groupID = strings.TrimSpace(groupID)
	if query.nextURL == "" && groupID == "" {
		return nil, fmt.Errorf("groupID is required")
	}

	path := fmt.Sprintf("/v1/betaGroups/%s/relationships/%s", groupID, relationship)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("betaGroupRelationships: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildLinkagesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response LinkagesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

func (c *Client) getBetaTesterLinkages(ctx context.Context, testerID, relationship string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	query := &linkagesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	testerID = strings.TrimSpace(testerID)
	if query.nextURL == "" && testerID == "" {
		return nil, fmt.Errorf("testerID is required")
	}

	path := fmt.Sprintf("/v1/betaTesters/%s/relationships/%s", testerID, relationship)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("betaTesterRelationships: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildLinkagesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response LinkagesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

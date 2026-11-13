package asc

import (
	"context"
	"fmt"
	"net/http"
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

// AddBuildsToBetaGroup adds builds to a beta group.
func (c *Client) AddBuildsToBetaGroup(ctx context.Context, groupID string, buildIDs []string) error {
	groupID = strings.TrimSpace(groupID)
	buildIDs = normalizeList(buildIDs)
	if groupID == "" {
		return fmt.Errorf("groupID is required")
	}
	if len(buildIDs) == 0 {
		return fmt.Errorf("buildIDs are required")
	}

	payload := RelationshipRequest{
		Data: make([]RelationshipData, len(buildIDs)),
	}
	for i, id := range buildIDs {
		payload.Data[i] = RelationshipData{
			Type: ResourceTypeBuilds,
			ID:   id,
		}
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/v1/betaGroups/%s/relationships/builds", groupID)
	_, err = c.do(ctx, http.MethodPost, path, body)
	return err
}

// RemoveBuildsFromBetaGroup removes builds from a beta group.
func (c *Client) RemoveBuildsFromBetaGroup(ctx context.Context, groupID string, buildIDs []string) error {
	groupID = strings.TrimSpace(groupID)
	buildIDs = normalizeList(buildIDs)
	if groupID == "" {
		return fmt.Errorf("groupID is required")
	}
	if len(buildIDs) == 0 {
		return fmt.Errorf("buildIDs are required")
	}

	payload := RelationshipRequest{
		Data: make([]RelationshipData, len(buildIDs)),
	}
	for i, id := range buildIDs {
		payload.Data[i] = RelationshipData{
			Type: ResourceTypeBuilds,
			ID:   id,
		}
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/v1/betaGroups/%s/relationships/builds", groupID)
	_, err = c.do(ctx, http.MethodDelete, path, body)
	return err
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
	return c.getResourceLinkages(
		ctx,
		groupID,
		relationship,
		"groupID",
		"/v1/betaGroups/%s/relationships/%s",
		"betaGroupRelationships",
		opts...,
	)
}

func (c *Client) getBetaTesterLinkages(ctx context.Context, testerID, relationship string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getResourceLinkages(
		ctx,
		testerID,
		relationship,
		"testerID",
		"/v1/betaTesters/%s/relationships/%s",
		"betaTesterRelationships",
		opts...,
	)
}

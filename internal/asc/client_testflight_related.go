package asc

import (
	"context"
	"fmt"
	"strings"
)

// GetBetaGroupApp retrieves the app for a beta group.
func (c *Client) GetBetaGroupApp(ctx context.Context, groupID string) (*AppResponse, error) {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return nil, fmt.Errorf("groupID is required")
	}
	return getJSON[AppResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/betaGroups/%s/app", groupID)})
}

// GetBetaGroupBetaRecruitmentCriteria retrieves recruitment criteria for a beta group.
func (c *Client) GetBetaGroupBetaRecruitmentCriteria(ctx context.Context, groupID string) (*BetaRecruitmentCriteriaResponse, error) {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return nil, fmt.Errorf("groupID is required")
	}
	return getJSON[BetaRecruitmentCriteriaResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/betaGroups/%s/betaRecruitmentCriteria", groupID)})
}

// GetBetaGroupBetaRecruitmentCriterionCompatibleBuildCheck retrieves compatible build check for a beta group.
func (c *Client) GetBetaGroupBetaRecruitmentCriterionCompatibleBuildCheck(ctx context.Context, groupID string) (*BetaRecruitmentCriterionCompatibleBuildCheckResponse, error) {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return nil, fmt.Errorf("groupID is required")
	}
	return getJSON[BetaRecruitmentCriterionCompatibleBuildCheckResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/betaGroups/%s/betaRecruitmentCriterionCompatibleBuildCheck", groupID)})
}

// GetBetaTesterApps retrieves apps for a beta tester.
func (c *Client) GetBetaTesterApps(ctx context.Context, testerID string, opts ...BetaTesterAppsOption) (*AppsResponse, error) {
	query := applyOptions(opts)
	testerID = strings.TrimSpace(testerID)
	if query.nextURL == "" && testerID == "" {
		return nil, fmt.Errorf("testerID is required")
	}
	return getJSON[AppsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/betaTesters/%s/apps", testerID),
		Query:       buildListQuery(query),
		Next:        query.nextURL,
		NextContext: "betaTesterApps",
	})
}

// GetBetaTesterBetaGroups retrieves beta groups for a beta tester.
func (c *Client) GetBetaTesterBetaGroups(ctx context.Context, testerID string, opts ...BetaTesterBetaGroupsOption) (*BetaGroupsResponse, error) {
	query := applyOptions(opts)
	testerID = strings.TrimSpace(testerID)
	if query.nextURL == "" && testerID == "" {
		return nil, fmt.Errorf("testerID is required")
	}
	return getJSON[BetaGroupsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/betaTesters/%s/betaGroups", testerID),
		Query:       buildListQuery(query),
		Next:        query.nextURL,
		NextContext: "betaTesterBetaGroups",
	})
}

// GetBetaTesterBuilds retrieves builds for a beta tester.
func (c *Client) GetBetaTesterBuilds(ctx context.Context, testerID string, opts ...BetaTesterBuildsOption) (*BuildsResponse, error) {
	query := applyOptions(opts)
	testerID = strings.TrimSpace(testerID)
	if query.nextURL == "" && testerID == "" {
		return nil, fmt.Errorf("testerID is required")
	}
	return getJSON[BuildsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/betaTesters/%s/builds", testerID),
		Query:       buildListQuery(query),
		Next:        query.nextURL,
		NextContext: "betaTesterBuilds",
	})
}

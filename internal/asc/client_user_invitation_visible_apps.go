package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// GetUserInvitationVisibleApps retrieves visible apps for a user invitation.
func (c *Client) GetUserInvitationVisibleApps(ctx context.Context, invitationID string, opts ...UserInvitationVisibleAppsOption) (*AppsResponse, error) {
	query := &userInvitationVisibleAppsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/userInvitations/%s/visibleApps", strings.TrimSpace(invitationID))
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("userInvitationVisibleApps: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildUserInvitationVisibleAppsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}
